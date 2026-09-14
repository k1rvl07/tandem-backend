package profile

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	dauth "github.com/tandem/tandem/internal/contracts/auth"
	dprofile "github.com/tandem/tandem/internal/contracts/profile"
	"github.com/tandem/tandem/internal/http/handler/common"
	"github.com/tandem/tandem/internal/pkg/ctxkeys"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
)

type fakeProfile struct {
	err         error
	getOut      *dauth.UserResponse
	updateOut   *dauth.UserResponse
	passwordOut *dprofile.ChangePasswordResponse
	avatarOut   *dauth.UserResponse

	userID      string
	updateReq   dprofile.UpdateProfileRequest
	passwordReq dprofile.ChangePasswordRequest
	avatarName  string
	avatarCT    string
	avatarSize  int64
}

func (f *fakeProfile) Get(_ context.Context, userID string) (*dauth.UserResponse, error) {
	f.userID = userID
	if f.err != nil {
		return nil, f.err
	}
	return f.getOut, nil
}

func (f *fakeProfile) UpdateProfile(_ context.Context, userID string, req dprofile.UpdateProfileRequest) (*dauth.UserResponse, error) {
	f.userID, f.updateReq = userID, req
	if f.err != nil {
		return nil, f.err
	}
	return f.updateOut, nil
}

func (f *fakeProfile) ChangePassword(_ context.Context, userID string, req dprofile.ChangePasswordRequest) (*dprofile.ChangePasswordResponse, error) {
	f.userID, f.passwordReq = userID, req
	if f.err != nil {
		return nil, f.err
	}
	return f.passwordOut, nil
}

func (f *fakeProfile) UploadAvatar(_ context.Context, userID, filename, contentType string, _ io.Reader, size int64) (*dauth.UserResponse, error) {
	f.userID, f.avatarName, f.avatarCT, f.avatarSize = userID, filename, contentType, size
	if f.err != nil {
		return nil, f.err
	}
	return f.avatarOut, nil
}

func (f *fakeProfile) RemoveAvatar(_ context.Context, userID string) (*dauth.UserResponse, error) {
	f.userID = userID
	if f.err != nil {
		return nil, f.err
	}
	return f.avatarOut, nil
}

func newProfileRouter(uc *fakeProfile) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewProfileHandler(uc, false, time.Hour)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkeys.CtxUserID, "user-1")
		c.Next()
	})
	r.GET("/api/v1/me", h.GetProfile)
	r.PATCH("/api/v1/me", h.UpdateProfile)
	r.POST("/api/v1/me/avatar", h.UploadAvatar)
	r.DELETE("/api/v1/me/avatar", h.DeleteAvatar)
	r.POST("/api/v1/me/password", h.ChangePassword)
	return r
}

func userFixture() *dauth.UserResponse {
	return &dauth.UserResponse{ID: "user-1", Login: "ivan", Role: "user", DisplayName: "Ivan"}
}

func multipartBody(t *testing.T, filename, contentType, data string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	h.Set("Content-Type", contentType)
	part, err := mw.CreatePart(h)
	require.NoError(t, err)
	_, err = part.Write([]byte(data))
	require.NoError(t, err)
	require.NoError(t, mw.Close())
	return &buf, mw.FormDataContentType()
}

func TestProfileGet(t *testing.T) {
	uc := &fakeProfile{getOut: userFixture()}
	r := newProfileRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var out dauth.UserResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Equal(t, "ivan", out.Login)
	require.Equal(t, "user-1", uc.userID)
}

func TestProfileGetNotFound(t *testing.T) {
	uc := &fakeProfile{err: pkgerrors.ErrNotFound}
	r := newProfileRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.JSONEq(t, `{"error":"not found"}`, w.Body.String())
}

func TestProfileUpdate(t *testing.T) {
	uc := &fakeProfile{updateOut: userFixture()}
	r := newProfileRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/me", strings.NewReader(`{"display_name":"Petr","bio":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "Petr", uc.updateReq.DisplayName)
	require.Equal(t, "hi", uc.updateReq.Bio)
}

func TestProfileUpdateBadJSON(t *testing.T) {
	uc := &fakeProfile{}
	r := newProfileRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/me", strings.NewReader(`{bad`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.JSONEq(t, `{"error":"invalid request body"}`, w.Body.String())
}

func TestProfileUploadAvatar(t *testing.T) {
	uc := &fakeProfile{avatarOut: userFixture()}
	r := newProfileRouter(uc)

	buf, ct := multipartBody(t, "avatar.png", "image/png", "fake-img")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/avatar", buf)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "avatar.png", uc.avatarName)
	require.Equal(t, "image/png", uc.avatarCT)
	require.Greater(t, uc.avatarSize, int64(0))
}

func TestProfileUploadAvatarNoFile(t *testing.T) {
	uc := &fakeProfile{}
	r := newProfileRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/avatar", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.JSONEq(t, `{"error":"file is required"}`, w.Body.String())
}

func TestProfileUploadAvatarError(t *testing.T) {
	uc := &fakeProfile{err: pkgerrors.NewValidationError("unsupported image type")}
	r := newProfileRouter(uc)

	buf, ct := multipartBody(t, "avatar.txt", "text/plain", "fake-img")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/avatar", buf)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.JSONEq(t, `{"error":"unsupported image type"}`, w.Body.String())
}

func TestProfileDeleteAvatar(t *testing.T) {
	uc := &fakeProfile{avatarOut: userFixture()}
	r := newProfileRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/me/avatar", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestProfileChangePassword(t *testing.T) {
	uc := &fakeProfile{passwordOut: &dprofile.ChangePasswordResponse{Token: "new-access", RefreshToken: "new-refresh"}}
	r := newProfileRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/password", strings.NewReader(`{"new_password":"secret2","current_password":"secret1"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"token":"new-access","message":"password updated"}`, w.Body.String())
	require.Equal(t, "secret2", uc.passwordReq.NewPassword)
	require.Equal(t, "secret1", uc.passwordReq.CurrentPassword)
	require.Contains(t, w.Header().Get("Set-Cookie"), common.RefreshCookieName+"=")
}

func TestProfileChangePasswordUnauthorized(t *testing.T) {
	uc := &fakeProfile{err: pkgerrors.ErrUnauthorized}
	r := newProfileRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/password", strings.NewReader(`{"new_password":"a","current_password":"b"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.JSONEq(t, `{"error":"invalid credentials"}`, w.Body.String())
}
