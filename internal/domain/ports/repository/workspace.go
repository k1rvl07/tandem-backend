package repository

import (
	"context"

	"github.com/tandem/tandem/internal/domain/models"
)

type WorkspaceRepository interface {
	CreateWorkspace(ctx context.Context, ws *models.Workspace) error
	FindWorkspaceByID(ctx context.Context, id string) (*models.Workspace, error)
	UpdateWorkspace(ctx context.Context, ws *models.Workspace) error
	DeleteWorkspace(ctx context.Context, id string) error
	ListWorkspacesForUser(ctx context.Context, userID string) ([]models.WorkspaceMembership, error)
	AddMember(ctx context.Context, workspaceID, userID string, role models.WorkspaceRole) error
	FindMember(ctx context.Context, workspaceID, userID string) (*models.WorkspaceMember, error)
	RemoveMember(ctx context.Context, workspaceID, userID string) error
	UpdateMemberRole(ctx context.Context, workspaceID, userID string, role models.WorkspaceRole) error
	ListMembers(ctx context.Context, workspaceID string) ([]models.WorkspaceMember, error)
	DeleteMembersByWorkspace(ctx context.Context, workspaceID string) error
}
