package core

import (
	"context"
	"time"

	muser "github.com/tandem/tandem/internal/domain/models/user"
	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/service"
	dauth "github.com/tandem/tandem/internal/http/dto/auth"
	file "github.com/tandem/tandem/internal/usecase/file"
	cacheutil "github.com/tandem/tandem/internal/usecase/shared/cache"
	"go.uber.org/zap"
)

type Core struct {
	Users      repository.UserRepository
	Hasher     service.PasswordHasher
	Files      *file.Service
	Cache      cache.Cache
	Tokens     service.TokenService
	TokenTTL   time.Duration
	RefreshTTL time.Duration
	Logger     *zap.Logger
}

func New(users repository.UserRepository, hasher service.PasswordHasher, files *file.Service, cache cache.Cache, tokens service.TokenService, tokenTTL, refreshTTL time.Duration, logger *zap.Logger) *Core {
	return &Core{Users: users, Hasher: hasher, Files: files, Cache: cache, Tokens: tokens, TokenTTL: tokenTTL, RefreshTTL: refreshTTL, Logger: logger}
}

func (c *Core) BumpUser(ctx context.Context, userID string) {
	cacheutil.Bump(ctx, c.Cache, cacheutil.UVerKey+userID)
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
