package task

import (
	"context"

	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	dtask "github.com/tandem/tandem/internal/http/dto/task"
	file "github.com/tandem/tandem/internal/usecase/file"
	"github.com/tandem/tandem/internal/usecase/task/core"
	"github.com/tandem/tandem/internal/usecase/task/crud"
)

type UseCase interface {
	Create(ctx context.Context, actorID, workspaceID, boardID string, req dtask.CreateTaskRequest) (*dtask.TaskResponse, error)
	Update(ctx context.Context, actorID, workspaceID, boardID, taskID string, req dtask.UpdateTaskRequest) (*dtask.TaskResponse, error)
	Delete(ctx context.Context, actorID, workspaceID, boardID, taskID string) error
	Get(ctx context.Context, actorID, workspaceID, taskID string) (*dtask.TaskDetailResponse, error)
	List(ctx context.Context, actorID, workspaceID string, query dtask.ListWorkspaceTasksQuery) ([]dtask.TaskResponse, error)
}

type Deps struct {
	Tasks      repository.TaskRepository
	Columns    repository.ColumnRepository
	Boards     repository.BoardRepository
	Workspaces repository.WorkspaceRepository
	Users      repository.UserRepository
	Files      *file.Service
	Hub        ws.Hub
	Cache      cache.Cache
}

type Service struct {
	*crud.Crud
}

func NewService(deps Deps) *Service {
	c := core.New(deps.Tasks, deps.Columns, deps.Boards, deps.Workspaces, deps.Users, deps.Files, deps.Hub, deps.Cache)
	return &Service{Crud: crud.New(c)}
}

var _ UseCase = (*Service)(nil)
