package dto

import "time"

type CreateWorkspaceRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UpdateWorkspaceRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type AddMemberRequest struct {
	Login string `json:"login"`
	Role  string `json:"role"`
}

type TransferOwnerRequest struct {
	UserID string `json:"user_id"`
}

type WorkspaceResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
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
	Role        string                    `json:"role"`
	CreatedAt   time.Time                 `json:"created_at"`
	UpdatedAt   time.Time                 `json:"updated_at"`
	Members     []WorkspaceMemberResponse `json:"members"`
}
