package app_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	tandemapp "github.com/tandem/tandem/internal/app"
	"github.com/tandem/tandem/internal/http/handler/common"
	"github.com/tandem/tandem/internal/pkg/config"
	"github.com/tandem/tandem/internal/repository/testutil"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const truncateSQL = "TRUNCATE users, workspaces, workspace_members, boards, board_columns, tasks, task_attachments, favorites CASCADE"

type harness struct {
	ts     *httptest.Server
	app    *tandemapp.App
	cfg    *config.Config
	db     *gorm.DB
	client *http.Client
}

type pair struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	testutil.Serialize(t)
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping http integration tests")
	}
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	dbName := strings.TrimPrefix(u.Path, "/")
	require.NotEqual(t, "tandem", dbName,
		"refusing to truncate the live database; point TEST_DATABASE_URL to a dedicated test database")
	pw, _ := u.User.Password()
	cfg := &config.Config{
		App: config.AppConfig{
			Env:            "testing",
			AdminLogin:     "admin",
			AdminPassword:  "admin12345",
			PasswordCost:   10,
			SwaggerEnabled: true,
		},
		Database: config.DatabaseConfig{
			Host:     u.Hostname(),
			Port:     u.Port(),
			User:     u.User.Username(),
			Password: pw,
			Name:     dbName,
		},
		Redis: config.RedisConfig{
			Addr:     envOr("TEST_REDIS_ADDR", "localhost:6379"),
			Password: envOr("TEST_REDIS_PASSWORD", "secret_redis"),
		},
		MinIO: config.MinIOConfig{
			Endpoint:  envOr("TEST_MINIO_ENDPOINT", "localhost:9000"),
			AccessKey: envOr("TEST_MINIO_ROOT_USER", "tandem"),
			SecretKey: envOr("TEST_MINIO_ROOT_PASSWORD", "secret_minio"),
			Region:    "us-east-1",
			UseSSL:    false,
			Bucket:    envOr("TEST_MINIO_BUCKET", "tandem-files"),
		},
		JWT: config.JWTConfig{
			Secret:     "test-secret-which-is-definitely-not-default",
			TokenTTL:   time.Hour,
			RefreshTTL: 24 * time.Hour,
		},
	}
	a, err := tandemapp.New(cfg, zap.NewNop())
	require.NoError(t, err)
	t.Cleanup(a.Close)
	db := a.Postgres()
	require.NoError(t, db.Exec(truncateSQL).Error)
	r, err := a.BuildRouter(context.Background())
	require.NoError(t, err)
	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	return &harness{ts: ts, app: a, cfg: cfg, db: db, client: &http.Client{Jar: jar}}
}

func (h *harness) do(t *testing.T, method, path, token, body string) (int, []byte) {
	t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, h.ts.URL+path, rd)
	require.NoError(t, err)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := h.ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, data
}

func (h *harness) login(t *testing.T, login, password string) pair {
	req, err := http.NewRequest(http.MethodPost, h.ts.URL+"/api/v1/auth/login",
		strings.NewReader(fmt.Sprintf(`{"login":%q,"password":%q}`, login, password)))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "login failed: %s", data)
	var claims struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.Unmarshal(data, &claims))
	require.NotEmpty(t, claims.Token)
	rt := refreshCookie(resp.Cookies())
	require.NotEmpty(t, rt, "login must set the tandem_refresh cookie")
	return pair{Token: claims.Token, RefreshToken: rt}
}

func (h *harness) refreshViaCookie(t *testing.T, path string) pair {
	req, err := http.NewRequest(http.MethodPost, h.ts.URL+path, nil)
	require.NoError(t, err)
	resp, err := h.client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "refresh failed: %s", data)
	var claims struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.Unmarshal(data, &claims))
	require.NotEmpty(t, claims.Token)
	return pair{Token: claims.Token, RefreshToken: refreshCookie(resp.Cookies())}
}

func refreshCookie(cookies []*http.Cookie) string {
	for _, c := range cookies {
		if c.Name == common.RefreshCookieName {
			return c.Value
		}
	}
	return ""
}

