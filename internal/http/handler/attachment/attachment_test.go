package attachment

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

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	dattachment "github.com/tandem/tandem/internal/contracts/attachment"
	"github.com/tandem/tandem/internal/pkg/ctxkeys"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
)

type fakeAttachUC struct {
	err         error
	createOut   *dattachment.AttachmentResponse
	listOut     []dattachment.AttachmentResponse
	downloadOut *dattachment.AttachmentResponse
	downloadRC  io.ReadCloser

	actorID    string
	workspace  string
	taskID     string
	attachID   string
	filename   string
	contentTyp string
	size       int64
}

func (f *fakeAttachUC) Create(_ context.Context, actorID, workspaceID, taskID, filename, contentType string, _ io.Reader, size int64) (*dattachment.AttachmentResponse, error) {
	f.actorID, f.workspace, f.taskID, f.filename, f.contentTyp, f.size = actorID, workspaceID, taskID, filename, contentType, size
	if f.err != nil {
		return nil, f.err
	}
	return f.createOut, nil
}

func (f *fakeAttachUC) List(_ context.Context, actorID, workspaceID, taskID string) ([]dattachment.AttachmentResponse, error) {
	f.actorID, f.workspace, f.taskID = actorID, workspaceID, taskID
	if f.err != nil {
		return nil, f.err
	}
	return f.listOut, nil
}

func (f *fakeAttachUC) Download(_ context.Context, actorID, workspaceID, taskID, attachmentID string) (*dattachment.AttachmentResponse, io.ReadCloser, error) {
	f.actorID, f.workspace, f.taskID, f.attachID = actorID, workspaceID, taskID, attachmentID
	if f.err != nil {
		return nil, nil, f.err
	}
	return f.downloadOut, f.downloadRC, nil
}

func (f *fakeAttachUC) Delete(_ context.Context, actorID, workspaceID, taskID, attachmentID string) error {
	f.actorID, f.workspace, f.taskID, f.attachID = actorID, workspaceID, taskID, attachmentID
	return f.err
}

func newAttachRouter(uc *fakeAttachUC) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewAttachmentHandler(uc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkeys.CtxUserID, "user-1")
		c.Next()
	})
	api := r.Group("/api/v1/workspaces/:id/tasks/:taskId/attachments")
	api.POST("", h.Create)
	api.GET("", h.List)
	api.GET("/:attachmentId", h.Download)
	api.DELETE("/:attachmentId", h.Delete)
	return r
}

func attachFixture() *dattachment.AttachmentResponse {
	return &dattachment.AttachmentResponse{ID: "a1", TaskID: "t1", Filename: "doc.pdf", ContentType: "application/pdf", Size: 10}
}

func TestAttachmentCreate(t *testing.T) {
	uc := &fakeAttachUC{createOut: attachFixture()}
	r := newAttachRouter(uc)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="doc.pdf"`)
	h.Set("Content-Type", "application/pdf")
	part, err := mw.CreatePart(h)
	require.NoError(t, err)
	_, err = part.Write([]byte("payload"))
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws1/tasks/t1/attachments", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var out dattachment.AttachmentResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Equal(t, "a1", out.ID)
	require.Equal(t, "doc.pdf", uc.filename)
	require.Equal(t, "application/pdf", uc.contentTyp)
	require.Greater(t, uc.size, int64(0))
	require.Equal(t, "ws1", uc.workspace)
	require.Equal(t, "t1", uc.taskID)
	require.Equal(t, "user-1", uc.actorID)
}

func TestAttachmentCreateNoFile(t *testing.T) {
	uc := &fakeAttachUC{}
	r := newAttachRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws1/tasks/t1/attachments", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.JSONEq(t, `{"error":"file is required"}`, w.Body.String())
}

func TestAttachmentCreateForbidden(t *testing.T) {
	uc := &fakeAttachUC{err: pkgerrors.ErrForbidden}
	r := newAttachRouter(uc)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="a.pdf"`)
	part, err := mw.CreatePart(h)
	require.NoError(t, err)
	_, err = part.Write([]byte("x"))
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws1/tasks/t1/attachments", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.JSONEq(t, `{"error":"forbidden"}`, w.Body.String())
}

func TestAttachmentList(t *testing.T) {
	uc := &fakeAttachUC{listOut: []dattachment.AttachmentResponse{*attachFixture()}}
	r := newAttachRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws1/tasks/t1/attachments", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var out []dattachment.AttachmentResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Len(t, out, 1)
	require.Equal(t, "t1", uc.taskID)
}

func TestAttachmentDownload(t *testing.T) {
	uc := &fakeAttachUC{
		downloadOut: attachFixture(),
		downloadRC:  io.NopCloser(strings.NewReader("file-content")),
	}
	r := newAttachRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws1/tasks/t1/attachments/a1", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
	require.Equal(t, `attachment; filename="doc.pdf"`, w.Header().Get("Content-Disposition"))
	require.Equal(t, "file-content", w.Body.String())
	require.Equal(t, "a1", uc.attachID)
}

func TestAttachmentDownloadDefaultContentType(t *testing.T) {
	fx := attachFixture()
	fx.ContentType = ""
	uc := &fakeAttachUC{downloadOut: fx, downloadRC: io.NopCloser(strings.NewReader("x"))}
	r := newAttachRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws1/tasks/t1/attachments/a1", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/octet-stream", w.Header().Get("Content-Type"))
}

func TestSanitizeFilename(t *testing.T) {
	require.Equal(t, "ab.pdf", sanitizeFilename("a\\\"b\r\n.pdf"))
}

func TestAttachmentDelete(t *testing.T) {
	uc := &fakeAttachUC{}
	r := newAttachRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/ws1/tasks/t1/attachments/a1", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"message":"attachment deleted"}`, w.Body.String())
	require.Equal(t, "a1", uc.attachID)
}

func TestAttachmentNotFound(t *testing.T) {
	uc := &fakeAttachUC{err: pkgerrors.ErrNotFound}
	r := newAttachRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws1/tasks/t1/attachments/a1", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.JSONEq(t, `{"error":"not found"}`, w.Body.String())
}
