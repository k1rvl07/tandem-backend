package models

import "time"

const (
	RoleAdmin     = "admin"
	RoleModerator = "moderator"
	RoleUser      = "user"
)

var StaffRoles = map[string]bool{
	RoleAdmin:     true,
	RoleModerator: true,
}

type User struct {
	ID           string
	Login        string
	PasswordHash string
	Role         string
	DisplayName  string
	Bio          string
	AvatarKey    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
