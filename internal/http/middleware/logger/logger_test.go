package logger

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zapcore.InfoLevel)
	log := zap.New(core)

	r := gin.New()
	r.Use(Logger(log))
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 1, logs.Len())

	entry := logs.All()[0]
	require.Equal(t, "http_request", entry.Message)

	fields := entry.ContextMap()
	require.Equal(t, "GET", fields["method"])
	require.Equal(t, "/ping", fields["path"])
	require.Equal(t, int64(200), fields["status"])
	require.NotEmpty(t, fields["client_ip"])
	require.Contains(t, fields, "latency")
}

func TestLoggerClientIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zapcore.InfoLevel)
	log := zap.New(core)

	r := gin.New()
	r.Use(Logger(log))
	r.GET("/", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "203.0.113.7:1234"
	r.ServeHTTP(w, req)

	fields := logs.All()[0].ContextMap()
	require.Equal(t, "203.0.113.7", fields["client_ip"])
	require.Equal(t, int64(204), fields["status"])
}
