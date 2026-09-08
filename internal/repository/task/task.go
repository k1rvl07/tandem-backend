package task

import (
	"context"
	"errors"

	mtask "github.com/tandem/tandem/internal/domain/models/task"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	eboard "github.com/tandem/tandem/internal/repository/entity/board"
	etask "github.com/tandem/tandem/internal/repository/entity/task"
	"gorm.io/gorm"
)

type TaskRepo struct {
	db *gorm.DB
}

func NewTaskRepo(db *gorm.DB) *TaskRepo {
	return &TaskRepo{db: db}
}

func (r *TaskRepo) CreateTask(ctx context.Context, task *mtask.Task) error {
	e := taskToEntity(task)
	err := r.db.WithContext(ctx).Create(e).Error
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	task.CreatedAt = e.CreatedAt
	task.UpdatedAt = e.UpdatedAt
	return nil
}

func (r *TaskRepo) FindTaskByID(ctx context.Context, id string) (*mtask.Task, error) {
	var e eboard.Task
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, err)
	}
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return taskToDomain(&e), nil
}

func (r *TaskRepo) UpdateTask(ctx context.Context, task *mtask.Task) error {
	updates := map[string]interface{}{
		"column_id":   task.ColumnID,
		"title":       task.Title,
		"description": task.Description,
		"author_id":   task.AuthorID,
		"assignee_id": task.AssigneeID,
		"curator_id":  task.CuratorID,
		"parent_id":   task.ParentID,
		"position":    task.Position,
		"is_urgent":   task.IsUrgent,
		"is_hidden":   task.IsHidden,
		"image_key":   task.ImageKey,
	}
	if task.DueDate != nil {
		updates["due_date"] = *task.DueDate
	} else {
		updates["due_date"] = nil
	}
	err := r.db.WithContext(ctx).Model(&eboard.Task{}).Where("id = ?", task.ID).Updates(updates).Error
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return nil
}

func (r *TaskRepo) DeleteTask(ctx context.Context, id string) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("task_id IN (SELECT id FROM tasks WHERE id = ? OR parent_id = ?)", id, id).Delete(&etask.TaskAttachment{}).Error; err != nil {
			return err
		}
		res := tx.Where("id = ? OR parent_id = ?", id, id).Delete(&eboard.Task{})
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

func (r *TaskRepo) ListTasksForBoard(ctx context.Context, boardID string) ([]*mtask.Task, error) {
	var es []eboard.Task
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

func (r *TaskRepo) ListTasksForColumn(ctx context.Context, columnID string) ([]*mtask.Task, error) {
	var es []eboard.Task
	err := r.db.WithContext(ctx).Where("column_id = ?", columnID).Order("position ASC, created_at ASC").Find(&es).Error
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return tasksToDomain(es), nil
}

func (r *TaskRepo) ListTasksForWorkspace(ctx context.Context, workspaceID string) ([]*mtask.Task, error) {
	var es []eboard.Task
	err := r.db.WithContext(ctx).
		Table("tasks").
		Select("tasks.*").
		Joins("JOIN board_columns ON board_columns.id = tasks.column_id").
		Joins("JOIN boards ON boards.id = board_columns.board_id").
		Where("boards.workspace_id = ?", workspaceID).
		Order("tasks.position ASC, tasks.created_at ASC").
		Find(&es).Error
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return tasksToDomain(es), nil
}

func (r *TaskRepo) FindTasksByIDs(ctx context.Context, ids []string) (map[string]*mtask.Task, error) {
	if len(ids) == 0 {
		return map[string]*mtask.Task{}, nil
	}
	var es []eboard.Task
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&es).Error
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	result := make(map[string]*mtask.Task, len(es))
	for i := range es {
		result[es[i].ID] = taskToDomain(&es[i])
	}
	return result, nil
}

func (r *TaskRepo) ListChildTasks(ctx context.Context, parentID string) ([]*mtask.Task, error) {
	var es []eboard.Task
	err := r.db.WithContext(ctx).Where("parent_id = ?", parentID).Order("position ASC, created_at ASC").Find(&es).Error
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return tasksToDomain(es), nil
}

