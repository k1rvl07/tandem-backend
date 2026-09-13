package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	dauth "github.com/tandem/tandem/internal/http/dto/auth"
	"github.com/tandem/tandem/internal/http/handler/common"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
)

type fakeAuth struct {
	refreshToken string
}

func (f *fakeAuth) Login(context.Context, dauth.LoginRequest) (*dauth.LoginResponse, error) {
	return nil, nil
}

func (f *fakeAuth) Refresh(_ context.Context, refreshToken string) (*dauth.RefreshResponse, error) {
	if f.refreshToken != "" && refreshToken == f.refreshToken {
		return &dauth.RefreshResponse{Token: "new-access"}, nil
	}
	return nil, pkgerrors.ErrUnauthorized
}

func (f *fakeAuth) Logout(context.Context, string) error {
	return nil
}

func newRouter() (*gin.Engine, *fakeAuth) {
	gin.SetMode(gin.TestMode)
	uc := &fakeAuth{refreshToken: "good-refresh"}
	h := NewAuthHandler(uc, false, time.Minute)
	r := gin.New()
	r.POST("/api/v1/auth/refresh", h.Refresh)
	return r, uc
}

func TestRefreshWithCookie(t *testing.T) {
	r, _ := newRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: common.RefreshCookieName, Value: "good-refresh"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp dauth.RefreshResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "new-access", resp.Token)
}

func TestRefreshWithBodyToken(t *testing.T) {
	r, _ := newRouter()
	body := `{"refresh_token":"good-refresh"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp dauth.RefreshResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "new-access", resp.Token)
}

func TestRefreshWithoutToken(t *testing.T) {
	r, _ := newRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRefreshWithMalformedBody(t *testing.T) {
	r, _ := newRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}
