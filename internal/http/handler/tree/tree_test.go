package tree

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	boardcontracts "github.com/tandem/tandem/internal/contracts/board"
	dtree "github.com/tandem/tandem/internal/contracts/tree"
	workspacecontracts "github.com/tandem/tandem/internal/contracts/workspace"
	"github.com/tandem/tandem/internal/pkg/ctxkeys"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
)

type fakeTreeUC struct {
	err     error
	out     []dtree.TreeWorkspaceResponse
	query   dtree.TreeQuery
	actorID string
}

func (f *fakeTreeUC) List(_ context.Context, actorID string, query dtree.TreeQuery) ([]dtree.TreeWorkspaceResponse, error) {
	f.actorID, f.query = actorID, query
	if f.err != nil {
		return nil, f.err
	}
	return f.out, nil
}

func newTreeRouter(uc *fakeTreeUC) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewTreeHandler(uc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkeys.CtxUserID, "user-1")
		c.Next()
	})
	r.GET("/api/v1/tasks/tree", h.List)
	return r
}

func TestTreeList(t *testing.T) {
	uc := &fakeTreeUC{out: []dtree.TreeWorkspaceResponse{{
		Workspace: workspacecontracts.WorkspaceResponse{ID: "ws1", Name: "WS"},
		Boards: []dtree.TreeBoardResponse{{
			Board: boardcontracts.BoardResponse{ID: "b1", Name: "Board"},
		}},
	}}}
	r := newTreeRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/tree?tasks=all&boards=fav&workspaces=all", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var out []dtree.TreeWorkspaceResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Len(t, out, 1)
	require.Equal(t, "ws1", out[0].Workspace.ID)
	require.Len(t, out[0].Boards, 1)
	require.Equal(t, "all", uc.query.Tasks)
	require.Equal(t, "fav", uc.query.Boards)
	require.Equal(t, "all", uc.query.Workspaces)
	require.Equal(t, "user-1", uc.actorID)
}

func TestTreeListQueryDefaults(t *testing.T) {
	uc := &fakeTreeUC{out: []dtree.TreeWorkspaceResponse{}}
	r := newTreeRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/tree", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `[]`, w.Body.String())
	require.Equal(t, "", uc.query.Tasks)
}

func TestTreeListMine(t *testing.T) {
	uc := &fakeTreeUC{out: []dtree.TreeWorkspaceResponse{}}
	r := newTreeRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/tree?tasks=mine", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "mine", uc.query.Tasks)
}

func TestTreeListError(t *testing.T) {
	uc := &fakeTreeUC{err: pkgerrors.ErrUnauthorized}
	r := newTreeRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/tree", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.JSONEq(t, `{"error":"invalid credentials"}`, w.Body.String())
}
