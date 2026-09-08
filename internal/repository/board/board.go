package board

import (
	"context"
	"errors"

	mboard "github.com/tandem/tandem/internal/domain/models/board"
	mfavorite "github.com/tandem/tandem/internal/domain/models/favorite"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	eboard "github.com/tandem/tandem/internal/repository/entity/board"
	efavorite "github.com/tandem/tandem/internal/repository/entity/favorite"
	etask "github.com/tandem/tandem/internal/repository/entity/task"
	"gorm.io/gorm"
)

type BoardRepo struct {
	db *gorm.DB
}

func NewBoardRepo(db *gorm.DB) *BoardRepo {
	return &BoardRepo{db: db}
}

func (r *BoardRepo) CreateBoard(ctx context.Context, board *mboard.Board) error {
	e := boardToEntity(board)
	err := r.db.WithContext(ctx).Create(e).Error
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	board.CreatedAt = e.CreatedAt
	board.UpdatedAt = e.UpdatedAt
	return nil
}

func (r *BoardRepo) FindBoardByID(ctx context.Context, id string) (*mboard.Board, error) {
	var e eboard.Board
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, err)
	}
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return boardToDomain(&e), nil
}

func (r *BoardRepo) UpdateBoard(ctx context.Context, board *mboard.Board) error {
	e := boardToEntity(board)
	err := r.db.WithContext(ctx).Model(&eboard.Board{}).Where("id = ?", e.ID).Updates(map[string]interface{}{
		"name":     e.Name,
		"position": e.Position,
		"is_main":  e.IsMain,
	}).Error
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return nil
}

func (r *BoardRepo) ClearMainBoards(ctx context.Context, workspaceID string) error {
	err := r.db.WithContext(ctx).Model(&eboard.Board{}).Where("workspace_id = ?", workspaceID).Update("is_main", false).Error
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return nil
}

func (r *BoardRepo) DeleteBoard(ctx context.Context, id string) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("task_id IN (SELECT tasks.id FROM tasks JOIN board_columns ON board_columns.id = tasks.column_id WHERE board_columns.board_id = ?)", id).Delete(&etask.TaskAttachment{}).Error; err != nil {
			return err
		}
		if err := tx.Where("column_id IN (SELECT id FROM board_columns WHERE board_id = ?)", id).Delete(&eboard.Task{}).Error; err != nil {
			return err
		}
		if err := tx.Where("board_id = ?", id).Delete(&eboard.Column{}).Error; err != nil {
			return err
		}
		if err := tx.Where("target_type = ? AND target_id = ?", mfavorite.FavoriteBoard, id).Delete(&efavorite.Favorite{}).Error; err != nil {
			return err
		}
		res := tx.Where("id = ?", id).Delete(&eboard.Board{})
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

func (r *BoardRepo) ListBoards(ctx context.Context, workspaceID string) ([]*mboard.Board, error) {
	var es []eboard.Board
	err := r.db.WithContext(ctx).Where("workspace_id = ?", workspaceID).Order("position ASC, created_at ASC").Find(&es).Error
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	boards := make([]*mboard.Board, 0, len(es))
	for i := range es {
		boards = append(boards, boardToDomain(&es[i]))
	}
	return boards, nil
}

func (r *BoardRepo) CountTasksByBoard(ctx context.Context, workspaceID string) (map[string]int, error) {
	type row struct {
		BoardID string
		Count   int
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Model(&eboard.Task{}).
		Select("cols.board_id as board_id, count(tasks.id) as count").
		Joins("JOIN board_columns cols ON cols.id = tasks.column_id").
		Where("cols.board_id IN (SELECT id FROM boards WHERE workspace_id = ?)", workspaceID).
		Group("cols.board_id").
		Scan(&rows).Error
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	counts := make(map[string]int, len(rows))
	for _, r := range rows {
		counts[r.BoardID] = r.Count
	}
	return counts, nil
}

func (r *BoardRepo) ReorderBoards(ctx context.Context, workspaceID string, boardIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i, id := range boardIDs {
			res := tx.Model(&eboard.Board{}).
				Where("id = ? AND workspace_id = ?", id, workspaceID).
				Update("position", i)
			if res.Error != nil {
				return res.Error
			}
		}
		return nil
	})
}

func boardToEntity(b *mboard.Board) *eboard.Board {
	return &eboard.Board{
		ID:          b.ID,
		WorkspaceID: b.WorkspaceID,
		Name:        b.Name,
		Position:    b.Position,
		IsMain:      b.IsMain,
		CreatedAt:   b.CreatedAt,
		UpdatedAt:   b.UpdatedAt,
	}
}

func boardToDomain(e *eboard.Board) *mboard.Board {
	return &mboard.Board{
		ID:          e.ID,
		WorkspaceID: e.WorkspaceID,
		Name:        e.Name,
		Position:    e.Position,
		IsMain:      e.IsMain,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

var _ repository.BoardRepository = (*BoardRepo)(nil)
