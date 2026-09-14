package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	dauth "github.com/tandem/tandem/internal/contracts/auth"
	"github.com/tandem/tandem/internal/http/handler/common"
	"github.com/tandem/tandem/internal/pkg/ctxkeys"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
)

type fakeAuth struct {
	refreshToken string
	loginResp    *dauth.LoginResponse
	refreshResp  *dauth.RefreshResponse
	loginErr     error
	logoutErr    error
}

func (f *fakeAuth) Login(context.Context, dauth.LoginRequest) (*dauth.LoginResponse, error) {
	if f.loginErr != nil {
		return nil, f.loginErr
	}
	return f.loginResp, nil
}

func (f *fakeAuth) Refresh(_ context.Context, refreshToken string) (*dauth.RefreshResponse, error) {
	if f.refreshToken != "" && refreshToken == f.refreshToken {
		if f.refreshResp != nil {
			return f.refreshResp, nil
		}
		return &dauth.RefreshResponse{Token: "new-access"}, nil
	}
	return nil, pkgerrors.ErrUnauthorized
}

func (f *fakeAuth) Logout(context.Context, string) error {
	return f.logoutErr
}

func newRouter() (*gin.Engine, *fakeAuth) {
	gin.SetMode(gin.TestMode)
	uc := &fakeAuth{refreshToken: "good-refresh"}
	h := NewAuthHandler(uc, false, time.Minute)
	r := gin.New()
	r.POST("/api/v1/auth/login", h.Login)
	r.POST("/api/v1/auth/refresh", h.Refresh)
	r.POST("/api/v1/auth/logout", func(c *gin.Context) {
		c.Set(ctxkeys.CtxUserID, "u1")
		h.Logout(c)
	})
	return r, uc
}

func newSecureRouter(uc *fakeAuth) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewAuthHandler(uc, true, time.Minute)
	r := gin.New()
	r.POST("/api/v1/auth/login", h.Login)
	r.POST("/api/v1/auth/refresh", h.Refresh)
	r.POST("/api/v1/auth/logout", func(c *gin.Context) {
		c.Set(ctxkeys.CtxUserID, "u1")
		h.Logout(c)
	})
	return r
}

func TestLoginSuccess(t *testing.T) {
	r, uc := newRouter()
	uc.loginResp = &dauth.LoginResponse{
		Token:        "access",
		RefreshToken: "refresh",
		User:         dauth.UserResponse{ID: "u1", Login: "alice", Role: "user"},
	}
	body := `{"login":"alice","password":"secret"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Token string `json:"token"`
		User  struct {
			ID    string `json:"id"`
			Login string `json:"login"`
		} `json:"user"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, "access", resp.Token)
	require.Equal(t, "u1", resp.User.ID)

	var found *http.Cookie
	for _, ck := range w.Result().Cookies() {
		if ck.Name == common.RefreshCookieName {
			found = ck
		}
	}
	require.NotNil(t, found)
	require.Equal(t, "refresh", found.Value)
	require.Equal(t, time.Minute.Seconds(), float64(found.MaxAge))
	require.Equal(t, "/api/v1/auth", found.Path)
	require.Equal(t, http.SameSiteLaxMode, found.SameSite)
}

func TestLoginBadBody(t *testing.T) {
	r, _ := newRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLoginUnauthorized(t *testing.T) {
	r, uc := newRouter()
	uc.loginErr = pkgerrors.ErrUnauthorized
	body := `{"login":"alice","password":"wrong"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLoginInternalError(t *testing.T) {
	r, uc := newRouter()
	uc.loginErr = errors.New("boom")
	body := `{"login":"alice","password":"secret"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusInternalServerError, w.Code)
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

func TestRefreshWithoutTokenWithEmptyCookie(t *testing.T) {
	r, _ := newRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: common.RefreshCookieName, Value: ""})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRefreshSetsCookie(t *testing.T) {
	r, uc := newRouter()
	uc.refreshResp = &dauth.RefreshResponse{Token: "next-access", RefreshToken: "next-refresh"}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: common.RefreshCookieName, Value: "good-refresh"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var found *http.Cookie
	for _, ck := range w.Result().Cookies() {
		if ck.Name == common.RefreshCookieName {
			found = ck
		}
	}
	require.NotNil(t, found)
	require.Equal(t, "next-refresh", found.Value)
}

func TestLoginAndRefreshSecureCookie(t *testing.T) {
	uc := &fakeAuth{refreshToken: "good-refresh"}
	uc.loginResp = &dauth.LoginResponse{Token: "access", RefreshToken: "refresh"}
	r := newSecureRouter(uc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"login":"a","password":"b"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	for _, ck := range w.Result().Cookies() {
		if ck.Name == common.RefreshCookieName {
			require.True(t, ck.Secure)
		}
	}
}

func TestLogoutSuccess(t *testing.T) {
	r, _ := newRouter()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)
	var found *http.Cookie
	for _, ck := range w.Result().Cookies() {
		if ck.Name == common.RefreshCookieName {
			found = ck
		}
	}
	require.NotNil(t, found)
	require.Equal(t, "", found.Value)
	require.Equal(t, -1, found.MaxAge)
}

func TestLogoutInternalError(t *testing.T) {
	r, uc := newRouter()
	uc.logoutErr = errors.New("redis down")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
}
