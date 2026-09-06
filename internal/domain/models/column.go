package models

import "time"

var DefaultColumnNames = []string{"Backlog", "To Do", "In Progress", "Done"}

type Column struct {
	ID        string
	BoardID   string
	Name      string
	Position  int
	CreatedAt time.Time
	UpdatedAt time.Time
}
