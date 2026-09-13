package auth

import (
	"context"

	dauth "github.com/tandem/tandem/internal/contracts/auth"
	"github.com/tandem/tandem/internal/usecase/auth/features/session"
	"github.com/tandem/tandem/internal/usecase/shared/base"
)

type UseCase interface {
	Login(ctx context.Context, req dauth.LoginRequest) (*dauth.LoginResponse, error)
	Refresh(ctx context.Context, refreshToken string) (*dauth.RefreshResponse, error)
	Logout(ctx context.Context, userID string) error
}

type Deps struct {
	base.Deps
}

type Service struct {
	*session.Session
}

func NewService(deps Deps) *Service {
	return &Service{Session: session.New(deps.Deps)}
}

var _ UseCase = (*Service)(nil)
