package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func setEnv(t *testing.T, values map[string]string) {
	t.Helper()
	for k, v := range values {
		t.Setenv(k, v)
	}
}

func TestLoadRequiresJWTSecret(t *testing.T) {
	setEnv(t, map[string]string{"JWT_SECRET": ""})
	_, err := Load("")
	require.Error(t, err)
}

func TestLoadProdRequiresNonDefaultMinIOSecret(t *testing.T) {
	tests := []struct {
		name   string
		secret string
		ok     bool
	}{
		{name: "default password rejected", secret: "change_me_minio", ok: false},
		{name: "empty password rejected", secret: "", ok: false},
		{name: "custom password accepted", secret: "s3cr3t-dev-2026", ok: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, map[string]string{
				"APP_ENV":             "production",
				"JWT_SECRET":          "test-secret",
				"MINIO_ROOT_PASSWORD": tt.secret,
				"POSTGRES_PASSWORD":   "p",
				"REDIS_PASSWORD":      "r",
				"ADMIN_PASSWORD":      "a",
			})
			cfg, err := Load("")
			if tt.ok {
				require.NoError(t, err)
			}
			if !tt.ok {
				require.Error(t, err)
			}
			if tt.ok {
				require.Equal(t, tt.secret, cfg.MinIO.SecretKey)
			}
		})
	}
}

func TestLoadProdRejectsPlaceholderInfraSecrets(t *testing.T) {
	for _, tt := range []struct {
		name string
		env  map[string]string
	}{
		{name: "postgres placeholder", env: map[string]string{"POSTGRES_PASSWORD": "secret_postgres"}},
		{name: "postgres legacy default", env: map[string]string{"POSTGRES_PASSWORD": "change_me"}},
		{name: "redis placeholder", env: map[string]string{"REDIS_PASSWORD": "secret_redis"}},
		{name: "minio placeholder", env: map[string]string{"MINIO_ROOT_PASSWORD": "secret_minio"}},
		{name: "admin placeholder", env: map[string]string{"ADMIN_PASSWORD": "secret_admin"}},
		{name: "jwt placeholder", env: map[string]string{"JWT_SECRET": "secret_jwt"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			env := map[string]string{
				"APP_ENV":             "production",
				"JWT_SECRET":          "real-jwt-secret",
				"POSTGRES_PASSWORD":   "real-pg",
				"REDIS_PASSWORD":      "real-redis",
				"MINIO_ROOT_PASSWORD": "real-minio",
				"ADMIN_PASSWORD":      "real-admin",
			}
			for k, v := range tt.env {
				env[k] = v
			}
			setEnv(t, env)
			_, err := Load("")
			require.Error(t, err)
		})
	}
}

func TestLoadTrustedProxies(t *testing.T) {
	t.Run("accepts ip and cidr", func(t *testing.T) {
		setEnv(t, map[string]string{
			"JWT_SECRET":      "test-secret",
			"TRUSTED_PROXIES": "172.16.0.0/12, 127.0.0.1",
		})
		cfg, err := Load("")
		require.NoError(t, err)
		require.Equal(t, []string{"172.16.0.0/12", "127.0.0.1"}, cfg.App.TrustedProxies)
	})
	t.Run("rejects garbage", func(t *testing.T) {
		setEnv(t, map[string]string{
			"JWT_SECRET":      "test-secret",
			"TRUSTED_PROXIES": "not-an-ip",
		})
		_, err := Load("")
		require.Error(t, err)
	})
	t.Run("empty stays nil", func(t *testing.T) {
		setEnv(t, map[string]string{
			"JWT_SECRET": "test-secret",
		})
		cfg, err := Load("")
		require.NoError(t, err)
		require.Nil(t, cfg.App.TrustedProxies)
	})
}

func TestLoadDevAllowsDefaultMinIOSecret(t *testing.T) {
	setEnv(t, map[string]string{
		"APP_ENV":             "dev",
		"JWT_SECRET":          "test-secret",
		"MINIO_ROOT_PASSWORD": "change_me_minio",
	})
	_, err := Load("")
	require.NoError(t, err)
}
