package ordering

import (
	"context"

	mtask "github.com/tandem/tandem/internal/domain/models/task"
	dtask "github.com/tandem/tandem/internal/http/dto/task"
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

func (s *Ordering) ApplyTaskMoves(ctx context.Context, workspaceID, boardID string, task *mtask.Task, req dtask.UpdateTaskRequest) error {
	if req.BoardID != nil && *req.BoardID != boardID {
		target, err := s.BoardInWorkspace(ctx, workspaceID, *req.BoardID)
		if err != nil {
			return err
		}
		columns, err := s.Columns.ListColumns(ctx, target.ID)
		if err != nil {
			return err
		}
		if len(columns) == 0 {
			return pkgerrors.NewValidationError("target board has no columns")
		}
		first := columns[0]
		return s.MoveTask(ctx, task, first.ID, 0)
	} else if req.ColumnID != nil || req.Position != nil {
		targetColumnID := task.ColumnID
		if req.ColumnID != nil {
			if err := validate.UUID(*req.ColumnID); err != nil {
				return err
			}
			if _, err := s.ColumnInBoard(ctx, boardID, *req.ColumnID); err != nil {
				return err
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
			return s.MoveTask(ctx, task, targetColumnID, position)
		}
	}
	return nil
}

func (s *Ordering) MoveTask(ctx context.Context, task *mtask.Task, targetColumnID string, position int) error {
	return s.Tasks.MoveTask(ctx, task, targetColumnID, position)
}

func (s *Ordering) ShiftPositions(ctx context.Context, order []*mtask.Task) error {
	for _, t := range order {
		t.Position++
		if err := s.Tasks.UpdateTask(ctx, t); err != nil {
			return err
		}
	}
	return nil
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
