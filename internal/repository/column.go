package repository

import (
	"context"
	"errors"

	"github.com/tandem/tandem/internal/domain/models"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/repository/entity"
	"gorm.io/gorm"
)

type ColumnRepo struct {
	db *gorm.DB
}

func NewColumnRepo(db *gorm.DB) *ColumnRepo {
	return &ColumnRepo{db: db}
}

func (r *ColumnRepo) CreateColumn(ctx context.Context, column *models.Column) error {
	e := columnToEntity(column)
	err := r.db.WithContext(ctx).Create(e).Error
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	column.CreatedAt = e.CreatedAt
	column.UpdatedAt = e.UpdatedAt
	return nil
}

func (r *ColumnRepo) FindColumnByID(ctx context.Context, id string) (*models.Column, error) {
	var e entity.Column
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, err)
	}
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return columnToDomain(&e), nil
}

func (r *ColumnRepo) UpdateColumn(ctx context.Context, column *models.Column) error {
	e := columnToEntity(column)
	err := r.db.WithContext(ctx).Model(&entity.Column{}).Where("id = ?", e.ID).Updates(map[string]interface{}{
		"name":     e.Name,
		"position": e.Position,
	}).Error
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return nil
}

func (r *ColumnRepo) DeleteColumn(ctx context.Context, id string) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("column_id = ?", id).Delete(&entity.Task{}).Error; err != nil {
			return err
		}
		res := tx.Where("id = ?", id).Delete(&entity.Column{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, err)
	}
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return nil
}

func (r *ColumnRepo) ListColumns(ctx context.Context, boardID string) ([]*models.Column, error) {
	var es []entity.Column
	err := r.db.WithContext(ctx).Where("board_id = ?", boardID).Order("position ASC, created_at ASC").Find(&es).Error
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	columns := make([]*models.Column, 0, len(es))
	for i := range es {
		columns = append(columns, columnToDomain(&es[i]))
	}
	return columns, nil
}

func columnToEntity(c *models.Column) *entity.Column {
	return &entity.Column{
		ID:        c.ID,
		BoardID:   c.BoardID,
		Name:      c.Name,
		Position:  c.Position,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

func columnToDomain(e *entity.Column) *models.Column {
	return &models.Column{
		ID:        e.ID,
		BoardID:   e.BoardID,
		Name:      e.Name,
		Position:  e.Position,
		CreatedAt: e.CreatedAt,
		UpdatedAt: e.UpdatedAt,
	}
}

var _ repository.ColumnRepository = (*ColumnRepo)(nil)
