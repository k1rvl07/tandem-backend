package board

import "time"

type Board struct {
	ID          string    `gorm:"type:uuid;primaryKey" json:"id"`
	WorkspaceID string    `gorm:"type:uuid;not null;index" json:"workspace_id"`
	Name        string    `gorm:"size:80;not null" json:"name"`
	Position    int       `gorm:"not null;default:0" json:"position"`
	IsMain      bool      `gorm:"not null;default:false" json:"is_main"`
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
	AuthorID    string     `gorm:"type:varchar(36)" json:"author_id"`
	AssigneeID  string     `gorm:"type:varchar(36)" json:"assignee_id"`
	CuratorID   string     `gorm:"type:varchar(36)" json:"curator_id"`
	ParentID    string     `gorm:"type:varchar(36)" json:"parent_id"`
	DueDate     *time.Time `json:"due_date"`
	Position    int        `gorm:"not null;default:0" json:"position"`
	IsUrgent    bool       `gorm:"not null;default:false" json:"is_urgent"`
	IsHidden    bool       `gorm:"not null;default:false" json:"is_hidden"`
	ImageKey    string     `gorm:"size:255" json:"image_key"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (Task) TableName() string {
	return "tasks"
}
