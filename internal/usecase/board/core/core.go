package core

import (
	"context"

	mboard "github.com/tandem/tandem/internal/domain/models/board"
	mtask "github.com/tandem/tandem/internal/domain/models/task"
	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	dtask "github.com/tandem/tandem/internal/http/dto/task"
	file "github.com/tandem/tandem/internal/usecase/file"
	"github.com/tandem/tandem/internal/usecase/shared/access"
	sharedbase "github.com/tandem/tandem/internal/usecase/shared/base"
	"github.com/tandem/tandem/internal/usecase/shared/taskmap"
)

const (
	EventBoardCreated    = "board.created"
	EventBoardUpdated    = "board.updated"
	EventBoardDeleted    = "board.deleted"
	EventBoardsReordered = "boards.reordered"
)

type Core struct {
	sharedbase.Base
	Boards    repository.BoardRepository
	Columns   repository.ColumnRepository
	Tasks     repository.TaskRepository
	Users     repository.UserRepository
	Favorites repository.FavoriteRepository
	Files     *file.Service
	Hub       ws.Hub
}

func New(boards repository.BoardRepository, columns repository.ColumnRepository, tasks repository.TaskRepository, workspaces repository.WorkspaceRepository, users repository.UserRepository, favorites repository.FavoriteRepository, files *file.Service, hub ws.Hub, cache cache.Cache) *Core {
	return &Core{Base: sharedbase.Base{Workspaces: workspaces, Cache: cache}, Boards: boards, Columns: columns, Tasks: tasks, Users: users, Favorites: favorites, Files: files, Hub: hub}
}

func (c *Core) BoardInWorkspace(ctx context.Context, workspaceID, boardID string) (*mboard.Board, error) {
	return access.BoardInWorkspace(ctx, c.Boards, workspaceID, boardID)
}

func (c *Core) ResolveUsers(ctx context.Context, tasks []*mtask.Task) (map[string]*dtask.TaskUserResponse, error) {
	return taskmap.ResolveUsers(ctx, c.Users, tasks)
}
