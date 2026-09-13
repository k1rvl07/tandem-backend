package core

import (
	"context"

	mboard "github.com/tandem/tandem/internal/domain/models/board"
	mtask "github.com/tandem/tandem/internal/domain/models/task"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	dtask "github.com/tandem/tandem/internal/http/dto/task"
	file "github.com/tandem/tandem/internal/usecase/file"
	"github.com/tandem/tandem/internal/usecase/shared/access"
	cacheutil "github.com/tandem/tandem/internal/usecase/shared/cache"
	"github.com/tandem/tandem/internal/usecase/shared/taskmap"
)

const (
	EventBoardCreated    = "board.created"
	EventBoardUpdated    = "board.updated"
	EventBoardDeleted    = "board.deleted"
	EventBoardsReordered = "boards.reordered"
)

type Core struct {
	Boards     repository.BoardRepository
	Columns    repository.ColumnRepository
	Tasks      repository.TaskRepository
	Workspaces repository.WorkspaceRepository
	Users      repository.UserRepository
	Favorites  repository.FavoriteRepository
	Files      *file.Service
	Hub        ws.Hub
	Cache      cache.Cache
}

func New(boards repository.BoardRepository, columns repository.ColumnRepository, tasks repository.TaskRepository, workspaces repository.WorkspaceRepository, users repository.UserRepository, favorites repository.FavoriteRepository, files *file.Service, hub ws.Hub, cache cache.Cache) *Core {
	return &Core{Boards: boards, Columns: columns, Tasks: tasks, Workspaces: workspaces, Users: users, Favorites: favorites, Files: files, Hub: hub, Cache: cache}
}

func (c *Core) BumpWorkspace(ctx context.Context, workspaceID string) {
	cacheutil.BumpWorkspace(ctx, c.Cache, workspaceID)
}

func (c *Core) MemberOf(ctx context.Context, actorID, workspaceID string) (*mworkspace.WorkspaceMember, error) {
	return access.MemberOrForbidden(ctx, c.Workspaces, workspaceID, actorID)
}

func (c *Core) RequireEditor(role mworkspace.WorkspaceRole) error {
	return access.RequireEditor(role)
}

func (c *Core) RequireOwner(role mworkspace.WorkspaceRole) error {
	return access.RequireOwner(role)
}

func (c *Core) BoardInWorkspace(ctx context.Context, workspaceID, boardID string) (*mboard.Board, error) {
	return access.BoardInWorkspace(ctx, c.Boards, workspaceID, boardID)
}

func (c *Core) ResolveUsers(ctx context.Context, tasks []*mtask.Task) (map[string]*dtask.TaskUserResponse, error) {
	return taskmap.ResolveUsers(ctx, c.Users, tasks)
}

func (c *Core) Room(workspaceID string) string {
	return "workspace:" + workspaceID
}