func (h *harness) createUser(t *testing.T, adminToken, login, password, role string) string {
	status, body := h.do(t, http.MethodPost, "/api/v1/admin/users", adminToken,
		fmt.Sprintf(`{"login":%q,"password":%q,"role":%q}`, login, password, role))
	require.Equal(t, http.StatusCreated, status, "create user failed: %s", body)
	var u struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(body, &u))
	require.NotEmpty(t, u.ID)
	return u.ID
}

func TestE2EAuthFlow(t *testing.T) {
	h := newHarness(t)

	admin := h.login(t, h.cfg.App.AdminLogin, h.cfg.App.AdminPassword)

	status, _ := h.do(t, http.MethodGet, "/api/v1/me", admin.Token, "")
	require.Equal(t, http.StatusOK, status)

	refreshed := h.refreshViaCookie(t, "/api/v1/auth/refresh")
	require.NotEmpty(t, refreshed.RefreshToken, "refresh must rotate the refresh cookie")
	require.NotEqual(t, admin.RefreshToken, refreshed.RefreshToken)

	status, _ = h.do(t, http.MethodPost, "/api/v1/auth/refresh", "", fmt.Sprintf(`{"refresh_token":%q}`, admin.RefreshToken))
	require.Equal(t, http.StatusUnauthorized, status, "reused refresh token must be rejected")

	status, body := h.do(t, http.MethodPost, "/api/v1/me/password", admin.Token,
		`{"current_password":"admin12345","new_password":"admin54321"}`)
	require.Equal(t, http.StatusOK, status, "change password failed: %s", body)
	var changed struct {
		Token string `json:"token"`
	}
	require.NoError(t, json.Unmarshal(body, &changed))
	require.NotEmpty(t, changed.Token)

	status, _ = h.do(t, http.MethodGet, "/api/v1/me", admin.Token, "")
	require.Equal(t, http.StatusUnauthorized, status, "old access token must be dead after password change")

	status, _ = h.do(t, http.MethodGet, "/api/v1/me", changed.Token, "")
	require.Equal(t, http.StatusOK, status)

	h.login(t, h.cfg.App.AdminLogin, "admin54321")
}

func TestE2ERequireStaff(t *testing.T) {
	h := newHarness(t)

	admin := h.login(t, h.cfg.App.AdminLogin, h.cfg.App.AdminPassword)

	h.createUser(t, admin.Token, "moder", "moder12345", "moderator")
	h.createUser(t, admin.Token, "regular", "regular12345", "user")

	mod := h.login(t, "moder", "moder12345")
	reg := h.login(t, "regular", "regular12345")

	status, _ := h.do(t, http.MethodGet, "/api/v1/admin/users", admin.Token, "")
	require.Equal(t, http.StatusOK, status)

	status, _ = h.do(t, http.MethodGet, "/api/v1/admin/users", mod.Token, "")
	require.Equal(t, http.StatusOK, status, "moderator must list users")

	status, _ = h.do(t, http.MethodGet, "/api/v1/admin/users", reg.Token, "")
	require.Equal(t, http.StatusForbidden, status, "regular user must be forbidden")
}

