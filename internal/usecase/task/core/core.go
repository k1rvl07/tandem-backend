package core

import (
	"context"
	"errors"

	mboard "github.com/tandem/tandem/internal/domain/models/board"
	mcolumn "github.com/tandem/tandem/internal/domain/models/column"
	mtask "github.com/tandem/tandem/internal/domain/models/task"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	dtask "github.com/tandem/tandem/internal/http/dto/task"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	file "github.com/tandem/tandem/internal/usecase/file"
	"github.com/tandem/tandem/internal/usecase/shared/access"
	cacheutil "github.com/tandem/tandem/internal/usecase/shared/cache"
	"github.com/tandem/tandem/internal/usecase/shared/taskmap"
)

const (
	EventTaskCreated = "task.created"
	EventTaskUpdated = "task.updated"
	EventTaskDeleted = "task.deleted"
)

type Core struct {
	Tasks      repository.TaskRepository
	Columns    repository.ColumnRepository
	Boards     repository.BoardRepository
	Workspaces repository.WorkspaceRepository
	Users      repository.UserRepository
	Files      *file.Service
	Hub        ws.Hub
	Cache      cache.Cache
}

func New(tasks repository.TaskRepository, columns repository.ColumnRepository, boards repository.BoardRepository, workspaces repository.WorkspaceRepository, users repository.UserRepository, files *file.Service, hub ws.Hub, cache cache.Cache) *Core {
	return &Core{Tasks: tasks, Columns: columns, Boards: boards, Workspaces: workspaces, Users: users, Files: files, Hub: hub, Cache: cache}
}

type WorkspaceColumn struct {
	Name      string
	BoardID   string
	BoardName string
}

func (c *Core) BumpWorkspace(ctx context.Context, workspaceID string) {
	cacheutil.BumpWorkspace(ctx, c.Cache, workspaceID)
}

func (c *Core) MemberOf(ctx context.Context, actorID, workspaceID string) (*mworkspace.WorkspaceMember, error) {
	return access.MemberOrForbidden(ctx, c.Workspaces, workspaceID, actorID)
}

func (c *Core) BoardInWorkspace(ctx context.Context, workspaceID, boardID string) (*mboard.Board, error) {
	return access.BoardInWorkspace(ctx, c.Boards, workspaceID, boardID)
}

func (c *Core) ColumnInBoard(ctx context.Context, boardID, columnID string) (*mcolumn.Column, error) {
	column, err := c.Columns.FindColumnByID(ctx, columnID)
	if err != nil {
		return nil, err
	}
	if column.BoardID != boardID {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, errors.New("column not in board"))
	}
	return column, nil
}

func (c *Core) TaskInBoard(ctx context.Context, boardID, taskID string) (*mtask.Task, error) {
	task, err := c.Tasks.FindTaskByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	column, err := c.Columns.FindColumnByID(ctx, task.ColumnID)
	if err != nil {
		return nil, err
	}
	if column.BoardID != boardID {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, errors.New("task not in board"))
	}
	return task, nil
}

func (c *Core) WorkspaceTask(ctx context.Context, workspaceID, taskID string) (*mtask.Task, error) {
	return access.TaskInWorkspace(ctx, c.Tasks, c.Columns, c.Boards, workspaceID, taskID)
}

func (c *Core) Room(workspaceID string) string {
	return "workspace:" + workspaceID
}

func (c *Core) ResolveUsers(ctx context.Context, tasks []*mtask.Task) (map[string]*dtask.TaskUserResponse, error) {
	return taskmap.ResolveUsers(ctx, c.Users, tasks)
}

func (c *Core) ResponseFor(ctx context.Context, workspaceID string, task *mtask.Task) (*dtask.TaskResponse, error) {
	responses, err := c.ResponsesFor(ctx, workspaceID, []*mtask.Task{task})
	if err != nil {
		return nil, err
	}
	return &responses[0], nil
}

func (c *Core) ResponsesFor(ctx context.Context, workspaceID string, tasks []*mtask.Task) ([]dtask.TaskResponse, error) {
	if len(tasks) == 0 {
		return []dtask.TaskResponse{}, nil
	}
	workspace, err := c.Workspaces.FindWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	columns, err := c.Columns.ListColumnsForWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	boards, err := c.Boards.ListBoards(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	boardNameByID := make(map[string]string, len(boards))
	for i := range boards {
		boardNameByID[boards[i].ID] = boards[i].Name
	}
	columnInfo := make(map[string]WorkspaceColumn, len(columns))
	for i := range columns {
		columnInfo[columns[i].ID] = WorkspaceColumn{Name: columns[i].Name, BoardID: columns[i].BoardID, BoardName: boardNameByID[columns[i].BoardID]}
	}
	users, err := c.ResolveUsers(ctx, tasks)
	if err != nil {
		return nil, err
	}
	responses := make([]dtask.TaskResponse, 0, len(tasks))
	for _, task := range tasks {
		info, ok := columnInfo[task.ColumnID]
		if !ok {
			info = WorkspaceColumn{BoardID: ""}
		}
		responses = append(responses, taskmap.BuildTaskResponse(task, users, taskmap.TaskResponseCtx{
			WorkspaceID: workspaceID,
			BoardID:     info.BoardID,
			BoardName:   info.BoardName,
			ColumnName:  info.Name,
			DisplayID:   taskmap.DisplayID(workspace.Prefix, task.ID),
		}))
	}
	return responses, nil
}
