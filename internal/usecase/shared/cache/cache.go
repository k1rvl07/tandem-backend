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

func WorkspaceVersionKey(workspaceID string) string {
	return WSVerKey + workspaceID
}

func UserVersionKey(userID string) string {
	return UVerKey + userID
}

func VersionWorkspace(ctx context.Context, c cache.Cache, workspaceID string) string {
	return Version(ctx, c, WorkspaceVersionKey(workspaceID))
}

func VersionUser(ctx context.Context, c cache.Cache, userID string) string {
	return Version(ctx, c, UserVersionKey(userID))
}

func VersionUsers(ctx context.Context, c cache.Cache) string {
	return Version(ctx, c, UsersVerKey)
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
	Bump(ctx, c, WorkspaceVersionKey(workspaceID))
}

func BumpUser(ctx context.Context, c cache.Cache, userID string) {
	Bump(ctx, c, UserVersionKey(userID))
}

func BumpUsers(ctx context.Context, c cache.Cache) {
	Bump(ctx, c, UsersVerKey)
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
