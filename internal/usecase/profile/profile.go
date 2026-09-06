package profile

import (
	"context"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/tandem/tandem/internal/domain/models"
	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/service"
	"github.com/tandem/tandem/internal/http/dto"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
	"github.com/tandem/tandem/internal/usecase/cacheutil"
	file "github.com/tandem/tandem/internal/usecase/file"
)

const (
	maxDisplayNameLen = 50
	maxBioLen         = 400
)

const avatarNamespace = "avatars"

type UserCase interface {
	Get(ctx context.Context, userID string) (*dto.UserResponse, error)
	UpdateProfile(ctx context.Context, userID string, req dto.UpdateProfileRequest) (*dto.UserResponse, error)
	ChangePassword(ctx context.Context, userID string, req dto.ChangePasswordRequest) error
	UploadAvatar(ctx context.Context, userID, filename, contentType string, reader io.Reader, size int64) (*dto.UserResponse, error)
}

type Service struct {
	users  repository.UserRepository
	hasher service.PasswordHasher
	files  *file.Service
	cache  cache.Cache
}

func NewService(users repository.UserRepository, hasher service.PasswordHasher, files *file.Service, cache cache.Cache) *Service {
	return &Service{users: users, hasher: hasher, files: files, cache: cache}
}

func (s *Service) Get(ctx context.Context, userID string) (*dto.UserResponse, error) {
	uver := cacheutil.Version(ctx, s.cache, cacheutil.UVerKey+userID)
	profileKey := fmt.Sprintf("u:%s:t:v1:profile:%s", userID, uver)
	var cached dto.UserResponse
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

func (s *Service) UpdateProfile(ctx context.Context, userID string, req dto.UpdateProfileRequest) (*dto.UserResponse, error) {
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

func (s *Service) ChangePassword(ctx context.Context, userID string, req dto.ChangePasswordRequest) error {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if !s.hasher.Check(user.PasswordHash, req.CurrentPassword) {
		return pkgerrors.NewValidationError("current password is incorrect")
	}
	if err := validate.Password(req.NewPassword); err != nil {
		return err
	}
	hash, err := s.hasher.Hash(req.NewPassword)
	if err != nil {
		return pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	user.PasswordHash = hash
	if err := s.users.Update(ctx, user); err != nil {
		return err
	}
	return nil
}

func (s *Service) UploadAvatar(ctx context.Context, userID, filename, contentType string, reader io.Reader, size int64) (*dto.UserResponse, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	key, err := s.files.UploadImage(ctx, userID, avatarNamespace, filename, contentType, reader, size)
	if err != nil {
		return nil, err
	}
	if user.AvatarKey != "" {
		_ = s.files.Remove(ctx, user.AvatarKey)
	}
	user.AvatarKey = key
	if err := s.users.Update(ctx, user); err != nil {
		return nil, err
	}
	s.bumpUser(ctx, userID)
	return toUserResponse(user), nil
}

func toUserResponse(u *models.User) *dto.UserResponse {
	return &dto.UserResponse{
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
