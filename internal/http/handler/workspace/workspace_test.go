package workspace

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	dworkspace "github.com/tandem/tandem/internal/contracts/workspace"
	"github.com/tandem/tandem/internal/pkg/ctxkeys"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
)

type fakeWS struct {
	err       error
	createOut *dworkspace.WorkspaceResponse
	listOut   []dworkspace.WorkspaceResponse
	getOut    *dworkspace.WorkspaceDetailResponse
	updateOut *dworkspace.WorkspaceResponse
	themeOut  *dworkspace.WorkspaceResponse
	memberOut *dworkspace.WorkspaceMemberResponse
	inviteOut *dworkspace.WorkspaceInviteResponse

	actorID    string
	workspace  string
	targetID   string
	theme      string
	token      string
	createReq  dworkspace.CreateWorkspaceRequest
	updateReq  dworkspace.UpdateWorkspaceRequest
	memberReq  dworkspace.AddMemberRequest
	roleReq    dworkspace.UpdateMemberRoleRequest
	transferRe dworkspace.TransferOwnerRequest
}

func (f *fakeWS) Create(_ context.Context, actorID string, req dworkspace.CreateWorkspaceRequest) (*dworkspace.WorkspaceResponse, error) {
	f.actorID, f.createReq = actorID, req
	if f.err != nil {
		return nil, f.err
	}
	return f.createOut, nil
}

func (f *fakeWS) List(_ context.Context, actorID string) ([]dworkspace.WorkspaceResponse, error) {
	f.actorID = actorID
	if f.err != nil {
		return nil, f.err
	}
	return f.listOut, nil
}

func (f *fakeWS) Get(_ context.Context, actorID, workspaceID string) (*dworkspace.WorkspaceDetailResponse, error) {
	f.actorID, f.workspace = actorID, workspaceID
	if f.err != nil {
		return nil, f.err
	}
	return f.getOut, nil
}

func (f *fakeWS) Update(_ context.Context, actorID, workspaceID string, req dworkspace.UpdateWorkspaceRequest) (*dworkspace.WorkspaceResponse, error) {
	f.actorID, f.workspace, f.updateReq = actorID, workspaceID, req
	if f.err != nil {
		return nil, f.err
	}
	return f.updateOut, nil
}

func (f *fakeWS) Delete(_ context.Context, actorID, workspaceID string) error {
	f.actorID, f.workspace = actorID, workspaceID
	return f.err
}

func (f *fakeWS) SetTheme(_ context.Context, actorID, workspaceID, theme string) (*dworkspace.WorkspaceResponse, error) {
	f.actorID, f.workspace, f.theme = actorID, workspaceID, theme
	if f.err != nil {
		return nil, f.err
	}
	return f.themeOut, nil
}

func (f *fakeWS) AddMember(_ context.Context, actorID, workspaceID string, req dworkspace.AddMemberRequest) (*dworkspace.WorkspaceMemberResponse, error) {
	f.actorID, f.workspace, f.memberReq = actorID, workspaceID, req
	if f.err != nil {
		return nil, f.err
	}
	return f.memberOut, nil
}

func (f *fakeWS) UpdateRole(_ context.Context, actorID, workspaceID, targetID string, req dworkspace.UpdateMemberRoleRequest) (*dworkspace.WorkspaceMemberResponse, error) {
	f.actorID, f.workspace, f.targetID, f.roleReq = actorID, workspaceID, targetID, req
	if f.err != nil {
		return nil, f.err
	}
	return f.memberOut, nil
}

func (f *fakeWS) RemoveMember(_ context.Context, actorID, workspaceID, targetID string) error {
	f.actorID, f.workspace, f.targetID = actorID, workspaceID, targetID
	return f.err
}

func (f *fakeWS) TransferOwner(_ context.Context, actorID, workspaceID string, req dworkspace.TransferOwnerRequest) error {
	f.actorID, f.workspace, f.transferRe = actorID, workspaceID, req
	return f.err
}

