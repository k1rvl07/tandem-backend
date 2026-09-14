package app

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	muser "github.com/tandem/tandem/internal/domain/models/user"
	"github.com/tandem/tandem/internal/infrastructure/password"
	"github.com/tandem/tandem/internal/pkg/config"
	"github.com/tandem/tandem/internal/usecase/testutil"
	"go.uber.org/zap"
)

type errHasher struct{}

func (errHasher) Hash(string) (string, error) { return "", errors.New("hash boom") }
func (errHasher) Check(string, string) bool   { return false }

func TestCookieSecure(t *testing.T) {
	a := &App{config: &config.Config{App: config.AppConfig{CookieSecure: true}}}
	if !a.cookieSecure() {
		t.Fatal("expected cookieSecure=true when explicitly enabled")
	}
	a = &App{config: &config.Config{App: config.AppConfig{Env: "production"}}}
	if !a.cookieSecure() {
		t.Fatal("expected cookieSecure=true in production")
	}
	a = &App{config: &config.Config{App: config.AppConfig{Env: "dev"}}}
	if a.cookieSecure() {
		t.Fatal("expected cookieSecure=false in dev")
	}
}

func TestSeedAdminSkippedInProductionWithoutFlag(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{
		Env:           "production",
		AdminLogin:    "admin",
		AdminPassword: "admin12345",
	}}
	a := &App{config: cfg, logger: zap.NewNop()}
	repo := testutil.NewFakeUserRepo()

	err := a.seedAdmin(context.Background(), repo, password.NewBCryptHasher(12))
	require.NoError(t, err)
	assert.Empty(t, repo.Users, "production must not seed an admin without ADMIN_SEED_ENABLED")
}

func TestSeedAdminRunsInProductionWhenFlagSet(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{
		Env:           "production",
		AdminLogin:    "admin",
		AdminPassword: "admin12345",
		AdminSeed:     true,
	}}
	a := &App{config: cfg, logger: zap.NewNop()}
	repo := testutil.NewFakeUserRepo()

	err := a.seedAdmin(context.Background(), repo, password.NewBCryptHasher(12))
	require.NoError(t, err)
	require.Len(t, repo.Users, 1)
	for _, u := range repo.Users {
		assert.Equal(t, muser.RoleAdmin, u.Role)
	}
}

func TestSeedAdminRunsInDevWithCredentials(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{
		Env:           "dev",
		AdminLogin:    "admin",
		AdminPassword: "admin12345",
	}}
	a := &App{config: cfg, logger: zap.NewNop()}
	repo := testutil.NewFakeUserRepo()

	err := a.seedAdmin(context.Background(), repo, password.NewBCryptHasher(12))
	require.NoError(t, err)
	assert.Len(t, repo.Users, 1)
}

func TestSeedAdminSkippedWithoutCredentials(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{Env: "dev"}}
	a := &App{config: cfg, logger: zap.NewNop()}
	repo := testutil.NewFakeUserRepo()

	err := a.seedAdmin(context.Background(), repo, password.NewBCryptHasher(12))
	require.NoError(t, err)
	assert.Empty(t, repo.Users)
}

func TestSeedAdminLoginValidationError(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{
		Env:           "dev",
		AdminLogin:    "bad login!",
		AdminPassword: "admin12345",
	}}
	a := &App{config: cfg, logger: zap.NewNop()}

	err := a.seedAdmin(context.Background(), testutil.NewFakeUserRepo(), password.NewBCryptHasher(12))
	require.Error(t, err)
}

func TestSeedAdminPasswordValidationError(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{
		Env:           "dev",
		AdminLogin:    "admin",
		AdminPassword: "short",
	}}
	a := &App{config: cfg, logger: zap.NewNop()}

	err := a.seedAdmin(context.Background(), testutil.NewFakeUserRepo(), password.NewBCryptHasher(12))
	require.Error(t, err)
}

func TestSeedAdminLookupError(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{
		Env:           "dev",
		AdminLogin:    "admin",
		AdminPassword: "admin12345",
	}}
	a := &App{config: cfg, logger: zap.NewNop()}
	repo := testutil.NewFakeUserRepo()
	repo.Err = errors.New("lookup boom")

	err := a.seedAdmin(context.Background(), repo, password.NewBCryptHasher(12))
	require.Error(t, err)
}

func TestSeedAdminSkipsWhenLoginExists(t *testing.T) {
	cfg := &config.Config{App: config.AppConfig{
		Env:           "dev",
		AdminLogin:    "admin",
		AdminPassword: "admin12345",
	}}
	a := &App{config: cfg, logger: zap.NewNop()}
	repo := testutil.NewFakeUserRepo()
	repo.Users["001"] = &muser.User{ID: "001", Login: "admin", Role: muser.RoleAdmin}

	err := a.seedAdmin(context.Background(), repo, password.NewBCryptHasher(12))
	require.NoError(t, err)
	assert.Len(t, repo.Users, 1, "existing admin must not be re-seeded")
}

func TestCreateAdminHashError(t *testing.T) {
	a := &App{config: &config.Config{
		App: config.AppConfig{AdminPassword: "admin12345"},
	}, logger: zap.NewNop()}

	err := a.createAdmin(context.Background(), testutil.NewFakeUserRepo(), errHasher{}, "admin")
	require.Error(t, err)
}

func TestCreateAdminCreateError(t *testing.T) {
	a := &App{config: &config.Config{
		App: config.AppConfig{AdminPassword: "admin12345"},
	}, logger: zap.NewNop()}
	repo := testutil.NewFakeUserRepo()
	repo.Err = errors.New("create boom")

	err := a.createAdmin(context.Background(), repo, password.NewBCryptHasher(12), "admin")
	require.Error(t, err)
	assert.Empty(t, repo.Users)
}
