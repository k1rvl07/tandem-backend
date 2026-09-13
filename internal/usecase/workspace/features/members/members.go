package members

import (
	"context"

	muser "github.com/tandem/tandem/internal/domain/models/user"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	dworkspace "github.com/tandem/tandem/internal/http/dto/workspace"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
	"github.com/tandem/tandem/internal/usecase/shared/errutil"
	"github.com/tandem/tandem/internal/usecase/workspace/core"
)

type Members struct {
	*core.Core
}

func New(c *core.Core) *Members {
	return &Members{Core: c}
}

func (s *Members) AddMember(ctx context.Context, actorID, workspaceID string, req dworkspace.AddMemberRequest) (*dworkspace.WorkspaceMemberResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	member, err := s.MemberOf(ctx, actorID, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.RequireOwner(member.Role); err != nil {
		return nil, err
	}

	role, err := normalizeMemberRole(mworkspace.WorkspaceRole(req.Role))
	if err != nil {
		return nil, err
	}

	login := validate.NormalizeLogin(req.Login)
	if err := validate.Login(login); err != nil {
		return nil, err
	}
	target, err := s.findUserForAdd(ctx, workspaceID, login)
	if err != nil {
		return nil, err
	}
	if err := s.Workspaces.AddMember(ctx, workspaceID, target.ID, role); err != nil {
		return nil, err
	}
	s.BumpWorkspace(ctx, workspaceID)
	s.BumpUser(ctx, target.ID)
	s.BroadcastUpdated(ctx, workspaceID, member.Role)
	return s.MemberResponse(target, role), nil
}

func (s *Members) findUserForAdd(ctx context.Context, workspaceID, login string) (*muser.User, error) {
	target, err := s.Users.FindByLogin(ctx, login)
	if err != nil {
		return nil, err
	}
	if _, err := s.Workspaces.FindMember(ctx, workspaceID, target.ID); err == nil {
		return nil, pkgerrors.ErrConflict
	} else if !errutil.IsNotFound(err) {
		return nil, err
	}
	return target, nil
}

func normalizeMemberRole(role mworkspace.WorkspaceRole) (mworkspace.WorkspaceRole, error) {
	normalized := mworkspace.RoleMember
	switch role {
	case "":
	case mworkspace.RoleEditor:
		normalized = mworkspace.RoleEditor
	case mworkspace.RoleMember:
		normalized = mworkspace.RoleMember
	case mworkspace.RoleOwner:
		return mworkspace.RoleMember, pkgerrors.NewValidationError("owner must be assigned via ownership transfer")
	default:
		return mworkspace.RoleMember, pkgerrors.NewValidationError("invalid role")
	}
	return normalized, nil
}

func (s *Members) UpdateRole(ctx context.Context, actorID, workspaceID, targetID string, req dworkspace.UpdateMemberRoleRequest) (*dworkspace.WorkspaceMemberResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	if err := validate.UUID(targetID); err != nil {
		return nil, err
	}
	member, err := s.MemberOf(ctx, actorID, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.RequireOwner(member.Role); err != nil {
		return nil, err
	}
	role := mworkspace.WorkspaceRole(req.Role)
	if role != mworkspace.RoleEditor && role != mworkspace.RoleMember {
		return nil, pkgerrors.NewValidationError("role must be editor or member")
	}
	target, err := s.Workspaces.FindMember(ctx, workspaceID, targetID)
	if err != nil {
		return nil, err
	}
	if target.Role == mworkspace.RoleOwner {
		return nil, pkgerrors.NewValidationError("cannot change the owner's role; transfer ownership instead")
	}
	if err := s.Workspaces.UpdateMemberRole(ctx, workspaceID, targetID, role); err != nil {
		return nil, err
	}
	user, err := s.Users.FindByID(ctx, targetID)
	if err != nil {
		return nil, err
	}
	s.BumpWorkspace(ctx, workspaceID)
	s.BumpUser(ctx, targetID)
	s.BroadcastUpdated(ctx, workspaceID, member.Role)
	return s.MemberResponse(user, role), nil
}

func (s *Members) RemoveMember(ctx context.Context, actorID, workspaceID, targetID string) error {
	if err := validate.UUID(workspaceID); err != nil {
		return err
	}
	if err := validate.UUID(targetID); err != nil {
		return err
	}
	member, err := s.MemberOf(ctx, actorID, workspaceID)
	if err != nil {
		return err
	}
	if err := s.RequireOwner(member.Role); err != nil {
		return err
	}
	target, err := s.Workspaces.FindMember(ctx, workspaceID, targetID)
	if err != nil {
		return err
	}
	if target.Role == mworkspace.RoleOwner {
		return pkgerrors.NewValidationError("transfer ownership before removing the owner")
	}
	if err := s.Workspaces.RemoveMember(ctx, workspaceID, targetID); err != nil {
		return err
	}
	s.BumpWorkspace(ctx, workspaceID)
	s.BumpUser(ctx, targetID)
	s.BroadcastUpdated(ctx, workspaceID, member.Role)
	s.Hub.BroadcastToRoom(s.Room(workspaceID), &ws.Message{
		Type: core.EventMemberKicked,
		Data: map[string]string{"workspace_id": workspaceID, "user_id": targetID},
	})
	return nil
}

func (s *Members) TransferOwner(ctx context.Context, actorID, workspaceID string, req dworkspace.TransferOwnerRequest) error {
	if err := validate.UUID(workspaceID); err != nil {
		return err
	}
	if err := validate.UUID(req.UserID); err != nil {
		return err
	}
	member, err := s.MemberOf(ctx, actorID, workspaceID)
	if err != nil {
		return err
	}
	if err := s.RequireOwner(member.Role); err != nil {
		return err
	}
	if req.UserID == actorID {
		return pkgerrors.NewValidationError("already the owner")
	}
	target, err := s.Workspaces.FindMember(ctx, workspaceID, req.UserID)
	if err != nil {
		return err
	}
	if err := s.Workspaces.TransferOwnership(ctx, workspaceID, actorID, target.UserID); err != nil {
		return err
	}
	s.BumpWorkspace(ctx, workspaceID)
	s.BumpUser(ctx, target.UserID)
	s.BumpUser(ctx, actorID)
	s.BroadcastUpdated(ctx, workspaceID, mworkspace.RoleEditor)
	return nil
}
