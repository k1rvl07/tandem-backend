package session

import (
	"context"
	"time"

	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/service"
	dauth "github.com/tandem/tandem/internal/http/dto/auth"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
	"github.com/tandem/tandem/internal/usecase/shared/taskmap"
)

type Session struct {
	users      repository.UserRepository
	tokens     service.TokenService
	hasher     service.PasswordHasher
	tokenTTL   time.Duration
	refreshTTL time.Duration
}

func New(users repository.UserRepository, tokens service.TokenService, hasher service.PasswordHasher, tokenTTL, refreshTTL time.Duration) *Session {
	return &Session{users: users, tokens: tokens, hasher: hasher, tokenTTL: tokenTTL, refreshTTL: refreshTTL}
}

func (s *Session) Login(ctx context.Context, req dauth.LoginRequest) (*dauth.LoginResponse, error) {
	login := validate.NormalizeLogin(req.Login)
	if login == "" {
		return nil, pkgerrors.NewValidationError("login is required")
	}
	if req.Password == "" {
		return nil, pkgerrors.NewValidationError("password is required")
	}

	user, err := s.users.FindByLogin(ctx, login)
	if err != nil {
		return nil, pkgerrors.ErrUnauthorized
	}
	if !s.hasher.Check(user.PasswordHash, req.Password) {
		return nil, pkgerrors.ErrUnauthorized
	}

	token, err := s.tokens.Generate(user.ID, s.tokenTTL)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	refreshToken, err := s.tokens.GenerateRefresh(user.ID, s.refreshTTL)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}

	return &dauth.LoginResponse{Token: token, RefreshToken: refreshToken, User: *taskmap.UserResponse(user)}, nil
}

func (s *Session) Refresh(ctx context.Context, refreshToken string) (*dauth.RefreshResponse, error) {
	access, nextRefresh, err := s.tokens.RotateRefresh(ctx, refreshToken, s.tokenTTL, s.refreshTTL)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrUnauthorized, err)
	}
	return &dauth.RefreshResponse{Token: access, RefreshToken: nextRefresh}, nil
}

func (s *Session) Logout(ctx context.Context, userID string) error {
	return s.tokens.Revoke(ctx, userID)
}
