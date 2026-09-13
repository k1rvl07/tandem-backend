package core

import (
	"context"

	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	cacheutil "github.com/tandem/tandem/internal/usecase/shared/cache"
)

const EventFavoritesUpdated = "favorites.updated"

type Core struct {
	Favorites  repository.FavoriteRepository
	Workspaces repository.WorkspaceRepository
	Boards     repository.BoardRepository
	Hub        ws.Hub
	Cache      cache.Cache
}

func New(favorites repository.FavoriteRepository, workspaces repository.WorkspaceRepository, boards repository.BoardRepository, hub ws.Hub, cache cache.Cache) *Core {
	return &Core{Favorites: favorites, Workspaces: workspaces, Boards: boards, Hub: hub, Cache: cache}
}

func (c *Core) BumpUser(ctx context.Context, actorID string) {
	cacheutil.Bump(ctx, c.Cache, cacheutil.UVerKey+actorID)
}

func (c *Core) BroadcastUpdated(actorID, targetType, targetID string) {
	if c.Hub == nil {
		return
	}
	c.Hub.SendToUser(actorID, &ws.Message{
		Type: EventFavoritesUpdated,
		Data: map[string]string{"target_type": targetType, "target_id": targetID},
	})
}
