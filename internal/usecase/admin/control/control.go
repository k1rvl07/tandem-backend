package control

import (
	"context"

	muser "github.com/tandem/tandem/internal/domain/models/user"
	dadmin "github.com/tandem/tandem/internal/http/dto/admin"
	dauth "github.com/tandem/tandem/internal/http/dto/auth"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
	"github.com/tandem/tandem/internal/usecase/admin/core"
)

type Control struct {
	*core.Core
}

func New(c *core.Core) *Control {
	return &Control{Core: c}
}

func (s *Control) UpdateUserRole(ctx context.Context, actor core.Actor, targetID string, req dadmin.UpdateUserRoleRequest) (*dauth.UserResponse, error) {
	if err := validate.UUID(targetID); err != nil {
		return nil, err
	}
	if actor.Role != muser.RoleAdmin {
		return nil, pkgerrors.ErrForbidden
	}
	role := muser.RoleUser
	switch req.Role {
	case muser.RoleUser:
	case muser.RoleModerator:
		role = muser.RoleModerator
	default:
		return nil, pkgerrors.NewValidationError("invalid role")
	}
	target, err := s.Users.FindByID(ctx, targetID)
	if err != nil {
		return nil, err
	}
	if target.Role == muser.RoleAdmin {
		return nil, pkgerrors.ErrForbidden
	}
	if target.ID == actor.ID {
		return nil, pkgerrors.ErrForbidden
	}
	target.Role = role
	if err := s.Users.Update(ctx, target); err != nil {
		return nil, err
	}
	s.BumpUsers(ctx)
	return s.UserResponse(target), nil
}

func (s *Control) DeleteUser(ctx context.Context, actor core.Actor, targetID string) error {
	if err := validate.UUID(targetID); err != nil {
		return err
	}
	target, err := s.Users.FindByID(ctx, targetID)
	if err != nil {
		return err
	}
	if target.ID == actor.ID {
		return pkgerrors.ErrForbidden
	}
	if target.Role == muser.RoleAdmin {
		return pkgerrors.ErrForbidden
	}
	if actor.Role == muser.RoleModerator && target.Role != muser.RoleUser {
		return pkgerrors.ErrForbidden
	}
	s.Files.RemoveMany(ctx, []string{target.AvatarKey})
	if err := s.Workspaces.DeleteMembersByUser(ctx, targetID); err != nil {
		return err
	}
	if err := s.Favorites.DeleteUserFavorites(ctx, targetID); err != nil {
		return err
	}
	if err := s.Users.Delete(ctx, targetID); err != nil {
		return err
	}
	_ = s.Tokens.Revoke(ctx, targetID)
	s.BumpUsers(ctx)
	return nil
}
