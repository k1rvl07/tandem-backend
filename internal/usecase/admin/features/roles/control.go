package roles

import (
	"context"

	dadmin "github.com/tandem/tandem/internal/contracts/admin"
	dauth "github.com/tandem/tandem/internal/contracts/auth"
	muser "github.com/tandem/tandem/internal/domain/models/user"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
	"github.com/tandem/tandem/internal/usecase/admin/core"
	"go.uber.org/zap"
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
	s.BumpUser(ctx, target.ID)
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
	memberships, err := s.Workspaces.ListWorkspacesForUser(ctx, targetID)
	if err != nil {
		return err
	}
	if err := s.reassignOwnedWorkspaces(ctx, memberships, targetID); err != nil {
		return err
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
	if err := s.Tokens.Revoke(ctx, targetID); err != nil {
		s.Logger.Warn("revoke tokens after user delete failed", zap.String("user_id", targetID), zap.Error(err))
	}
	for _, membership := range memberships {
		s.BumpWorkspace(ctx, membership.Workspace.ID)
	}
	s.BumpUsers(ctx)
	return nil
}

func (s *Control) reassignOwnedWorkspaces(ctx context.Context, memberships []mworkspace.WorkspaceMembership, targetID string) error {
	for i := range memberships {
		if memberships[i].Role != mworkspace.RoleOwner {
			continue
		}
		wsMembers, err := s.Workspaces.ListMembers(ctx, memberships[i].Workspace.ID)
		if err != nil {
			return err
		}
		if countOwners(wsMembers) > 1 {
			continue
		}
		successor := pickSuccessor(wsMembers, targetID)
		if successor == nil {
			return pkgerrors.NewValidationError("cannot delete the last owner of workspace %s", memberships[i].Workspace.ID)
		}
		if err := s.Workspaces.TransferOwnership(ctx, memberships[i].Workspace.ID, targetID, successor.UserID); err != nil {
			return err
		}
	}
	return nil
}

func countOwners(members []mworkspace.WorkspaceMember) int {
	count := 0
	for i := range members {
		if members[i].Role == mworkspace.RoleOwner {
			count++
		}
	}
	return count
}

func pickSuccessor(members []mworkspace.WorkspaceMember, excludeID string) *mworkspace.WorkspaceMember {
	var best *mworkspace.WorkspaceMember
	for i := range members {
		m := &members[i]
		if m.UserID == excludeID {
			continue
		}
		if best == nil || m.CreatedAt.Before(best.CreatedAt) ||
			(m.CreatedAt.Equal(best.CreatedAt) && roleRank(m.Role) > roleRank(best.Role)) {
			best = m
		}
	}
	return best
}

func roleRank(role mworkspace.WorkspaceRole) int {
	switch role {
	case mworkspace.RoleOwner:
		return 3
	case mworkspace.RoleEditor:
		return 2
	default:
		return 1
	}
}
