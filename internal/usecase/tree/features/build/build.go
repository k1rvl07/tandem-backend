package build

import (
	"context"
	"sort"

	dtask "github.com/tandem/tandem/internal/contracts/task"
	dtree "github.com/tandem/tandem/internal/contracts/tree"
	dworkspace "github.com/tandem/tandem/internal/contracts/workspace"
	mboard "github.com/tandem/tandem/internal/domain/models/board"
	mtask "github.com/tandem/tandem/internal/domain/models/task"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	"github.com/tandem/tandem/internal/usecase/board/mapping"
	sharedmap "github.com/tandem/tandem/internal/usecase/shared/mapping"
	"github.com/tandem/tandem/internal/usecase/tree/core"
)

type Build struct {
	*core.Core
}

func New(c *core.Core) *Build {
	return &Build{Core: c}
}

func (s *Build) BuildBrick(ctx context.Context, actorID string, workspace *mworkspace.Workspace, role mworkspace.WorkspaceRole, tasksFilter string) (core.TreeBrick, error) {
	boards, err := s.Boards.ListBoards(ctx, workspace.ID)
	if err != nil {
		return core.TreeBrick{}, err
	}
	tasks, err := s.Tasks.ListTasksForWorkspace(ctx, workspace.ID)
	if err != nil {
		return core.TreeBrick{}, err
	}
	columns, err := s.Columns.ListColumnsForWorkspace(ctx, workspace.ID)
	if err != nil {
		return core.TreeBrick{}, err
	}
	columnBoard := make(map[string]string, len(columns))
	columnOrder := make(map[string]int, len(columns))
	for j := range columns {
		columnBoard[columns[j].ID] = columns[j].BoardID
		columnOrder[columns[j].ID] = columns[j].Position
	}
	treeBoards := make([]dtree.TreeBoardResponse, 0, len(boards))
	for _, board := range boards {
		matching := s.matchingTasks(tasks, board, columnBoard, tasksFilter, actorID)
		if len(matching) == 0 {
			continue
		}
		sortTasksByColumnOrder(matching, columnOrder)
		responses, err := s.taskResponses(ctx, workspace, board, matching)
		if err != nil {
			return core.TreeBrick{}, err
		}
		treeBoards = append(treeBoards, dtree.TreeBoardResponse{
			Board: *mapping.BoardToResponse(board),
			Tasks: responses,
		})
	}
	return core.TreeBrick{
		Workspace: dworkspace.WorkspaceResponse{
			ID:         workspace.ID,
			Name:       workspace.Name,
			Prefix:     workspace.Prefix,
			Theme:      workspace.Theme,
			Role:       string(role),
			IsFavorite: false,
			CreatedAt:  workspace.CreatedAt,
			UpdatedAt:  workspace.UpdatedAt,
		},
		Boards: treeBoards,
	}, nil
}

func (s *Build) matchingTasks(tasks []*mtask.Task, board *mboard.Board, columnBoard map[string]string, tasksFilter string, actorID string) []*mtask.Task {
	matching := make([]*mtask.Task, 0, 4)
	for _, task := range tasks {
		if task.IsHidden {
			continue
		}
		if columnBoard[task.ColumnID] != board.ID {
			continue
		}
		if !s.taskMatches(tasksFilter, task, actorID) {
			continue
		}
		matching = append(matching, task)
	}
	return matching
}

func sortTasksByColumnOrder(tasks []*mtask.Task, columnOrder map[string]int) {
	sort.SliceStable(tasks, func(i, j int) bool {
		if columnOrder[tasks[i].ColumnID] != columnOrder[tasks[j].ColumnID] {
			return columnOrder[tasks[i].ColumnID] < columnOrder[tasks[j].ColumnID]
		}
		if tasks[i].Position != tasks[j].Position {
			return tasks[i].Position < tasks[j].Position
		}
		return tasks[i].CreatedAt.Before(tasks[j].CreatedAt)
	})
}

func (s *Build) taskMatches(filter string, task *mtask.Task, actorID string) bool {
	switch filter {
	case "mine":
		return task.AuthorID == actorID || task.CuratorID == actorID || task.AssigneeID == actorID
	case "for_me":
		return task.AssigneeID == actorID
	default:
		return true
	}
}

func (s *Build) taskResponses(ctx context.Context, workspace *mworkspace.Workspace, board *mboard.Board, tasks []*mtask.Task) ([]dtask.TaskResponse, error) {
	columnNames := make(map[string]string)
	for _, task := range tasks {
		columnNames[task.ColumnID] = ""
	}
	columns, err := s.Columns.ListColumnsForWorkspace(ctx, workspace.ID)
	if err != nil {
		return nil, err
	}
	for i := range columns {
		columnNames[columns[i].ID] = columns[i].Name
	}
	users, err := sharedmap.ResolveUsers(ctx, s.Users, tasks)
	if err != nil {
		return nil, err
	}
	responses := make([]dtask.TaskResponse, 0, len(tasks))
	for _, task := range tasks {
		responses = append(responses, sharedmap.BuildTaskResponse(task, users, sharedmap.TaskResponseCtx{
			WorkspaceID: workspace.ID,
			BoardID:     board.ID,
			BoardName:   board.Name,
			ColumnName:  columnNames[task.ColumnID],
			DisplayID:   sharedmap.DisplayID(workspace.Prefix, task.ID),
		}))
	}
	return responses, nil
}
