package mapping

import (
	mattachment "github.com/tandem/tandem/internal/domain/models/attachment"
	dattachment "github.com/tandem/tandem/internal/http/dto/attachment"
)

func AttachmentResponse(attachment *mattachment.TaskAttachment, workspaceID, taskID string) *dattachment.AttachmentResponse {
	return &dattachment.AttachmentResponse{
		ID:          attachment.ID,
		TaskID:      attachment.TaskID,
		Filename:    attachment.Filename,
		ContentType: attachment.ContentType,
		Size:        attachment.Size,
		UploadedBy:  attachment.UploadedBy,
		CreatedAt:   attachment.CreatedAt,
		URL:         "/api/v1/workspaces/" + workspaceID + "/tasks/" + taskID + "/attachments/" + attachment.ID,
	}
}
