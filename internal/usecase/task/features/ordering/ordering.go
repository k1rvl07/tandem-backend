package ordering

import (
	"context"

	dtask "github.com/tandem/tandem/internal/contracts/task"
	mtask "github.com/tandem/tandem/internal/domain/models/task"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
	"github.com/tandem/tandem/internal/usecase/task/core"
)

type Ordering struct {
	*core.Core
}

func New(c *core.Core) *Ordering {
	return &Ordering{Core: c}
}

func (s *Ordering) PlanTaskMoves(ctx context.Context, workspaceID, boardID string, task *mtask.Task, req dtask.UpdateTaskRequest) (bool, string, int, error) {
	if req.BoardID != nil && *req.BoardID != boardID {
		target, err := s.BoardInWorkspace(ctx, workspaceID, *req.BoardID)
		if err != nil {
			return false, "", 0, err
		}
		columns, err := s.Columns.ListColumns(ctx, target.ID)
		if err != nil {
			return false, "", 0, err
		}
		if len(columns) == 0 {
			return false, "", 0, pkgerrors.NewValidationError("target board has no columns")
		}
		return true, columns[0].ID, 0, nil
	}
	if req.ColumnID != nil || req.Position != nil {
		targetColumnID := task.ColumnID
		if req.ColumnID != nil {
			if err := validate.UUID(*req.ColumnID); err != nil {
				return false, "", 0, err
			}
			if _, err := s.ColumnInBoard(ctx, boardID, *req.ColumnID); err != nil {
				return false, "", 0, err
			}
			targetColumnID = *req.ColumnID
		}
		position := 0
		if req.Position != nil {
			position = *req.Position
			if position < 0 {
				position = 0
			}
		}
		if targetColumnID != task.ColumnID || req.Position != nil {
			return true, targetColumnID, position, nil
		}
	}
	return false, "", 0, nil
}

func (s *Ordering) NormalizePagination(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}
