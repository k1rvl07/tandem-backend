package filestore

import (
	"context"
	"io"
	"time"
)

type FileStore interface {
	Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
}
