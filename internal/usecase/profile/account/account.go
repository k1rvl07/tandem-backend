package account

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	dauth "github.com/tandem/tandem/internal/http/dto/auth"
	dprofile "github.com/tandem/tandem/internal/http/dto/profile"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/usecase/profile/core"
	cacheutil "github.com/tandem/tandem/internal/usecase/shared/cache"
)

const (
	maxDisplayNameLen = 50
	maxBioLen         = 400
)

type Account struct {
	*core.Core
}

func New(c *core.Core) *Account {
	return &Account{Core: c}
}

func (s *Account) Get(ctx context.Context, userID string) (*dauth.UserResponse, error) {
	uver := cacheutil.Version(ctx, s.Cache, cacheutil.UVerKey+userID)
	profileKey := fmt.Sprintf("u:%s:t:v1:profile:%s", userID, uver)
	var cached dauth.UserResponse
	if cacheutil.Load(ctx, s.Cache, profileKey, &cached) {
		return &cached, nil
	}
	user, err := s.Users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	resp := s.UserResponse(user)
	cacheutil.Store(ctx, s.Cache, profileKey, resp, cacheutil.TTL)
	return resp, nil
}

func (s *Account) UpdateProfile(ctx context.Context, userID string, req dprofile.UpdateProfileRequest) (*dauth.UserResponse, error) {
	user, err := s.Users.FindByID(ctx, userID)
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
	if err := s.Users.Update(ctx, user); err != nil {
		return nil, err
	}
	s.BumpUser(ctx, userID)
	return s.UserResponse(user), nil
}
