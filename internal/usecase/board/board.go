package board

import (
	"context"

	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	dboard "github.com/tandem/tandem/internal/http/dto/board"
	"github.com/tandem/tandem/internal/usecase/board/boards"
	"github.com/tandem/tandem/internal/usecase/board/core"
	file "github.com/tandem/tandem/internal/usecase/file"
)

type UseCase interface {
	Create(ctx context.Context, actorID, workspaceID string, req dboard.CreateBoardRequest) (*dboard.BoardResponse, error)
	List(ctx context.Context, actorID, workspaceID string) ([]dboard.BoardResponse, error)
	Get(ctx context.Context, actorID, workspaceID, boardID string) (*dboard.BoardDetailResponse, error)
	Update(ctx context.Context, actorID, workspaceID, boardID string, req dboard.UpdateBoardRequest) (*dboard.BoardResponse, error)
	Delete(ctx context.Context, actorID, workspaceID, boardID string) error
	SetMain(ctx context.Context, actorID, workspaceID, boardID string) (*dboard.BoardResponse, error)
	Reorder(ctx context.Context, actorID, workspaceID string, req dboard.ReorderBoardsRequest) ([]dboard.BoardResponse, error)
}

type Deps struct {
	Boards     repository.BoardRepository
	Columns    repository.ColumnRepository
	Tasks      repository.TaskRepository
	Workspaces repository.WorkspaceRepository
	Users      repository.UserRepository
	Favorites  repository.FavoriteRepository
	Files      *file.Service
	Hub        ws.Hub
	Cache      cache.Cache
}

type Service struct {
	*boards.Boards
}

func NewService(deps Deps) *Service {
	c := core.New(deps.Boards, deps.Columns, deps.Tasks, deps.Workspaces, deps.Users, deps.Favorites, deps.Files, deps.Hub, deps.Cache)
	return &Service{Boards: boards.New(c)}
}

var _ UseCase = (*Service)(nil)
