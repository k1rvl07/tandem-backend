package router

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	dadmin "github.com/tandem/tandem/internal/contracts/admin"
	dattachment "github.com/tandem/tandem/internal/contracts/attachment"
	dauth "github.com/tandem/tandem/internal/contracts/auth"
	dboard "github.com/tandem/tandem/internal/contracts/board"
	dprofile "github.com/tandem/tandem/internal/contracts/profile"
	dtask "github.com/tandem/tandem/internal/contracts/task"
	dtree "github.com/tandem/tandem/internal/contracts/tree"
	dworkspace "github.com/tandem/tandem/internal/contracts/workspace"
	muser "github.com/tandem/tandem/internal/domain/models/user"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	hadmin "github.com/tandem/tandem/internal/http/handler/admin"
	hattachment "github.com/tandem/tandem/internal/http/handler/attachment"
	hauth "github.com/tandem/tandem/internal/http/handler/auth"
	hboard "github.com/tandem/tandem/internal/http/handler/board"
	hfavorite "github.com/tandem/tandem/internal/http/handler/favorite"
	hprofile "github.com/tandem/tandem/internal/http/handler/profile"
	htask "github.com/tandem/tandem/internal/http/handler/task"
	htree "github.com/tandem/tandem/internal/http/handler/tree"
	hworkspace "github.com/tandem/tandem/internal/http/handler/workspace"
	wshub "github.com/tandem/tandem/internal/infrastructure/ws"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	padmin "github.com/tandem/tandem/internal/usecase/admin"
	file "github.com/tandem/tandem/internal/usecase/file"
	"go.uber.org/zap"
)

var errUnauthorized = errors.New("unauthorized")

type fakeTokenService struct {
	userID string
	err    error
}

func (f *fakeTokenService) Generate(string, time.Duration) (string, error) {
	return "access", nil
}

func (f *fakeTokenService) GenerateRefresh(string, time.Duration) (string, error) {
	return "refresh", nil
}

func (f *fakeTokenService) Parse(token string) (string, error) {
	if f.err != nil || token == "" {
		return "", errUnauthorized
	}
	return f.userID, nil
}

func (f *fakeTokenService) ParseRefresh(string) (string, error) {
	return f.Parse("refresh")
}

func (f *fakeTokenService) RotateRefresh(context.Context, string, time.Duration, time.Duration) (string, string, error) {
	return "access", "refresh", nil
}

func (f *fakeTokenService) Revoke(context.Context, string) error {
	return nil
}

type fakeFavoriteUC struct {
	err      error
	actorID  string
	targetID string
	kind     string
}

func (f *fakeFavoriteUC) Add(_ context.Context, actorID, targetType, targetID string) error {
	f.actorID, f.kind, f.targetID = actorID, targetType, targetID
	if f.err != nil {
		return f.err
	}
	return nil
}

func (f *fakeFavoriteUC) Remove(_ context.Context, actorID, targetType, targetID string) error {
	f.actorID, f.kind, f.targetID = actorID, targetType, targetID
	if f.err != nil {
		return f.err
	}
	return nil
}

func newMinimalRouter(uc *hfavorite.FavoriteHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	return New(Dependencies{
		Logger:          zap.NewNop(),
		TokenService:    &fakeTokenService{userID: "user-1"},
		FavoriteHandler: uc,
		TrustedProxies:  []string{},
	})
}

func TestHealthz(t *testing.T) {
	r := New(Dependencies{Logger: zap.NewNop()})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"status":"ok"}`, w.Body.String())
}

func TestUnknownRouteIsNotFound(t *testing.T) {
	r := New(Dependencies{Logger: zap.NewNop()})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/nope", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestNillableGroupsAreNotRegistered(t *testing.T) {
	r := New(Dependencies{Logger: zap.NewNop()})

	for _, path := range []string{"/api/v1/auth/login", "/api/v1/me", "/api/v1/tasks/tree", "/ws"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusNotFound, w.Code, "expected 404 for %s", path)
	}
}

func TestNilFavoriteHandlerSkipsRegistration(t *testing.T) {
	r := newMinimalRouter(nil)

	for _, path := range []string{
		"/api/v1/favorites/workspaces/ws1",
		"/api/v1/favorites/boards/b1",
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPut, path, nil))
		require.Equal(t, http.StatusNotFound, w.Code, "expected 404 for %s", path)
	}
}

func TestAuthenticatedRouteRequiresToken(t *testing.T) {
	r := newMinimalRouter(hfavorite.NewFavoriteHandler(&fakeFavoriteUC{}))

	tt := []struct {
		name string
		auth string
		code int
		body string
	}{
		{name: "no header", code: http.StatusUnauthorized, body: `{"error":"unauthorized"}`},
		{name: "malformed header", auth: "Basic abc", code: http.StatusUnauthorized, body: `{"error":"unauthorized"}`},
		{name: "empty bearer", auth: "Bearer ", code: http.StatusUnauthorized, body: `{"error":"unauthorized"}`},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPut, "/api/v1/favorites/workspaces/ws1", nil)
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}
			r.ServeHTTP(w, req)
			require.Equal(t, tc.code, w.Code)
			require.JSONEq(t, tc.body, w.Body.String())
		})
	}
}

func TestAuthenticatedRouteSucceedsWithToken(t *testing.T) {
	fav := &fakeFavoriteUC{}
	r := newMinimalRouter(hfavorite.NewFavoriteHandler(fav))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/favorites/workspaces/ws1", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"message":"favorited"}`, w.Body.String())
	require.Equal(t, "user-1", fav.actorID)
	require.Equal(t, "workspace", fav.kind)
	require.Equal(t, "ws1", fav.targetID)
}

