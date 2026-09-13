package core

import (
	"context"

	muser "github.com/tandem/tandem/internal/domain/models/user"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	dworkspace "github.com/tandem/tandem/internal/http/dto/workspace"
	file "github.com/tandem/tandem/internal/usecase/file"
	"github.com/tandem/tandem/internal/usecase/shared/access"
	cacheutil "github.com/tandem/tandem/internal/usecase/shared/cache"
	"github.com/tandem/tandem/internal/usecase/shared/errutil"
	"go.uber.org/zap"
)

const (
	EventWorkspaceUpdated = "workspace.updated"
	EventMemberKicked     = "workspace.kicked"
)

type Core struct {
	Workspaces repository.WorkspaceRepository
	Users      repository.UserRepository
	Favorites  repository.FavoriteRepository
	Boards     repository.BoardRepository
	Columns    repository.ColumnRepository
	Tasks      repository.TaskRepository
	Files      *file.Service
	Hub        ws.Hub
	Cache      cache.Cache
	Logger     *zap.Logger
}

func New(workspaces repository.WorkspaceRepository, users repository.UserRepository, favorites repository.FavoriteRepository, boards repository.BoardRepository, columns repository.ColumnRepository, tasks repository.TaskRepository, files *file.Service, hub ws.Hub, cache cache.Cache, logger *zap.Logger) *Core {
	return &Core{Workspaces: workspaces, Users: users, Favorites: favorites, Boards: boards, Columns: columns, Tasks: tasks, Files: files, Hub: hub, Cache: cache, Logger: logger}
}

func (c *Core) MemberOf(ctx context.Context, actorID, workspaceID string) (*mworkspace.WorkspaceMember, error) {
	return access.MemberOrForbidden(ctx, c.Workspaces, workspaceID, actorID)
}

func (c *Core) RequireOwner(role mworkspace.WorkspaceRole) error {
	return access.RequireOwner(role)
}

func (c *Core) RequireEditor(role mworkspace.WorkspaceRole) error {
	return access.RequireEditor(role)
}

func (c *Core) BumpWorkspace(ctx context.Context, workspaceID string) {
	cacheutil.BumpWorkspace(ctx, c.Cache, workspaceID)
}

func (c *Core) BumpUser(ctx context.Context, userID string) {
	cacheutil.Bump(ctx, c.Cache, cacheutil.UVerKey+userID)
}

func (c *Core) BumpMembers(ctx context.Context, workspaceID string) {
	members, err := c.Workspaces.ListMembers(ctx, workspaceID)
	if err != nil {
		return
	}
	for i := range members {
		c.BumpUser(ctx, members[i].UserID)
	}
}

func (c *Core) Room(workspaceID string) string {
	return "workspace:" + workspaceID
}

func (c *Core) BroadcastUpdated(ctx context.Context, workspaceID string, role mworkspace.WorkspaceRole) {
	wsModel, err := c.Workspaces.FindWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return
	}
	c.Hub.BroadcastToRoom(c.Room(workspaceID), &ws.Message{Type: EventWorkspaceUpdated, Data: c.WorkspaceResponse(wsModel, role, false)})
}

func (c *Core) ResolveOwner(ctx context.Context, workspaceID string) (*dworkspace.WorkspaceMemberResponse, error) {
	members, err := c.Workspaces.ListMembers(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	user, idx, err := c.FindOwnerUser(ctx, members)
	if err != nil {
		return nil, err
	}
	if idx < 0 {
		return nil, nil
	}
	return c.MemberResponse(user, members[idx].Role), nil
}

func (c *Core) FindOwnerUser(ctx context.Context, members []mworkspace.WorkspaceMember) (*muser.User, int, error) {
	for i := range members {
		if members[i].Role == mworkspace.RoleOwner {
			user, err := c.Users.FindByID(ctx, members[i].UserID)
			if err != nil {
				if errutil.IsNotFound(err) {
					continue
				}
				return nil, -1, err
			}
			return user, i, nil
		}
	}
	return nil, -1, nil
}

func (c *Core) BuildMemberResponses(ctx context.Context, members []mworkspace.WorkspaceMember) ([]dworkspace.WorkspaceMemberResponse, error) {
	memberResponses := make([]dworkspace.WorkspaceMemberResponse, 0, len(members))
	for i := range members {
		user, err := c.Users.FindByID(ctx, members[i].UserID)
		if err != nil {
			if errutil.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		memberResponses = append(memberResponses, dworkspace.WorkspaceMemberResponse{
			ID:          user.ID,
			Login:       user.Login,
			DisplayName: user.DisplayName,
			AvatarKey:   user.AvatarKey,
			Role:        string(members[i].Role),
			JoinedAt:    members[i].CreatedAt,
		})
	}
	return memberResponses, nil
}

func (c *Core) WorkspaceResponse(ws *mworkspace.Workspace, role mworkspace.WorkspaceRole, isFavorite bool) *dworkspace.WorkspaceResponse {
	return &dworkspace.WorkspaceResponse{
		ID:          ws.ID,
		Name:        ws.Name,
		Description: ws.Description,
		Prefix:      ws.Prefix,
		Theme:       ws.Theme,
		Role:        string(role),
		IsFavorite:  isFavorite,
		CreatedAt:   ws.CreatedAt,
		UpdatedAt:   ws.UpdatedAt,
	}
}

func (c *Core) MemberResponse(user *muser.User, role mworkspace.WorkspaceRole) *dworkspace.WorkspaceMemberResponse {
	return &dworkspace.WorkspaceMemberResponse{
		ID:          user.ID,
		Login:       user.Login,
		DisplayName: user.DisplayName,
		AvatarKey:   user.AvatarKey,
		Role:        string(role),
	}
}
