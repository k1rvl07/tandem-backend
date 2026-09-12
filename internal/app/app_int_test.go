package app_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	tandemapp "github.com/tandem/tandem/internal/app"
	"github.com/tandem/tandem/internal/pkg/config"
	"github.com/tandem/tandem/internal/repository/testutil"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const truncateSQL = "TRUNCATE users, workspaces, workspace_members, boards, board_columns, tasks, task_attachments, favorites CASCADE"

type harness struct {
	ts  *httptest.Server
	app *tandemapp.App
	cfg *config.Config
	db  *gorm.DB
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
			Env:           "testing",
			AdminLogin:    "admin",
			AdminPassword: "admin12345",
			PasswordCost:  10,
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
	return &harness{ts: ts, app: a, cfg: cfg, db: db}
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
	var resp pair
	status, body := h.do(t, http.MethodPost, "/api/v1/auth/login", "", fmt.Sprintf(`{"login":%q,"password":%q}`, login, password))
	require.Equal(t, http.StatusOK, status, "login failed: %s", body)
	require.NoError(t, json.Unmarshal(body, &resp))
	require.NotEmpty(t, resp.Token)
	require.NotEmpty(t, resp.RefreshToken)
	return resp
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

	var refreshed pair
	status, body := h.do(t, http.MethodPost, "/api/v1/auth/refresh", "", fmt.Sprintf(`{"refresh_token":%q}`, admin.RefreshToken))
	require.Equal(t, http.StatusOK, status, "refresh failed: %s", body)
	require.NoError(t, json.Unmarshal(body, &refreshed))
	require.NotEmpty(t, refreshed.Token)
	require.NotEqual(t, admin.RefreshToken, refreshed.RefreshToken)

	status, _ = h.do(t, http.MethodPost, "/api/v1/auth/refresh", "", fmt.Sprintf(`{"refresh_token":%q}`, admin.RefreshToken))
	require.Equal(t, http.StatusUnauthorized, status, "reused refresh token must be rejected")

	status, body = h.do(t, http.MethodPost, "/api/v1/me/password", admin.Token,
		`{"current_password":"admin12345","new_password":"admin54321"}`)
	require.Equal(t, http.StatusOK, status, "change password failed: %s", body)
	var changed pair
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

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
