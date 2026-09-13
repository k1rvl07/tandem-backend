package favorite

import (
	"context"
	"errors"

	"github.com/google/uuid"
	mfavorite "github.com/tandem/tandem/internal/domain/models/favorite"
	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
	"github.com/tandem/tandem/internal/usecase/cacheutil"
)

const eventFavoritesUpdated = "favorites.updated"

type UseCase interface {
	Add(ctx context.Context, actorID, targetType, targetID string) error
	Remove(ctx context.Context, actorID, targetType, targetID string) error
}

type Service struct {
	favorites  repository.FavoriteRepository
	workspaces repository.WorkspaceRepository
	boards     repository.BoardRepository
	hub        ws.Hub
	cache      cache.Cache
}

type Deps struct {
	Favorites  repository.FavoriteRepository
	Workspaces repository.WorkspaceRepository
	Boards     repository.BoardRepository
	Hub        ws.Hub
	Cache      cache.Cache
}

func NewService(deps Deps) *Service {
	return &Service{
		favorites:  deps.Favorites,
		workspaces: deps.Workspaces,
		boards:     deps.Boards,
		hub:        deps.Hub,
		cache:      deps.Cache,
	}
}

func (s *Service) bumpUser(ctx context.Context, actorID string) {
	cacheutil.Bump(ctx, s.cache, cacheutil.UVerKey+actorID)
}

func (s *Service) Add(ctx context.Context, actorID, targetType, targetID string) error {
	if err := validate.UUID(targetID); err != nil {
		return err
	}
	switch targetType {
	case mfavorite.FavoriteWorkspace:
		if _, err := s.workspaces.FindMember(ctx, targetID, actorID); err != nil {
			if errors.Is(err, pkgerrors.ErrNotFound) {
				return pkgerrors.ErrForbidden
			}
			return err
		}
	case mfavorite.FavoriteBoard:
		board, err := s.boards.FindBoardByID(ctx, targetID)
		if err != nil {
			return err
		}
		if _, err := s.workspaces.FindMember(ctx, board.WorkspaceID, actorID); err != nil {
			if errors.Is(err, pkgerrors.ErrNotFound) {
				return pkgerrors.ErrForbidden
			}
			return err
		}
	default:
		return pkgerrors.NewValidationError("invalid target type")
	}
	if err := s.favorites.AddFavorite(ctx, &mfavorite.Favorite{
		ID:         uuid.New().String(),
		UserID:     actorID,
		TargetType: targetType,
		TargetID:   targetID,
	}); err != nil {
		return err
	}
	s.bumpUser(ctx, actorID)
	s.broadcastUpdated(actorID, targetType, targetID)
	return nil
}

func (s *Service) broadcastUpdated(actorID, targetType, targetID string) {
	if s.hub == nil {
		return
	}
	s.hub.SendToUser(actorID, &ws.Message{
		Type: eventFavoritesUpdated,
		Data: map[string]string{"target_type": targetType, "target_id": targetID},
	})
}

func (s *Service) Remove(ctx context.Context, actorID, targetType, targetID string) error {
	if err := validate.UUID(targetID); err != nil {
		return err
	}
	switch targetType {
	case mfavorite.FavoriteWorkspace, mfavorite.FavoriteBoard:
	default:
		return pkgerrors.NewValidationError("invalid target type")
	}
	if err := s.favorites.RemoveFavorite(ctx, actorID, targetType, targetID); err != nil {
		return err
	}
	s.bumpUser(ctx, actorID)
	s.broadcastUpdated(actorID, targetType, targetID)
	return nil
}

var _ UseCase = (*Service)(nil)
