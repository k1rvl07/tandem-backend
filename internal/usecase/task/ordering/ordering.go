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
		targetTasks, err := s.Tasks.ListTasksForColumn(ctx, first.ID)
		if err != nil {
			return err
		}
		if err := s.LeaveColumn(ctx, task); err != nil {
			return err
		}
		task.ColumnID = first.ID
		if err := s.ShiftPositions(ctx, targetTasks); err != nil {
			return err
		}
		task.Position = 0
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
		position := -1
		if req.Position != nil {
			position = *req.Position
		}
		if position < 0 {
			position = 0
		}
		shouldMove := targetColumnID != task.ColumnID || req.Position != nil
		if shouldMove {
			if err := s.MoveTask(ctx, task, targetColumnID, position); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Ordering) MoveTask(ctx context.Context, task *mtask.Task, targetColumnID string, position int) error {
	if task.ColumnID == targetColumnID {
		order, err := s.Tasks.ListTasksForColumn(ctx, targetColumnID)
		if err != nil {
			return err
		}
		order = removeTask(order, task.ID)
		order = insertTask(order, position, task)
		return s.RewritePositions(ctx, order)
	}

	if err := s.LeaveColumn(ctx, task); err != nil {
		return err
	}
	target, err := s.Tasks.ListTasksForColumn(ctx, targetColumnID)
	if err != nil {
		return err
	}
	target = insertTask(target, position, task)
	if err := s.RewritePositions(ctx, target); err != nil {
		return err
	}
	task.ColumnID = targetColumnID
	return nil
}

func (s *Ordering) LeaveColumn(ctx context.Context, task *mtask.Task) error {
	source, err := s.Tasks.ListTasksForColumn(ctx, task.ColumnID)
	if err != nil {
		return err
	}
	source = removeTask(source, task.ID)
	return s.RewritePositions(ctx, source)
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

func (s *Ordering) RewritePositions(ctx context.Context, order []*mtask.Task) error {
	for i, t := range order {
		if t.Position == i {
			continue
		}
		t.Position = i
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

func removeTask(tasks []*mtask.Task, id string) []*mtask.Task {
	filtered := make([]*mtask.Task, 0, len(tasks))
	for _, t := range tasks {
		if t.ID != id {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func insertTask(tasks []*mtask.Task, position int, task *mtask.Task) []*mtask.Task {
	if position < 0 || position > len(tasks) {
		position = len(tasks)
	}
	tasks = append(tasks, nil)
	copy(tasks[position+1:], tasks[position:])
	tasks[position] = task
	return tasks
}
