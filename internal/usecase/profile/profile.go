package profile

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	muser "github.com/tandem/tandem/internal/domain/models/user"
	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/service"
	dauth "github.com/tandem/tandem/internal/http/dto/auth"
	dprofile "github.com/tandem/tandem/internal/http/dto/profile"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
	file "github.com/tandem/tandem/internal/usecase/file"
	cacheutil "github.com/tandem/tandem/internal/usecase/shared/cache"
	"go.uber.org/zap"
)

const (
	maxDisplayNameLen = 50
	maxBioLen         = 400
)

const avatarNamespace = "avatars"

type UserCase interface {
	Get(ctx context.Context, userID string) (*dauth.UserResponse, error)
	UpdateProfile(ctx context.Context, userID string, req dprofile.UpdateProfileRequest) (*dauth.UserResponse, error)
	ChangePassword(ctx context.Context, userID string, req dprofile.ChangePasswordRequest) (*dprofile.ChangePasswordResponse, error)
	UploadAvatar(ctx context.Context, userID, filename, contentType string, reader io.Reader, size int64) (*dauth.UserResponse, error)
	RemoveAvatar(ctx context.Context, userID string) (*dauth.UserResponse, error)
}

type Service struct {
	users      repository.UserRepository
	hasher     service.PasswordHasher
	files      *file.Service
	cache      cache.Cache
	tokens     service.TokenService
	tokenTTL   time.Duration
	refreshTTL time.Duration
	logger     *zap.Logger
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

func NewService(deps Deps) *Service {
	return &Service{users: deps.Users, hasher: deps.Hasher, files: deps.Files, cache: deps.Cache, tokens: deps.Tokens, tokenTTL: deps.TokenTTL, refreshTTL: deps.RefreshTTL, logger: deps.Logger}
}

func (s *Service) Get(ctx context.Context, userID string) (*dauth.UserResponse, error) {
	uver := cacheutil.Version(ctx, s.cache, cacheutil.UVerKey+userID)
	profileKey := fmt.Sprintf("u:%s:t:v1:profile:%s", userID, uver)
	var cached dauth.UserResponse
	if cacheutil.Load(ctx, s.cache, profileKey, &cached) {
		return &cached, nil
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	resp := toUserResponse(user)
	cacheutil.Store(ctx, s.cache, profileKey, resp, cacheutil.TTL)
	return resp, nil
}

func (s *Service) bumpUser(ctx context.Context, userID string) {
	cacheutil.Bump(ctx, s.cache, cacheutil.UVerKey+userID)
}

func (s *Service) UpdateProfile(ctx context.Context, userID string, req dprofile.UpdateProfileRequest) (*dauth.UserResponse, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	displayName := strings.TrimSpace(req.DisplayName)
	bio := strings.TrimSpace(req.Bio)
	if utf8.RuneCountInString(displayName) > maxDisplayNameLen {
		return nil, pkgerrors.NewValidationError("display name is too long")
	}
	if utf8.RuneCountInString(bio) > maxBioLen {
		return nil, pkgerrors.NewValidationError("bio is too long")
	}
	user.DisplayName = displayName
	user.Bio = bio
	if err := s.users.Update(ctx, user); err != nil {
		return nil, err
	}
	s.bumpUser(ctx, userID)
	return toUserResponse(user), nil
}

func (s *Service) ChangePassword(ctx context.Context, userID string, req dprofile.ChangePasswordRequest) (*dprofile.ChangePasswordResponse, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !s.hasher.Check(user.PasswordHash, req.CurrentPassword) {
		return nil, pkgerrors.NewValidationError("current password is incorrect")
	}
	if err := validate.Password(req.NewPassword); err != nil {
		return nil, err
	}
	hash, err := s.hasher.Hash(req.NewPassword)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	user.PasswordHash = hash
	if err := s.users.Update(ctx, user); err != nil {
		return nil, err
	}
	if err := s.tokens.Revoke(ctx, userID); err != nil {
		return nil, err
	}
	s.bumpUser(ctx, userID)
	access, err := s.tokens.Generate(userID, s.tokenTTL)
	if err != nil {
		return nil, err
	}
	refresh, err := s.tokens.GenerateRefresh(userID, s.refreshTTL)
	if err != nil {
		return nil, err
	}
	return &dprofile.ChangePasswordResponse{Token: access, RefreshToken: refresh}, nil
}

func (s *Service) UploadAvatar(ctx context.Context, userID, filename, contentType string, reader io.Reader, size int64) (*dauth.UserResponse, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	key, err := s.files.UploadImage(ctx, userID, avatarNamespace, filename, contentType, reader, size)
	if err != nil {
		return nil, err
	}
	oldKey := user.AvatarKey
	user.AvatarKey = key
	if err := s.users.Update(ctx, user); err != nil {
		s.files.RemoveMany(ctx, []string{key})
		return nil, err
	}
	if oldKey != "" {
		s.files.RemoveMany(ctx, []string{oldKey})
	}
	s.bumpUser(ctx, userID)
	return toUserResponse(user), nil
}

func (s *Service) RemoveAvatar(ctx context.Context, userID string) (*dauth.UserResponse, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.AvatarKey == "" {
		return toUserResponse(user), nil
	}
	oldKey := user.AvatarKey
	user.AvatarKey = ""
	if err := s.users.Update(ctx, user); err != nil {
		return nil, err
	}
	s.files.RemoveMany(ctx, []string{oldKey})
	s.bumpUser(ctx, userID)
	return toUserResponse(user), nil
}

func toUserResponse(u *muser.User) *dauth.UserResponse {
	return &dauth.UserResponse{
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

var _ UserCase = (*Service)(nil)
