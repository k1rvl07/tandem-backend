package core

import (
	"time"

	muser "github.com/tandem/tandem/internal/domain/models/user"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/service"
	dauth "github.com/tandem/tandem/internal/http/dto/auth"
	file "github.com/tandem/tandem/internal/usecase/file"
	sharedbase "github.com/tandem/tandem/internal/usecase/shared/base"
	"github.com/tandem/tandem/internal/usecase/shared/mapping"
	"go.uber.org/zap"
)

type Core struct {
	sharedbase.Base
	Users      repository.UserRepository
	Hasher     service.PasswordHasher
	Files      *file.Service
	Tokens     service.TokenService
	TokenTTL   time.Duration
	RefreshTTL time.Duration
	Logger     *zap.Logger
}

func New(d sharedbase.Deps) *Core {
	return &Core{
		Base:       sharedbase.Base{Cache: d.Cache},
		Users:      d.Users,
		Hasher:     d.Hasher,
		Files:      d.Files,
		Tokens:     d.Tokens,
		TokenTTL:   d.TokenTTL,
		RefreshTTL: d.RefreshTTL,
		Logger:     d.Logger,
	}
}

func (c *Core) UserResponse(u *muser.User) *dauth.UserResponse {
	return mapping.UserResponse(u)
}
