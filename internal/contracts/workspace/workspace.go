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
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Prefix      string                   `json:"prefix"`
	Theme       string                   `json:"theme"`
	Role        string                   `json:"role"`
	IsFavorite  bool                     `json:"is_favorite"`
	Owner       *WorkspaceMemberResponse `json:"owner"`
	CreatedAt   time.Time                `json:"created_at"`
	UpdatedAt   time.Time                `json:"updated_at"`
}

type WorkspaceMemberResponse struct {
	ID          string    `json:"id"`
	Login       string    `json:"login"`
	DisplayName string    `json:"display_name"`
	AvatarKey   string    `json:"avatar_key"`
	Role        string    `json:"role"`
	JoinedAt    time.Time `json:"joined_at"`
}

type WorkspaceDetailResponse struct {
	WorkspaceResponse
	Members []WorkspaceMemberResponse `json:"members"`
}

type WorkspaceInviteResponse struct {
	InviteToken *string
	ExpiresAt   *time.Time
}
