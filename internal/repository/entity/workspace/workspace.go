package workspace

import (
	"time"

	euser "github.com/tandem/tandem/internal/repository/entity/user"
)

type Workspace struct {
	ID              string     `gorm:"type:uuid;primaryKey" json:"id"`
	Name            string     `gorm:"size:80;not null" json:"name"`
	Description     string     `gorm:"size:400" json:"description"`
	Prefix          string     `gorm:"size:10" json:"prefix"`
	Theme           string     `gorm:"size:10" json:"theme"`
	InviteToken     string     `gorm:"size:64;index" json:"invite_token"`
	InviteExpiresAt *time.Time `gorm:"index" json:"invite_expires_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (Workspace) TableName() string {
	return "workspaces"
}

type WorkspaceMember struct {
	WorkspaceID string    `gorm:"type:uuid;primaryKey" json:"workspace_id"`
	UserID      string    `gorm:"type:uuid;primaryKey" json:"user_id"`
	Role        string    `gorm:"size:20;not null" json:"role"`
	CreatedAt   time.Time `json:"created_at"`

	Workspace Workspace  `gorm:"foreignKey:WorkspaceID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	User      euser.User `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (WorkspaceMember) TableName() string {
	return "workspace_members"
}
