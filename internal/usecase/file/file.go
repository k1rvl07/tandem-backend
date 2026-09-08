package file

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tandem/tandem/internal/domain/ports/filestore"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"go.uber.org/zap"
)

var allowedImageExts = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".webp": "image/webp",
	".gif":  "image/gif",
}

var allowedNamespaces = map[string]bool{
	"avatars": true,
	"images":  true,
	"covers":  true,
}

var imageNamespaces = map[string]bool{
	"avatars": true,
	"images":  true,
	"covers":  true,
	"tasks":   true,
}

const signedURLTTL = time.Hour

type Service struct {
	store  filestore.FileStore
	logger *zap.Logger
}

func NewService(store filestore.FileStore, logger *zap.Logger) *Service {
	return &Service{store: store, logger: logger}
}

func (s *Service) UploadImage(ctx context.Context, userID, namespace, filename, contentType string, reader io.Reader, size int64) (string, error) {
	if !allowedNamespaces[namespace] {
		return "", pkgerrors.NewValidationError("invalid namespace")
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

func (s *Service) ValidateImageKey(key, ownerID string) error {
	if key == "" {
		return nil
	}
	parts := strings.Split(key, "/")
	if len(parts) != 3 || !imageNamespaces[parts[0]] || parts[1] != ownerID {
		return pkgerrors.NewValidationError("invalid image key")
	}
	ext := strings.ToLower(path.Ext(parts[2]))
	if _, ok := allowedImageExts[ext]; !ok {
		return pkgerrors.NewValidationError("invalid image key")
	}
	name := parts[2][:len(parts[2])-len(ext)]
	if _, err := uuid.Parse(name); err != nil {
		return pkgerrors.NewValidationError("invalid image key")
	}
	return nil
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

func (s *Service) RemoveMany(ctx context.Context, keys []string) {
	for _, key := range keys {
		if key == "" {
			continue
		}
		if err := s.store.Delete(ctx, key); err != nil {
			s.logger.Warn("remove object failed", zap.String("key", key), zap.Error(err))
		}
	}
}

func (s *Service) SignImage(ctx context.Context, key string) (string, error) {
	if key == "" {
		return "", pkgerrors.NewValidationError("missing key")
	}
	parts := strings.Split(key, "/")
	if len(parts) != 3 || !imageNamespaces[parts[0]] {
		return "", pkgerrors.ErrNotFound
	}
	ext := strings.ToLower(path.Ext(parts[2]))
	if _, ok := allowedImageExts[ext]; !ok {
		return "", pkgerrors.ErrNotFound
	}
	if _, err := uuid.Parse(parts[1]); err != nil {
		return "", pkgerrors.ErrNotFound
	}
	name := parts[2][:len(parts[2])-len(ext)]
	if _, err := uuid.Parse(name); err != nil {
		return "", pkgerrors.ErrNotFound
	}
	url, err := s.store.PresignGet(ctx, key, signedURLTTL)
	if err != nil {
		return "", fmt.Errorf("%w: sign image", pkgerrors.ErrInternal)
	}
	return url, nil
}

const maxAttachmentSize = 20 << 20

func (s *Service) PrepareAttachment(workspaceID, userID, filename string, size int64) (string, error) {
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

func (s *Service) PutAttachment(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if err := s.store.Put(ctx, key, reader, size, contentType); err != nil {
		return fmt.Errorf("%w: put attachment", pkgerrors.ErrInternal)
	}
	return nil
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
