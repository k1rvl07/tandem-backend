package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	muser "github.com/tandem/tandem/internal/domain/models/user"
	"github.com/tandem/tandem/internal/infrastructure/password"
	"github.com/tandem/tandem/internal/pkg/config"
	"github.com/tandem/tandem/internal/usecase/testutil"
	"go.uber.org/zap"
)

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