func TestE2EWorkspaceDeleteCascades(t *testing.T) {
	h := newHarness(t)

	admin := h.login(t, h.cfg.App.AdminLogin, h.cfg.App.AdminPassword)
	h.createUser(t, admin.Token, "owner", "owner12345", "user")
	user := h.login(t, "owner", "owner12345")

	var ws struct {
		ID string `json:"id"`
	}
	status, body := h.do(t, http.MethodPost, "/api/v1/workspaces", user.Token,
		`{"name":"E2E WS","description":"d","prefix":"E2E"}`)
	require.Equal(t, http.StatusCreated, status, "create workspace failed: %s", body)
	require.NoError(t, json.Unmarshal(body, &ws))

	var board struct {
		ID string `json:"id"`
	}
	status, body = h.do(t, http.MethodPost, "/api/v1/workspaces/"+ws.ID+"/boards", user.Token, `{"name":"Board"}`)
	require.Equal(t, http.StatusCreated, status, "create board failed: %s", body)
	require.NoError(t, json.Unmarshal(body, &board))

	var detail struct {
		Columns []struct {
			ID string `json:"id"`
		} `json:"columns"`
	}
	status, body = h.do(t, http.MethodGet, "/api/v1/workspaces/"+ws.ID+"/boards/"+board.ID, user.Token, "")
	require.Equal(t, http.StatusOK, status, "get board failed: %s", body)
	require.NoError(t, json.Unmarshal(body, &detail))
	require.NotEmpty(t, detail.Columns)

	status, body = h.do(t, http.MethodPost, "/api/v1/workspaces/"+ws.ID+"/boards/"+board.ID+"/tasks", user.Token,
		fmt.Sprintf(`{"column_id":%q,"title":"Task"}`, detail.Columns[0].ID))
	require.Equal(t, http.StatusCreated, status, "create task failed: %s", body)

	status, _ = h.do(t, http.MethodDelete, "/api/v1/workspaces/"+ws.ID, user.Token, "")
	require.Equal(t, http.StatusOK, status, "delete workspace failed")

	for _, table := range []string{"boards", "board_columns", "tasks", "task_attachments", "workspace_members"} {
		var n int64
		require.NoError(t, h.db.Table(table).Count(&n).Error)
		require.Zero(t, n, "%s not cascade deleted", table)
	}
}

func TestE2EWorkspaceInvitesAndMembers(t *testing.T) {
	h := newHarness(t)

	admin := h.login(t, h.cfg.App.AdminLogin, h.cfg.App.AdminPassword)
	h.createUser(t, admin.Token, "invowner", "invowner12345", "user")
	aliceID := h.createUser(t, admin.Token, "alice", "alice12345", "user")
	bobID := h.createUser(t, admin.Token, "bob", "bob12345", "user")
	owner := h.login(t, "invowner", "invowner12345")
	alice := h.login(t, "alice", "alice12345")
	bob := h.login(t, "bob", "bob12345")

	var ws struct {
		ID string `json:"id"`
	}
	status, body := h.do(t, http.MethodPost, "/api/v1/workspaces", owner.Token,
		`{"name":"Invite WS","description":"d","prefix":"INV"}`)
	require.Equal(t, http.StatusCreated, status, "create workspace failed: %s", body)
	require.NoError(t, json.Unmarshal(body, &ws))
	require.NotEmpty(t, ws.ID)

	var invite struct {
		InviteToken string `json:"invite_token"`
		ExpiresAt   string `json:"expires_at"`
	}
	status, body = h.do(t, http.MethodPost, "/api/v1/workspaces/"+ws.ID+"/invite", owner.Token, "")
	require.Equal(t, http.StatusOK, status, "generate invite failed: %s", body)
	require.NoError(t, json.Unmarshal(body, &invite))
	require.NotEmpty(t, invite.InviteToken)
	require.NotEmpty(t, invite.ExpiresAt)

	status, body = h.do(t, http.MethodPost, "/api/v1/invite/"+invite.InviteToken+"/join", alice.Token, "")
	require.Equal(t, http.StatusOK, status, "join by invite failed: %s", body)
	var joined struct {
		ID    string `json:"id"`
		Owner struct {
			ID   string `json:"id"`
			Role string `json:"role"`
		} `json:"owner"`
	}
	require.NoError(t, json.Unmarshal(body, &joined))
	require.Equal(t, ws.ID, joined.ID)

	status, _ = h.do(t, http.MethodDelete, "/api/v1/workspaces/"+ws.ID+"/invite", owner.Token, "")
	require.Equal(t, http.StatusOK, status, "disable invite failed")
	status, body = h.do(t, http.MethodPost, "/api/v1/workspaces/"+ws.ID+"/invite", owner.Token, "")
	require.Equal(t, http.StatusOK, status, "invite must be re-enabled after disable")
	var renewed struct {
		InviteToken string `json:"invite_token"`
	}
	require.NoError(t, json.Unmarshal(body, &renewed))
	require.NotEmpty(t, renewed.InviteToken)
	require.NotEqual(t, invite.InviteToken, renewed.InviteToken, "disabled invite must not be reused")

	status, body = h.do(t, http.MethodPost, "/api/v1/workspaces/"+ws.ID+"/members", owner.Token,
		`{"login":"bob"}`)
	require.Equal(t, http.StatusCreated, status, "add member failed: %s", body)

	status, _ = h.do(t, http.MethodGet, "/api/v1/workspaces/"+ws.ID+"/invite", bob.Token, "")
	require.Equal(t, http.StatusForbidden, status, "plain member must not read the invite")

	status, _ = h.do(t, http.MethodPatch, "/api/v1/workspaces/"+ws.ID+"/members/"+aliceID, owner.Token,
		`{"role":"editor"}`)
	require.Equal(t, http.StatusOK, status, "promote member to editor failed")

	status, _ = h.do(t, http.MethodGet, "/api/v1/workspaces/"+ws.ID+"/invite", alice.Token, "")
	require.Equal(t, http.StatusOK, status, "editor must read the invite")

	status, body = h.do(t, http.MethodPost, "/api/v1/workspaces/"+ws.ID+"/owner", owner.Token,
		fmt.Sprintf(`{"user_id":%q}`, aliceID))
	require.Equal(t, http.StatusOK, status, "transfer ownership failed: %s", body)

	status, _ = h.do(t, http.MethodPost, "/api/v1/workspaces/"+ws.ID+"/owner", owner.Token,
		fmt.Sprintf(`{"user_id":%q}`, aliceID))
	require.Equal(t, http.StatusForbidden, status, "former owner must not transfer")
	status, _ = h.do(t, http.MethodPost, "/api/v1/workspaces/"+ws.ID+"/members", owner.Token,
		`{"login":"admin"}`)
	require.Equal(t, http.StatusForbidden, status, "former owner must not add members")
	status, _ = h.do(t, http.MethodDelete, "/api/v1/workspaces/"+ws.ID+"/members/"+aliceID, owner.Token, "")
	require.Equal(t, http.StatusForbidden, status, "former owner must not kick members")
	status, _ = h.do(t, http.MethodGet, "/api/v1/workspaces/"+ws.ID+"/invite", owner.Token, "")
	require.Equal(t, http.StatusOK, status, "former owner (now editor) may still manage invites")

	status, _ = h.do(t, http.MethodDelete, "/api/v1/workspaces/"+ws.ID+"/members/"+bobID, alice.Token, "")
	require.Equal(t, http.StatusOK, status, "new owner must be able to kick members")

	status, _ = h.do(t, http.MethodGet, "/api/v1/workspaces/"+ws.ID, bob.Token, "")
	require.Equal(t, http.StatusForbidden, status, "kicked member must lose access")

	status, _ = h.do(t, http.MethodGet, "/api/v1/workspaces/"+ws.ID, alice.Token, "")
	require.Equal(t, http.StatusOK, status, "owner must keep access after kicking a member")

	status, _ = h.do(t, http.MethodDelete, "/api/v1/workspaces/"+ws.ID+"/members/"+aliceID, alice.Token, "")
	require.Equal(t, http.StatusBadRequest, status, "self-removal of the owner must be rejected")
}

