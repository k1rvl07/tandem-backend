package cache

import (
	"context"
	"time"
)

type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	MGet(ctx context.Context, keys ...string) ([]string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	GetDel(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
	Expire(ctx context.Context, key string, ttl time.Duration) error
	Incr(ctx context.Context, key string) (int64, error)
	Ping(ctx context.Context) error
	Close() error
}