func TestWriteGroupDeleteRoute(t *testing.T) {
	fav := &fakeFavoriteUC{}
	r := newMinimalRouter(hfavorite.NewFavoriteHandler(fav))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/favorites/boards/b1", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.JSONEq(t, `{"message":"unfavorited"}`, w.Body.String())
	require.Equal(t, "board", fav.kind)
	require.Equal(t, "b1", fav.targetID)
}

type fakeAuthUC struct {
	loginResp *dauth.LoginResponse
	err       error
}

func (f *fakeAuthUC) Login(context.Context, dauth.LoginRequest) (*dauth.LoginResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.loginResp, nil
}

func (f *fakeAuthUC) Refresh(context.Context, string) (*dauth.RefreshResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &dauth.RefreshResponse{Token: "access"}, nil
}

func (f *fakeAuthUC) Logout(context.Context, string) error {
	return f.err
}

type fakeProfileUC struct {
	err error
}

func (f *fakeProfileUC) Get(context.Context, string) (*dauth.UserResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &dauth.UserResponse{}, nil
}

func (f *fakeProfileUC) UpdateProfile(context.Context, string, dprofile.UpdateProfileRequest) (*dauth.UserResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &dauth.UserResponse{}, nil
}

func (f *fakeProfileUC) ChangePassword(context.Context, string, dprofile.ChangePasswordRequest) (*dprofile.ChangePasswordResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &dprofile.ChangePasswordResponse{}, nil
}

func (f *fakeProfileUC) UploadAvatar(context.Context, string, string, string, io.Reader, int64) (*dauth.UserResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &dauth.UserResponse{}, nil
}

func (f *fakeProfileUC) RemoveAvatar(context.Context, string) (*dauth.UserResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &dauth.UserResponse{}, nil
}

type fakeAdminUC struct {
	err error
}

func (f *fakeAdminUC) CreateUser(context.Context, padmin.Actor, dadmin.CreateUserRequest) (*dauth.UserResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &dauth.UserResponse{}, nil
}

func (f *fakeAdminUC) ListUsers(context.Context, dadmin.AdminListQuery) (dadmin.AdminPage, error) {
	if f.err != nil {
		return dadmin.AdminPage{}, f.err
	}
	return dadmin.AdminPage{}, nil
}

func (f *fakeAdminUC) UpdateUserRole(context.Context, padmin.Actor, string, dadmin.UpdateUserRoleRequest) (*dauth.UserResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &dauth.UserResponse{}, nil
}

func (f *fakeAdminUC) DeleteUser(context.Context, padmin.Actor, string) error {
	return f.err
}

type fakeWorkspaceUC struct {
	err error
}

func (f *fakeWorkspaceUC) Create(context.Context, string, dworkspace.CreateWorkspaceRequest) (*dworkspace.WorkspaceResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &dworkspace.WorkspaceResponse{}, nil
}

func (f *fakeWorkspaceUC) List(context.Context, string) ([]dworkspace.WorkspaceResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return nil, nil
}

func (f *fakeWorkspaceUC) Get(context.Context, string, string) (*dworkspace.WorkspaceDetailResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &dworkspace.WorkspaceDetailResponse{}, nil
}

func (f *fakeWorkspaceUC) Update(context.Context, string, string, dworkspace.UpdateWorkspaceRequest) (*dworkspace.WorkspaceResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &dworkspace.WorkspaceResponse{}, nil
}

func (f *fakeWorkspaceUC) Delete(context.Context, string, string) error {
	return f.err
}

func (f *fakeWorkspaceUC) SetTheme(context.Context, string, string, string) (*dworkspace.WorkspaceResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &dworkspace.WorkspaceResponse{}, nil
}

func (f *fakeWorkspaceUC) AddMember(context.Context, string, string, dworkspace.AddMemberRequest) (*dworkspace.WorkspaceMemberResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &dworkspace.WorkspaceMemberResponse{}, nil
}

func (f *fakeWorkspaceUC) UpdateRole(context.Context, string, string, string, dworkspace.UpdateMemberRoleRequest) (*dworkspace.WorkspaceMemberResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &dworkspace.WorkspaceMemberResponse{}, nil
}

