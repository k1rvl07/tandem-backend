package rediscache

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/tandem/tandem/internal/pkg/ctxkeys"
)

const rlKeyPrefix = "rl:sw:"

const (
	AuthLimit    = 15
	AuthWindow   = 5 * time.Minute
	WriteLimit   = 400
	WriteWindow  = time.Minute
	ReadLimit    = 1200
	ReadWindow   = time.Minute
	UploadLimit  = 120
	UploadWindow = time.Minute
	WsLimit      = 300
	WsWindow     = 5 * time.Minute
)

var slidingWindowScript = redis.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local min = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local member = ARGV[4]
local window = tonumber(ARGV[5])
redis.call('ZREMRANGEBYSCORE', key, 0, min)
local count = redis.call('ZCARD', key)
if count < limit then
    redis.call('ZADD', key, now, member)
    redis.call('EXPIRE', key, window)
    return 1
end
return 0
`)

type RateLimiter struct {
	client     *redis.Client
	name       string
	limit      int64
	window     time.Duration
	resolveKey func(c *gin.Context) string
}

type RateLimiterOption func(*RateLimiter)

func WithKeyResolver(fn func(c *gin.Context) string) RateLimiterOption {
	return func(rl *RateLimiter) { rl.resolveKey = fn }
}

func ByIP() RateLimiterOption {
	return WithKeyResolver(func(c *gin.Context) string {
		return "ip:" + c.ClientIP()
	})
}

func ByUser() RateLimiterOption {
	return WithKeyResolver(func(c *gin.Context) string {
		if uid := c.GetString(ctxkeys.CtxUserID); uid != "" {
			return "user:" + uid
		}
		return "ip:" + c.ClientIP()
	})
}

func NewRateLimiter(r *Redis, name string, limit int64, window time.Duration, opts ...RateLimiterOption) *RateLimiter {
	rl := &RateLimiter{
		client:     r.Raw(),
		name:       name,
		limit:      limit,
		window:     window,
		resolveKey: func(c *gin.Context) string { return "ip:" + c.ClientIP() },
	}
	for _, opt := range opts {
		opt(rl)
	}
	return rl
}

var rlNonce atomic.Int64

func (rl *RateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	now := time.Now().UnixMilli()
	min := now - rl.window.Milliseconds()
	window := int64(rl.window.Seconds())
	if window < 1 {
		window = 1
	}
	member := strconv.FormatInt(now, 10) + "-" + strconv.FormatInt(rlNonce.Add(1), 10)
	res, err := slidingWindowScript.Run(
		ctx,
		rl.client,
		[]string{rlKeyPrefix + rl.name + ":" + key},
		now,
		min,
		rl.limit,
		member,
		window,
	).Int()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := rl.resolveKey(c)
		allowed, err := rl.Allow(c.Request.Context(), key)
		if err != nil {
			allowed = memFallback.allow(key, rl.limit, rl.window)
		}
		if !allowed {
			c.Header("Retry-After", strconv.FormatInt(int64(rl.window.Seconds()), 10))
			c.AbortWithStatusJSON(429, gin.H{"error": "too many requests"})
			return
		}
		c.Next()
	}
}

type memEntry struct {
	ts int64
}

type memSlidingWindow struct {
	mu     sync.Mutex
	points map[string][]memEntry
}

var memFallback = &memSlidingWindow{points: make(map[string][]memEntry)}

func (mw *memSlidingWindow) allow(key string, limit int64, window time.Duration) bool {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	now := time.Now().UnixMilli()
	min := now - window.Milliseconds()
	points := prunePoints(mw.points[key], min)
	if int64(len(points)) >= limit {
		mw.points[key] = points
		return false
	}
	mw.points[key] = append(points, memEntry{ts: now})
	if len(mw.points) > 20000 {
		mw.gc(min)
	}
	return true
}

func (mw *memSlidingWindow) gc(min int64) {
	for key, points := range mw.points {
		kept := prunePoints(points, min)
		if len(kept) == 0 {
			delete(mw.points, key)
		} else {
			mw.points[key] = kept
		}
	}
}

func prunePoints(points []memEntry, min int64) []memEntry {
	kept := points[:0]
	for _, p := range points {
		if p.ts >= min {
			kept = append(kept, p)
		}
	}
	return kept
}
