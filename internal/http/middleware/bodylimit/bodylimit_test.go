package bodylimit

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestLimitBodySmallPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/echo", LimitBody(1024), func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		c.String(http.StatusOK, string(body))
	})
	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader("hello"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "hello", w.Body.String())
}

func TestLimitBodyTooLarge(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/echo", LimitBody(1024), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(strings.Repeat("a", 2048)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestLimitBodySkipsMultipart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/upload", LimitBody(16), func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		c.String(http.StatusOK, string(body))
	})
	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(strings.Repeat("a", 1024)))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, w.Body.String(), 1024)
}
