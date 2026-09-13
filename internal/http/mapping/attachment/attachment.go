package attachment

import (
	cattachment "github.com/tandem/tandem/internal/contracts/attachment"
	dtoattachment "github.com/tandem/tandem/internal/http/dto/attachment"
)

func ToWireAttachment(c cattachment.AttachmentResponse) dtoattachment.AttachmentResponse {
	return dtoattachment.AttachmentResponse{
		ID:          c.ID,
		TaskID:      c.TaskID,
		Filename:    c.Filename,
		ContentType: c.ContentType,
		Size:        c.Size,
		UploadedBy:  c.UploadedBy,
		CreatedAt:   c.CreatedAt,
		URL:         c.URL,
	}
}

func ToWireAttachments(cc []cattachment.AttachmentResponse) []dtoattachment.AttachmentResponse {
	out := make([]dtoattachment.AttachmentResponse, 0, len(cc))
	for _, c := range cc {
		out = append(out, ToWireAttachment(c))
	}
	return out
}
