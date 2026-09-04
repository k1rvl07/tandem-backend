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

type BoardRepo struct {
	db *gorm.DB
}

func NewBoardRepo(db *gorm.DB) *BoardRepo {
	return &BoardRepo{db: db}
}

func (r *BoardRepo) CreateBoard(ctx context.Context, board *models.Board) error {
	e := boardToEntity(board)
	err := r.db.WithContext(ctx).Create(e).Error
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	board.CreatedAt = e.CreatedAt
	board.UpdatedAt = e.UpdatedAt
	return nil
}

func (r *BoardRepo) FindBoardByID(ctx context.Context, id string) (*models.Board, error) {
	var e entity.Board
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, err)
	}
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return boardToDomain(&e), nil
}

func (r *BoardRepo) UpdateBoard(ctx context.Context, board *models.Board) error {
	e := boardToEntity(board)
	err := r.db.WithContext(ctx).Model(&entity.Board{}).Where("id = ?", e.ID).Updates(map[string]interface{}{
		"name":     e.Name,
		"position": e.Position,
	}).Error
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return nil
}

func (r *BoardRepo) DeleteBoard(ctx context.Context, id string) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("column_id IN (SELECT id FROM board_columns WHERE board_id = ?)", id).Delete(&entity.Task{}).Error; err != nil {
			return err
		}
		if err := tx.Where("board_id = ?", id).Delete(&entity.Column{}).Error; err != nil {
			return err
		}
		res := tx.Where("id = ?", id).Delete(&entity.Board{})
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

func (r *BoardRepo) ListBoards(ctx context.Context, workspaceID string) ([]*models.Board, error) {
	var es []entity.Board
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Order("position ASC, created_at ASC").Find(&es).Error
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	boards := make([]*models.Board, 0, len(es))
	for i := range es {
		boards = append(boards, boardToDomain(&es[i]))
	}
	return boards, nil
}

func boardToEntity(b *models.Board) *entity.Board {
	return &entity.Board{
		ID:          b.ID,
		WorkspaceID: b.WorkspaceID,
		Name:        b.Name,
		Position:    b.Position,
		CreatedAt:   b.CreatedAt,
		UpdatedAt:   b.UpdatedAt,
	}
}

func boardToDomain(e *entity.Board) *models.Board {
	return &models.Board{
		ID:          e.ID,
		WorkspaceID: e.WorkspaceID,
		Name:        e.Name,
		Position:    e.Position,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

var _ repository.BoardRepository = (*BoardRepo)(nil)