func (f *fakeWorkspaceUC) RemoveMember(context.Context, string, string, string) error {
	return f.err
}

func (f *fakeWorkspaceUC) TransferOwner(context.Context, string, string, dworkspace.TransferOwnerRequest) error {
	return f.err
}

func (f *fakeWorkspaceUC) GetInvite(context.Context, string, string) (*dworkspace.WorkspaceInviteResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &dworkspace.WorkspaceInviteResponse{}, nil
}

func (f *fakeWorkspaceUC) GenerateInvite(context.Context, string, string) (*dworkspace.WorkspaceInviteResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &dworkspace.WorkspaceInviteResponse{}, nil
}

func (f *fakeWorkspaceUC) DisableInvite(context.Context, string, string) error {
	return f.err
}

func (f *fakeWorkspaceUC) JoinByInvite(context.Context, string, string) (*dworkspace.WorkspaceResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &dworkspace.WorkspaceResponse{}, nil
}

type fakeBoardUC struct {
	err error
}

func (f *fakeBoardUC) Create(context.Context, string, string, dboard.CreateBoardRequest) (*dboard.BoardResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &dboard.BoardResponse{}, nil
}

func (f *fakeBoardUC) List(context.Context, string, string) ([]dboard.BoardResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return nil, nil
}

func (f *fakeBoardUC) Get(context.Context, string, string, string) (*dboard.BoardDetailResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &dboard.BoardDetailResponse{}, nil
}

func (f *fakeBoardUC) Update(context.Context, string, string, string, dboard.UpdateBoardRequest) (*dboard.BoardResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &dboard.BoardResponse{}, nil
}

func (f *fakeBoardUC) Delete(context.Context, string, string, string) error {
	return f.err
}

func (f *fakeBoardUC) SetMain(context.Context, string, string, string) (*dboard.BoardResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &dboard.BoardResponse{}, nil
}

func (f *fakeBoardUC) Reorder(context.Context, string, string, dboard.ReorderBoardsRequest) ([]dboard.BoardResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return nil, nil
}

type fakeTaskUC struct {
	err error
}

func (f *fakeTaskUC) Create(context.Context, string, string, string, dtask.CreateTaskRequest) (*dtask.TaskResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &dtask.TaskResponse{}, nil
}

func (f *fakeTaskUC) Update(context.Context, string, string, string, string, dtask.UpdateTaskRequest) (*dtask.TaskResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &dtask.TaskResponse{}, nil
}

func (f *fakeTaskUC) Delete(context.Context, string, string, string, string) error {
	return f.err
}

func (f *fakeTaskUC) Get(context.Context, string, string, string) (*dtask.TaskDetailResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &dtask.TaskDetailResponse{}, nil
}

func (f *fakeTaskUC) List(context.Context, string, string, dtask.ListWorkspaceTasksQuery) ([]dtask.TaskResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return nil, nil
}

type fakeAttachmentUC struct {
	err error
}

func (f *fakeAttachmentUC) Create(context.Context, string, string, string, string, string, io.Reader, int64) (*dattachment.AttachmentResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &dattachment.AttachmentResponse{}, nil
}

func (f *fakeAttachmentUC) List(context.Context, string, string, string) ([]dattachment.AttachmentResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return nil, nil
}

func (f *fakeAttachmentUC) Download(context.Context, string, string, string, string) (*dattachment.AttachmentResponse, io.ReadCloser, error) {
	if f.err != nil {
		return nil, nil, f.err
	}
	return &dattachment.AttachmentResponse{}, io.NopCloser(strings.NewReader("data")), nil
}

func (f *fakeAttachmentUC) Delete(context.Context, string, string, string, string) error {
	return f.err
}

type fakeTreeUC struct {
	err error
}

func (f *fakeTreeUC) List(context.Context, string, dtree.TreeQuery) ([]dtree.TreeWorkspaceResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return nil, nil
}

type fakeStaffRepo struct{}

func (f *fakeStaffRepo) Create(context.Context, *muser.User) error { return nil }
func (f *fakeStaffRepo) FindByID(context.Context, string) (*muser.User, error) {
	return &muser.User{Role: muser.RoleAdmin}, nil
}
func (f *fakeStaffRepo) FindByLogin(context.Context, string) (*muser.User, error) { return nil, nil }
func (f *fakeStaffRepo) ExistsByLogin(context.Context, string) (bool, error)      { return false, nil }
func (f *fakeStaffRepo) List(context.Context) ([]*muser.User, error)              { return nil, nil }
func (f *fakeStaffRepo) ListPage(context.Context, string, int, int) ([]*muser.User, error) {
	return nil, nil
}
func (f *fakeStaffRepo) Count(context.Context, string) (int, error) { return 0, nil }
func (f *fakeStaffRepo) Update(context.Context, *muser.User) error  { return nil }
func (f *fakeStaffRepo) Delete(context.Context, string) error       { return nil }

