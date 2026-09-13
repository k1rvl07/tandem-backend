package profile

import (
	"context"
	"io"
	"time"

	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/service"
	dauth "github.com/tandem/tandem/internal/http/dto/auth"
	dprofile "github.com/tandem/tandem/internal/http/dto/profile"
	file "github.com/tandem/tandem/internal/usecase/file"
	"github.com/tandem/tandem/internal/usecase/profile/account"
	"github.com/tandem/tandem/internal/usecase/profile/avatar"
	"github.com/tandem/tandem/internal/usecase/profile/core"
	"github.com/tandem/tandem/internal/usecase/profile/password"
	"go.uber.org/zap"
)

type UserCase interface {
	Get(ctx context.Context, userID string) (*dauth.UserResponse, error)
	UpdateProfile(ctx context.Context, userID string, req dprofile.UpdateProfileRequest) (*dauth.UserResponse, error)
	ChangePassword(ctx context.Context, userID string, req dprofile.ChangePasswordRequest) (*dprofile.ChangePasswordResponse, error)
	UploadAvatar(ctx context.Context, userID, filename, contentType string, reader io.Reader, size int64) (*dauth.UserResponse, error)
	RemoveAvatar(ctx context.Context, userID string) (*dauth.UserResponse, error)
}

type Deps struct {
	Users      repository.UserRepository
	Hasher     service.PasswordHasher
	Files      *file.Service
	Cache      cache.Cache
	Tokens     service.TokenService
	TokenTTL   time.Duration
	RefreshTTL time.Duration
	Logger     *zap.Logger
}

type Service struct {
	*account.Account
	*password.Password
	*avatar.Avatar
}

func NewService(deps Deps) *Service {
	c := core.New(deps.Users, deps.Hasher, deps.Files, deps.Cache, deps.Tokens, deps.TokenTTL, deps.RefreshTTL, deps.Logger)
	return &Service{
		Account:  account.New(c),
		Password: password.New(c),
		Avatar:   avatar.New(c),
	}
}

var _ UserCase = (*Service)(nil)
