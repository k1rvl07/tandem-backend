package workspace

import "time"

type WorkspaceRole string

const (
	RoleOwner  WorkspaceRole = "owner"
	RoleEditor WorkspaceRole = "editor"
	RoleMember WorkspaceRole = "member"
)

const DefaultWorkspaceColor = "1d4ed8"

type Workspace struct {
	ID              string
	Name            string
	Description     string
	Prefix          string
	Theme           string
	InviteToken     string
	InviteExpiresAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type WorkspaceMember struct {
	WorkspaceID string
	UserID      string
	Role        WorkspaceRole
	CreatedAt   time.Time
}

type WorkspaceMembership struct {
	Workspace
	Role WorkspaceRole
}
