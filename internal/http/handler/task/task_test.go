package task

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
	dtask "github.com/tandem/tandem/internal/contracts/task"
	"github.com/tandem/tandem/internal/pkg/ctxkeys"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
)

type fakeTaskUC struct {
	err       error
	createOut *dtask.TaskResponse
	updateOut *dtask.TaskResponse
	getOut    *dtask.TaskDetailResponse
	listOut   []dtask.TaskResponse

	actorID   string
	workspace string
	boardID   string
	taskID    string
	createReq dtask.CreateTaskRequest
	updateReq dtask.UpdateTaskRequest
	listQuery dtask.ListWorkspaceTasksQuery
}

func (f *fakeTaskUC) Create(_ context.Context, actorID, workspaceID, boardID string, req dtask.CreateTaskRequest) (*dtask.TaskResponse, error) {
	f.actorID, f.workspace, f.boardID, f.createReq = actorID, workspaceID, boardID, req
	if f.err != nil {
		return nil, f.err
	}
	return f.createOut, nil
}

func (f *fakeTaskUC) Update(_ context.Context, actorID, workspaceID, boardID, taskID string, req dtask.UpdateTaskRequest) (*dtask.TaskResponse, error) {
	f.actorID, f.workspace, f.boardID, f.taskID, f.updateReq = actorID, workspaceID, boardID, taskID, req
	if f.err != nil {
		return nil, f.err
	}
	return f.updateOut, nil
}

func (f *fakeTaskUC) Delete(_ context.Context, actorID, workspaceID, boardID, taskID string) error {
	f.actorID, f.workspace, f.boardID, f.taskID = actorID, workspaceID, boardID, taskID
	return f.err
}

func (f *fakeTaskUC) Get(_ context.Context, actorID, workspaceID, taskID string) (*dtask.TaskDetailResponse, error) {
	f.actorID, f.workspace, f.taskID = actorID, workspaceID, taskID
	if f.err != nil {
		return nil, f.err
	}
	return f.getOut, nil
}

func (f *fakeTaskUC) List(_ context.Context, actorID, workspaceID string, query dtask.ListWorkspaceTasksQuery) ([]dtask.TaskResponse, error) {
	f.actorID, f.workspace, f.listQuery = actorID, workspaceID, query
	if f.err != nil {
		return nil, f.err
	}
	return f.listOut, nil
}

func newTaskRouter(uc *fakeTaskUC) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewTaskHandler(uc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkeys.CtxUserID, "user-1")
		c.Next()
	})
	api := r.Group("/api/v1/workspaces/:id")
	api.GET("/tasks/:taskId", h.Get)
	api.GET("/tasks", h.List)
	board := api.Group("/boards/:boardId")
	board.POST("/tasks", h.Create)
	board.PATCH("/tasks/:taskId", h.Update)
	board.DELETE("/tasks/:taskId", h.Delete)
	return r
}

func taskFixture() *dtask.TaskResponse {
	return &dtask.TaskResponse{ID: "t1", WorkspaceID: "ws1", BoardID: "b1", BoardName: "Board", ColumnID: "c1", ColumnName: "Todo", Title: "Task"}
}

func TestTaskCreate(t *testing.T) {
	uc := &fakeTaskUC{createOut: taskFixture()}
	r := newTaskRouter(uc)

	w := httptest.NewRecorder()
	body := `{"column_id":"c1","title":"New Task","is_urgent":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws1/boards/b1/tasks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var out dtask.TaskResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Equal(t, "t1", out.ID)
	require.Equal(t, "c1", uc.createReq.ColumnID)
	require.Equal(t, "New Task", uc.createReq.Title)
	require.True(t, uc.createReq.IsUrgent)
	require.Equal(t, "user-1", uc.actorID)
	require.Equal(t, "ws1", uc.workspace)
	require.Equal(t, "b1", uc.boardID)
}

func TestTaskCreateBadJSON(t *testing.T) {
	uc := &fakeTaskUC{}
	r := newTaskRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws1/boards/b1/tasks", strings.NewReader("@@"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.JSONEq(t, `{"error":"invalid request body"}`, w.Body.String())
}

func TestTaskCreateConflict(t *testing.T) {
	uc := &fakeTaskUC{err: pkgerrors.ErrConflict}
	r := newTaskRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws1/boards/b1/tasks", strings.NewReader(`{"column_id":"c1","title":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusConflict, w.Code)
	require.JSONEq(t, `{"error":"conflict"}`, w.Body.String())
}

