package users

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	muser "github.com/tandem/tandem/internal/domain/models/user"
	dadmin "github.com/tandem/tandem/internal/http/dto/admin"
	dauth "github.com/tandem/tandem/internal/http/dto/auth"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
	"github.com/tandem/tandem/internal/usecase/admin/core"
	cacheutil "github.com/tandem/tandem/internal/usecase/shared/cache"
)

type Users struct {
	*core.Core
}

func New(c *core.Core) *Users {
	return &Users{Core: c}
}

func (s *Users) CreateUser(ctx context.Context, actor core.Actor, req dadmin.CreateUserRequest) (*dauth.UserResponse, error) {
	login := validate.NormalizeLogin(req.Login)
	if err := validate.Login(login); err != nil {
		return nil, err
	}
	if err := validate.Password(req.Password); err != nil {
		return nil, err
	}

	role := muser.RoleUser
	switch req.Role {
	case "":
	case muser.RoleUser:
	case muser.RoleModerator:
		if actor.Role != muser.RoleAdmin {
			return nil, pkgerrors.ErrForbidden
		}
		role = muser.RoleModerator
	default:
		return nil, pkgerrors.NewValidationError("invalid role")
	}

	exists, err := s.Users.ExistsByLogin(ctx, login)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, pkgerrors.ErrConflict
	}

	passwordHash, err := s.Hasher.Hash(req.Password)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}

	displayName := req.DisplayName
	if displayName == "" {
		displayName = login
	}

	user := &muser.User{
		ID:           uuid.New().String(),
		Login:        login,
		PasswordHash: passwordHash,
		Role:         role,
		DisplayName:  displayName,
	}
	if err := s.Users.Create(ctx, user); err != nil {
		return nil, err
	}
	s.BumpUsers(ctx)
	return s.UserResponse(user), nil
}

func (s *Users) ListUsers(ctx context.Context, actorID string, query dadmin.AdminListQuery) (dadmin.AdminPage, error) {
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

	usersver := cacheutil.Version(ctx, s.Cache, cacheutil.UsersVerKey)
	searchKey := cacheutil.QueryHash("q", search)

	totalKey := fmt.Sprintf("adminusers:total:v2:%s:%s", usersver, searchKey)
	var total int
	if !cacheutil.Load(ctx, s.Cache, totalKey, &total) {
		count, err := s.Users.Count(ctx, search)
		if err != nil {
			return dadmin.AdminPage{}, err
		}
		total = count
		cacheutil.Store(ctx, s.Cache, totalKey, total, cacheutil.TTL)
	}

	listKey := fmt.Sprintf("adminusers:list:v2:%s:%s:%d:%d", usersver, searchKey, page, pageSize)
	var items []dauth.UserResponse
	if !cacheutil.Load(ctx, s.Cache, listKey, &items) {
		users, err := s.Users.ListPage(ctx, search, pageSize, (page-1)*pageSize)
		if err != nil {
			return dadmin.AdminPage{}, err
		}
		items = make([]dauth.UserResponse, 0, len(users))
		for _, u := range users {
			items = append(items, *s.UserResponse(u))
		}
		cacheutil.Store(ctx, s.Cache, listKey, items, cacheutil.TTL)
	}
	return dadmin.AdminPage{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}
