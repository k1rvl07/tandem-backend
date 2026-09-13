package favorite

import (
	"context"

	favcore "github.com/tandem/tandem/internal/usecase/favorite/core"
	"github.com/tandem/tandem/internal/usecase/favorite/features/crud"
	"github.com/tandem/tandem/internal/usecase/shared/base"
)

type UseCase interface {
	Add(ctx context.Context, actorID, targetType, targetID string) error
	Remove(ctx context.Context, actorID, targetType, targetID string) error
}

type Deps struct {
	base.Deps
}

type Service struct {
	*crud.Crud
}

func NewService(deps Deps) *Service {
	c := favcore.New(deps.Deps)
	return &Service{Crud: crud.New(c)}
}

var _ UseCase = (*Service)(nil)
