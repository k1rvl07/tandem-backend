package core

import (
	"context"

	muser "github.com/tandem/tandem/internal/domain/models/user"
	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/service"
	dauth "github.com/tandem/tandem/internal/http/dto/auth"
	file "github.com/tandem/tandem/internal/usecase/file"
	sharedbase "github.com/tandem/tandem/internal/usecase/shared/base"
	cacheutil "github.com/tandem/tandem/internal/usecase/shared/cache"
	"github.com/tandem/tandem/internal/usecase/shared/taskmap"
)

type Actor struct {
	ID   string
	Role string
}

type Core struct {
	sharedbase.Base
	Users     repository.UserRepository
	Hasher    service.PasswordHasher
	Tokens    service.TokenService
	Favorites repository.FavoriteRepository
	Files     *file.Service
}

func New(users repository.UserRepository, hasher service.PasswordHasher, cache cache.Cache, tokens service.TokenService, workspaces repository.WorkspaceRepository, favorites repository.FavoriteRepository, files *file.Service) *Core {
	return &Core{Base: sharedbase.Base{Workspaces: workspaces, Cache: cache}, Users: users, Hasher: hasher, Tokens: tokens, Favorites: favorites, Files: files}
}

func (c *Core) BumpUsers(ctx context.Context) {
	cacheutil.Bump(ctx, c.Cache, cacheutil.UsersVerKey)
}

func (c *Core) UserResponse(u *muser.User) *dauth.UserResponse {
	return taskmap.UserResponse(u)
}
