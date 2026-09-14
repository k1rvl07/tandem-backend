package ws_identity

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tandem/tandem/internal/domain/ports/service"
	"github.com/tandem/tandem/internal/pkg/ctxkeys"
)

type fakeTokens struct {
	service.TokenService
	userID string
	err    error
}

func (f *fakeTokens) Parse(token string) (string, error) {
	if token == "valid-token" {
		return f.userID, nil
	}
	return "", f.err
}

func TestWSIdentAcceptSubprotocolToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var gotUserID string
	r.GET("/ws", WSIdentity(&fakeTokens{userID: "u1"}), func(c *gin.Context) {
		gotUserID = c.GetString(ctxkeys.CtxUserID)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Sec-WebSocket-Protocol", "tandem, valid-token")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "u1", gotUserID)
}

func TestWSIdentMissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ws", WSIdentity(&fakeTokens{userID: "u1"}), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestWSIdentRejectsNonTandemProtocol(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ws", WSIdentity(&fakeTokens{userID: "u1"}), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Sec-WebSocket-Protocol", "graphql")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestWSIdentRejectsTandemWithoutToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ws", WSIdentity(&fakeTokens{userID: "u1"}), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Sec-WebSocket-Protocol", "tandem")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestWSIdentRejectsInvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ws", WSIdentity(&fakeTokens{err: http.ErrNoCookie}), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Sec-WebSocket-Protocol", "tandem, garbage")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}
