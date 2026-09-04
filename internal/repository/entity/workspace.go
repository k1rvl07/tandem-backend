package entity

import "time"

type Workspace struct {
	ID          string    `gorm:"type:uuid;primaryKey" json:"id"`
	Name        string    `gorm:"size:80;not null" json:"name"`
	Description string    `gorm:"size:400" json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (Workspace) TableName() string {
	return "workspaces"
}

type WorkspaceMember struct {
	WorkspaceID string    `gorm:"type:uuid;primaryKey" json:"workspace_id"`
	UserID      string    `gorm:"type:uuid;primaryKey" json:"user_id"`
	Role        string    `gorm:"size:20;not null" json:"role"`
	CreatedAt   time.Time `json:"created_at"`
}

func (WorkspaceMember) TableName() string {
	return "workspace_members"
}
