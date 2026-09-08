package repository

import (
	"context"

	"github.com/tandem/tandem/internal/domain/models"
)

type AttachmentRepository interface {
	CreateAttachment(ctx context.Context, attachment *models.TaskAttachment) error
	FindAttachmentByID(ctx context.Context, id string) (*models.TaskAttachment, error)
	ListAttachmentsByTask(ctx context.Context, taskID string) ([]models.TaskAttachment, error)
	DeleteAttachment(ctx context.Context, id string) error
}

type FavoriteRepository interface {
	AddFavorite(ctx context.Context, favorite *models.Favorite) error
	RemoveFavorite(ctx context.Context, userID, targetType, targetID string) error
	IsFavorite(ctx context.Context, userID, targetType, targetID string) (bool, error)
	ListFavoriteTargets(ctx context.Context, userID, targetType string) (map[string]bool, error)
	DeleteUserFavorites(ctx context.Context, userID string) error
}
