package models

import "time"

const (
	FavoriteWorkspace = "workspace"
	FavoriteBoard     = "board"
)

type Favorite struct {
	ID         string
	UserID     string
	TargetType string
	TargetID   string
	CreatedAt  time.Time
}
