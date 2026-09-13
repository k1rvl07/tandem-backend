package favorite

import (
	"context"

	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	favcore "github.com/tandem/tandem/internal/usecase/favorite/core"
	"github.com/tandem/tandem/internal/usecase/favorite/crud"
)

type UseCase interface {
	Add(ctx context.Context, actorID, targetType, targetID string) error
	Remove(ctx context.Context, actorID, targetType, targetID string) error
}

type Deps struct {
	Favorites  repository.FavoriteRepository
	Workspaces repository.WorkspaceRepository
	Boards     repository.BoardRepository
	Hub        ws.Hub
	Cache      cache.Cache
}

type Service struct {
	*crud.Crud
}

func NewService(deps Deps) *Service {
	c := favcore.New(deps.Favorites, deps.Workspaces, deps.Boards, deps.Hub, deps.Cache)
	return &Service{Crud: crud.New(c)}
}

var _ UseCase = (*Service)(nil)
