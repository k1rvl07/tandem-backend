package models

import "time"

const (
	PriorityLow    = "low"
	PriorityMedium = "medium"
	PriorityHigh   = "high"
)

type Task struct {
	ID          string
	ColumnID    string
	Title       string
	Description string
	AssigneeID  string
	Priority    string
	DueDate     *time.Time
	Position    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
