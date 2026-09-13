package admin

import (
	"context"

	dadmin "github.com/tandem/tandem/internal/http/dto/admin"
	dauth "github.com/tandem/tandem/internal/http/dto/auth"
	"github.com/tandem/tandem/internal/usecase/admin/control"
	"github.com/tandem/tandem/internal/usecase/admin/core"
	"github.com/tandem/tandem/internal/usecase/admin/users"
	"github.com/tandem/tandem/internal/usecase/shared/base"
)

type Actor = core.Actor

type UseCase interface {
	CreateUser(ctx context.Context, actor Actor, req dadmin.CreateUserRequest) (*dauth.UserResponse, error)
	ListUsers(ctx context.Context, query dadmin.AdminListQuery) (dadmin.AdminPage, error)
	UpdateUserRole(ctx context.Context, actor Actor, targetID string, req dadmin.UpdateUserRoleRequest) (*dauth.UserResponse, error)
	DeleteUser(ctx context.Context, actor Actor, targetID string) error
}

type Deps struct {
	base.Deps
}

type Service struct {
	*users.Users
	*control.Control
}

func NewService(deps Deps) *Service {
	c := core.New(deps.Users, deps.Hasher, deps.Cache, deps.Tokens, deps.Workspaces, deps.Favorites, deps.Files)
	return &Service{
		Users:   users.New(c),
		Control: control.New(c),
	}
}

var _ UseCase = (*Service)(nil)
