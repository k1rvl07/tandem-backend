package file

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tandem/tandem/internal/pkg/ctxkeys"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	fileuc "github.com/tandem/tandem/internal/usecase/file"
	"go.uber.org/zap"
)

type fakeFileStore struct {
	putErr     error
	deleteErr  error
	presignURL string
	presignErr error
	putCalls   []string
}

func (f *fakeFileStore) Put(_ context.Context, key string, _ io.Reader, _ int64, _ string) error {
	f.putCalls = append(f.putCalls, key)
	return f.putErr
}

func (f *fakeFileStore) Get(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (f *fakeFileStore) Delete(_ context.Context, key string) error {
	return f.deleteErr
}

func (f *fakeFileStore) Exists(context.Context, string) (bool, error) {
	return true, nil
}

func (f *fakeFileStore) PresignGet(_ context.Context, _ string, _ time.Duration) (string, error) {
	if f.presignErr != nil {
		return "", f.presignErr
	}
	return f.presignURL, nil
}

func newFileRouter(store *fakeFileStore) (*gin.Engine, *FileHandler) {
	gin.SetMode(gin.TestMode)
	svc := fileuc.NewService(store, zap.NewNop())
	h := NewFileHandler(svc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkeys.CtxUserID, testUserID)
		c.Next()
	})
	r.POST("/api/v1/files/images", h.UploadImage)
	r.DELETE("/api/v1/files/images", h.DeleteImage)
	r.GET("/api/v1/files/sign", h.Sign)
	return r, h
}

var testUserID = "11111111-1111-1111-1111-111111111111"

func imageMultipart(t *testing.T, filename, contentType string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	h.Set("Content-Type", contentType)
	part, err := mw.CreatePart(h)
	require.NoError(t, err)
	_, err = part.Write([]byte("img-bytes"))
	require.NoError(t, err)
	require.NoError(t, mw.Close())
	return &buf, mw.FormDataContentType()
}

func TestFileUploadImageDefaultsAvatars(t *testing.T) {
	store := &fakeFileStore{}
	r, _ := newFileRouter(store)

	buf, ct := imageMultipart(t, "photo.png", "image/png")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/files/images", buf)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	require.JSONEq(t, `{"key":"`+store.putCalls[0]+`"}`, w.Body.String())
	require.True(t, strings.HasPrefix(store.putCalls[0], "avatars/"+testUserID+"/"))
}

func TestFileUploadImageCustomNamespace(t *testing.T) {
	store := &fakeFileStore{}
	r, _ := newFileRouter(store)

	buf, ct := imageMultipart(t, "cover.webp", "image/webp")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/files/images?namespace=covers", buf)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	require.True(t, strings.HasPrefix(store.putCalls[0], "covers/"))
}

func TestFileUploadImageInvalidNamespace(t *testing.T) {
	store := &fakeFileStore{}
	r, _ := newFileRouter(store)

	buf, ct := imageMultipart(t, "photo.png", "image/png")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/files/images?namespace=attachments", buf)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.JSONEq(t, `{"error":"invalid namespace"}`, w.Body.String())
	require.Len(t, store.putCalls, 0)
}

func TestFileUploadImageUnsupportedExt(t *testing.T) {
	store := &fakeFileStore{}
	r, _ := newFileRouter(store)

	buf, ct := imageMultipart(t, "doc.txt", "text/plain")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/files/images", buf)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.JSONEq(t, `{"error":"unsupported image type"}`, w.Body.String())
}

func TestFileUploadImageNoFile(t *testing.T) {
	store := &fakeFileStore{}
	r, _ := newFileRouter(store)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/files/images", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.JSONEq(t, `{"error":"file is required"}`, w.Body.String())
}

func TestFileUploadImageStoreError(t *testing.T) {
	store := &fakeFileStore{putErr: pkgerrors.ErrInternal}
	r, _ := newFileRouter(store)

	buf, ct := imageMultipart(t, "photo.png", "image/png")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/files/images", buf)
	req.Header.Set("Content-Type", ct)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.JSONEq(t, `{"error":"internal server error"}`, w.Body.String())
}

func validKey(owner, namespace string) string {
	return namespace + "/" + owner + "/" + uuid.NewString() + ".png"
}

func TestFileDeleteImage(t *testing.T) {
	store := &fakeFileStore{}
	r, _ := newFileRouter(store)

	key := validKey(testUserID, "avatars")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/files/images?key="+key, nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)
	require.Empty(t, w.Body.String())
}

func TestFileDeleteImageForeignOwner(t *testing.T) {
	store := &fakeFileStore{}
	r, _ := newFileRouter(store)

	key := validKey("someone-else", "avatars")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/files/images?key="+key, nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.JSONEq(t, `{"error":"forbidden"}`, w.Body.String())
}

func TestFileDeleteImageMalformedKey(t *testing.T) {
	store := &fakeFileStore{}
	r, _ := newFileRouter(store)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/files/images?key=avatars/"+testUserID+"/notauuid.png", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.JSONEq(t, `{"error":"invalid image key"}`, w.Body.String())
}

func TestFileSignAvatar(t *testing.T) {
	store := &fakeFileStore{presignURL: "https://s3.example/presigned"}
	r, _ := newFileRouter(store)

	key := validKey(testUserID, "avatars")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/sign?key="+key, nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"url":"https://s3.example/presigned"}`, w.Body.String())
}

func TestFileSignMissingKey(t *testing.T) {
	store := &fakeFileStore{}
	r, _ := newFileRouter(store)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/sign", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.JSONEq(t, `{"error":"missing key"}`, w.Body.String())
}

func TestFileSignMalformedKey(t *testing.T) {
	store := &fakeFileStore{}
	r, _ := newFileRouter(store)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/sign?key=garbage", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.JSONEq(t, `{"error":"not found"}`, w.Body.String())
}

func TestFileSignNonAvatarForbiddenWithoutAccessCheck(t *testing.T) {
	store := &fakeFileStore{}
	r, _ := newFileRouter(store)

	key := validKey("22222222-2222-2222-2222-222222222222", "images")
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/files/sign?key="+key, nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.JSONEq(t, `{"error":"forbidden"}`, w.Body.String())
}
