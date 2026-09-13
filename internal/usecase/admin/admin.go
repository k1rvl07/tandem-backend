package admin

import (
	"context"

	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/service"
	dadmin "github.com/tandem/tandem/internal/http/dto/admin"
	dauth "github.com/tandem/tandem/internal/http/dto/auth"
	"github.com/tandem/tandem/internal/usecase/admin/control"
	"github.com/tandem/tandem/internal/usecase/admin/core"
	"github.com/tandem/tandem/internal/usecase/admin/users"
	file "github.com/tandem/tandem/internal/usecase/file"
)

type Actor = core.Actor

type UseCase interface {
	CreateUser(ctx context.Context, actor Actor, req dadmin.CreateUserRequest) (*dauth.UserResponse, error)
	ListUsers(ctx context.Context, actorID string, query dadmin.AdminListQuery) (dadmin.AdminPage, error)
	UpdateUserRole(ctx context.Context, actor Actor, targetID string, req dadmin.UpdateUserRoleRequest) (*dauth.UserResponse, error)
	DeleteUser(ctx context.Context, actor Actor, targetID string) error
}

type Deps struct {
	Users      repository.UserRepository
	Hasher     service.PasswordHasher
	Cache      cache.Cache
	Tokens     service.TokenService
	Workspaces repository.WorkspaceRepository
	Favorites  repository.FavoriteRepository
	Files      *file.Service
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
