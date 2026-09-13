package auth

import (
	"context"
	"time"

	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/service"
	dauth "github.com/tandem/tandem/internal/http/dto/auth"
	"github.com/tandem/tandem/internal/usecase/auth/session"
)

type UseCase interface {
	Login(ctx context.Context, req dauth.LoginRequest) (*dauth.LoginResponse, error)
	Refresh(ctx context.Context, refreshToken string) (*dauth.RefreshResponse, error)
	Logout(ctx context.Context, userID string) error
}

type Deps struct {
	Users      repository.UserRepository
	Tokens     service.TokenService
	Hasher     service.PasswordHasher
	TokenTTL   time.Duration
	RefreshTTL time.Duration
}

type Service struct {
	*session.Session
}

func NewService(deps Deps) *Service {
	return &Service{Session: session.New(deps.Users, deps.Tokens, deps.Hasher, deps.TokenTTL, deps.RefreshTTL)}
}

var _ UseCase = (*Service)(nil)
