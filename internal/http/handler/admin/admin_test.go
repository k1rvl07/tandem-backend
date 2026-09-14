package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	dadmin "github.com/tandem/tandem/internal/contracts/admin"
	dauth "github.com/tandem/tandem/internal/contracts/auth"
	mwstaff "github.com/tandem/tandem/internal/http/middleware/require_staff"
	"github.com/tandem/tandem/internal/pkg/ctxkeys"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	padmin "github.com/tandem/tandem/internal/usecase/admin"
)

type fakeAdminUC struct {
	err       error
	createOut *dauth.UserResponse
	updateOut *dauth.UserResponse
	pageOut   dadmin.AdminPage

	actor     padmin.Actor
	createReq dadmin.CreateUserRequest
	updateReq dadmin.UpdateUserRoleRequest
	listQuery dadmin.AdminListQuery
	targetID  string
}

func (f *fakeAdminUC) CreateUser(_ context.Context, actor padmin.Actor, req dadmin.CreateUserRequest) (*dauth.UserResponse, error) {
	f.actor, f.createReq = actor, req
	if f.err != nil {
		return nil, f.err
	}
	return f.createOut, nil
}

func (f *fakeAdminUC) ListUsers(_ context.Context, query dadmin.AdminListQuery) (dadmin.AdminPage, error) {
	f.listQuery = query
	if f.err != nil {
		return dadmin.AdminPage{}, f.err
	}
	return f.pageOut, nil
}

func (f *fakeAdminUC) UpdateUserRole(_ context.Context, actor padmin.Actor, targetID string, req dadmin.UpdateUserRoleRequest) (*dauth.UserResponse, error) {
	f.actor, f.targetID, f.updateReq = actor, targetID, req
	if f.err != nil {
		return nil, f.err
	}
	return f.updateOut, nil
}

func (f *fakeAdminUC) DeleteUser(_ context.Context, actor padmin.Actor, targetID string) error {
	f.actor, f.targetID = actor, targetID
	return f.err
}

func newAdminRouter(uc *fakeAdminUC) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewAdminHandler(uc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkeys.CtxUserID, "admin-1")
		c.Set(mwstaff.CtxUserRole, "admin")
		c.Next()
	})
	api := r.Group("/api/v1/admin/users")
	api.POST("", h.CreateUser)
	api.GET("", h.ListUsers)
	api.DELETE("/:id", h.DeleteUser)
	api.PATCH("/:id/role", h.UpdateUserRole)
	return r
}

func userFixture() *dauth.UserResponse {
	return &dauth.UserResponse{ID: "u1", Login: "ivan", Role: "user", DisplayName: "Ivan"}
}

func TestAdminCreateUser(t *testing.T) {
	uc := &fakeAdminUC{createOut: userFixture()}
	r := newAdminRouter(uc)

	w := httptest.NewRecorder()
	body := `{"login":"newbie","password":"secret123","display_name":"Newbie","role":"moderator"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var out dauth.UserResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Equal(t, "ivan", out.Login)
	require.Equal(t, "admin-1", uc.actor.ID)
	require.Equal(t, "admin", uc.actor.Role)
	require.Equal(t, "newbie", uc.createReq.Login)
	require.Equal(t, "moderator", uc.createReq.Role)
}

func TestAdminCreateUserBadJSON(t *testing.T) {
	uc := &fakeAdminUC{}
	r := newAdminRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", strings.NewReader("oops"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.JSONEq(t, `{"error":"invalid request body"}`, w.Body.String())
}

func TestAdminCreateUserConflict(t *testing.T) {
	uc := &fakeAdminUC{err: pkgerrors.ErrConflict}
	r := newAdminRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", strings.NewReader(`{"login":"a","password":"b"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusConflict, w.Code)
	require.JSONEq(t, `{"error":"conflict"}`, w.Body.String())
}

func TestAdminListUsers(t *testing.T) {
	uc := &fakeAdminUC{pageOut: dadmin.AdminPage{Total: 1, Page: 1, PageSize: 20, Items: []dauth.UserResponse{*userFixture()}}}
	r := newAdminRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?page=2&page_size=10&q=iv", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var out dadmin.AdminPage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Equal(t, 1, out.Total)
	require.Equal(t, 2, uc.listQuery.Page)
	require.Equal(t, 10, uc.listQuery.PageSize)
	require.Equal(t, "iv", uc.listQuery.Query)
}

func TestAdminListUsersDefaults(t *testing.T) {
	uc := &fakeAdminUC{pageOut: dadmin.AdminPage{}}
	r := newAdminRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 0, uc.listQuery.Page)
	require.Equal(t, 0, uc.listQuery.PageSize)
	require.Equal(t, "", uc.listQuery.Query)
}

func TestAdminUpdateUserRole(t *testing.T) {
	uc := &fakeAdminUC{updateOut: userFixture()}
	r := newAdminRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/u1/role", strings.NewReader(`{"role":"moderator"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "u1", uc.targetID)
	require.Equal(t, "moderator", uc.updateReq.Role)
}

func TestAdminUpdateUserRoleBadJSON(t *testing.T) {
	uc := &fakeAdminUC{}
	r := newAdminRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/u1/role", strings.NewReader(`{`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAdminUpdateUserRoleNotFound(t *testing.T) {
	uc := &fakeAdminUC{err: pkgerrors.ErrNotFound}
	r := newAdminRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/users/u1/role", strings.NewReader(`{"role":"admin"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.JSONEq(t, `{"error":"not found"}`, w.Body.String())
}

func TestAdminDeleteUser(t *testing.T) {
	uc := &fakeAdminUC{}
	r := newAdminRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/u1", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"message":"user deleted"}`, w.Body.String())
	require.Equal(t, "u1", uc.targetID)
	require.Equal(t, "admin-1", uc.actor.ID)
}

func TestAdminDeleteUserForbidden(t *testing.T) {
	uc := &fakeAdminUC{err: pkgerrors.ErrForbidden}
	r := newAdminRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/u1", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.JSONEq(t, `{"error":"forbidden"}`, w.Body.String())
}
