package repository

import (
	"context"

	"github.com/tandem/tandem/internal/domain/models"
)

type TaskRepository interface {
	CreateTask(ctx context.Context, task *models.Task) error
	FindTaskByID(ctx context.Context, id string) (*models.Task, error)
	FindTasksByIDs(ctx context.Context, ids []string) (map[string]*models.Task, error)
	UpdateTask(ctx context.Context, task *models.Task) error
	DeleteTask(ctx context.Context, id string) error
	ListTasksForBoard(ctx context.Context, boardID string) ([]*models.Task, error)
	ListTasksForColumn(ctx context.Context, columnID string) ([]*models.Task, error)
	ListTasksForWorkspace(ctx context.Context, workspaceID string) ([]*models.Task, error)
	ListChildTasks(ctx context.Context, parentID string) ([]*models.Task, error)
	CollectTaskKeys(ctx context.Context, taskID string) ([]string, error)
	CollectBoardKeys(ctx context.Context, boardID string) ([]string, error)
	CollectWorkspaceKeys(ctx context.Context, workspaceID string) ([]string, error)
}
