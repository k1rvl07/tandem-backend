package repository

import (
	"context"
	"time"

	"github.com/tandem/tandem/internal/domain/models"
)

type WorkspaceRepository interface {
	CreateWorkspace(ctx context.Context, ws *models.Workspace) error
	FindWorkspaceByID(ctx context.Context, id string) (*models.Workspace, error)
	FindWorkspaceByInvite(ctx context.Context, token string) (*models.Workspace, error)
	UpdateWorkspace(ctx context.Context, ws *models.Workspace) error
	UpdateInvite(ctx context.Context, workspaceID, token string, expiresAt *time.Time) error
	DeleteWorkspace(ctx context.Context, id string) error
	ListWorkspacesForUser(ctx context.Context, userID string) ([]models.WorkspaceMembership, error)
	AddMember(ctx context.Context, workspaceID, userID string, role models.WorkspaceRole) error
	FindMember(ctx context.Context, workspaceID, userID string) (*models.WorkspaceMember, error)
	RemoveMember(ctx context.Context, workspaceID, userID string) error
	UpdateMemberRole(ctx context.Context, workspaceID, userID string, role models.WorkspaceRole) error
	ListMembers(ctx context.Context, workspaceID string) ([]models.WorkspaceMember, error)
	DeleteMembersByWorkspace(ctx context.Context, workspaceID string) error
	DeleteMembersByUser(ctx context.Context, userID string) error
	TransferOwnership(ctx context.Context, workspaceID, oldOwnerID, newOwnerID string) error
}
