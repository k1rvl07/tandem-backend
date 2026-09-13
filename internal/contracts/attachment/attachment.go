package attachmentcontracts

import "time"

type AttachmentResponse struct {
	ID          string
	TaskID      string
	Filename    string
	ContentType string
	Size        int64
	UploadedBy  string
	CreatedAt   time.Time
	URL         string
}
