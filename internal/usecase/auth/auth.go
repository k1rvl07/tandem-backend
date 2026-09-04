package auth

import (
	"context"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/tandem/tandem/internal/domain/models"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/service"
	"github.com/tandem/tandem/internal/http/dto"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
)

const (
	minPasswordLen = 8
	maxPasswordLen = 72
	maxEmailLen    = 255
)

var emailRegexp = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

type UseCase interface {
	Register(ctx context.Context, req dto.RegisterRequest) (*dto.RegisterResponse, error)
	Login(ctx context.Context, req dto.LoginRequest) (*dto.LoginResponse, error)
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

func (s *Service) Register(ctx context.Context, req dto.RegisterRequest) (*dto.RegisterResponse, error) {
	if err := validateRegister(req); err != nil {
		return nil, err
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	exists, err := s.users.ExistsByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, pkgerrors.ErrConflict
	}

	passwordHash, err := s.hasher.Hash(req.Password)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}

	user := &models.User{
		ID:           uuid.New().String(),
		Email:        email,
		PasswordHash: passwordHash,
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}

	return &dto.RegisterResponse{User: toUserResponse(user)}, nil
}

func (s *Service) Login(ctx context.Context, req dto.LoginRequest) (*dto.LoginResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		return nil, pkgerrors.NewValidationError("email is required")
	}
	if req.Password == "" {
		return nil, pkgerrors.NewValidationError("password is required")
	}

	user, err := s.users.FindByEmail(ctx, email)
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

	return &dto.LoginResponse{Token: token, User: toUserResponse(user)}, nil
}

func validateRegister(req dto.RegisterRequest) error {
	email := strings.TrimSpace(req.Email)
	if email == "" {
		return pkgerrors.NewValidationError("email is required")
	}
	if utf8.RuneCountInString(email) > maxEmailLen {
		return pkgerrors.NewValidationError("email is too long")
	}
	if !emailRegexp.MatchString(email) {
		return pkgerrors.NewValidationError("invalid email format")
	}

	if err := validatePassword(req.Password); err != nil {
		return err
	}
	if req.ConfirmPassword != req.Password {
		return pkgerrors.NewValidationError("passwords do not match")
	}
	return nil
}

func validatePassword(password string) error {
	if password == "" {
		return pkgerrors.NewValidationError("password is required")
	}
	if len([]byte(password)) < minPasswordLen {
		return pkgerrors.NewValidationError("password must be at least %d characters", minPasswordLen)
	}
	if len([]byte(password)) > maxPasswordLen {
		return pkgerrors.NewValidationError("password must be at most %d bytes", maxPasswordLen)
	}
	return nil
}

func toUserResponse(u *models.User) dto.UserResponse {
	return dto.UserResponse{
		ID:        u.ID,
		Email:     u.Email,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

var _ UseCase = (*Service)(nil)
