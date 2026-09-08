package auth

import (
	"context"
	"time"

	muser "github.com/tandem/tandem/internal/domain/models/user"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/service"
	dauth "github.com/tandem/tandem/internal/http/dto/auth"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
)

type UseCase interface {
	Login(ctx context.Context, req dauth.LoginRequest) (*dauth.LoginResponse, error)
	Logout(ctx context.Context, userID string) error
}

type Service struct {
	users    repository.UserRepository
	tokens   service.TokenService
	hasher   service.PasswordHasher
	tokenTTL time.Duration
}

func NewService(users repository.UserRepository, tokens service.TokenService, hasher service.PasswordHasher, tokenTTL time.Duration) *Service {
	return &Service{users: users, tokens: tokens, hasher: hasher, tokenTTL: tokenTTL}
}

func (s *Service) Login(ctx context.Context, req dauth.LoginRequest) (*dauth.LoginResponse, error) {
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

	return &dauth.LoginResponse{Token: token, User: toUserResponse(user)}, nil
}

func (s *Service) Logout(ctx context.Context, userID string) error {
	return s.tokens.Revoke(ctx, userID)
}

func toUserResponse(u *muser.User) dauth.UserResponse {
	return dauth.UserResponse{
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

var _ UseCase = (*Service)(nil)
