package column

import (
	"context"

	mcolumn "github.com/tandem/tandem/internal/domain/models/column"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	eboard "github.com/tandem/tandem/internal/repository/entity/board"
	"github.com/tandem/tandem/internal/repository/postgres"
	"gorm.io/gorm"
)

type ColumnRepo struct {
	helper *postgres.Helper
}

func NewColumnRepo(db *gorm.DB) *ColumnRepo {
	return &ColumnRepo{helper: postgres.NewHelper(db)}
}

func (r *ColumnRepo) CreateColumn(ctx context.Context, column *mcolumn.Column) error {
	e := columnToEntity(column)
	err := r.helper.DB.WithContext(ctx).Create(e).Error
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	column.CreatedAt = e.CreatedAt
	column.UpdatedAt = e.UpdatedAt
	return nil
}

func (r *ColumnRepo) FindColumnByID(ctx context.Context, id string) (*mcolumn.Column, error) {
	var e eboard.Column
	if err := r.helper.First(ctx, &e, "id = ?", id); err != nil {
		return nil, err
	}
	return columnToDomain(&e), nil
}

func (r *ColumnRepo) ListColumns(ctx context.Context, boardID string) ([]*mcolumn.Column, error) {
	var es []eboard.Column
	if err := r.helper.List(ctx, &es, "position ASC, created_at ASC", "board_id = ?", boardID); err != nil {
		return nil, err
	}
	columns := make([]*mcolumn.Column, 0, len(es))
	for i := range es {
		columns = append(columns, columnToDomain(&es[i]))
	}
	return columns, nil
}

func (r *ColumnRepo) ListColumnsForWorkspace(ctx context.Context, workspaceID string) ([]*mcolumn.Column, error) {
	var es []eboard.Column
	err := r.helper.DB.WithContext(ctx).
		Table("board_columns").
		Select("board_columns.*").
		Joins("JOIN boards ON boards.id = board_columns.board_id").
		Where("boards.workspace_id = ?", workspaceID).
		Order("board_columns.position ASC, board_columns.created_at ASC").
		Find(&es).Error
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	columns := make([]*mcolumn.Column, 0, len(es))
	for i := range es {
		columns = append(columns, columnToDomain(&es[i]))
	}
	return columns, nil
}

func columnToEntity(c *mcolumn.Column) *eboard.Column {
	return &eboard.Column{
		ID:        c.ID,
		BoardID:   c.BoardID,
		Name:      c.Name,
		Position:  c.Position,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

func columnToDomain(e *eboard.Column) *mcolumn.Column {
	return &mcolumn.Column{
		ID:        e.ID,
		BoardID:   e.BoardID,
		Name:      e.Name,
		Position:  e.Position,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	}
}

var _ repository.ColumnRepository = (*ColumnRepo)(nil)