type fakeWSRepo struct{}

func (f *fakeWSRepo) CreateWorkspace(context.Context, *mworkspace.Workspace) error { return nil }
func (f *fakeWSRepo) FindWorkspaceByID(context.Context, string) (*mworkspace.Workspace, error) {
	return nil, nil
}
func (f *fakeWSRepo) FindWorkspaceByInvite(context.Context, string) (*mworkspace.Workspace, error) {
	return nil, nil
}
func (f *fakeWSRepo) UpdateWorkspace(context.Context, *mworkspace.Workspace) error { return nil }
func (f *fakeWSRepo) UpdateInvite(context.Context, string, string, *time.Time) error {
	return nil
}
func (f *fakeWSRepo) DeleteWorkspace(context.Context, string) error { return nil }
func (f *fakeWSRepo) ListWorkspacesForUser(context.Context, string) ([]mworkspace.WorkspaceMembership, error) {
	return nil, nil
}
func (f *fakeWSRepo) AddMember(context.Context, string, string, mworkspace.WorkspaceRole) error {
	return nil
}
func (f *fakeWSRepo) FindMember(context.Context, string, string) (*mworkspace.WorkspaceMember, error) {
	return nil, nil
}
func (f *fakeWSRepo) RemoveMember(context.Context, string, string) error { return nil }
func (f *fakeWSRepo) UpdateMemberRole(context.Context, string, string, mworkspace.WorkspaceRole) error {
	return nil
}
func (f *fakeWSRepo) ListMembers(context.Context, string) ([]mworkspace.WorkspaceMember, error) {
	return nil, nil
}
func (f *fakeWSRepo) DeleteMembersByWorkspace(context.Context, string) error { return nil }
func (f *fakeWSRepo) DeleteMembersByUser(context.Context, string) error      { return nil }
func (f *fakeWSRepo) TransferOwnership(context.Context, string, string, string) error {
	return nil
}

type fakeFileStore struct{}

func (f *fakeFileStore) Put(context.Context, string, io.Reader, int64, string) error {
	return nil
}

func (f *fakeFileStore) Get(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (f *fakeFileStore) Delete(context.Context, string) error { return nil }

func (f *fakeFileStore) Exists(context.Context, string) (bool, error) { return true, nil }

func (f *fakeFileStore) PresignGet(context.Context, string, time.Duration) (string, error) {
	return "http://presigned", nil
}

type nopLimiter struct{}

func (nopLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) { c.Next() }
}

func newFullRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	hub := wshub.New()
	t.Cleanup(hub.Close)
	limiter := nopLimiter{}
	return New(Dependencies{
		Logger:              zap.NewNop(),
		AllowedOrigin:       []string{"http://localhost:5173"},
		TrustedProxies:      []string{"127.0.0.1"},
		Hub:                 hub,
		TokenService:        &fakeTokenService{userID: "user-1"},
		UserRepository:      &fakeStaffRepo{},
		WorkspaceRepository: &fakeWSRepo{},
		AuthHandler:         hauth.NewAuthHandler(&fakeAuthUC{loginResp: &dauth.LoginResponse{Token: "access"}}, false, time.Minute),
		ProfileHandler:      hprofile.NewProfileHandler(&fakeProfileUC{}, false, time.Minute),
		AdminHandler:        hadmin.NewAdminHandler(&fakeAdminUC{}),
		WorkspaceHandler:    hworkspace.NewWorkspaceHandler(&fakeWorkspaceUC{}),
		BoardHandler:        hboard.NewBoardHandler(&fakeBoardUC{}),
		TaskHandler:         htask.NewTaskHandler(&fakeTaskUC{}),
		AttachmentHandler:   hattachment.NewAttachmentHandler(&fakeAttachmentUC{}),
		FavoriteHandler:     hfavorite.NewFavoriteHandler(&fakeFavoriteUC{}),
		TreeHandler:         htree.NewTreeHandler(&fakeTreeUC{}),
		Files:               file.NewService(&fakeFileStore{}, zap.NewNop()),
		EnableSwagger:       true,
		AuthLimiter:         limiter,
		ReadLimiter:         limiter,
		WriteLimiter:        limiter,
		UploadLimiter:       limiter,
		WSLimiter:           limiter,
	})
}

