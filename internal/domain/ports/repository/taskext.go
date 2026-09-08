package repository

import (
	"context"

	mattachment "github.com/tandem/tandem/internal/domain/models/attachment"
	mfavorite "github.com/tandem/tandem/internal/domain/models/favorite"
)

type AttachmentRepository interface {
	CreateAttachment(ctx context.Context, attachment *mattachment.TaskAttachment) error
	FindAttachmentByID(ctx context.Context, id string) (*mattachment.TaskAttachment, error)
	ListAttachmentsByTask(ctx context.Context, taskID string) ([]mattachment.TaskAttachment, error)
	DeleteAttachment(ctx context.Context, id string) error
}

type FavoriteRepository interface {
	AddFavorite(ctx context.Context, favorite *mfavorite.Favorite) error
	RemoveFavorite(ctx context.Context, userID, targetType, targetID string) error
	IsFavorite(ctx context.Context, userID, targetType, targetID string) (bool, error)
	ListFavoriteTargets(ctx context.Context, userID, targetType string) (map[string]bool, error)
	DeleteUserFavorites(ctx context.Context, userID string) error
}
