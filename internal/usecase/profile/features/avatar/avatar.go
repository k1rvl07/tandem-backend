package avatar

import (
	"context"
	"io"

	dauth "github.com/tandem/tandem/internal/http/dto/auth"
	"github.com/tandem/tandem/internal/usecase/profile/core"
)

const avatarNamespace = "avatars"

type Avatar struct {
	*core.Core
}

func New(c *core.Core) *Avatar {
	return &Avatar{Core: c}
}

func (s *Avatar) UploadAvatar(ctx context.Context, userID, filename, contentType string, reader io.Reader, size int64) (*dauth.UserResponse, error) {
	user, err := s.Users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	key, err := s.Files.UploadImage(ctx, userID, avatarNamespace, filename, contentType, reader, size)
	if err != nil {
		return nil, err
	}
	oldKey := user.AvatarKey
	user.AvatarKey = key
	if err := s.Users.Update(ctx, user); err != nil {
		s.Files.RemoveMany(ctx, []string{key})
		return nil, err
	}
	if oldKey != "" {
		s.Files.RemoveMany(ctx, []string{oldKey})
	}
	s.BumpUser(ctx, userID)
	return s.UserResponse(user), nil
}

func (s *Avatar) RemoveAvatar(ctx context.Context, userID string) (*dauth.UserResponse, error) {
	user, err := s.Users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user.AvatarKey == "" {
		return s.UserResponse(user), nil
	}
	oldKey := user.AvatarKey
	user.AvatarKey = ""
	if err := s.Users.Update(ctx, user); err != nil {
		return nil, err
	}
	s.Files.RemoveMany(ctx, []string{oldKey})
	s.BumpUser(ctx, userID)
	return s.UserResponse(user), nil
}
