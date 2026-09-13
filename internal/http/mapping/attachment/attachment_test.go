package attachment

import (
	"testing"
	"time"

	cattachment "github.com/tandem/tandem/internal/contracts/attachment"
)

func TestToWireAttachment(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := cattachment.AttachmentResponse{
		ID: "a1", TaskID: "t1", Filename: "file.png", ContentType: "image/png",
		Size: 1024, UploadedBy: "u1", CreatedAt: at, URL: "http://x/a1",
	}
	got := ToWireAttachment(c)
	if got.ID != "a1" || got.Filename != "file.png" || got.URL != "http://x/a1" {
		t.Fatalf("ToWireAttachment = %+v", got)
	}
}

func TestToWireAttachments(t *testing.T) {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	got := ToWireAttachments([]cattachment.AttachmentResponse{{ID: "a1", CreatedAt: at}, {ID: "a2", CreatedAt: at}})
	if len(got) != 2 || got[1].ID != "a2" {
		t.Fatalf("ToWireAttachments len = %d", len(got))
	}
}
