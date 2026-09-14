package board

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	dboard "github.com/tandem/tandem/internal/contracts/board"
	"github.com/tandem/tandem/internal/pkg/ctxkeys"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
)

type fakeBoardUC struct {
	err        error
	listOut    []dboard.BoardResponse
	createOut  *dboard.BoardResponse
	getOut     *dboard.BoardDetailResponse
	updateOut  *dboard.BoardResponse
	setMainOut *dboard.BoardResponse
	reorderOut []dboard.BoardResponse

	actorID    string
	workspace  string
	boardID    string
	createReq  dboard.CreateBoardRequest
	updateReq  dboard.UpdateBoardRequest
	reorderReq dboard.ReorderBoardsRequest
}

func (f *fakeBoardUC) Create(_ context.Context, actorID, workspaceID string, req dboard.CreateBoardRequest) (*dboard.BoardResponse, error) {
	f.actorID, f.workspace, f.createReq = actorID, workspaceID, req
	if f.err != nil {
		return nil, f.err
	}
	return f.createOut, nil
}

func (f *fakeBoardUC) List(_ context.Context, actorID, workspaceID string) ([]dboard.BoardResponse, error) {
	f.actorID, f.workspace = actorID, workspaceID
	if f.err != nil {
		return nil, f.err
	}
	return f.listOut, nil
}

func (f *fakeBoardUC) Get(_ context.Context, actorID, workspaceID, boardID string) (*dboard.BoardDetailResponse, error) {
	f.actorID, f.workspace, f.boardID = actorID, workspaceID, boardID
	if f.err != nil {
		return nil, f.err
	}
	return f.getOut, nil
}

func (f *fakeBoardUC) Update(_ context.Context, actorID, workspaceID, boardID string, req dboard.UpdateBoardRequest) (*dboard.BoardResponse, error) {
	f.actorID, f.workspace, f.boardID, f.updateReq = actorID, workspaceID, boardID, req
	if f.err != nil {
		return nil, f.err
	}
	return f.updateOut, nil
}

func (f *fakeBoardUC) Delete(_ context.Context, actorID, workspaceID, boardID string) error {
	f.actorID, f.workspace, f.boardID = actorID, workspaceID, boardID
	return f.err
}

func (f *fakeBoardUC) SetMain(_ context.Context, actorID, workspaceID, boardID string) (*dboard.BoardResponse, error) {
	f.actorID, f.workspace, f.boardID = actorID, workspaceID, boardID
	if f.err != nil {
		return nil, f.err
	}
	return f.setMainOut, nil
}

func (f *fakeBoardUC) Reorder(_ context.Context, actorID, workspaceID string, req dboard.ReorderBoardsRequest) ([]dboard.BoardResponse, error) {
	f.actorID, f.workspace, f.reorderReq = actorID, workspaceID, req
	if f.err != nil {
		return nil, f.err
	}
	return f.reorderOut, nil
}

func newBoardRouter(uc *fakeBoardUC) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewBoardHandler(uc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkeys.CtxUserID, "user-1")
		c.Next()
	})
	api := r.Group("/api/v1/workspaces/:id/boards")
	api.GET("", h.List)
	api.POST("", h.Create)
	api.PUT("/reorder", h.Reorder)
	api.GET("/:boardId", h.Get)
	api.PUT("/:boardId/main", h.SetMain)
	api.PATCH("/:boardId", h.Update)
	api.DELETE("/:boardId", h.Delete)
	return r
}

func boardFixture() *dboard.BoardResponse {
	return &dboard.BoardResponse{ID: "b1", WorkspaceID: "ws1", Name: "Board", Position: 0, IsMain: true}
}

func TestBoardList(t *testing.T) {
	uc := &fakeBoardUC{listOut: []dboard.BoardResponse{*boardFixture()}}
	r := newBoardRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws1/boards", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var out []dboard.BoardResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Len(t, out, 1)
	require.Equal(t, "b1", out[0].ID)
	require.Equal(t, "user-1", uc.actorID)
	require.Equal(t, "ws1", uc.workspace)
}

