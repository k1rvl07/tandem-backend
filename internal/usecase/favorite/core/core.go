package core

import (
	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	sharedbase "github.com/tandem/tandem/internal/usecase/shared/base"
)

const EventFavoritesUpdated = "favorites.updated"

type Core struct {
	sharedbase.Base
	Favorites repository.FavoriteRepository
	Boards    repository.BoardRepository
	Hub       ws.Hub
}

func New(favorites repository.FavoriteRepository, workspaces repository.WorkspaceRepository, boards repository.BoardRepository, hub ws.Hub, cache cache.Cache) *Core {
	return &Core{Base: sharedbase.Base{Workspaces: workspaces, Cache: cache}, Favorites: favorites, Boards: boards, Hub: hub}
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