func hit(t *testing.T, r *gin.Engine, method, path, auth, contentType, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if auth != "" {
		req.Header.Set("Authorization", "Bearer "+auth)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func multipartHit(t *testing.T, r *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "pic.png")
	require.NoError(t, err)
	_, err = fw.Write([]byte("png-data"))
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer t")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestFullRouterRegistersAllRoutes(t *testing.T) {
	r := newFullRouter(t)
	token := "t"

	tt := []struct {
		name        string
		method      string
		path        string
		auth        bool
		contentType string
		body        string
	}{
		{name: "healthz", method: http.MethodGet, path: "/healthz"},
		{name: "swagger", method: http.MethodGet, path: "/swagger"},
		{name: "swagger doc", method: http.MethodGet, path: "/swagger/doc.json"},
		{name: "auth login", method: http.MethodPost, path: "/api/v1/auth/login", contentType: "application/json", body: `{"login":"a","password":"b"}`},
		{name: "auth refresh", method: http.MethodPost, path: "/api/v1/auth/refresh", contentType: "application/json", body: `{"refresh_token":"x"}`},
		{name: "auth logout", method: http.MethodPost, path: "/api/v1/auth/logout", auth: true},
		{name: "me", method: http.MethodGet, path: "/api/v1/me", auth: true},
		{name: "me update", method: http.MethodPatch, path: "/api/v1/me", auth: true, contentType: "application/json", body: `{"display_name":"n"}`},
		{name: "me avatar upload", method: http.MethodPost, path: "/api/v1/me/avatar", auth: true, contentType: "multipart/form-data", body: ""},
		{name: "me avatar delete", method: http.MethodDelete, path: "/api/v1/me/avatar", auth: true},
		{name: "me password", method: http.MethodPost, path: "/api/v1/me/password", auth: true, contentType: "application/json", body: `{"new_password":"n","current_password":"c"}`},
		{name: "admin create", method: http.MethodPost, path: "/api/v1/admin/users", auth: true, contentType: "application/json", body: `{"login":"x"}`},
		{name: "admin list", method: http.MethodGet, path: "/api/v1/admin/users", auth: true},
		{name: "admin role", method: http.MethodPatch, path: "/api/v1/admin/users/u1/role", auth: true, contentType: "application/json", body: `{"role":"admin"}`},
		{name: "admin delete", method: http.MethodDelete, path: "/api/v1/admin/users/u1", auth: true},
		{name: "workspace create", method: http.MethodPost, path: "/api/v1/workspaces", auth: true, contentType: "application/json", body: `{"name":"w"}`},
		{name: "workspace list", method: http.MethodGet, path: "/api/v1/workspaces", auth: true},
		{name: "workspace get", method: http.MethodGet, path: "/api/v1/workspaces/w1", auth: true},
		{name: "workspace update", method: http.MethodPatch, path: "/api/v1/workspaces/w1", auth: true, contentType: "application/json", body: `{"name":"w"}`},
		{name: "workspace theme", method: http.MethodPut, path: "/api/v1/workspaces/w1/theme", auth: true, contentType: "application/json", body: `{"theme":"dark"}`},
		{name: "workspace delete", method: http.MethodDelete, path: "/api/v1/workspaces/w1", auth: true},
		{name: "workspace add member", method: http.MethodPost, path: "/api/v1/workspaces/w1/members", auth: true, contentType: "application/json", body: `{"login":"x","role":"member"}`},
		{name: "workspace role", method: http.MethodPatch, path: "/api/v1/workspaces/w1/members/u2", auth: true, contentType: "application/json", body: `{"role":"editor"}`},
		{name: "workspace remove member", method: http.MethodDelete, path: "/api/v1/workspaces/w1/members/u2", auth: true},
		{name: "workspace owner", method: http.MethodPost, path: "/api/v1/workspaces/w1/owner", auth: true, contentType: "application/json", body: `{"user_id":"u2"}`},
		{name: "workspace invite get", method: http.MethodGet, path: "/api/v1/workspaces/w1/invite", auth: true},
		{name: "workspace invite gen", method: http.MethodPost, path: "/api/v1/workspaces/w1/invite", auth: true},
		{name: "workspace invite disable", method: http.MethodDelete, path: "/api/v1/workspaces/w1/invite", auth: true},
		{name: "invite join", method: http.MethodPost, path: "/api/v1/invite/tok/join", auth: true},
		{name: "board list", method: http.MethodGet, path: "/api/v1/workspaces/w1/boards", auth: true},
		{name: "board create", method: http.MethodPost, path: "/api/v1/workspaces/w1/boards", auth: true, contentType: "application/json", body: `{"name":"b"}`},
		{name: "board reorder", method: http.MethodPut, path: "/api/v1/workspaces/w1/boards/reorder", auth: true, contentType: "application/json", body: `{"board_ids":[]}`},
		{name: "board get", method: http.MethodGet, path: "/api/v1/workspaces/w1/boards/b1", auth: true},
		{name: "board update", method: http.MethodPatch, path: "/api/v1/workspaces/w1/boards/b1", auth: true, contentType: "application/json", body: `{"name":"b"}`},
		{name: "board main", method: http.MethodPut, path: "/api/v1/workspaces/w1/boards/b1/main", auth: true},
		{name: "board delete", method: http.MethodDelete, path: "/api/v1/workspaces/w1/boards/b1", auth: true},
		{name: "task list", method: http.MethodGet, path: "/api/v1/workspaces/w1/tasks", auth: true},
		{name: "task get", method: http.MethodGet, path: "/api/v1/workspaces/w1/tasks/t1", auth: true},
		{name: "task create", method: http.MethodPost, path: "/api/v1/workspaces/w1/boards/b1/tasks", auth: true, contentType: "application/json", body: `{"column_id":"c1","title":"x"}`},
		{name: "task update", method: http.MethodPatch, path: "/api/v1/workspaces/w1/boards/b1/tasks/t1", auth: true, contentType: "application/json", body: `{"title":"x"}`},
		{name: "task delete", method: http.MethodDelete, path: "/api/v1/workspaces/w1/boards/b1/tasks/t1", auth: true},
		{name: "attachment list", method: http.MethodGet, path: "/api/v1/workspaces/w1/tasks/t1/attachments", auth: true},
		{name: "attachment download", method: http.MethodGet, path: "/api/v1/workspaces/w1/tasks/t1/attachments/a1", auth: true},
		{name: "attachment delete", method: http.MethodDelete, path: "/api/v1/workspaces/w1/tasks/t1/attachments/a1", auth: true},
		{name: "favorite ws add", method: http.MethodPut, path: "/api/v1/favorites/workspaces/ws1", auth: true},
		{name: "favorite ws remove", method: http.MethodDelete, path: "/api/v1/favorites/workspaces/ws1", auth: true},
		{name: "favorite board add", method: http.MethodPut, path: "/api/v1/favorites/boards/b1", auth: true},
		{name: "favorite board remove", method: http.MethodDelete, path: "/api/v1/favorites/boards/b1", auth: true},
		{name: "tree", method: http.MethodGet, path: "/api/v1/tasks/tree", auth: true},
		{name: "file delete", method: http.MethodDelete, path: "/api/v1/files/images?key=avatars/user-1/x.png", auth: true},
		{name: "file sign", method: http.MethodGet, path: "/api/v1/files/sign?key=avatars/user-1/x.png", auth: true},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			hdr := ""
			if tc.auth {
				hdr = token
			}
			w := hit(t, r, tc.method, tc.path, hdr, tc.contentType, tc.body)
			if tc.name == "swagger doc" {
				require.NotEqual(t, http.StatusNotFound, w.Code, "%s %s routed to 404", tc.method, tc.path)
				return
			}
			require.Less(t, w.Code, http.StatusInternalServerError, "%s %s failed with %d: %s", tc.method, tc.path, w.Code, w.Body.String())
		})
	}
}

