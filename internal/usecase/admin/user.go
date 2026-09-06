package admin

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tandem/tandem/internal/domain/models"
	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/service"
	"github.com/tandem/tandem/internal/http/dto"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
	"github.com/tandem/tandem/internal/usecase/cacheutil"
)

type Actor struct {
	ID   string
	Role string
}

type UseCase interface {
	CreateUser(ctx context.Context, actor Actor, req dto.CreateUserRequest) (*dto.UserResponse, error)
	ListUsers(ctx context.Context, actorID string, query dto.AdminListQuery) (dto.AdminPage, error)
	UpdateUserRole(ctx context.Context, actor Actor, targetID string, req dto.UpdateUserRoleRequest) (*dto.UserResponse, error)
	DeleteUser(ctx context.Context, actor Actor, targetID string) error
}

type Service struct {
	users  repository.UserRepository
	hasher service.PasswordHasher
	cache  cache.Cache
}

func NewService(users repository.UserRepository, hasher service.PasswordHasher, cache cache.Cache) *Service {
	return &Service{users: users, hasher: hasher, cache: cache}
}

func (s *Service) bumpUsers(ctx context.Context) {
	cacheutil.Bump(ctx, s.cache, cacheutil.UsersVerKey)
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
	s.bumpUsers(ctx)
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

func (s *Service) ListUsers(ctx context.Context, actorID string, query dto.AdminListQuery) (dto.AdminPage, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	search := strings.TrimSpace(query.Query)

	usersver := cacheutil.Version(ctx, s.cache, cacheutil.UsersVerKey)
	searchKey := cacheutil.QueryHash("q", search)

	totalKey := fmt.Sprintf("adminusers:total:v2:%s:%s", usersver, searchKey)
	var total int
	if !cacheutil.Load(ctx, s.cache, totalKey, &total) {
		count, err := s.users.Count(ctx, search)
		if err != nil {
			return dto.AdminPage{}, err
		}
		total = count
		cacheutil.Store(ctx, s.cache, totalKey, total, cacheutil.TTL)
	}

	listKey := fmt.Sprintf("adminusers:list:v2:%s:%s:%d:%d", usersver, searchKey, page, pageSize)
	var items []dto.UserResponse
	if !cacheutil.Load(ctx, s.cache, listKey, &items) {
		users, err := s.users.ListPage(ctx, search, pageSize, (page-1)*pageSize)
		if err != nil {
			return dto.AdminPage{}, err
		}
		items = make([]dto.UserResponse, 0, len(users))
		for _, u := range users {
			items = append(items, dto.UserResponse{
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
		cacheutil.Store(ctx, s.cache, listKey, items, cacheutil.TTL)
	}
	return dto.AdminPage{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *Service) UpdateUserRole(ctx context.Context, actor Actor, targetID string, req dto.UpdateUserRoleRequest) (*dto.UserResponse, error) {
	if err := validate.UUID(targetID); err != nil {
		return nil, err
	}
	if actor.Role != models.RoleAdmin {
		return nil, pkgerrors.ErrForbidden
	}
	role := models.RoleUser
	switch req.Role {
	case models.RoleUser:
	case models.RoleModerator:
		role = models.RoleModerator
	default:
		return nil, pkgerrors.NewValidationError("invalid role")
	}
	target, err := s.users.FindByID(ctx, targetID)
	if err != nil {
		return nil, err
	}
	if target.Role == models.RoleAdmin {
		return nil, pkgerrors.ErrForbidden
	}
	if target.ID == actor.ID {
		return nil, pkgerrors.ErrForbidden
	}
	target.Role = role
	if err := s.users.Update(ctx, target); err != nil {
		return nil, err
	}
	s.bumpUsers(ctx)
	return &dto.UserResponse{
		ID:          target.ID,
		Login:       target.Login,
		Role:        target.Role,
		DisplayName: target.DisplayName,
		Bio:         target.Bio,
		AvatarKey:   target.AvatarKey,
		CreatedAt:   target.CreatedAt,
		UpdatedAt:   target.UpdatedAt,
	}, nil
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
	if err := s.users.Delete(ctx, targetID); err != nil {
		return err
	}
	s.bumpUsers(ctx)
	return nil
}

var _ UseCase = (*Service)(nil)
