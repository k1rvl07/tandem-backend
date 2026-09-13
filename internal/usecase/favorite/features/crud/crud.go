package crud

import (
	"context"

	"github.com/google/uuid"
	mfavorite "github.com/tandem/tandem/internal/domain/models/favorite"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
	favcore "github.com/tandem/tandem/internal/usecase/favorite/core"
	"github.com/tandem/tandem/internal/usecase/shared/access"
)

type Crud struct {
	*favcore.Core
}

func New(c *favcore.Core) *Crud {
	return &Crud{Core: c}
}

func (s *Crud) Add(ctx context.Context, actorID, targetType, targetID string) error {
	if err := validate.UUID(targetID); err != nil {
		return err
	}
	switch targetType {
	case mfavorite.FavoriteWorkspace:
		if _, err := access.MemberOrForbidden(ctx, s.Workspaces, targetID, actorID); err != nil {
			return err
		}
	case mfavorite.FavoriteBoard:
		board, err := s.Boards.FindBoardByID(ctx, targetID)
		if err != nil {
			return err
		}
		if _, err := access.MemberOrForbidden(ctx, s.Workspaces, board.WorkspaceID, actorID); err != nil {
			return err
		}
	default:
		return pkgerrors.NewValidationError("invalid target type")
	}
	if err := s.Favorites.AddFavorite(ctx, &mfavorite.Favorite{
		ID:         uuid.New().String(),
		UserID:     actorID,
		TargetType: targetType,
		TargetID:   targetID,
	}); err != nil {
		return err
	}
	s.BumpUser(ctx, actorID)
	s.BroadcastUpdated(actorID, targetType, targetID)
	return nil
}

func (s *Crud) Remove(ctx context.Context, actorID, targetType, targetID string) error {
	if err := validate.UUID(targetID); err != nil {
		return err
	}
	switch targetType {
	case mfavorite.FavoriteWorkspace, mfavorite.FavoriteBoard:
	default:
		return pkgerrors.NewValidationError("invalid target type")
	}
	if err := s.Favorites.RemoveFavorite(ctx, actorID, targetType, targetID); err != nil {
		return err
	}
	s.BumpUser(ctx, actorID)
	s.BroadcastUpdated(actorID, targetType, targetID)
	return nil
}
