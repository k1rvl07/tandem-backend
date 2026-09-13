package task

import (
	"context"

	dtask "github.com/tandem/tandem/internal/http/dto/task"
	"github.com/tandem/tandem/internal/usecase/shared/base"
	"github.com/tandem/tandem/internal/usecase/task/core"
	"github.com/tandem/tandem/internal/usecase/task/features/crud"
)

type UseCase interface {
	Create(ctx context.Context, actorID, workspaceID, boardID string, req dtask.CreateTaskRequest) (*dtask.TaskResponse, error)
	Update(ctx context.Context, actorID, workspaceID, boardID, taskID string, req dtask.UpdateTaskRequest) (*dtask.TaskResponse, error)
	Delete(ctx context.Context, actorID, workspaceID, boardID, taskID string) error
	Get(ctx context.Context, actorID, workspaceID, taskID string) (*dtask.TaskDetailResponse, error)
	List(ctx context.Context, actorID, workspaceID string, query dtask.ListWorkspaceTasksQuery) ([]dtask.TaskResponse, error)
}

type Deps struct {
	base.Deps
}

type Service struct {
	*crud.Crud
}

func NewService(deps Deps) *Service {
	c := core.New(deps.Tasks, deps.Columns, deps.Boards, deps.Workspaces, deps.Users, deps.Files, deps.Hub, deps.Cache)
	return &Service{Crud: crud.New(c)}
}

var _ UseCase = (*Service)(nil)