func TestFullRouterAttachmentAndFileUploads(t *testing.T) {
	r := newFullRouter(t)

	w := hit(t, r, http.MethodPost, "/api/v1/workspaces/w1/tasks/t1/attachments", "t", "multipart/form-data", "")
	require.Less(t, w.Code, http.StatusInternalServerError)

	w = hit(t, r, http.MethodPost, "/api/v1/files/images?namespace=avatars", "t", "multipart/form-data", "")
	require.Less(t, w.Code, http.StatusInternalServerError)
}

func TestFullRouterMultipartUploads(t *testing.T) {
	r := newFullRouter(t)

	w := multipartHit(t, r, "/api/v1/workspaces/w1/tasks/t1/attachments")
	require.Less(t, w.Code, http.StatusInternalServerError)

	w = multipartHit(t, r, "/api/v1/files/images?namespace=avatars")
	require.Less(t, w.Code, http.StatusInternalServerError)
}

func TestFullRouterHealthzAndSwagger(t *testing.T) {
	r := newFullRouter(t)

	w := hit(t, r, http.MethodGet, "/healthz", "", "", "")
	require.Equal(t, http.StatusOK, w.Code)

	w = hit(t, r, http.MethodGet, "/swagger", "", "", "")
	require.Equal(t, http.StatusFound, w.Code)

	w = hit(t, r, http.MethodGet, "/swagger/index.html", "", "", "")
	require.Equal(t, http.StatusOK, w.Code)
}

