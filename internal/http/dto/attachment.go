package dto

import "time"

type AttachmentResponse struct {
	ID          string    `json:"id"`
	TaskID      string    `json:"task_id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	UploadedBy  string    `json:"uploaded_by"`
	CreatedAt   time.Time `json:"created_at"`
	URL         string    `json:"url"`
}
