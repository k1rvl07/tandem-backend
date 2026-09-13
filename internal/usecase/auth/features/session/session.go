package session

import (
	"context"
	"time"

	dauth "github.com/tandem/tandem/internal/contracts/auth"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/service"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
	"github.com/tandem/tandem/internal/usecase/shared/base"
	"github.com/tandem/tandem/internal/usecase/shared/mapping"
)

type Session struct {
	users      repository.UserRepository
	tokens     service.TokenService
	hasher     service.PasswordHasher
	tokenTTL   time.Duration
	refreshTTL time.Duration
}

func New(d base.Deps) *Session {
	return &Session{users: d.Users, tokens: d.Tokens, hasher: d.Hasher, tokenTTL: d.TokenTTL, refreshTTL: d.RefreshTTL}
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

	return &dauth.LoginResponse{Token: token, RefreshToken: refreshToken, User: *mapping.UserResponse(user)}, nil
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
