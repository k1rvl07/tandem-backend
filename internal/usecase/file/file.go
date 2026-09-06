package file

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/google/uuid"
	"github.com/tandem/tandem/internal/domain/ports/filestore"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
)

var allowedImageExts = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".webp": "image/webp",
	".gif":  "image/gif",
}

type Service struct {
	store filestore.FileStore
}

func NewService(store filestore.FileStore) *Service {
	return &Service{store: store}
}

func (s *Service) UploadImage(ctx context.Context, userID, namespace, filename, contentType string, reader io.Reader, size int64) (string, error) {
	if namespace == "" {
		return "", pkgerrors.NewValidationError("namespace is required")
	}
	ext := strings.ToLower(path.Ext(filename))
	expected, ok := allowedImageExts[ext]
	if !ok {
		return "", pkgerrors.NewValidationError("unsupported image type")
	}
	if contentType != "" && contentType != expected {
		return "", pkgerrors.NewValidationError("content type does not match extension")
	}
	if size <= 0 {
		return "", pkgerrors.NewValidationError("empty file")
	}

	key := fmt.Sprintf("%s/%s/%s%s", namespace, userID, uuid.NewString(), ext)
	if err := s.store.Put(ctx, key, reader, size, expected); err != nil {
		return "", fmt.Errorf("%w: put image", pkgerrors.ErrInternal)
	}
	return key, nil
}

func (s *Service) Open(ctx context.Context, key string) (io.ReadCloser, string, error) {
	if key == "" {
		return nil, "", pkgerrors.NewValidationError("missing key")
	}
	ext := strings.ToLower(path.Ext(key))
	contentType, ok := allowedImageExts[ext]
	if !ok {
		return nil, "", pkgerrors.NewValidationError("unsupported file")
	}
	exists, err := s.store.Exists(ctx, key)
	if err != nil {
		return nil, "", fmt.Errorf("%w: check image", pkgerrors.ErrInternal)
	}
	if !exists {
		return nil, "", pkgerrors.ErrNotFound
	}
	rc, err := s.store.Get(ctx, key)
	if err != nil {
		return nil, "", fmt.Errorf("%w: get image", pkgerrors.ErrInternal)
	}
	return rc, contentType, nil
}

func (s *Service) Remove(ctx context.Context, key string) error {
	if key == "" {
		return nil
	}
	if err := s.store.Delete(ctx, key); err != nil {
		return fmt.Errorf("%w: delete image", pkgerrors.ErrInternal)
	}
	return nil
}

const maxAttachmentSize = 20 << 20

func (s *Service) UploadAttachment(ctx context.Context, workspaceID, userID, filename, contentType string, reader io.Reader, size int64) (string, error) {
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
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	base := path.Base(filename)
	key := fmt.Sprintf("attachments/%s/%s/%s_%s", workspaceID, userID, uuid.NewString(), base)
	if err := s.store.Put(ctx, key, reader, size, contentType); err != nil {
		return "", fmt.Errorf("%w: put attachment", pkgerrors.ErrInternal)
	}
	return key, nil
}

func (s *Service) OpenFile(ctx context.Context, key string) (io.ReadCloser, error) {
	if key == "" {
		return nil, pkgerrors.NewValidationError("missing key")
	}
	exists, err := s.store.Exists(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("%w: check file", pkgerrors.ErrInternal)
	}
	if !exists {
		return nil, pkgerrors.ErrNotFound
	}
	rc, err := s.store.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("%w: get file", pkgerrors.ErrInternal)
	}
	return rc, nil
}
