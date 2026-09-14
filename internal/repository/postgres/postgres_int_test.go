package postgres_test

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tandem/tandem/internal/pkg/config"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	euser "github.com/tandem/tandem/internal/repository/entity/user"
	"github.com/tandem/tandem/internal/repository/postgres"
	"github.com/tandem/tandem/internal/repository/testutil"
)

func testDBCfg(t *testing.T) config.DatabaseConfig {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping postgres integration tests")
	}
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	require.NotEqual(t, "tandem", strings.TrimPrefix(u.Path, "/"),
		"refusing to run against the live database")
	pw, _ := u.User.Password()
	return config.DatabaseConfig{
		Host: u.Hostname(), Port: u.Port(),
		User: u.User.Username(), Password: pw,
		Name: strings.TrimPrefix(u.Path, "/"),
	}
}

func TestNewPostgresConnectAndClose(t *testing.T) {
	cfg := testDBCfg(t)

	p, err := postgres.NewPostgres(cfg, postgres.WithConnectTimeout(5*time.Second))
	require.NoError(t, err)
	require.NotNil(t, p.DB)

	require.NoError(t, p.AutoMigrate(&euser.User{}))

	require.NoError(t, p.Close())
	require.NotNil(t, p.DB)
}

func TestNewPostgresConnectionError(t *testing.T) {
	cfg := testDBCfg(t)
	cfg.Port = "25439"

	_, err := postgres.NewPostgres(cfg, postgres.WithConnectTimeout(2*time.Second))
	require.Error(t, err)
}

func TestPostgresQueryErrorAfterClose(t *testing.T) {
	cfg := testDBCfg(t)
	p, err := postgres.NewPostgres(cfg, postgres.WithConnectTimeout(5*time.Second))
	require.NoError(t, err)
	require.NoError(t, p.Close())

	var users []euser.User
	err = p.DB.Find(&users).Error
	require.Error(t, err, "queries must fail after the pool is closed")
}

func TestHelperFirstFindListExistsPaginateCount(t *testing.T) {
	db := testutil.NewTestDB(t)
	h := postgres.NewHelper(db)
	ctx := context.Background()

	first := &euser.User{ID: testutil.NewID(), Login: "first", PasswordHash: "h", Role: "user", DisplayName: "First"}
	second := &euser.User{ID: testutil.NewID(), Login: "second", PasswordHash: "h", Role: "user", DisplayName: "Second"}
	require.NoError(t, db.Create(first).Error)
	require.NoError(t, db.Create(second).Error)

	var got euser.User
	require.NoError(t, h.First(ctx, &got, "id = ?", first.ID))
	require.Equal(t, first.ID, got.ID)

	require.ErrorIs(t, h.First(ctx, &got, "id = ?", testutil.NewID()), pkgerrors.ErrNotFound)

	var all []euser.User
	require.NoError(t, h.Find(ctx, &all, "display_name IN ?", []string{"First", "Second"}))
	require.Len(t, all, 2)

	require.NoError(t, h.List(ctx, &all, "display_name", "login IN ?", []string{"first", "second"}))
	require.Len(t, all, 2)

	ok, err := h.Exists(ctx, &euser.User{}, "id = ?", first.ID)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = h.Exists(ctx, &euser.User{}, "id = ?", testutil.NewID())
	require.NoError(t, err)
	require.False(t, ok)

	count, err := h.Count(ctx, &euser.User{}, "login IN ?", []string{"first", "second"})
	require.NoError(t, err)
	require.Equal(t, int64(2), count)

	require.NoError(t, h.Paginate(ctx, &all, "login IN ?", []interface{}{[]string{"first", "second"}}, "display_name", 1, 0))
	require.Len(t, all, 1)
}