func (f *fakeWS) GetInvite(_ context.Context, actorID, workspaceID string) (*dworkspace.WorkspaceInviteResponse, error) {
	f.actorID, f.workspace = actorID, workspaceID
	if f.err != nil {
		return nil, f.err
	}
	return f.inviteOut, nil
}

func (f *fakeWS) GenerateInvite(_ context.Context, actorID, workspaceID string) (*dworkspace.WorkspaceInviteResponse, error) {
	f.actorID, f.workspace = actorID, workspaceID
	if f.err != nil {
		return nil, f.err
	}
	return f.inviteOut, nil
}

func (f *fakeWS) DisableInvite(_ context.Context, actorID, workspaceID string) error {
	f.actorID, f.workspace = actorID, workspaceID
	return f.err
}

func (f *fakeWS) JoinByInvite(_ context.Context, actorID, token string) (*dworkspace.WorkspaceResponse, error) {
	f.actorID, f.token = actorID, token
	if f.err != nil {
		return nil, f.err
	}
	return f.createOut, nil
}

func newWSRouter(uc *fakeWS) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewWorkspaceHandler(uc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxkeys.CtxUserID, "user-1")
		c.Next()
	})
	api := r.Group("/api/v1/workspaces")
	api.POST("", h.Create)
	api.GET("", h.List)
	api.GET("/:id", h.Get)
	api.PATCH("/:id", h.Update)
	api.DELETE("/:id", h.Delete)
	api.PUT("/:id/theme", h.SetTheme)
	api.POST("/:id/members", h.AddMember)
	api.PATCH("/:id/members/:userId", h.UpdateRole)
	api.DELETE("/:id/members/:userId", h.RemoveMember)
	api.POST("/:id/owner", h.TransferOwner)
	api.GET("/:id/invite", h.GetInvite)
	api.POST("/:id/invite", h.GenerateInvite)
	api.DELETE("/:id/invite", h.DisableInvite)
	r.POST("/api/v1/invite/:token/join", h.JoinByInvite)
	return r
}

func wsFixture() *dworkspace.WorkspaceResponse {
	return &dworkspace.WorkspaceResponse{ID: "ws1", Name: "Team", Prefix: "TM"}
}

