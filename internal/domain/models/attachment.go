package models

import "time"

type TaskAttachment struct {
	ID          string
	TaskID      string
	Filename    string
	ObjectKey   string
	Size        int64
	ContentType string
	UploadedBy  string
	CreatedAt   time.Time
}
