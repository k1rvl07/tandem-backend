package tree

import (
	"context"

	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	dtree "github.com/tandem/tandem/internal/http/dto/tree"
	"github.com/tandem/tandem/internal/usecase/tree/build"
	"github.com/tandem/tandem/internal/usecase/tree/core"
	"github.com/tandem/tandem/internal/usecase/tree/query"
)

type UseCase interface {
	List(ctx context.Context, actorID string, query dtree.TreeQuery) ([]dtree.TreeWorkspaceResponse, error)
}

type Deps struct {
	Workspaces repository.WorkspaceRepository
	Boards     repository.BoardRepository
	Columns    repository.ColumnRepository
	Tasks      repository.TaskRepository
	Users      repository.UserRepository
	Favorites  repository.FavoriteRepository
	Cache      cache.Cache
}

type Service struct {
	*query.Query
}

func NewService(deps Deps) *Service {
	c := core.New(deps.Workspaces, deps.Boards, deps.Columns, deps.Tasks, deps.Users, deps.Favorites, deps.Cache)
	return &Service{Query: query.New(c, build.New(c))}
}

var _ UseCase = (*Service)(nil)
