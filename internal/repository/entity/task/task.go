package task

import "time"

type TaskAttachment struct {
	ID          string    `gorm:"type:uuid;primaryKey" json:"id"`
	TaskID      string    `gorm:"type:uuid;not null;index" json:"task_id"`
	Filename    string    `gorm:"size:255;not null" json:"filename"`
	ObjectKey   string    `gorm:"size:255;not null" json:"object_key"`
	Size        int64     `gorm:"not null" json:"size"`
	ContentType string    `gorm:"size:100" json:"content_type"`
	UploadedBy  string    `gorm:"type:uuid;not null" json:"uploaded_by"`
	CreatedAt   time.Time `json:"created_at"`
}

func (TaskAttachment) TableName() string {
	return "task_attachments"
}
