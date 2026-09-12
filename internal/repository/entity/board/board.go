package board

import (
	"time"

	euser "github.com/tandem/tandem/internal/repository/entity/user"
	eworkspace "github.com/tandem/tandem/internal/repository/entity/workspace"
)

type Board struct {
	ID          string    `gorm:"type:uuid;primaryKey" json:"id"`
	WorkspaceID string    `gorm:"type:uuid;not null;index" json:"workspace_id"`
	Name        string    `gorm:"size:80;not null" json:"name"`
	Position    int       `gorm:"not null;default:0" json:"position"`
	IsMain      bool      `gorm:"not null;default:false" json:"is_main"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	Workspace eworkspace.Workspace `gorm:"foreignKey:WorkspaceID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
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

	Board Board `gorm:"foreignKey:BoardID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (Column) TableName() string {
	return "board_columns"
}

type Task struct {
	ID          string     `gorm:"type:uuid;primaryKey" json:"id"`
	ColumnID    string     `gorm:"type:uuid;not null;index" json:"column_id"`
	Title       string     `gorm:"size:120;not null" json:"title"`
	Description string     `gorm:"type:text" json:"description"`
	AuthorID    *string    `gorm:"type:uuid" json:"author_id"`
	AssigneeID  *string    `gorm:"type:uuid" json:"assignee_id"`
	CuratorID   *string    `gorm:"type:uuid" json:"curator_id"`
	ParentID    *string    `gorm:"type:uuid" json:"parent_id"`
	DueDate     *time.Time `json:"due_date"`
	Position    int        `gorm:"not null;default:0" json:"position"`
	IsUrgent    bool       `gorm:"not null;default:false" json:"is_urgent"`
	IsHidden    bool       `gorm:"not null;default:false" json:"is_hidden"`
	ImageKey    string     `gorm:"size:255" json:"image_key"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	Column     Column     `gorm:"foreignKey:ColumnID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Author     euser.User `gorm:"foreignKey:AuthorID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	Assignee   euser.User `gorm:"foreignKey:AssigneeID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	Curator    euser.User `gorm:"foreignKey:CuratorID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	ParentTask *Task      `gorm:"foreignKey:ParentID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
}

func (Task) TableName() string {
	return "tasks"
}
