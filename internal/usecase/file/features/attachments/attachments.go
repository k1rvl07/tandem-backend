package attachments

import (
	"context"
	"fmt"
	"io"
	"path"

	"github.com/google/uuid"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/usecase/file/core"
)

const maxAttachmentSize = 20 << 20

type Attachments struct {
	*core.Core
}

func New(c *core.Core) *Attachments {
	return &Attachments{Core: c}
}

func (s *Attachments) PrepareAttachment(workspaceID, userID, filename string, size int64) (string, error) {
	if workspaceID == "" {
		return "", pkgerrors.NewValidationError("workspace is required")
	}
	if size <= 0 {
		return "", pkgerrors.NewValidationError("empty file")
	}
	if size > maxAttachmentSize {
		return "", pkgerrors.NewValidationError("file is too large")
	}
	if filename == "" {
		return "", pkgerrors.NewValidationError("filename is required")
	}
	base := path.Base(filename)
	return fmt.Sprintf("attachments/%s/%s/%s_%s", workspaceID, userID, uuid.NewString(), base), nil
}

func (s *Attachments) PutAttachment(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if err := s.Store.Put(ctx, key, reader, size, contentType); err != nil {
		return fmt.Errorf("%w: put attachment", pkgerrors.ErrInternal)
	}
	return nil
}

func (s *Attachments) OpenFile(ctx context.Context, key string) (io.ReadCloser, error) {
	if key == "" {
		return nil, pkgerrors.NewValidationError("missing key")
	}
	exists, err := s.Store.Exists(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("%w: check file", pkgerrors.ErrInternal)
	}
	if !exists {
		return nil, pkgerrors.ErrNotFound
	}
	rc, err := s.Store.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("%w: get file", pkgerrors.ErrInternal)
	}
	return rc, nil
}