func TestFullRouterWSRouteRegistered(t *testing.T) {
	r := newFullRouter(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Sec-WebSocket-Protocol", "tandem, tok")
	r.ServeHTTP(w, req)
	require.NotEqual(t, http.StatusNotFound, w.Code)
}

type handlerHarness struct {
	auth       *fakeAuthUC
	profile    *fakeProfileUC
	admin      *fakeAdminUC
	workspace  *fakeWorkspaceUC
	board      *fakeBoardUC
	task       *fakeTaskUC
	attachment *fakeAttachmentUC
	favorite   *fakeFavoriteUC
	tree       *fakeTreeUC
}

func newHandlerErrorRouter(t *testing.T) (*gin.Engine, *handlerHarness) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	hub := wshub.New()
	t.Cleanup(hub.Close)
	limiter := nopLimiter{}
	h := &handlerHarness{
		auth:       &fakeAuthUC{loginResp: &dauth.LoginResponse{Token: "access"}},
		profile:    &fakeProfileUC{},
		admin:      &fakeAdminUC{},
		workspace:  &fakeWorkspaceUC{},
		board:      &fakeBoardUC{},
		task:       &fakeTaskUC{},
		attachment: &fakeAttachmentUC{},
		favorite:   &fakeFavoriteUC{},
		tree:       &fakeTreeUC{},
	}
	r := New(Dependencies{
		Logger:              zap.NewNop(),
		AllowedOrigin:       []string{"http://localhost:5173"},
		TrustedProxies:      []string{"127.0.0.1"},
		Hub:                 hub,
		TokenService:        &fakeTokenService{userID: "user-1"},
		UserRepository:      &fakeStaffRepo{},
		WorkspaceRepository: &fakeWSRepo{},
		AuthHandler:         hauth.NewAuthHandler(h.auth, false, time.Minute),
		ProfileHandler:      hprofile.NewProfileHandler(h.profile, false, time.Minute),
		AdminHandler:        hadmin.NewAdminHandler(h.admin),
		WorkspaceHandler:    hworkspace.NewWorkspaceHandler(h.workspace),
		BoardHandler:        hboard.NewBoardHandler(h.board),
		TaskHandler:         htask.NewTaskHandler(h.task),
		AttachmentHandler:   hattachment.NewAttachmentHandler(h.attachment),
		FavoriteHandler:     hfavorite.NewFavoriteHandler(h.favorite),
		TreeHandler:         htree.NewTreeHandler(h.tree),
		Files:               file.NewService(&fakeFileStore{}, zap.NewNop()),
		EnableSwagger:       true,
		AuthLimiter:         limiter,
		ReadLimiter:         limiter,
		WriteLimiter:        limiter,
		UploadLimiter:       limiter,
		WSLimiter:           limiter,
	})
	return r, h
}

func errBody() string {
	return errors.New("boom").Error()
}

