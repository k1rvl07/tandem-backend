package build

import (
	"context"
	"sort"

	mboard "github.com/tandem/tandem/internal/domain/models/board"
	mtask "github.com/tandem/tandem/internal/domain/models/task"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	dboard "github.com/tandem/tandem/internal/http/dto/board"
	dtask "github.com/tandem/tandem/internal/http/dto/task"
	dtree "github.com/tandem/tandem/internal/http/dto/tree"
	dworkspace "github.com/tandem/tandem/internal/http/dto/workspace"
	"github.com/tandem/tandem/internal/usecase/shared/errutil"
	"github.com/tandem/tandem/internal/usecase/shared/taskmap"
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
			Board: dboard.BoardResponse{
				ID:          board.ID,
				WorkspaceID: board.WorkspaceID,
				Name:        board.Name,
				Position:    board.Position,
				IsMain:      board.IsMain,
				IsFavorite:  false,
				CreatedAt:   board.CreatedAt,
				UpdatedAt:   board.UpdatedAt,
			},
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
	ids := make(map[string]bool)
	for _, task := range tasks {
		columnNames[task.ColumnID] = ""
		for _, id := range []string{task.AuthorID, task.AssigneeID, task.CuratorID} {
			if id != "" {
				ids[id] = true
			}
		}
	}
	columns, err := s.Columns.ListColumnsForWorkspace(ctx, workspace.ID)
	if err != nil {
		return nil, err
	}
	for i := range columns {
		columnNames[columns[i].ID] = columns[i].Name
	}
	users := make(map[string]*dtask.TaskUserResponse)
	for id := range ids {
		user, err := s.Users.FindByID(ctx, id)
		if err != nil {
			if errutil.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		users[id] = &dtask.TaskUserResponse{
			ID:          user.ID,
			Login:       user.Login,
			DisplayName: user.DisplayName,
			AvatarKey:   user.AvatarKey,
		}
	}
	responses := make([]dtask.TaskResponse, 0, len(tasks))
	for _, task := range tasks {
		responses = append(responses, dtask.TaskResponse{
			ID:          task.ID,
			DisplayID:   taskmap.DisplayID(workspace.Prefix, task.ID),
			WorkspaceID: workspace.ID,
			BoardID:     board.ID,
			BoardName:   board.Name,
			ColumnID:    task.ColumnID,
			ColumnName:  columnNames[task.ColumnID],
			Title:       task.Title,
			Description: task.Description,
			Author:      users[task.AuthorID],
			Assignee:    users[task.AssigneeID],
			Curator:     users[task.CuratorID],
			ParentID:    task.ParentID,
			DueDate:     task.DueDate,
			Position:    task.Position,
			IsUrgent:    task.IsUrgent,
			IsHidden:    task.IsHidden,
			ImageKey:    task.ImageKey,
			CreatedAt:   task.CreatedAt,
			UpdatedAt:   task.UpdatedAt,
		})
	}
	return responses, nil
}