func TestE2EBoardTasksAndFavorites(t *testing.T) {
	h := newHarness(t)

	admin := h.login(t, h.cfg.App.AdminLogin, h.cfg.App.AdminPassword)
	h.createUser(t, admin.Token, "bowner", "bowner12345", "user")
	user := h.login(t, "bowner", "bowner12345")

	var ws struct {
		ID string `json:"id"`
	}
	status, body := h.do(t, http.MethodPost, "/api/v1/workspaces", user.Token,
		`{"name":"Board WS","description":"d","prefix":"BRD"}`)
	require.Equal(t, http.StatusCreated, status, "create workspace failed: %s", body)
	require.NoError(t, json.Unmarshal(body, &ws))

	var b1, b2 struct {
		ID string `json:"id"`
	}
	status, body = h.do(t, http.MethodPost, "/api/v1/workspaces/"+ws.ID+"/boards", user.Token, `{"name":"Board One"}`)
	require.Equal(t, http.StatusCreated, status, "create board failed: %s", body)
	require.NoError(t, json.Unmarshal(body, &b1))
	status, body = h.do(t, http.MethodPost, "/api/v1/workspaces/"+ws.ID+"/boards", user.Token, `{"name":"Board Two"}`)
	require.Equal(t, http.StatusCreated, status, "create second board failed: %s", body)
	require.NoError(t, json.Unmarshal(body, &b2))

	status, body = h.do(t, http.MethodGet, "/api/v1/workspaces/"+ws.ID+"/boards", user.Token, "")
	require.Equal(t, http.StatusOK, status)
	var allBoards []struct {
		ID     string `json:"id"`
		IsMain bool   `json:"is_main"`
	}
	require.NoError(t, json.Unmarshal(body, &allBoards))
	require.Len(t, allBoards, 3, "workspace must seed a main board plus the two created")
	var mainBoardID string
	for _, b := range allBoards {
		if b.IsMain {
			mainBoardID = b.ID
		}
	}
	require.NotEmpty(t, mainBoardID, "seeded main board missing")

	status, _ = h.do(t, http.MethodPut, "/api/v1/workspaces/"+ws.ID+"/boards/reorder", user.Token,
		fmt.Sprintf(`{"board_ids":[%q,%q,%q]}`, b2.ID, mainBoardID, b1.ID))
	require.Equal(t, http.StatusOK, status, "reorder boards failed")
	status, body = h.do(t, http.MethodGet, "/api/v1/workspaces/"+ws.ID+"/boards", user.Token, "")
	require.Equal(t, http.StatusOK, status)
	var boards []struct {
		ID       string `json:"id"`
		IsMain   bool   `json:"is_main"`
		Position int    `json:"position"`
	}
	require.NoError(t, json.Unmarshal(body, &boards))
	require.Len(t, boards, 3)
	require.Equal(t, b2.ID, boards[0].ID, "reorder must persist")
	require.Less(t, boards[0].Position, boards[1].Position)
	require.Less(t, boards[1].Position, boards[2].Position)

	status, _ = h.do(t, http.MethodPut, "/api/v1/workspaces/"+ws.ID+"/boards/"+b2.ID+"/main", user.Token, "")
	require.Equal(t, http.StatusOK, status, "set main board failed")
	status, _ = h.do(t, http.MethodPatch, "/api/v1/workspaces/"+ws.ID+"/boards/"+b1.ID, user.Token,
		`{"name":"Board One Renamed"}`)
	require.Equal(t, http.StatusOK, status, "rename board failed")

	var detail struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Columns []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			TaskCount int    `json:"task_count"`
		} `json:"columns"`
	}
	status, body = h.do(t, http.MethodGet, "/api/v1/workspaces/"+ws.ID+"/boards/"+b1.ID, user.Token, "")
	require.Equal(t, http.StatusOK, status, "get board detail failed: %s", body)
	require.NoError(t, json.Unmarshal(body, &detail))
	require.Equal(t, "Board One Renamed", detail.Name, "renamed board must be returned")
	var backlogID, todoID string
	for _, c := range detail.Columns {
		switch c.Name {
		case "Backlog":
			backlogID = c.ID
		case "To Do":
			todoID = c.ID
		}
	}
	require.NotEmpty(t, backlogID, "default Backlog column missing")
	require.NotEmpty(t, todoID, "default To Do column missing")

	var alpha, beta struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	status, body = h.do(t, http.MethodPost, "/api/v1/workspaces/"+ws.ID+"/boards/"+b1.ID+"/tasks", user.Token,
		fmt.Sprintf(`{"column_id":%q,"title":"Alpha design","assignee_id":%q}`, backlogID, h.selfID(t, user.Token)))
	require.Equal(t, http.StatusCreated, status, "create alpha failed: %s", body)
	require.NoError(t, json.Unmarshal(body, &alpha))
	require.Equal(t, "Alpha design", alpha.Title)
	status, body = h.do(t, http.MethodPost, "/api/v1/workspaces/"+ws.ID+"/boards/"+b1.ID+"/tasks", user.Token,
		fmt.Sprintf(`{"column_id":%q,"title":"Beta bug","assignee_id":%q}`, backlogID, h.selfID(t, user.Token)))
	require.Equal(t, http.StatusCreated, status, "create beta failed: %s", body)
	require.NoError(t, json.Unmarshal(body, &beta))

	var sub struct {
		ID       string `json:"id"`
		ParentID string `json:"parent_id"`
	}
	status, body = h.do(t, http.MethodPost, "/api/v1/workspaces/"+ws.ID+"/boards/"+b1.ID+"/tasks", user.Token,
		fmt.Sprintf(`{"column_id":%q,"title":"sub of alpha","parent_id":%q}`, todoID, alpha.ID))
	require.Equal(t, http.StatusCreated, status, "create subtask failed: %s", body)
	require.NoError(t, json.Unmarshal(body, &sub))
	require.Equal(t, alpha.ID, sub.ParentID)

	var tList []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	listTasks := func(query string) []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} {
		var out []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		}
		status, body := h.do(t, http.MethodGet, "/api/v1/workspaces/"+ws.ID+"/tasks"+query, user.Token, "")
		require.Equal(t, http.StatusOK, status, "list tasks with %s failed: %s", query, body)
		require.NoError(t, json.Unmarshal(body, &out))
		return out
	}
	tList = listTasks("?status=Backlog")
	require.Len(t, tList, 2, "Backlog must hold alpha and beta")
	tList = listTasks("?q=alpha&exclude_subtasks=true")
	require.Len(t, tList, 1)
	require.Equal(t, alpha.ID, tList[0].ID)
	tList = listTasks("?exclude_subtasks=true")
	require.Len(t, tList, 2, "subtask must be excluded")
	tList = listTasks(fmt.Sprintf("?board_id=%s&only=mine", b1.ID))
	require.Len(t, tList, 3)
	tList = listTasks(fmt.Sprintf("?assignee_id=%s&only=for_me", h.selfID(t, user.Token)))
	require.Len(t, tList, 2, "unassigned subtask must not match for_me")

	status, body = h.do(t, http.MethodGet, "/api/v1/workspaces/"+ws.ID+"/tasks/"+alpha.ID, user.Token, "")
	require.Equal(t, http.StatusOK, status, "get alpha detail failed: %s", body)
	var alphaDetail struct {
		ID       string `json:"id"`
		Subtasks []struct {
			ID string `json:"id"`
		} `json:"subtasks"`
	}
	require.NoError(t, json.Unmarshal(body, &alphaDetail))
	require.Len(t, alphaDetail.Subtasks, 1)
	require.Equal(t, sub.ID, alphaDetail.Subtasks[0].ID)

	status, body = h.do(t, http.MethodGet, "/api/v1/workspaces/"+ws.ID+"/tasks/"+sub.ID, user.Token, "")
	require.Equal(t, http.StatusOK, status)
	var subDetail struct {
		Parent struct {
			ID string `json:"id"`
		} `json:"parent"`
	}
	require.NoError(t, json.Unmarshal(body, &subDetail))
	require.Equal(t, alpha.ID, subDetail.Parent.ID)

	status, body = h.do(t, http.MethodPatch, "/api/v1/workspaces/"+ws.ID+"/boards/"+b1.ID+"/tasks/"+beta.ID, user.Token,
		fmt.Sprintf(`{"title":"Beta urgent","column_id":%q,"is_urgent":true,"is_hidden":true}`, todoID))
	require.Equal(t, http.StatusOK, status, "update task failed: %s", body)
	status, body = h.do(t, http.MethodGet, "/api/v1/workspaces/"+ws.ID+"/tasks/"+beta.ID, user.Token, "")
	require.Equal(t, http.StatusOK, status)
	var betaDetail struct {
		ID       string `json:"id"`
		IsUrgent bool   `json:"is_urgent"`
		IsHidden bool   `json:"is_hidden"`
	}
	require.NoError(t, json.Unmarshal(body, &betaDetail))
	require.True(t, betaDetail.IsUrgent, "move+urgent update must persist")
	require.True(t, betaDetail.IsHidden)
	tList = listTasks("?status=Backlog")
	require.Len(t, tList, 1, "beta must leave Backlog after the move")

	status, _ = h.do(t, http.MethodPut, "/api/v1/favorites/boards/"+b1.ID, user.Token, "")
	require.Equal(t, http.StatusOK, status, "favorite board failed")
	status, _ = h.do(t, http.MethodPut, "/api/v1/favorites/workspaces/"+ws.ID, user.Token, "")
	require.Equal(t, http.StatusOK, status, "favorite workspace failed")
	status, body = h.do(t, http.MethodGet, "/api/v1/workspaces", user.Token, "")
	require.Equal(t, http.StatusOK, status)
	var wsList []struct {
		ID         string `json:"id"`
		IsFavorite bool   `json:"is_favorite"`
	}
	require.NoError(t, json.Unmarshal(body, &wsList))
	require.Len(t, wsList, 1)
	require.True(t, wsList[0].IsFavorite)
	status, body = h.do(t, http.MethodGet, "/api/v1/workspaces/"+ws.ID+"/boards", user.Token, "")
	require.Equal(t, http.StatusOK, status)
	var favBoards []struct {
		ID         string `json:"id"`
		IsFavorite bool   `json:"is_favorite"`
	}
	require.NoError(t, json.Unmarshal(body, &favBoards))
	require.Len(t, favBoards, 3)
	seenFavorite := false
	for _, b := range favBoards {
		if b.ID == b1.ID {
			seenFavorite = b.IsFavorite
		}
	}
	require.True(t, seenFavorite, "board favorite flag must be reflected")

	status, _ = h.do(t, http.MethodDelete, "/api/v1/favorites/boards/"+b1.ID, user.Token, "")
	require.Equal(t, http.StatusOK, status)
	status, body = h.do(t, http.MethodGet, "/api/v1/workspaces/"+ws.ID+"/boards", user.Token, "")
	require.Equal(t, http.StatusOK, status)
	require.NoError(t, json.Unmarshal(body, &favBoards))
	for _, b := range favBoards {
		if b.ID == b1.ID {
			require.False(t, b.IsFavorite, "unfavorite must persist")
		}
	}

	status, body = h.do(t, http.MethodGet, "/api/v1/tasks/tree", user.Token, "")
	require.Equal(t, http.StatusOK, status, "tree failed: %s", body)
	var tree []struct {
		Workspace struct {
			ID string `json:"id"`
		} `json:"workspace"`
		Boards []struct {
			Board struct {
				ID string `json:"id"`
			} `json:"board"`
			Tasks []struct {
				ID string `json:"id"`
			} `json:"tasks"`
		} `json:"boards"`
	}
	require.NoError(t, json.Unmarshal(body, &tree))
	require.Len(t, tree, 1)
	require.Equal(t, ws.ID, tree[0].Workspace.ID)
	tasksOnB1 := 0
	for _, b := range tree[0].Boards {
		if b.Board.ID == b1.ID {
			tasksOnB1 = len(b.Tasks)
		}
	}
	require.NotEqual(t, 0, tasksOnB1, "board one must be in the tree with its tasks")

	status, _ = h.do(t, http.MethodDelete, "/api/v1/workspaces/"+ws.ID+"/boards/"+b1.ID+"/tasks/"+alpha.ID, user.Token, "")
	require.Equal(t, http.StatusOK, status, "delete task failed")
	status, _ = h.do(t, http.MethodGet, "/api/v1/workspaces/"+ws.ID+"/tasks/"+alpha.ID, user.Token, "")
	require.Equal(t, http.StatusNotFound, status, "deleted task must be gone")
}

func (h *harness) selfID(t *testing.T, token string) string {
	t.Helper()
	status, body := h.do(t, http.MethodGet, "/api/v1/me", token, "")
	require.Equal(t, http.StatusOK, status, "get profile failed: %s", body)
	var me struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(body, &me))
	require.NotEmpty(t, me.ID)
	return me.ID
}

func TestE2ESwaggerUI(t *testing.T) {
	h := newHarness(t)

	status, body := h.do(t, http.MethodGet, "/swagger/index.html", "", "")
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, string(body), "swagger-ui-bundle.js")

	status, body = h.do(t, http.MethodGet, "/swagger/swagger-initializer.js", "", "")
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, string(body), "SwaggerUIBundle")

	status, body = h.do(t, http.MethodGet, "/swagger/doc.json", "", "")
	require.Equal(t, http.StatusOK, status)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(body, &doc))
	require.Equal(t, "3.1.0", doc["openapi"])
	paths, ok := doc["paths"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, paths, "/api/v1/auth/login")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
