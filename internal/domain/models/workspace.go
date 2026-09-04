package models

import "time"

type WorkspaceRole string

const (
	RoleOwner  WorkspaceRole = "owner"
	RoleEditor WorkspaceRole = "editor"
	RoleViewer WorkspaceRole = "viewer"
)

type Workspace struct {
	ID          string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
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
