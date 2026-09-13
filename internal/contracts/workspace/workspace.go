package workspacecontracts

import "time"

type CreateWorkspaceRequest struct {
	Name        string
	Description string
	Prefix      string
}

type UpdateWorkspaceRequest struct {
	Name        string
	Description string
	Prefix      string
}

type SetThemeRequest struct {
	Theme string
}

type AddMemberRequest struct {
	Login string
	Role  string
}

type TransferOwnerRequest struct {
	UserID string
}

type UpdateMemberRoleRequest struct {
	Role string
}

type WorkspaceResponse struct {
	ID          string
	Name        string
	Description string
	Prefix      string
	Theme       string
	Role        string
	IsFavorite  bool
	Owner       *WorkspaceMemberResponse
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type WorkspaceMemberResponse struct {
	ID          string
	Login       string
	DisplayName string
	AvatarKey   string
	Role        string
	JoinedAt    time.Time
}

type WorkspaceDetailResponse struct {
	WorkspaceResponse
	Members []WorkspaceMemberResponse
}

type WorkspaceInviteResponse struct {
	InviteToken *string
	ExpiresAt   *time.Time
}
