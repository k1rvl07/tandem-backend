package images

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/usecase/file/core"
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

type Images struct {
	*core.Core
}

func New(c *core.Core) *Images {
	return &Images{Core: c}
}

func (s *Images) UploadImage(ctx context.Context, userID, namespace, filename, contentType string, reader io.Reader, size int64) (string, error) {
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
	if err := s.Store.Put(ctx, key, reader, size, expected); err != nil {
		return "", fmt.Errorf("%w: put image", pkgerrors.ErrInternal)
	}
	return key, nil
}

func (s *Images) ValidateImageKey(key, ownerID string) error {
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

func (s *Images) Remove(ctx context.Context, key string) error {
	if key == "" {
		return nil
	}
	if err := s.Store.Delete(ctx, key); err != nil {
		return fmt.Errorf("%w: delete image", pkgerrors.ErrInternal)
	}
	return nil
}

func (s *Images) RemoveMany(ctx context.Context, keys []string) {
	for _, key := range keys {
		if key == "" {
			continue
		}
		if err := s.Store.Delete(ctx, key); err != nil {
			s.Logger.Warn("remove object failed", zap.String("key", key), zap.Error(err))
		}
	}
}

func (s *Images) DeleteImage(ctx context.Context, ownerID, key string) error {
	if err := s.ValidateImageKey(key, ownerID); err != nil {
		return err
	}
	return s.Remove(ctx, key)
}

func (s *Images) SignImage(ctx context.Context, key string) (string, error) {
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
	url, err := s.Store.PresignGet(ctx, key, signedURLTTL)
	if err != nil {
		return "", fmt.Errorf("%w: sign image", pkgerrors.ErrInternal)
	}
	return url, nil
}
