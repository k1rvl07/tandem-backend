package workspace

import "time"

type CreateWorkspaceRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Prefix      string `json:"prefix"`
}

type UpdateWorkspaceRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Prefix      string `json:"prefix"`
}

type SetThemeRequest struct {
	Theme string `json:"theme"`
}

type AddMemberRequest struct {
	Login string `json:"login"`
	Role  string `json:"role"`
}

type TransferOwnerRequest struct {
	UserID string `json:"user_id"`
}

type UpdateMemberRoleRequest struct {
	Role string `json:"role"`
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
	ID          string                    `json:"id"`
	Name        string                    `json:"name"`
	Description string                    `json:"description"`
	Prefix      string                    `json:"prefix"`
	Theme       string                    `json:"theme"`
	Role        string                    `json:"role"`
	IsFavorite  bool                      `json:"is_favorite"`
	CreatedAt   time.Time                 `json:"created_at"`
	UpdatedAt   time.Time                 `json:"updated_at"`
	Members     []WorkspaceMemberResponse `json:"members"`
}

type WorkspaceInviteResponse struct {
	InviteToken *string    `json:"invite_token"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}