func TestTaskUpdate(t *testing.T) {
	uc := &fakeTaskUC{updateOut: taskFixture()}
	r := newTaskRouter(uc)

	w := httptest.NewRecorder()
	body := `{"title":"Renamed","position":3}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/workspaces/ws1/boards/b1/tasks/t1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, uc.updateReq.Title)
	require.Equal(t, "Renamed", *uc.updateReq.Title)
	require.NotNil(t, uc.updateReq.Position)
	require.Equal(t, 3, *uc.updateReq.Position)
	require.Equal(t, "t1", uc.taskID)
}

func TestTaskUpdateBadJSON(t *testing.T) {
	uc := &fakeTaskUC{}
	r := newTaskRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/workspaces/ws1/boards/b1/tasks/t1", strings.NewReader(`{"title":`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTaskUpdateForbidden(t *testing.T) {
	uc := &fakeTaskUC{err: pkgerrors.ErrForbidden}
	r := newTaskRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/workspaces/ws1/boards/b1/tasks/t1", strings.NewReader(`{"title":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.JSONEq(t, `{"error":"forbidden"}`, w.Body.String())
}

func TestTaskGet(t *testing.T) {
	detail := &dtask.TaskDetailResponse{TaskResponse: *taskFixture()}
	uc := &fakeTaskUC{getOut: detail}
	r := newTaskRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws1/tasks/t1", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var out dtask.TaskDetailResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Equal(t, "t1", out.ID)
	require.Equal(t, "t1", uc.taskID)
	require.Equal(t, "ws1", uc.workspace)
}

func TestTaskGetNotFound(t *testing.T) {
	uc := &fakeTaskUC{err: pkgerrors.ErrNotFound}
	r := newTaskRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws1/tasks/t1", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.JSONEq(t, `{"error":"not found"}`, w.Body.String())
}

func TestTaskList(t *testing.T) {
	uc := &fakeTaskUC{listOut: []dtask.TaskResponse{*taskFixture()}}
	r := newTaskRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws1/tasks?q=bug&board_id=b1&only=mine&exclude_subtasks=true", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "bug", uc.listQuery.Q)
	require.Equal(t, "b1", uc.listQuery.BoardID)
	require.Equal(t, "mine", uc.listQuery.Only)
	require.True(t, uc.listQuery.ExcludeSubtasks)
	require.Equal(t, "user-1", uc.actorID)
}

func TestTaskListBadQuery(t *testing.T) {
	uc := &fakeTaskUC{}
	r := newTaskRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws1/tasks?limit=abc", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.JSONEq(t, `{"error":"invalid query"}`, w.Body.String())
}

func TestTaskListEmpty(t *testing.T) {
	uc := &fakeTaskUC{listOut: []dtask.TaskResponse{}}
	r := newTaskRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws1/tasks", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `[]`, w.Body.String())
}

func TestTaskDelete(t *testing.T) {
	uc := &fakeTaskUC{}
	r := newTaskRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/ws1/boards/b1/tasks/t1", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"message":"task deleted"}`, w.Body.String())
	require.Equal(t, "t1", uc.taskID)
	require.Equal(t, "b1", uc.boardID)
}

func TestTaskDeleteGenericError(t *testing.T) {
	uc := &fakeTaskUC{err: errors.New("boom")}
	r := newTaskRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/ws1/boards/b1/tasks/t1", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.JSONEq(t, `{"error":"internal server error"}`, w.Body.String())
}
