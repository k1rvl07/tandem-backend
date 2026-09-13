package tree

import (
	"context"

	dtree "github.com/tandem/tandem/internal/http/dto/tree"
	"github.com/tandem/tandem/internal/usecase/shared/base"
	"github.com/tandem/tandem/internal/usecase/tree/core"
	"github.com/tandem/tandem/internal/usecase/tree/features/build"
	"github.com/tandem/tandem/internal/usecase/tree/features/query"
)

type UseCase interface {
	List(ctx context.Context, actorID string, query dtree.TreeQuery) ([]dtree.TreeWorkspaceResponse, error)
}

type Deps struct {
	base.Deps
}

type Service struct {
	*query.Query
}

func NewService(deps Deps) *Service {
	c := core.New(deps.Deps)
	return &Service{Query: query.New(c, build.New(c))}
}

var _ UseCase = (*Service)(nil)
