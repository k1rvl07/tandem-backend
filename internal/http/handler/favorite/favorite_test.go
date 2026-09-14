package favorite

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tandem/tandem/internal/pkg/ctxkeys"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
)

type fakeFavUC struct {
	err      error
	actorID  string
	target   string
	targetID string
}

func (f *fakeFavUC) Add(_ context.Context, actorID, targetType, targetID string) error {
	f.actorID, f.target, f.targetID = actorID, targetType, targetID
	return f.err
}

func (f *fakeFavUC) Remove(_ context.Context, actorID, targetType, targetID string) error {
	f.actorID, f.target, f.targetID = actorID, targetType, targetID
	return f.err
}

func newFavRouter(uc *fakeFavUC) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewFavoriteHandler(uc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkeys.CtxUserID, "user-1")
		c.Next()
	})
	r.PUT("/api/v1/favorites/workspaces/:workspaceId", h.AddWorkspace)
	r.DELETE("/api/v1/favorites/workspaces/:workspaceId", h.RemoveWorkspace)
	r.PUT("/api/v1/favorites/boards/:boardId", h.AddBoard)
	r.DELETE("/api/v1/favorites/boards/:boardId", h.RemoveBoard)
	return r
}

func TestAddWorkspace(t *testing.T) {
	uc := &fakeFavUC{}
	r := newFavRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/favorites/workspaces/ws1", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"message":"favorited"}`, w.Body.String())
	require.Equal(t, "user-1", uc.actorID)
	require.Equal(t, "workspace", uc.target)
	require.Equal(t, "ws1", uc.targetID)
}

func TestRemoveWorkspace(t *testing.T) {
	uc := &fakeFavUC{}
	r := newFavRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/favorites/workspaces/ws1", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"message":"unfavorited"}`, w.Body.String())
	require.Equal(t, "workspace", uc.target)
}

func TestAddBoard(t *testing.T) {
	uc := &fakeFavUC{}
	r := newFavRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/favorites/boards/b1", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"message":"favorited"}`, w.Body.String())
	require.Equal(t, "board", uc.target)
	require.Equal(t, "b1", uc.targetID)
}

func TestRemoveBoard(t *testing.T) {
	uc := &fakeFavUC{}
	r := newFavRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/favorites/boards/b1", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"message":"unfavorited"}`, w.Body.String())
	require.Equal(t, "board", uc.target)
}

func TestAddWorkspaceForbidden(t *testing.T) {
	uc := &fakeFavUC{err: pkgerrors.ErrForbidden}
	r := newFavRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/favorites/workspaces/ws1", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.JSONEq(t, `{"error":"forbidden"}`, w.Body.String())
}

func TestRemoveBoardNotFound(t *testing.T) {
	uc := &fakeFavUC{err: pkgerrors.ErrNotFound}
	r := newFavRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/favorites/boards/b1", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.JSONEq(t, `{"error":"not found"}`, w.Body.String())
}
