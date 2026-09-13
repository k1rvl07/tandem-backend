package cache

import (
	"context"
	"encoding/json"
	"hash/fnv"
	"strconv"
	"time"

	"github.com/tandem/tandem/internal/domain/ports/cache"
)

const (
	TTL        = 60 * time.Second
	versionTTL = 7 * 24 * time.Hour

	WSVerKey    = "t:wsver:"
	UVerKey     = "t:uver:"
	UsersVerKey = "t:usersver"
)

func Version(ctx context.Context, c cache.Cache, key string) string {
	if c == nil {
		return "0"
	}
	v, err := c.Get(ctx, key)
	if err != nil || v == "" {
		return "0"
	}
	return v
}

func Bump(ctx context.Context, c cache.Cache, key string) {
	if c == nil {
		return
	}
	if _, err := c.Incr(ctx, key); err != nil {
		return
	}
	_ = c.Expire(ctx, key, versionTTL)
}

func BumpWorkspace(ctx context.Context, c cache.Cache, workspaceID string) {
	Bump(ctx, c, WSVerKey+workspaceID)
}

func Load(ctx context.Context, c cache.Cache, key string, dst any) bool {
	if c == nil {
		return false
	}
	raw, err := c.Get(ctx, key)
	if err != nil || raw == "" {
		return false
	}
	return Unmarshal(raw, dst)
}

func Unmarshal(raw string, dst any) bool {
	if raw == "" {
		return false
	}
	if json.Unmarshal([]byte(raw), dst) != nil {
		return false
	}
	return true
}

func Store(ctx context.Context, c cache.Cache, key string, src any, ttl time.Duration) {
	if c == nil {
		return
	}
	raw, err := json.Marshal(src)
	if err != nil {
		return
	}
	_ = c.Set(ctx, key, string(raw), ttl)
}

func QueryHash(parts ...string) string {
	h := fnv.New32a()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	return strconv.FormatUint(uint64(h.Sum32()), 36)
}
