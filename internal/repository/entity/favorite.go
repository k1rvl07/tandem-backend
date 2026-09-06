package entity

import "time"

type Favorite struct {
	ID         string    `gorm:"type:uuid;primaryKey" json:"id"`
	UserID     string    `gorm:"type:uuid;not null;index" json:"user_id"`
	TargetType string    `gorm:"size:20;not null" json:"target_type"`
	TargetID   string    `gorm:"type:uuid;not null" json:"target_id"`
	CreatedAt  time.Time `json:"created_at"`
}

func (Favorite) TableName() string {
	return "favorites"
}