func (r *TaskRepo) CollectTaskKeys(ctx context.Context, taskID string) ([]string, error) {
	var imageKeys []string
	err := r.db.WithContext(ctx).
		Model(&eboard.Task{}).
		Where("(id = ? OR parent_id = ?) AND image_key <> ''", taskID, taskID).
		Pluck("image_key", &imageKeys).Error
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	keys, err := r.collectAttachmentKeys(ctx, "tasks.id = ? OR tasks.parent_id = ?", taskID, taskID)
	if err != nil {
		return nil, err
	}
	keys = append(keys, imageKeys...)
	return keys, nil
}

func (r *TaskRepo) CollectBoardKeys(ctx context.Context, boardID string) ([]string, error) {
	var imageKeys []string
	err := r.db.WithContext(ctx).
		Table("tasks").
		Joins("JOIN board_columns ON board_columns.id = tasks.column_id").
		Where("board_columns.board_id = ? AND tasks.image_key <> ''", boardID).
		Pluck("tasks.image_key", &imageKeys).Error
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	keys, err := r.collectAttachmentKeys(ctx, "board_columns.board_id = ?", boardID)
	if err != nil {
		return nil, err
	}
	keys = append(keys, imageKeys...)
	return keys, nil
}

func (r *TaskRepo) CollectWorkspaceKeys(ctx context.Context, workspaceID string) ([]string, error) {
	var imageKeys []string
	err := r.db.WithContext(ctx).
		Table("tasks").
		Joins("JOIN board_columns ON board_columns.id = tasks.column_id").
		Joins("JOIN boards ON boards.id = board_columns.board_id").
		Where("boards.workspace_id = ? AND tasks.image_key <> ''", workspaceID).
		Pluck("tasks.image_key", &imageKeys).Error
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	keys, err := r.collectAttachmentKeys(ctx, "boards.workspace_id = ?", workspaceID)
	if err != nil {
		return nil, err
	}
	keys = append(keys, imageKeys...)
	return keys, nil
}

func (r *TaskRepo) collectAttachmentKeys(ctx context.Context, scope string, args ...interface{}) ([]string, error) {
	var keys []string
	err := r.db.WithContext(ctx).
		Table("task_attachments").
		Select("task_attachments.object_key").
		Joins("JOIN tasks ON tasks.id = task_attachments.task_id").
		Joins("JOIN board_columns ON board_columns.id = tasks.column_id").
		Joins("JOIN boards ON boards.id = board_columns.board_id").
		Where(scope, args...).
		Scan(&keys).Error
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	return keys, nil
}

func taskToEntity(t *mtask.Task) *eboard.Task {
	return &eboard.Task{
		ID:          t.ID,
		ColumnID:    t.ColumnID,
		Title:       t.Title,
		Description: t.Description,
		AuthorID:    t.AuthorID,
		AssigneeID:  t.AssigneeID,
		CuratorID:   t.CuratorID,
		ParentID:    t.ParentID,
		DueDate:     t.DueDate,
		Position:    t.Position,
		IsUrgent:    t.IsUrgent,
		IsHidden:    t.IsHidden,
		ImageKey:    t.ImageKey,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

func taskToDomain(e *eboard.Task) *mtask.Task {
	return &mtask.Task{
		ID:          e.ID,
		ColumnID:    e.ColumnID,
		Title:       e.Title,
		Description: e.Description,
		AuthorID:    e.AuthorID,
		AssigneeID:  e.AssigneeID,
		CuratorID:   e.CuratorID,
		ParentID:    e.ParentID,
		DueDate:     e.DueDate,
		Position:    e.Position,
		IsUrgent:    e.IsUrgent,
		IsHidden:    e.IsHidden,
		ImageKey:    e.ImageKey,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

func tasksToDomain(es []eboard.Task) []*mtask.Task {
	tasks := make([]*mtask.Task, 0, len(es))
	for i := range es {
		tasks = append(tasks, taskToDomain(&es[i]))
	}
	return tasks
}

var _ repository.TaskRepository = (*TaskRepo)(nil)
