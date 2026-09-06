package models

import "time"

type Task struct {
	ID          string
	ColumnID    string
	Title       string
	Description string
	AuthorID    string
	AssigneeID  string
	CuratorID   string
	ParentID    string
	DueDate     *time.Time
	Position    int
	IsUrgent    bool
	IsHidden    bool
	ImageKey    string
	ArchivedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