func TestWSCreate(t *testing.T) {
	uc := &fakeWS{createOut: wsFixture()}
	r := newWSRouter(uc)

	w := httptest.NewRecorder()
	body := `{"name":"Team","description":"d","prefix":"TM"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var out dworkspace.WorkspaceResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Equal(t, "Team", uc.createReq.Name)
	require.Equal(t, "TM", uc.createReq.Prefix)
	require.Equal(t, "user-1", uc.actorID)
}

func TestWSCreateBadJSON(t *testing.T) {
	uc := &fakeWS{}
	r := newWSRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces", strings.NewReader(`{`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.JSONEq(t, `{"error":"invalid request body"}`, w.Body.String())
}

func TestWSList(t *testing.T) {
	uc := &fakeWS{listOut: []dworkspace.WorkspaceResponse{*wsFixture()}}
	r := newWSRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var out []dworkspace.WorkspaceResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Len(t, out, 1)
}

func TestWSGet(t *testing.T) {
	detail := &dworkspace.WorkspaceDetailResponse{WorkspaceResponse: *wsFixture(), Members: []dworkspace.WorkspaceMemberResponse{{
		ID: "m1", Login: "ivan", Role: "owner",
	}}}
	uc := &fakeWS{getOut: detail}
	r := newWSRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws1", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var out dworkspace.WorkspaceDetailResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Equal(t, "ws1", out.ID)
	require.Len(t, out.Members, 1)
	require.Equal(t, "ws1", uc.workspace)
}

func TestWSGetForbidden(t *testing.T) {
	uc := &fakeWS{err: pkgerrors.ErrForbidden}
	r := newWSRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws1", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.JSONEq(t, `{"error":"forbidden"}`, w.Body.String())
}

func TestWSUpdate(t *testing.T) {
	uc := &fakeWS{updateOut: wsFixture()}
	r := newWSRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/workspaces/ws1", strings.NewReader(`{"name":"New"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "New", uc.updateReq.Name)
}

func TestWSDelete(t *testing.T) {
	uc := &fakeWS{}
	r := newWSRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/ws1", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"message":"workspace deleted"}`, w.Body.String())
	require.Equal(t, "ws1", uc.workspace)
}

func TestWSSetTheme(t *testing.T) {
	uc := &fakeWS{themeOut: wsFixture()}
	r := newWSRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/workspaces/ws1/theme", strings.NewReader(`{"theme":"midnight"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "midnight", uc.theme)
}

func TestWSSetThemeBadJSON(t *testing.T) {
	uc := &fakeWS{}
	r := newWSRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/workspaces/ws1/theme", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWSAddMember(t *testing.T) {
	uc := &fakeWS{memberOut: &dworkspace.WorkspaceMemberResponse{ID: "m1", Login: "petya", Role: "editor"}}
	r := newWSRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws1/members", strings.NewReader(`{"login":"petya","role":"editor"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	var out dworkspace.WorkspaceMemberResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Equal(t, "m1", out.ID)
	require.Equal(t, "petya", uc.memberReq.Login)
	require.Equal(t, "editor", uc.memberReq.Role)
}

func TestWSAddMemberConflict(t *testing.T) {
	uc := &fakeWS{err: pkgerrors.ErrConflict}
	r := newWSRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws1/members", strings.NewReader(`{"login":"petya"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusConflict, w.Code)
	require.JSONEq(t, `{"error":"conflict"}`, w.Body.String())
}

func TestWSUpdateRole(t *testing.T) {
	uc := &fakeWS{memberOut: &dworkspace.WorkspaceMemberResponse{ID: "m1", Role: "viewer"}}
	r := newWSRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/workspaces/ws1/members/m1", strings.NewReader(`{"role":"viewer"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "m1", uc.targetID)
	require.Equal(t, "viewer", uc.roleReq.Role)
}

func TestWSRemoveMember(t *testing.T) {
	uc := &fakeWS{}
	r := newWSRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/ws1/members/m1", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"message":"member removed"}`, w.Body.String())
	require.Equal(t, "m1", uc.targetID)
}

func TestWSTransferOwner(t *testing.T) {
	uc := &fakeWS{}
	r := newWSRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws1/owner", strings.NewReader(`{"user_id":"m1"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"message":"ownership transferred"}`, w.Body.String())
	require.Equal(t, "m1", uc.transferRe.UserID)
}

func TestWSGetInvite(t *testing.T) {
	tok := "invite-token"
	uc := &fakeWS{inviteOut: &dworkspace.WorkspaceInviteResponse{InviteToken: &tok}}
	r := newWSRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws1/invite", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var out dworkspace.WorkspaceInviteResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.NotNil(t, out.InviteToken)
	require.Equal(t, "invite-token", *out.InviteToken)
}

func TestWSGenerateInvite(t *testing.T) {
	uc := &fakeWS{inviteOut: &dworkspace.WorkspaceInviteResponse{}}
	r := newWSRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/ws1/invite", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "ws1", uc.workspace)
}

func TestWSDisableInvite(t *testing.T) {
	uc := &fakeWS{}
	r := newWSRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/workspaces/ws1/invite", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"message":"invite disabled"}`, w.Body.String())
}

func TestWSJoinByInvite(t *testing.T) {
	uc := &fakeWS{createOut: wsFixture()}
	r := newWSRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/invite/some-token/join", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "some-token", uc.token)
}

func TestWSJoinByInviteInvalidToken(t *testing.T) {
	uc := &fakeWS{err: pkgerrors.ErrNotFound}
	r := newWSRouter(uc)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/invite/bad-token/join", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.JSONEq(t, `{"error":"not found"}`, w.Body.String())
}
