package common

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tandem/tandem/internal/pkg/ctxkeys"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
)

func newTestContext(w *httptest.ResponseRecorder, method, target string, body io.Reader, contentType string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(method, target, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	c.Request = req
	return c
}

func TestCurrentUserID(t *testing.T) {
	w := httptest.NewRecorder()
	c := newTestContext(w, http.MethodGet, "/x", nil, "")
	require.Empty(t, CurrentUserID(c))

	c.Set(ctxkeys.CtxUserID, "u1")
	require.Equal(t, "u1", CurrentUserID(c))

	c.Set(ctxkeys.CtxUserID, 42)
	require.Empty(t, CurrentUserID(c))
}

func multipartBody(t *testing.T, field, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile(field, filename)
	require.NoError(t, err)
	_, err = fw.Write(content)
	require.NoError(t, err)
	require.NoError(t, mw.Close())
	return &buf, mw.FormDataContentType()
}

func TestUploadFileSuccess(t *testing.T) {
	body, ct := multipartBody(t, "file", "doc.txt", []byte("hello"))
	w := httptest.NewRecorder()
	c := newTestContext(w, http.MethodPost, "/upload", body, ct)
	file, header, ok := UploadFile(c, 1<<20)
	require.True(t, ok)
	require.NotNil(t, file)
	defer file.Close()
	require.NotNil(t, header)
	require.Equal(t, "doc.txt", header.Filename)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestUploadFileMissing(t *testing.T) {
	w := httptest.NewRecorder()
	c := newTestContext(w, http.MethodPost, "/upload", strings.NewReader(""), "multipart/form-data")
	file, header, ok := UploadFile(c, 1<<20)
	require.False(t, ok)
	require.Nil(t, file)
	require.Nil(t, header)
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUploadFileTooLarge(t *testing.T) {
	body, ct := multipartBody(t, "file", "big.bin", bytes.Repeat([]byte("x"), 64))
	w := httptest.NewRecorder()
	c := newTestContext(w, http.MethodPost, "/upload", body, ct)
	file, header, ok := UploadFile(c, 16)
	require.False(t, ok)
	require.Nil(t, file)
	require.Nil(t, header)
	require.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestParseJSON(t *testing.T) {
	w := httptest.NewRecorder()
	c := newTestContext(w, http.MethodPost, "/x", strings.NewReader(`{"a":1}`), "application/json")
	var dst struct {
		A int `json:"a"`
	}
	require.True(t, ParseJSON(c, &dst))
	require.Equal(t, 1, dst.A)

	w2 := httptest.NewRecorder()
	c2 := newTestContext(w2, http.MethodPost, "/x", strings.NewReader("broken"), "application/json")
	require.False(t, ParseJSON(c2, &dst))
	require.Equal(t, http.StatusBadRequest, w2.Code)
}

func TestParseQuery(t *testing.T) {
	w := httptest.NewRecorder()
	c := newTestContext(w, http.MethodGet, "/x?page=2", nil, "")
	var dst struct {
		Page int `form:"page"`
	}
	require.True(t, ParseQuery(c, &dst, "bad"))
	require.Equal(t, 2, dst.Page)

	w2 := httptest.NewRecorder()
	c2 := newTestContext(w2, http.MethodGet, "/x?page=abc", nil, "")
	require.False(t, ParseQuery(c2, &dst, "bad query"))
	require.Equal(t, http.StatusBadRequest, w2.Code)
}

func TestRespondError(t *testing.T) {
	tt := []struct {
		err      error
		wantCode int
		wantBody string
	}{
		{pkgerrors.ErrValidation, http.StatusBadRequest, `{"error":"validation failed"}`},
		{pkgerrors.ErrConflict, http.StatusConflict, `{"error":"conflict"}`},
		{pkgerrors.ErrUnauthorized, http.StatusUnauthorized, `{"error":"invalid credentials"}`},
		{pkgerrors.ErrNotFound, http.StatusNotFound, `{"error":"not found"}`},
		{pkgerrors.ErrForbidden, http.StatusForbidden, `{"error":"forbidden"}`},
		{pkgerrors.ErrTooManyRequests, http.StatusTooManyRequests, `{"error":"too many requests"}`},
		{pkgerrors.ErrUnavailable, http.StatusServiceUnavailable, `{"error":"service unavailable"}`},
		{errors.New("boom"), http.StatusInternalServerError, `{"error":"internal server error"}`},
	}
	for _, tc := range tt {
		w := httptest.NewRecorder()
		c := newTestContext(w, http.MethodGet, "/x", nil, "")
		RespondError(c, tc.err)
		require.Equal(t, tc.wantCode, w.Code, "for %v", tc.err)
		require.JSONEq(t, tc.wantBody, w.Body.String(), "for %v", tc.err)
	}
}

func TestSetAndClearRefreshCookie(t *testing.T) {
	w := httptest.NewRecorder()
	SetRefreshCookie(w, "tok", 3600, true)
	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)
	ck := cookies[0]
	require.Equal(t, RefreshCookieName, ck.Name)
	require.Equal(t, "tok", ck.Value)
	require.Equal(t, "/api/v1/auth", ck.Path)
	require.True(t, ck.Secure)
	require.True(t, ck.HttpOnly)

	w2 := httptest.NewRecorder()
	ClearRefreshCookie(w2, false)
	cookies = w2.Result().Cookies()
	require.Len(t, cookies, 1)
	ck = cookies[0]
	require.Empty(t, ck.Value)
	require.Equal(t, -1, ck.MaxAge)
	require.False(t, ck.Secure)
}
