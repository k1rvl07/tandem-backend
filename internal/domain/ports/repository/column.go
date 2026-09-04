package repository

import (
	"context"

	"github.com/tandem/tandem/internal/domain/models"
)

type ColumnRepository interface {
	CreateColumn(ctx context.Context, column *models.Column) error
	FindColumnByID(ctx context.Context, id string) (*models.Column, error)
	UpdateColumn(ctx context.Context, column *models.Column) error
	DeleteColumn(ctx context.Context, id string) error
	ListColumns(ctx context.Context, boardID string) ([]*models.Column, error)
}
