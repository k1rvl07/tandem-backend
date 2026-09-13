package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tandem/tandem/internal/domain/ports/service"
)

type fakeTokens struct {
	service.TokenService
	userID string
	err    error
}

func (f *fakeTokens) Parse(token string) (string, error) {
	if token == "valid" {
		return f.userID, nil
	}
	return "", f.err
}

func TestAuthValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var gotUserID string
	r.GET("/protected", Auth(&fakeTokens{userID: "u1"}), func(c *gin.Context) {
		gotUserID = c.GetString(CtxUserID)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer valid")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "u1", gotUserID)
}

func TestAuthMissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", Auth(&fakeTokens{userID: "u1"}), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthMalformedHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", Auth(&fakeTokens{userID: "u1"}), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "token-without-bearer")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuthInvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/protected", Auth(&fakeTokens{err: errors.New("invalid")}), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer garbage")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}
