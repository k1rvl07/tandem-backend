package rediscache

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testGinMiddleware(rl *RateLimiter) gin.HandlerFunc {
	gin.SetMode(gin.TestMode)
	return rl.Middleware()
}

func TestRateLimitMiddleware_allowsAndThenBlocks(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	rl := &RateLimiter{client: rdb, limit: 3, window: time.Minute,
		resolveKey: func(c *gin.Context) string { return "ip:" + c.ClientIP() },
	}

	router := gin.New()
	router.Use(testGinMiddleware(rl))
	router.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })

	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "60", w.Header().Get("Retry-After"))
}

func TestRateLimitMiddleware_byUserKey(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	rl := &RateLimiter{client: rdb, limit: 2, window: time.Minute,
		resolveKey: func(c *gin.Context) string {
			if uid := c.GetString("userID"); uid != "" {
				return "user:" + uid
			}
			return "ip:" + c.ClientIP()
		},
	}

	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("userID", "u1"); c.Next() })
	router.Use(testGinMiddleware(rl))
	router.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })

	mk := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.RemoteAddr = "9.9.9.9:1234"
		router.ServeHTTP(w, req)
		return w
	}

	assert.Equal(t, http.StatusOK, mk().Code)
	assert.Equal(t, http.StatusOK, mk().Code)
	assert.Equal(t, http.StatusTooManyRequests, mk().Code)
}

func TestRateLimitMiddleware_differentIPsIndependent(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	rl := &RateLimiter{client: rdb, limit: 1, window: time.Minute,
		resolveKey: func(c *gin.Context) string { return "ip:" + c.ClientIP() },
	}

	router := gin.New()
	router.Use(testGinMiddleware(rl))
	router.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })

	mk := func(ip string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.RemoteAddr = ip + ":1234"
		router.ServeHTTP(w, req)
		return w
	}

	assert.Equal(t, http.StatusOK, mk("1.1.1.1").Code)
	assert.Equal(t, http.StatusOK, mk("2.2.2.2").Code)
	assert.Equal(t, http.StatusTooManyRequests, mk("1.1.1.1").Code)
	assert.Equal(t, http.StatusTooManyRequests, mk("2.2.2.2").Code)
	require.NotNil(t, mk("3.3.3.3"))
}
