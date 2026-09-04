package repository

import (
	"context"

	"github.com/tandem/tandem/internal/domain/models"
)

type TaskRepository interface {
	CreateTask(ctx context.Context, task *models.Task) error
	FindTaskByID(ctx context.Context, id string) (*models.Task, error)
	UpdateTask(ctx context.Context, task *models.Task) error
	DeleteTask(ctx context.Context, id string) error
	ListTasksForBoard(ctx context.Context, boardID string) ([]*models.Task, error)
	ListTasksForColumn(ctx context.Context, columnID string) ([]*models.Task, error)
}
