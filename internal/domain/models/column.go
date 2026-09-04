package models

import "time"

type Column struct {
	ID        string
	BoardID   string
	Name      string
	Position  int
	CreatedAt time.Time
	UpdatedAt time.Time
}
