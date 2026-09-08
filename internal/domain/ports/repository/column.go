package repository

import (
	"context"

	mcolumn "github.com/tandem/tandem/internal/domain/models/column"
)

type ColumnRepository interface {
	CreateColumn(ctx context.Context, column *mcolumn.Column) error
	FindColumnByID(ctx context.Context, id string) (*mcolumn.Column, error)
	ListColumns(ctx context.Context, boardID string) ([]*mcolumn.Column, error)
	ListColumnsForWorkspace(ctx context.Context, workspaceID string) ([]*mcolumn.Column, error)
}
