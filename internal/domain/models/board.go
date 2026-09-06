package models

import "time"

type Board struct {
	ID          string
	WorkspaceID string
	Name        string
	Position    int
	IsMain      bool
	ArchivedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
