package admin

import (
	"context"

	"github.com/google/uuid"
	"github.com/tandem/tandem/internal/domain/models"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/service"
	"github.com/tandem/tandem/internal/http/dto"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
)

type Actor struct {
	ID   string
	Role string
}

type UseCase interface {
	CreateUser(ctx context.Context, actor Actor, req dto.CreateUserRequest) (*dto.UserResponse, error)
	ListUsers(ctx context.Context) ([]dto.UserResponse, error)
	DeleteUser(ctx context.Context, actor Actor, targetID string) error
}

type Service struct {
	users  repository.UserRepository
	hasher service.PasswordHasher
}

func NewService(users repository.UserRepository, hasher service.PasswordHasher) *Service {
	return &Service{users: users, hasher: hasher}
}

func (s *Service) CreateUser(ctx context.Context, actor Actor, req dto.CreateUserRequest) (*dto.UserResponse, error) {
	login := validate.NormalizeLogin(req.Login)
	if err := validate.Login(login); err != nil {
		return nil, err
	}
	if err := validate.Password(req.Password); err != nil {
		return nil, err
	}

	role := models.RoleUser
	switch req.Role {
	case "":
	case models.RoleUser:
	case models.RoleModerator:
		if actor.Role != models.RoleAdmin {
			return nil, pkgerrors.ErrForbidden
		}
		role = models.RoleModerator
	default:
		return nil, pkgerrors.NewValidationError("invalid role")
	}

	exists, err := s.users.ExistsByLogin(ctx, login)
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

	displayName := req.DisplayName
	if displayName == "" {
		displayName = login
	}

	user := &models.User{
		ID:           uuid.New().String(),
		Login:        login,
		PasswordHash: passwordHash,
		Role:         role,
		DisplayName:  displayName,
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}
	return &dto.UserResponse{
		ID:          user.ID,
		Login:       user.Login,
		Role:        user.Role,
		DisplayName: user.DisplayName,
		Bio:         user.Bio,
		AvatarKey:   user.AvatarKey,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
	}, nil
}

func (s *Service) ListUsers(ctx context.Context) ([]dto.UserResponse, error) {
	users, err := s.users.List(ctx)
	if err != nil {
		return nil, err
	}
	responses := make([]dto.UserResponse, 0, len(users))
	for _, u := range users {
		responses = append(responses, dto.UserResponse{
			ID:          u.ID,
			Login:       u.Login,
			Role:        u.Role,
			DisplayName: u.DisplayName,
			Bio:         u.Bio,
			AvatarKey:   u.AvatarKey,
			CreatedAt:   u.CreatedAt,
			UpdatedAt:   u.UpdatedAt,
		})
	}
	return responses, nil
}

func (s *Service) DeleteUser(ctx context.Context, actor Actor, targetID string) error {
	if err := validate.UUID(targetID); err != nil {
		return err
	}
	target, err := s.users.FindByID(ctx, targetID)
	if err != nil {
		return err
	}
	if target.ID == actor.ID {
		return pkgerrors.ErrForbidden
	}
	if target.Role == models.RoleAdmin {
		return pkgerrors.ErrForbidden
	}
	if actor.Role == models.RoleModerator && target.Role != models.RoleUser {
		return pkgerrors.ErrForbidden
	}
	return s.users.Delete(ctx, targetID)
}

var _ UseCase = (*Service)(nil)
