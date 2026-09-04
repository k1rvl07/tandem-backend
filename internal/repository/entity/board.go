package entity

import "time"

type Board struct {
	ID          string    `gorm:"type:uuid;primaryKey" json:"id"`
	WorkspaceID string    `gorm:"type:uuid;not null;index" json:"workspace_id"`
	Name        string    `gorm:"size:80;not null" json:"name"`
	Position    int       `gorm:"not null;default:0" json:"position"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (Board) TableName() string {
	return "boards"
}

type Column struct {
	ID        string    `gorm:"type:uuid;primaryKey" json:"id"`
	BoardID   string    `gorm:"type:uuid;not null;index" json:"board_id"`
	Name      string    `gorm:"size:80;not null" json:"name"`
	Position  int       `gorm:"not null;default:0" json:"position"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Column) TableName() string {
	return "board_columns"
}

type Task struct {
	ID          string     `gorm:"type:uuid;primaryKey" json:"id"`
	ColumnID    string     `gorm:"type:uuid;not null;index" json:"column_id"`
	Title       string     `gorm:"size:120;not null" json:"title"`
	Description string     `gorm:"type:text" json:"description"`
	AssigneeID  string     `gorm:"type:varchar(36)" json:"assignee_id"`
	Priority    string     `gorm:"size:10;not null;default:medium" json:"priority"`
	DueDate     *time.Time `json:"due_date"`
	Position    int        `gorm:"not null;default:0" json:"position"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (Task) TableName() string {
	return "tasks"
}
