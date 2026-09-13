package core

import (
	"context"

	muser "github.com/tandem/tandem/internal/domain/models/user"
	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/service"
	dauth "github.com/tandem/tandem/internal/http/dto/auth"
	file "github.com/tandem/tandem/internal/usecase/file"
	cacheutil "github.com/tandem/tandem/internal/usecase/shared/cache"
)

type Actor struct {
	ID   string
	Role string
}

type Core struct {
	Users      repository.UserRepository
	Hasher     service.PasswordHasher
	Cache      cache.Cache
	Tokens     service.TokenService
	Workspaces repository.WorkspaceRepository
	Favorites  repository.FavoriteRepository
	Files      *file.Service
}

func New(users repository.UserRepository, hasher service.PasswordHasher, cache cache.Cache, tokens service.TokenService, workspaces repository.WorkspaceRepository, favorites repository.FavoriteRepository, files *file.Service) *Core {
	return &Core{Users: users, Hasher: hasher, Cache: cache, Tokens: tokens, Workspaces: workspaces, Favorites: favorites, Files: files}
}

func (c *Core) BumpUsers(ctx context.Context) {
	cacheutil.Bump(ctx, c.Cache, cacheutil.UsersVerKey)
}

func (c *Core) UserResponse(u *muser.User) *dauth.UserResponse {
	return &dauth.UserResponse{
		ID:          u.ID,
		Login:       u.Login,
		Role:        u.Role,
		DisplayName: u.DisplayName,
		Bio:         u.Bio,
		AvatarKey:   u.AvatarKey,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}
