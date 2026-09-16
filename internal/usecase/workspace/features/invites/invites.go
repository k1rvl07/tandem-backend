package invites

import (
	"context"
	"time"

	"github.com/google/uuid"
	dworkspace "github.com/tandem/tandem/internal/contracts/workspace"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
	"github.com/tandem/tandem/internal/usecase/workspace/core"
	"go.uber.org/zap"
)

const InviteTTL = 7 * 24 * time.Hour

type Invites struct {
	*core.Core
}

func New(c *core.Core) *Invites {
	return &Invites{Core: c}
}

func (s *Invites) GetInvite(ctx context.Context, actorID, workspaceID string) (*dworkspace.WorkspaceInviteResponse, error) {
	if err := s.requireInviteAccess(ctx, actorID, workspaceID); err != nil {
		return nil, err
	}
	ws, err := s.Workspaces.FindWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if ws.InviteToken == "" {
		return &dworkspace.WorkspaceInviteResponse{ExpiresAt: ws.InviteExpiresAt}, nil
	}
	token := ws.InviteToken
	return &dworkspace.WorkspaceInviteResponse{InviteToken: &token, ExpiresAt: ws.InviteExpiresAt}, nil
}

func (s *Invites) GenerateInvite(ctx context.Context, actorID, workspaceID string) (*dworkspace.WorkspaceInviteResponse, error) {
	if err := s.requireInviteAccess(ctx, actorID, workspaceID); err != nil {
		return nil, err
	}
	ws, err := s.Workspaces.FindWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	ws.InviteToken = uuid.New().String()
	expiresAt := time.Now().Add(InviteTTL)
	ws.InviteExpiresAt = &expiresAt
	if err := s.Workspaces.UpdateInvite(ctx, ws.ID, ws.InviteToken, ws.InviteExpiresAt); err != nil {
		return nil, err
	}
	token := ws.InviteToken
	return &dworkspace.WorkspaceInviteResponse{InviteToken: &token, ExpiresAt: ws.InviteExpiresAt}, nil
}

func (s *Invites) requireInviteAccess(ctx context.Context, actorID, workspaceID string) error {
	if err := validate.UUID(workspaceID); err != nil {
		return err
	}
	member, err := s.MemberOf(ctx, actorID, workspaceID)
	if err != nil {
		return err
	}
	return s.RequireEditor(member.Role)
}

func (s *Invites) DisableInvite(ctx context.Context, actorID, workspaceID string) error {
	if err := validate.UUID(workspaceID); err != nil {
		return err
	}
	member, err := s.MemberOf(ctx, actorID, workspaceID)
	if err != nil {
		return err
	}
	if err := s.RequireEditor(member.Role); err != nil {
		return err
	}
	if err := s.Workspaces.UpdateInvite(ctx, workspaceID, "", nil); err != nil {
		return err
	}
	return nil
}

func (s *Invites) JoinByInvite(ctx context.Context, actorID, token string) (*dworkspace.WorkspaceResponse, error) {
	if token == "" {
		return nil, pkgerrors.NewValidationError("invite token is required")
	}
	ws, err := s.Workspaces.FindWorkspaceByInvite(ctx, token)
	if err != nil {
		return nil, err
	}
	if ws.InviteExpiresAt != nil && ws.InviteExpiresAt.Before(time.Now()) {
		if err := s.Workspaces.UpdateInvite(ctx, ws.ID, "", nil); err != nil {
			s.Logger.Warn("clear expired invite failed", zap.String("workspace_id", ws.ID), zap.Error(err))
		}
		return nil, pkgerrors.NewValidationError("invite link has expired")
	}
	claimed, added, err := s.Workspaces.ClaimInviteAndAddMember(ctx, ws.ID, token, actorID, mworkspace.RoleMember)
	if err != nil {
		return nil, err
	}
	if !claimed {
		return nil, pkgerrors.NewValidationError("invite link has expired")
	}
	role := mworkspace.RoleMember
	if !added {
		member, findErr := s.Workspaces.FindMember(ctx, ws.ID, actorID)
		if findErr != nil {
			return nil, findErr
		}
		role = member.Role
	}
	owner, err := s.ResolveOwner(ctx, ws.ID)
	if err != nil {
		return nil, err
	}
	resp := s.WorkspaceResponse(ws, role, false)
	resp.Owner = owner
	if added {
		s.BumpWorkspace(ctx, ws.ID)
		s.BumpUser(ctx, actorID)
		s.BroadcastUpdated(ctx, ws.ID)
	}
	return resp, nil
}
