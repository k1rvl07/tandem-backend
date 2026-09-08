package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRateLimiter(t *testing.T, limit int64, window time.Duration) (*RateLimiter, func()) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	rl := &RateLimiter{client: rdb, limit: limit, window: window}
	return rl, func() { _ = rdb.Close(); mr.Close() }
}

func TestRateLimiter_allowsUnderLimit(t *testing.T) {
	rl, cleanup := newTestRateLimiter(t, 10, time.Minute)
	defer cleanup()

	allowed, err := rl.Allow(context.Background(), "ip:1.2.3.4")
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestRateLimiter_blocksAtLimit(t *testing.T) {
	rl, cleanup := newTestRateLimiter(t, 3, time.Minute)
	defer cleanup()

	ctx := context.Background()
	for i := 0; i < 3; i++ {
		allowed, err := rl.Allow(ctx, "ip:1.2.3.4")
		require.NoError(t, err)
		assert.True(t, allowed)
	}

	allowed, err := rl.Allow(ctx, "ip:1.2.3.4")
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestRateLimiter_windowSlides(t *testing.T) {
	rl, cleanup := newTestRateLimiter(t, 2, 100*time.Millisecond)
	defer cleanup()

	ctx := context.Background()
	require.True(t, mustAllow(t, rl, ctx))
	require.True(t, mustAllow(t, rl, ctx))
	require.False(t, mustAllow(t, rl, ctx))

	time.Sleep(150 * time.Millisecond)

	allowed, err := rl.Allow(ctx, "ip:1.2.3.4")
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestRateLimiter_differentKeysIndependent(t *testing.T) {
	rl, cleanup := newTestRateLimiter(t, 2, time.Minute)
	defer cleanup()

	ctx := context.Background()
	for i := 0; i < 2; i++ {
		require.True(t, mustAllow(t, rl, ctx))
	}
	assert.False(t, mustAllow(t, rl, ctx))

	allowed, err := rl.Allow(ctx, "ip:5.6.7.8")
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestRateLimiter_namesIndependent(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	upload := &RateLimiter{client: rdb, name: "upload", limit: 3, window: time.Minute}
	read := &RateLimiter{client: rdb, name: "read", limit: 3, window: time.Minute}

	ctx := context.Background()
	for i := 0; i < 3; i++ {
		require.True(t, mustAllowNamed(t, upload, ctx))
	}
	assert.False(t, mustAllowNamed(t, upload, ctx))
	for i := 0; i < 3; i++ {
		require.True(t, mustAllowNamed(t, read, ctx))
	}
	assert.False(t, mustAllowNamed(t, read, ctx))
}

func TestRateLimiter_failOpenOnClosedClient(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	rl := &RateLimiter{client: rdb, limit: 1, window: time.Minute}

	require.True(t, mustAllow(t, rl, context.Background()))

	_ = rdb.Close()
	mr.Close()

	allowed, err := rl.Allow(context.Background(), "ip:1.2.3.4")
	assert.False(t, allowed)
	assert.Error(t, err)
}

func mustAllow(t *testing.T, rl *RateLimiter, ctx context.Context) bool {
	t.Helper()
	allowed, err := rl.Allow(ctx, "ip:1.2.3.4")
	require.NoError(t, err)
	return allowed
}

func mustAllowNamed(t *testing.T, rl *RateLimiter, ctx context.Context) bool {
	t.Helper()
	allowed, err := rl.Allow(ctx, "user:u1")
	require.NoError(t, err)
	return allowed
}
