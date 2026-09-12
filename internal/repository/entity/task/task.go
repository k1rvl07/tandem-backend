package task

import (
	"time"

	eboard "github.com/tandem/tandem/internal/repository/entity/board"
	euser "github.com/tandem/tandem/internal/repository/entity/user"
)

type TaskAttachment struct {
	ID          string    `gorm:"type:uuid;primaryKey" json:"id"`
	TaskID      string    `gorm:"type:uuid;not null;index" json:"task_id"`
	Filename    string    `gorm:"size:255;not null" json:"filename"`
	ObjectKey   string    `gorm:"size:255;not null" json:"object_key"`
	Size        int64     `gorm:"not null" json:"size"`
	ContentType string    `gorm:"size:100" json:"content_type"`
	UploadedBy  *string   `gorm:"type:uuid" json:"uploaded_by"`
	CreatedAt   time.Time `json:"created_at"`

	Task           eboard.Task `gorm:"foreignKey:TaskID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	UploadedByUser euser.User  `gorm:"foreignKey:UploadedBy;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
}

func (TaskAttachment) TableName() string {
	return "task_attachments"
}
