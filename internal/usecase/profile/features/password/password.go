package password

import (
	"context"

	dprofile "github.com/tandem/tandem/internal/http/dto/profile"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
	"github.com/tandem/tandem/internal/usecase/profile/core"
)

type Password struct {
	*core.Core
}

func New(c *core.Core) *Password {
	return &Password{Core: c}
}

func (s *Password) ChangePassword(ctx context.Context, userID string, req dprofile.ChangePasswordRequest) (*dprofile.ChangePasswordResponse, error) {
	user, err := s.Users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !s.Hasher.Check(user.PasswordHash, req.CurrentPassword) {
		return nil, pkgerrors.NewValidationError("current password is incorrect")
	}
	if err := validate.Password(req.NewPassword); err != nil {
		return nil, err
	}
	hash, err := s.Hasher.Hash(req.NewPassword)
	if err != nil {
		return nil, pkgerrors.Wrap(pkgerrors.ErrInternal, err)
	}
	user.PasswordHash = hash
	if err := s.Users.Update(ctx, user); err != nil {
		return nil, err
	}
	if err := s.Tokens.Revoke(ctx, userID); err != nil {
		return nil, err
	}
	s.BumpUser(ctx, userID)
	access, err := s.Tokens.Generate(userID, s.TokenTTL)
	if err != nil {
		return nil, err
	}
	refresh, err := s.Tokens.GenerateRefresh(userID, s.RefreshTTL)
	if err != nil {
		return nil, err
	}
	return &dprofile.ChangePasswordResponse{Token: access, RefreshToken: refresh}, nil
}
