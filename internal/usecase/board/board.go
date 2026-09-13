package board

import (
	"context"

	dboard "github.com/tandem/tandem/internal/http/dto/board"
	"github.com/tandem/tandem/internal/usecase/board/boards"
	"github.com/tandem/tandem/internal/usecase/board/core"
	"github.com/tandem/tandem/internal/usecase/shared/base"
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
	base.Deps
}

type Service struct {
	*boards.Boards
}

func NewService(deps Deps) *Service {
	c := core.New(deps.Boards, deps.Columns, deps.Tasks, deps.Workspaces, deps.Users, deps.Favorites, deps.Files, deps.Hub, deps.Cache)
	return &Service{Boards: boards.New(c)}
}

var _ UseCase = (*Service)(nil)
