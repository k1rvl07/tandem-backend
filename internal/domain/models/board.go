package models

import "time"

type Board struct {
	ID          string
	WorkspaceID string
	Name        string
	Position    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
