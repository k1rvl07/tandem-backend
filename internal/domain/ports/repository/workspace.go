package repository

import (
	"context"
	"time"

	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
)

type WorkspaceRepository interface {
	CreateWorkspace(ctx context.Context, ws *mworkspace.Workspace) error
	FindWorkspaceByID(ctx context.Context, id string) (*mworkspace.Workspace, error)
	FindWorkspaceByInvite(ctx context.Context, token string) (*mworkspace.Workspace, error)
	UpdateWorkspace(ctx context.Context, ws *mworkspace.Workspace) error
	UpdateInvite(ctx context.Context, workspaceID, token string, expiresAt *time.Time) error
	DeleteWorkspace(ctx context.Context, id string) error
	ListWorkspacesForUser(ctx context.Context, userID string) ([]mworkspace.WorkspaceMembership, error)
	AddMember(ctx context.Context, workspaceID, userID string, role mworkspace.WorkspaceRole) error
	FindMember(ctx context.Context, workspaceID, userID string) (*mworkspace.WorkspaceMember, error)
	RemoveMember(ctx context.Context, workspaceID, userID string) error
	UpdateMemberRole(ctx context.Context, workspaceID, userID string, role mworkspace.WorkspaceRole) error
	ListMembers(ctx context.Context, workspaceID string) ([]mworkspace.WorkspaceMember, error)
	DeleteMembersByWorkspace(ctx context.Context, workspaceID string) error
	DeleteMembersByUser(ctx context.Context, userID string) error
	TransferOwnership(ctx context.Context, workspaceID, oldOwnerID, newOwnerID string) error
}
