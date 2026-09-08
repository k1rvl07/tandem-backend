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

func TestLoadDevAllowsDefaultMinIOSecret(t *testing.T) {
	setEnv(t, map[string]string{
		"APP_ENV":             "dev",
		"JWT_SECRET":          "test-secret",
		"MINIO_ROOT_PASSWORD": "change_me_minio",
	})
	_, err := Load("")
	require.NoError(t, err)
}
