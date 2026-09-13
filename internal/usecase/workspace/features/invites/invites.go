package invites

import (
	"context"
	"time"

	"github.com/google/uuid"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	dworkspace "github.com/tandem/tandem/internal/http/dto/workspace"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
	"github.com/tandem/tandem/internal/usecase/shared/errutil"
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
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	member, err := s.MemberOf(ctx, actorID, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.RequireEditor(member.Role); err != nil {
		return nil, err
	}
	ws, err := s.Workspaces.FindWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if ws.InviteToken == "" || ws.InviteExpiresAt == nil || ws.InviteExpiresAt.Before(time.Now()) {
		ws.InviteToken = uuid.New().String()
		expiresAt := time.Now().Add(InviteTTL)
		ws.InviteExpiresAt = &expiresAt
		if err := s.Workspaces.UpdateInvite(ctx, ws.ID, ws.InviteToken, ws.InviteExpiresAt); err != nil {
			return nil, err
		}
	}
	return &dworkspace.WorkspaceInviteResponse{InviteToken: &ws.InviteToken, ExpiresAt: ws.InviteExpiresAt}, nil
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
	member, findErr := s.Workspaces.FindMember(ctx, ws.ID, actorID)
	if findErr != nil {
		if !errutil.IsNotFound(findErr) {
			return nil, findErr
		}
		member = nil
	}
	role := mworkspace.RoleMember
	added := false
	if member != nil {
		role = member.Role
	} else {
		if err := s.Workspaces.AddMember(ctx, ws.ID, actorID, mworkspace.RoleMember); err != nil {
			return nil, err
		}
		added = true
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
		s.BroadcastUpdated(ctx, ws.ID, role)
	}
	if err := s.Workspaces.UpdateInvite(ctx, ws.ID, "", nil); err != nil {
		s.Logger.Warn("invalidate invite failed", zap.String("workspace_id", ws.ID), zap.Error(err))
	}
	return resp, nil
}
