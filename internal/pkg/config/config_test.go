package config

import (
	"testing"
	"time"

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

func TestLoadProdValidConfig(t *testing.T) {
	setEnv(t, map[string]string{
		"APP_ENV":              "production",
		"JWT_SECRET":           "hex-0123456789abcdef",
		"POSTGRES_PASSWORD":    "real-pg",
		"REDIS_PASSWORD":       "real-redis",
		"MINIO_ROOT_PASSWORD":  "real-minio",
		"ADMIN_PASSWORD":       "real-admin",
		"ADMIN_SEED_ENABLED":   "true",
		"COOKIE_SECURE":        "true",
		"SWAGGER_ENABLED":      "false",
		"CORS_ALLOWED_ORIGINS": "",
	})
	cfg, err := Load("")
	require.NoError(t, err)
	require.Equal(t, "production", cfg.App.Env)
	require.True(t, cfg.App.AdminSeed)
	require.True(t, cfg.App.CookieSecure)
	require.False(t, cfg.App.SwaggerEnabled)
	require.Nil(t, cfg.App.AllowedOrigins)
}

func TestLoadSwaggerEnabled(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		swagger string
		want    bool
	}{
		{name: "dev default", env: "dev", swagger: "", want: true},
		{name: "prod default", env: "production", swagger: "", want: false},
		{name: "prod explicitly on", env: "production", swagger: "true", want: true},
		{name: "dev explicitly off", env: "dev", swagger: "false", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := map[string]string{"JWT_SECRET": "test-secret", "APP_ENV": tt.env}
			if tt.env == "production" {
				env["POSTGRES_PASSWORD"] = "real-pg"
				env["REDIS_PASSWORD"] = "real-redis"
				env["MINIO_ROOT_PASSWORD"] = "real-minio"
				env["ADMIN_PASSWORD"] = "real-admin"
			}
			if tt.swagger != "" {
				env["SWAGGER_ENABLED"] = tt.swagger
			}
			setEnv(t, env)
			cfg, err := Load("")
			require.NoError(t, err)
			require.Equal(t, tt.want, cfg.App.SwaggerEnabled)
		})
	}
}

func TestLoadAllowedOrigins(t *testing.T) {
	t.Run("dev defaults", func(t *testing.T) {
		setEnv(t, map[string]string{
			"JWT_SECRET":           "test-secret",
			"APP_ENV":              "dev",
			"CORS_ALLOWED_ORIGINS": "",
		})
		cfg, err := Load("")
		require.NoError(t, err)
		require.Equal(t, []string{"http://localhost:5173", "http://127.0.0.1:5173"}, cfg.App.AllowedOrigins)
	})
	t.Run("prod stays empty without override", func(t *testing.T) {
		setEnv(t, map[string]string{
			"JWT_SECRET":           "test-secret",
			"APP_ENV":              "production",
			"POSTGRES_PASSWORD":    "real-pg",
			"REDIS_PASSWORD":       "real-redis",
			"MINIO_ROOT_PASSWORD":  "real-minio",
			"ADMIN_PASSWORD":       "real-admin",
			"CORS_ALLOWED_ORIGINS": "",
		})
		cfg, err := Load("")
		require.NoError(t, err)
		require.Nil(t, cfg.App.AllowedOrigins)
	})
	t.Run("parses csv override", func(t *testing.T) {
		setEnv(t, map[string]string{
			"JWT_SECRET":           "test-secret",
			"CORS_ALLOWED_ORIGINS": "https://app.example.com, https://admin.example.com",
		})
		cfg, err := Load("")
		require.NoError(t, err)
		require.Equal(t, []string{"https://app.example.com", "https://admin.example.com"}, cfg.App.AllowedOrigins)
	})
}

func TestLoadPasswordCost(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int
	}{
		{name: "default", raw: "", want: 12},
		{name: "valid", raw: "14", want: 14},
		{name: "below range", raw: "9", want: 12},
		{name: "above range", raw: "16", want: 12},
		{name: "garbage", raw: "x", want: 12},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := map[string]string{"JWT_SECRET": "test-secret", "BCRYPT_COST": tt.raw}
			setEnv(t, env)
			cfg, err := Load("")
			require.NoError(t, err)
			require.Equal(t, tt.want, cfg.App.PasswordCost)
		})
	}
}

func TestLoadRateLimits(t *testing.T) {
	tests := []struct {
		name string
		env  string
		raw  string
		want int64
	}{
		{name: "auth default", env: "AUTH_RATE_LIMIT", raw: "", want: 15},
		{name: "auth valid", env: "AUTH_RATE_LIMIT", raw: "100", want: 100},
		{name: "auth below range", env: "AUTH_RATE_LIMIT", raw: "0", want: 15},
		{name: "auth garbage", env: "AUTH_RATE_LIMIT", raw: "a", want: 15},
		{name: "read default", env: "READ_RATE_LIMIT", raw: "", want: 1200},
		{name: "read valid", env: "READ_RATE_LIMIT", raw: "5000", want: 5000},
		{name: "read below range", env: "READ_RATE_LIMIT", raw: "0", want: 1200},
		{name: "read garbage", env: "READ_RATE_LIMIT", raw: "x", want: 1200},
		{name: "write default", env: "WRITE_RATE_LIMIT", raw: "", want: 400},
		{name: "write valid", env: "WRITE_RATE_LIMIT", raw: "900", want: 900},
		{name: "write below range", env: "WRITE_RATE_LIMIT", raw: "0", want: 400},
		{name: "upload default", env: "UPLOAD_RATE_LIMIT", raw: "", want: 120},
		{name: "upload valid", env: "UPLOAD_RATE_LIMIT", raw: "300", want: 300},
		{name: "upload garbage", env: "UPLOAD_RATE_LIMIT", raw: "b", want: 120},
		{name: "ws default", env: "WS_RATE_LIMIT", raw: "", want: 300},
		{name: "ws valid", env: "WS_RATE_LIMIT", raw: "600", want: 600},
		{name: "ws below range", env: "WS_RATE_LIMIT", raw: "-5", want: 300},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := map[string]string{"JWT_SECRET": "test-secret"}
			if tt.raw != "" {
				env[tt.env] = tt.raw
			}
			setEnv(t, env)
			cfg, err := Load("")
			require.NoError(t, err)
			switch tt.env {
			case "AUTH_RATE_LIMIT":
				require.Equal(t, tt.want, cfg.RateLimit.Auth.Limit)
			case "READ_RATE_LIMIT":
				require.Equal(t, tt.want, cfg.RateLimit.Read.Limit)
			case "WRITE_RATE_LIMIT":
				require.Equal(t, tt.want, cfg.RateLimit.Write.Limit)
			case "UPLOAD_RATE_LIMIT":
				require.Equal(t, tt.want, cfg.RateLimit.Upload.Limit)
			case "WS_RATE_LIMIT":
				require.Equal(t, tt.want, cfg.RateLimit.WS.Limit)
			}
		})
	}
}

