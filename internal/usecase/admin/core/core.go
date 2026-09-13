package core

import (
	"context"

	muser "github.com/tandem/tandem/internal/domain/models/user"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/service"
	dauth "github.com/tandem/tandem/internal/http/dto/auth"
	file "github.com/tandem/tandem/internal/usecase/file"
	sharedbase "github.com/tandem/tandem/internal/usecase/shared/base"
	cacheutil "github.com/tandem/tandem/internal/usecase/shared/cache"
	"github.com/tandem/tandem/internal/usecase/shared/mapping"
	"go.uber.org/zap"
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
	Logger    *zap.Logger
}

func New(d sharedbase.Deps) *Core {
	return &Core{
		Base:      sharedbase.Base{Workspaces: d.Workspaces, Cache: d.Cache},
		Users:     d.Users,
		Hasher:    d.Hasher,
		Tokens:    d.Tokens,
		Favorites: d.Favorites,
		Files:     d.Files,
		Logger:    d.Logger,
	}
}

func (c *Core) BumpUsers(ctx context.Context) {
	cacheutil.BumpUsers(ctx, c.Cache)
}

func (c *Core) UserResponse(u *muser.User) *dauth.UserResponse {
	return mapping.UserResponse(u)
}