func TestBoardListError(t *testing.T) {
	uc := &fakeBoardUC{err: pkgerrors.ErrForbidden}
	r := newBoardRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws1/boards", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.JSONEq(t, `{"error":"forbidden"}`, w.Body.String())
}

func TestBoardCreate(t *testing.T) {
	uc := &fakeBoardUC{createOut: boardFixture()}
	r := newBoardRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws1/boards", strings.NewReader(`{"name":"New Board"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var out dboard.BoardResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Equal(t, "New Board", uc.createReq.Name)
	require.Equal(t, "ws1", uc.workspace)
}

func TestBoardCreateBadJSON(t *testing.T) {
	uc := &fakeBoardUC{}
	r := newBoardRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws1/boards", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.JSONEq(t, `{"error":"invalid request body"}`, w.Body.String())
}

func TestBoardCreateNotFound(t *testing.T) {
	uc := &fakeBoardUC{err: pkgerrors.ErrNotFound}
	r := newBoardRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws1/boards", strings.NewReader(`{"name":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.JSONEq(t, `{"error":"not found"}`, w.Body.String())
}

func TestBoardGet(t *testing.T) {
	uc := &fakeBoardUC{getOut: &dboard.BoardDetailResponse{ID: "b1", WorkspaceID: "ws1", Name: "Board"}}
	r := newBoardRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws1/boards/b1", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var out dboard.BoardDetailResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Equal(t, "b1", out.ID)
	require.Equal(t, "b1", uc.boardID)
}

func TestBoardGetUnauthorized(t *testing.T) {
	uc := &fakeBoardUC{err: pkgerrors.ErrUnauthorized}
	r := newBoardRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws1/boards/b1", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.JSONEq(t, `{"error":"invalid credentials"}`, w.Body.String())
}

func TestBoardSetMain(t *testing.T) {
	uc := &fakeBoardUC{setMainOut: boardFixture()}
	r := newBoardRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/workspaces/ws1/boards/b1/main", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "b1", uc.boardID)
	require.True(t, uc.setMainOut.IsMain)
}

func TestBoardReorder(t *testing.T) {
	uc := &fakeBoardUC{reorderOut: []dboard.BoardResponse{*boardFixture()}}
	r := newBoardRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/workspaces/ws1/boards/reorder", strings.NewReader(`{"board_ids":["b2","b1"]}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, []string{"b2", "b1"}, uc.reorderReq.BoardIDs)
}

func TestBoardReorderBadJSON(t *testing.T) {
	uc := &fakeBoardUC{}
	r := newBoardRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/workspaces/ws1/boards/reorder", strings.NewReader("nope"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBoardUpdate(t *testing.T) {
	uc := &fakeBoardUC{updateOut: boardFixture()}
	r := newBoardRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/workspaces/ws1/boards/b1", strings.NewReader(`{"name":"Renamed"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "Renamed", uc.updateReq.Name)
}

func TestBoardUpdateBadJSON(t *testing.T) {
	uc := &fakeBoardUC{}
	r := newBoardRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/workspaces/ws1/boards/b1", strings.NewReader(`{`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.JSONEq(t, `{"error":"invalid request body"}`, w.Body.String())
}

func TestBoardDelete(t *testing.T) {
	uc := &fakeBoardUC{}
	r := newBoardRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/ws1/boards/b1", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"message":"board deleted"}`, w.Body.String())
	require.Equal(t, "b1", uc.boardID)
}

func TestBoardDeleteError(t *testing.T) {
	uc := &fakeBoardUC{err: pkgerrors.NewValidationError("invalid board id")}
	r := newBoardRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/ws1/boards/b1", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.JSONEq(t, `{"error":"invalid board id"}`, w.Body.String())
}

func TestBoardGenericErrorMapsTo500(t *testing.T) {
	uc := &fakeBoardUC{err: errors.New("boom")}
	r := newBoardRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws1/boards", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.JSONEq(t, `{"error":"internal server error"}`, w.Body.String())
}