func TestLoadRateLimitWindows(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		setEnv(t, map[string]string{"JWT_SECRET": "test-secret"})
		cfg, err := Load("")
		require.NoError(t, err)
		require.Equal(t, 5*time.Minute, cfg.RateLimit.Auth.Window)
		require.Equal(t, time.Minute, cfg.RateLimit.Read.Window)
		require.Equal(t, time.Minute, cfg.RateLimit.Write.Window)
		require.Equal(t, time.Minute, cfg.RateLimit.Upload.Window)
		require.Equal(t, 5*time.Minute, cfg.RateLimit.WS.Window)
	})
	t.Run("valid durations are parsed", func(t *testing.T) {
		setEnv(t, map[string]string{
			"JWT_SECRET":         "test-secret",
			"AUTH_RATE_WINDOW":   "10m",
			"READ_RATE_WINDOW":   "30s",
			"WRITE_RATE_WINDOW":  "2m",
			"UPLOAD_RATE_WINDOW": "1h",
			"WS_RATE_WINDOW":     "15m",
		})
		cfg, err := Load("")
		require.NoError(t, err)
		require.Equal(t, 10*time.Minute, cfg.RateLimit.Auth.Window)
		require.Equal(t, 30*time.Second, cfg.RateLimit.Read.Window)
		require.Equal(t, 2*time.Minute, cfg.RateLimit.Write.Window)
		require.Equal(t, time.Hour, cfg.RateLimit.Upload.Window)
		require.Equal(t, 15*time.Minute, cfg.RateLimit.WS.Window)
	})
	t.Run("invalid duration falls back", func(t *testing.T) {
		setEnv(t, map[string]string{"JWT_SECRET": "test-secret", "AUTH_RATE_WINDOW": "soon"})
		cfg, err := Load("")
		require.NoError(t, err)
		require.Equal(t, 5*time.Minute, cfg.RateLimit.Auth.Window)
	})
}

func TestLoadRateLimitEnabled(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{name: "dev default on", env: map[string]string{"APP_ENV": "dev"}, want: true},
		{name: "testing default off", env: map[string]string{"APP_ENV": "testing"}, want: false},
		{name: "production default on", env: map[string]string{"APP_ENV": "production"}, want: true},
		{name: "explicitly off", env: map[string]string{"APP_ENV": "dev", "RATE_LIMIT_ENABLED": "false"}, want: false},
		{name: "explicitly on in testing", env: map[string]string{"APP_ENV": "testing", "RATE_LIMIT_ENABLED": "true"}, want: true},
		{name: "explicitly off in production", env: map[string]string{"APP_ENV": "production", "RATE_LIMIT_ENABLED": "false"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := map[string]string{"JWT_SECRET": "test-secret"}
			if tt.env["APP_ENV"] == "production" {
				env["POSTGRES_PASSWORD"] = "real-pg"
				env["REDIS_PASSWORD"] = "real-redis"
				env["MINIO_ROOT_PASSWORD"] = "real-minio"
				env["ADMIN_PASSWORD"] = "real-admin"
			}
			for k, v := range tt.env {
				env[k] = v
			}
			setEnv(t, env)
			cfg, err := Load("")
			require.NoError(t, err)
			require.Equal(t, tt.want, cfg.RateLimit.Enabled)
		})
	}
}

func TestLoadTokenTTLs(t *testing.T) {
	t.Run("invalid duration falls back", func(t *testing.T) {
		setEnv(t, map[string]string{"JWT_SECRET": "test-secret", "JWT_TTL": "soon"})
		cfg, err := Load("")
		require.NoError(t, err)
		require.Equal(t, 24*time.Hour, cfg.JWT.TokenTTL)
	})
	t.Run("valid duration is parsed", func(t *testing.T) {
		setEnv(t, map[string]string{"JWT_SECRET": "test-secret", "JWT_TTL": "30m"})
		cfg, err := Load("")
		require.NoError(t, err)
		require.Equal(t, 30*time.Minute, cfg.JWT.TokenTTL)
	})
}

func TestLoadIgnoresMissingEnvFile(t *testing.T) {
	setEnv(t, map[string]string{"JWT_SECRET": "test-secret"})
	cfg, err := Load("missing.env")
	require.NoError(t, err)
	require.Equal(t, "dev", cfg.App.Env)
}
