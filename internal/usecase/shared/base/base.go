package base

import (
	"context"

	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/usecase/shared/access"
	cacheutil "github.com/tandem/tandem/internal/usecase/shared/cache"
)

const RoomPrefix = "workspace:"

type Base struct {
	Workspaces repository.WorkspaceRepository
	Cache      cache.Cache
}

func (b *Base) MemberOf(ctx context.Context, actorID, workspaceID string) (*mworkspace.WorkspaceMember, error) {
	return access.MemberOrForbidden(ctx, b.Workspaces, workspaceID, actorID)
}

func (b *Base) RequireOwner(role mworkspace.WorkspaceRole) error {
	return access.RequireOwner(role)
}

func (b *Base) RequireEditor(role mworkspace.WorkspaceRole) error {
	return access.RequireEditor(role)
}

func (b *Base) BumpWorkspace(ctx context.Context, workspaceID string) {
	cacheutil.BumpWorkspace(ctx, b.Cache, workspaceID)
}

func (b *Base) BumpUser(ctx context.Context, userID string) {
	cacheutil.BumpUser(ctx, b.Cache, userID)
}

func (b *Base) BumpUsers(ctx context.Context) {
	cacheutil.BumpUsers(ctx, b.Cache)
}

func (b *Base) Room(workspaceID string) string {
	return RoomPrefix + workspaceID
}
