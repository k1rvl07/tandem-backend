package ratelimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tandem/tandem/internal/pkg/ctxkeys"
)

func newGinContext(remoteAddr string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = &http.Request{RemoteAddr: remoteAddr + ":1234"}
	return c
}

func newTestRateLimiter(t *testing.T, limit int64, window time.Duration) (*RateLimiter, func()) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	rl := &RateLimiter{client: rdb, limit: limit, window: window, mem: &memSlidingWindow{points: make(map[string][]memEntry)}, now: time.Now}
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
	clock := time.Now()
	rl.now = func() time.Time { return clock }
	require.True(t, mustAllow(t, rl, ctx))
	require.True(t, mustAllow(t, rl, ctx))
	require.False(t, mustAllow(t, rl, ctx))

	clock = clock.Add(150 * time.Millisecond)

	allowed, err := rl.Allow(ctx, "ip:1.2.3.4")
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestRateLimiter_memFallbackSlides(t *testing.T) {
	mw := &memSlidingWindow{points: make(map[string][]memEntry)}
	window := 100 * time.Millisecond
	now := int64(1_000_000)

	assert.True(t, mw.allow("ip:1.2.3.4", 2, window, now))
	assert.True(t, mw.allow("ip:1.2.3.4", 2, window, now))
	assert.False(t, mw.allow("ip:1.2.3.4", 2, window, now))

	now += window.Milliseconds() + 50
	assert.True(t, mw.allow("ip:1.2.3.4", 2, window, now))
}

func TestRateLimiter_memFallbackIsolatedPerInstance(t *testing.T) {
	window := time.Minute
	now := int64(1_000_000)
	left := &memSlidingWindow{points: make(map[string][]memEntry)}
	right := &memSlidingWindow{points: make(map[string][]memEntry)}

	for i := 0; i < 5; i++ {
		assert.True(t, left.allow("ip:1.2.3.4", 5, window, now), "left must stay under its own limit")
	}
	assert.True(t, right.allow("ip:1.2.3.4", 5, window, now), "instances must not share counters")
}

func TestRateLimiter_memFallbackDoesNotInterfereWithRedis(t *testing.T) {
	rl, cleanup := newTestRateLimiter(t, 1, time.Minute)
	defer cleanup()

	ctx := context.Background()
	require.True(t, mustAllow(t, rl, ctx))
	require.False(t, mustAllow(t, rl, ctx))
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
	upload := &RateLimiter{client: rdb, name: "upload", limit: 3, window: time.Minute, now: time.Now}
	read := &RateLimiter{client: rdb, name: "read", limit: 3, window: time.Minute, now: time.Now}

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
	rl := &RateLimiter{client: rdb, limit: 1, window: time.Minute, now: time.Now}

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

func TestRateLimiter_NewWithByIPAndClock(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	clock := time.Now()
	rl := New(rdb, "req", 2, time.Minute, ByIP(), WithClock(func() time.Time { return clock }))
	require.NotNil(t, rl.resolveKey)
	require.NotNil(t, rl.mem)
	require.NotNil(t, rl.now)

	ctx := context.Background()
	for i := 0; i < 2; i++ {
		allowed, err := rl.Allow(ctx, rl.resolveKey(newGinContext("9.9.9.9")))
		require.NoError(t, err)
		assert.True(t, allowed)
	}
	allowed, err := rl.Allow(ctx, rl.resolveKey(newGinContext("9.9.9.9")))
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestRateLimiter_NewByUserFallbackToIP(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	rl := New(rdb, "req", 1, time.Minute, ByUser())
	ctx := context.Background()

	withUser := newGinContext("1.1.1.1")
	withUser.Set(ctxkeys.CtxUserID, "u1")
	allowed, err := rl.Allow(ctx, rl.resolveKey(withUser))
	require.NoError(t, err)
	assert.True(t, allowed)
	allowed, err = rl.Allow(ctx, rl.resolveKey(withUser))
	require.NoError(t, err)
	assert.False(t, allowed)

	noUser := newGinContext("1.1.1.1")
	allowed, err = rl.Allow(ctx, rl.resolveKey(noUser))
	require.NoError(t, err)
	assert.True(t, allowed)

	other := newGinContext("2.2.2.2")
	other.Set(ctxkeys.CtxUserID, "u2")
	allowed, err = rl.Allow(ctx, rl.resolveKey(other))
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestRateLimiter_NewDefaultResolverIsIP(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	rl := New(rdb, "req", 1, time.Minute)
	require.Equal(t, "ip:8.8.8.8", rl.resolveKey(newGinContext("8.8.8.8")))
}

func TestRateLimiter_NewCustomKeyResolver(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	rl := New(rdb, "req", 1, time.Minute, WithKeyResolver(func(c *gin.Context) string { return "custom:x" }))
	require.Equal(t, "custom:x", rl.resolveKey(nil))
}

func TestRateLimiter_memGC(t *testing.T) {
	mw := &memSlidingWindow{points: make(map[string][]memEntry)}
	window := time.Minute
	now := int64(1_000_000)
	for i := 0; i < 20001; i++ {
		k := "agent:old:" + strconv.Itoa(i)
		mw.points[k] = []memEntry{{ts: now - window.Milliseconds() - 1000}}
	}
	assert.True(t, mw.allow("agent:current", 1, window, now))
	assert.LessOrEqual(t, len(mw.points), 1)
}
