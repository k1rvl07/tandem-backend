package user

import "time"

type User struct {
	ID           string    `gorm:"type:uuid;primaryKey" json:"id"`
	Login        string    `gorm:"uniqueIndex;size:50;not null" json:"login"`
	PasswordHash string    `gorm:"not null" json:"-"`
	Role         string    `gorm:"size:20;not null;default:user" json:"role"`
	DisplayName  string    `gorm:"size:50" json:"display_name"`
	Bio          string    `gorm:"size:400" json:"bio"`
	AvatarKey    string    `gorm:"size:255" json:"avatar_key"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (User) TableName() string {
	return "users"
}