func TestFullRouterHandlerErrorResponses(t *testing.T) {
	r, f := newHandlerErrorRouter(t)
	token := "t"

	t.Run("workspace errors", func(t *testing.T) {
		f.workspace.err = pkgerrors.ErrNotFound
		defer func() { f.workspace.err = nil }()
		for _, tc := range []struct {
			method, path, ctype, body string
		}{
			{http.MethodPost, "/api/v1/workspaces", "application/json", `{"name":"w"}`},
			{http.MethodGet, "/api/v1/workspaces", "", ""},
			{http.MethodGet, "/api/v1/workspaces/w1", "", ""},
			{http.MethodPatch, "/api/v1/workspaces/w1", "application/json", `{"name":"w"}`},
			{http.MethodDelete, "/api/v1/workspaces/w1", "", ""},
			{http.MethodPut, "/api/v1/workspaces/w1/theme", "application/json", `{"theme":"dark"}`},
			{http.MethodPost, "/api/v1/workspaces/w1/members", "application/json", `{"login":"x"}`},
			{http.MethodPatch, "/api/v1/workspaces/w1/members/u2", "application/json", `{"role":"editor"}`},
			{http.MethodDelete, "/api/v1/workspaces/w1/members/u2", "", ""},
			{http.MethodPost, "/api/v1/workspaces/w1/owner", "application/json", `{"user_id":"u2"}`},
			{http.MethodGet, "/api/v1/workspaces/w1/invite", "", ""},
			{http.MethodPost, "/api/v1/workspaces/w1/invite", "", ""},
			{http.MethodDelete, "/api/v1/workspaces/w1/invite", "", ""},
			{http.MethodPost, "/api/v1/invite/tok/join", "", ""},
		} {
			w := hit(t, r, tc.method, tc.path, token, tc.ctype, tc.body)
			require.Equal(t, http.StatusNotFound, w.Code, "%s %s", tc.method, tc.path)
		}
	})

	t.Run("board errors", func(t *testing.T) {
		f.board.err = pkgerrors.ErrForbidden
		defer func() { f.board.err = nil }()
		for _, tc := range []struct {
			method, path, ctype, body string
		}{
			{http.MethodPost, "/api/v1/workspaces/w1/boards", "application/json", `{"name":"b"}`},
			{http.MethodGet, "/api/v1/workspaces/w1/boards", "", ""},
			{http.MethodGet, "/api/v1/workspaces/w1/boards/b1", "", ""},
			{http.MethodPatch, "/api/v1/workspaces/w1/boards/b1", "application/json", `{"name":"b"}`},
			{http.MethodDelete, "/api/v1/workspaces/w1/boards/b1", "", ""},
			{http.MethodPut, "/api/v1/workspaces/w1/boards/b1/main", "", ""},
			{http.MethodPut, "/api/v1/workspaces/w1/boards/reorder", "application/json", `{"board_ids":[]}`},
		} {
			w := hit(t, r, tc.method, tc.path, token, tc.ctype, tc.body)
			require.Equal(t, http.StatusForbidden, w.Code, "%s %s", tc.method, tc.path)
		}
	})

	t.Run("task and attachment errors", func(t *testing.T) {
		f.task.err = pkgerrors.ErrConflict
		defer func() { f.task.err = nil }()
		require.Equal(t, http.StatusConflict, hit(t, r, http.MethodGet, "/api/v1/workspaces/w1/tasks", token, "", "").Code)

		f.attachment.err = pkgerrors.ErrTooManyRequests
		defer func() { f.attachment.err = nil }()
		require.Equal(t, http.StatusTooManyRequests, hit(t, r, http.MethodGet, "/api/v1/workspaces/w1/tasks/t1/attachments", token, "", "").Code)
		require.Equal(t, http.StatusTooManyRequests, hit(t, r, http.MethodDelete, "/api/v1/workspaces/w1/tasks/t1/attachments/a1", token, "", "").Code)
	})

	t.Run("profile errors", func(t *testing.T) {
		f.profile.err = pkgerrors.ErrUnauthorized
		defer func() { f.profile.err = nil }()
		require.Equal(t, http.StatusUnauthorized, hit(t, r, http.MethodGet, "/api/v1/me", token, "", "").Code)
		require.Equal(t, http.StatusUnauthorized, hit(t, r, http.MethodPatch, "/api/v1/me", token, "application/json", `{"display_name":"n"}`).Code)
		require.Equal(t, http.StatusUnauthorized, hit(t, r, http.MethodDelete, "/api/v1/me/avatar", token, "", "").Code)
		require.Equal(t, http.StatusUnauthorized, hit(t, r, http.MethodPost, "/api/v1/me/password", token, "application/json", `{"new_password":"n","current_password":"c"}`).Code)
	})

	t.Run("admin errors", func(t *testing.T) {
		f.admin.err = pkgerrors.ErrConflict
		defer func() { f.admin.err = nil }()
		require.Equal(t, http.StatusConflict, hit(t, r, http.MethodGet, "/api/v1/admin/users", token, "", "").Code)
		require.Equal(t, http.StatusConflict, hit(t, r, http.MethodPost, "/api/v1/admin/users", token, "application/json", `{"login":"x"}`).Code)
		require.Equal(t, http.StatusConflict, hit(t, r, http.MethodPatch, "/api/v1/admin/users/u1/role", token, "application/json", `{"role":"admin"}`).Code)
		require.Equal(t, http.StatusConflict, hit(t, r, http.MethodDelete, "/api/v1/admin/users/u1", token, "", "").Code)
	})

	t.Run("tree errors", func(t *testing.T) {
		f.tree.err = pkgerrors.ErrUnavailable
		defer func() { f.tree.err = nil }()
		require.Equal(t, http.StatusServiceUnavailable, hit(t, r, http.MethodGet, "/api/v1/tasks/tree", token, "", "").Code)
	})

	t.Run("favorite remove errors", func(t *testing.T) {
		f.favorite.err = pkgerrors.ErrNotFound
		defer func() { f.favorite.err = nil }()
		require.Equal(t, http.StatusNotFound, hit(t, r, http.MethodDelete, "/api/v1/favorites/workspaces/ws1", token, "", "").Code)
		require.Equal(t, http.StatusNotFound, hit(t, r, http.MethodPut, "/api/v1/favorites/boards/b1", token, "", "").Code)
		require.Equal(t, http.StatusNotFound, hit(t, r, http.MethodPut, "/api/v1/favorites/workspaces/ws1", token, "", "").Code)
	})

	t.Run("refresh error", func(t *testing.T) {
		f.auth.err = pkgerrors.ErrUnauthorized
		defer func() { f.auth.err = nil }()
		require.Equal(t, http.StatusUnauthorized, hit(t, r, http.MethodPost, "/api/v1/auth/refresh", "", "application/json", `{"refresh_token":"x"}`).Code)
	})
}

func TestFullRouterParseFailures(t *testing.T) {
	r, _ := newHandlerErrorRouter(t)
	token := "t"

	w := hit(t, r, http.MethodGet, "/api/v1/admin/users?page=abc", token, "", "")
	require.Equal(t, http.StatusBadRequest, w.Code)

	w = hit(t, r, http.MethodPost, "/api/v1/me/password", token, "application/json", "not-json")
	require.Equal(t, http.StatusBadRequest, w.Code)

	for _, tc := range []struct {
		method, path string
	}{
		{http.MethodPatch, "/api/v1/workspaces/w1"},
		{http.MethodPost, "/api/v1/workspaces/w1/members"},
		{http.MethodPatch, "/api/v1/workspaces/w1/members/u1"},
		{http.MethodPost, "/api/v1/workspaces/w1/owner"},
	} {
		w = hit(t, r, tc.method, tc.path, token, "application/json", "not-json")
		require.Equal(t, http.StatusBadRequest, w.Code, "%s %s", tc.method, tc.path)
	}
}
