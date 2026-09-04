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

type TaskRepo struct {
	db *gorm.DB
}

func NewTaskRepo(db *gorm.DB) *TaskRepo {
	return &TaskRepo{db: db}
}

func (r *TaskRepo) CreateTask(ctx context.Context, task *models.Task) error {
	e := taskToEntity(task)
	err := r.db.WithContext(ctx).Create(e).Error
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	task.CreatedAt = e.CreatedAt
	task.UpdatedAt = e.UpdatedAt
	return nil
}

func (r *TaskRepo) FindTaskByID(ctx context.Context, id string) (*models.Task, error) {
	var e entity.Task
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, err)
	}
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return taskToDomain(&e), nil
}

func (r *TaskRepo) UpdateTask(ctx context.Context, task *models.Task) error {
	updates := map[string]interface{}{
		"column_id":   task.ColumnID,
		"title":       task.Title,
		"description": task.Description,
		"assignee_id": task.AssigneeID,
		"priority":    task.Priority,
		"position":    task.Position,
	}
	if task.DueDate != nil {
		updates["due_date"] = *task.DueDate
	} else {
		updates["due_date"] = nil
	}
	err := r.db.WithContext(ctx).Model(&entity.Task{}).Where("id = ?", task.ID).Updates(updates).Error
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return nil
}

func (r *TaskRepo) DeleteTask(ctx context.Context, id string) error {
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&entity.Task{})
	if res.Error != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, res.Error)
	}
	if res.RowsAffected == 0 {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	return nil
}

func (r *TaskRepo) ListTasksForBoard(ctx context.Context, boardID string) ([]*models.Task, error) {
	var es []entity.Task
	err := r.db.WithContext(ctx).
		Table("tasks").
		Select("tasks.*").
		Joins("JOIN board_columns ON board_columns.id = tasks.column_id").
		Where("board_columns.board_id = ?", boardID).
		Order("tasks.position ASC, tasks.created_at ASC").
		Find(&es).Error
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return tasksToDomain(es), nil
}

func (r *TaskRepo) ListTasksForColumn(ctx context.Context, columnID string) ([]*models.Task, error) {
	var es []entity.Task
	err := r.db.WithContext(ctx).Where("column_id = ?", columnID).Order("position ASC, created_at ASC").Find(&es).Error
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return tasksToDomain(es), nil
}

func taskToEntity(t *models.Task) *entity.Task {
	return &entity.Task{
		ID:          t.ID,
		ColumnID:    t.ColumnID,
		Title:       t.Title,
		Description: t.Description,
		AssigneeID:  t.AssigneeID,
		Priority:    t.Priority,
		DueDate:     t.DueDate,
		Position:    t.Position,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

func taskToDomain(e *entity.Task) *models.Task {
	return &models.Task{
		ID:          e.ID,
		ColumnID:    e.ColumnID,
		Title:       e.Title,
		Description: e.Description,
		AssigneeID:  e.AssigneeID,
		Priority:    e.Priority,
		DueDate:     e.DueDate,
		Position:    e.Position,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

func tasksToDomain(es []entity.Task) []*models.Task {
	tasks := make([]*models.Task, 0, len(es))
	for i := range es {
		tasks = append(tasks, taskToDomain(&es[i]))
	}
	return tasks
}

var _ repository.TaskRepository = (*TaskRepo)(nil)
