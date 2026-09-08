package repository

import (
	"context"

	mtask "github.com/tandem/tandem/internal/domain/models/task"
)

type TaskRepository interface {
	CreateTask(ctx context.Context, task *mtask.Task) error
	FindTaskByID(ctx context.Context, id string) (*mtask.Task, error)
	FindTasksByIDs(ctx context.Context, ids []string) (map[string]*mtask.Task, error)
	UpdateTask(ctx context.Context, task *mtask.Task) error
	DeleteTask(ctx context.Context, id string) error
	ListTasksForBoard(ctx context.Context, boardID string) ([]*mtask.Task, error)
	ListTasksForColumn(ctx context.Context, columnID string) ([]*mtask.Task, error)
	ListTasksForWorkspace(ctx context.Context, workspaceID string) ([]*mtask.Task, error)
	ListChildTasks(ctx context.Context, parentID string) ([]*mtask.Task, error)
	CollectTaskKeys(ctx context.Context, taskID string) ([]string, error)
	CollectBoardKeys(ctx context.Context, boardID string) ([]string, error)
	CollectWorkspaceKeys(ctx context.Context, workspaceID string) ([]string, error)
}
