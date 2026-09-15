package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	App       AppConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	MinIO     MinIOConfig
	JWT       JWTConfig
	RateLimit RateLimitConfig
}

type AppConfig struct {
	Port           string
	Env            string
	AllowedOrigins []string
	TrustedProxies []string
	AdminLogin     string
	AdminPassword  string
	AdminSeed      bool
	PasswordCost   int
	SwaggerEnabled bool
	CookieSecure   bool
}

type LimitConfig struct {
	Limit  int64
	Window time.Duration
}

type RateLimitConfig struct {
	Enabled bool
	Auth    LimitConfig
	Read    LimitConfig
	Write   LimitConfig
	Upload  LimitConfig
	WS      LimitConfig
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type MinIOConfig struct {
	Endpoint       string
	AccessKey      string
	SecretKey      string
	UseSSL         bool
	Region         string
	PublicEndpoint string
	PublicUseSSL   bool
	Bucket         string
}

type JWTConfig struct {
	Secret     string
	TokenTTL   time.Duration
	RefreshTTL time.Duration
}

func Load(envFile string) (*Config, error) {
	if envFile != "" {
		if err := godotenv.Load(envFile); err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("load env file %q: %w", envFile, err)
			}
		}
	}

	cfg := &Config{
		App: AppConfig{
			Port:          getEnv("APP_PORT", "8080"),
			Env:           getEnv("APP_ENV", "dev"),
			AdminLogin:    getEnv("ADMIN_LOGIN", ""),
			AdminPassword: getEnv("ADMIN_PASSWORD", ""),
			AdminSeed:     getEnv("ADMIN_SEED_ENABLED", "") == "true",
			CookieSecure:  getEnv("COOKIE_SECURE", "") == "true",
		},
		Database: DatabaseConfig{
			Host:     getEnv("POSTGRES_HOST", "localhost"),
			Port:     getEnv("POSTGRES_PORT", "5432"),
			User:     getEnv("POSTGRES_USER", "tandem"),
			Password: getEnv("POSTGRES_PASSWORD", "change_me"),
			Name:     getEnv("POSTGRES_DB", "tandem"),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		MinIO: MinIOConfig{
			Endpoint:       getEnv("MINIO_ENDPOINT", "localhost:9000"),
			AccessKey:      getEnv("MINIO_ROOT_USER", "tandem"),
			SecretKey:      getEnv("MINIO_ROOT_PASSWORD", "change_me_minio"),
			UseSSL:         getEnv("MINIO_USE_SSL", "false") == "true",
			Region:         getEnv("MINIO_REGION", "us-east-1"),
			PublicEndpoint: getEnv("MINIO_PUBLIC_ENDPOINT", ""),
			PublicUseSSL:   getEnv("MINIO_PUBLIC_USE_SSL", "false") == "true",
			Bucket:         getEnv("MINIO_BUCKET", "tandem-files"),
		},
		JWT: JWTConfig{
			Secret:     getEnv("JWT_SECRET", ""),
			TokenTTL:   getEnvDuration("JWT_TTL", 24*time.Hour),
			RefreshTTL: getEnvDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
		},
	}

	if isDefault(cfg.JWT.Secret, "CHANGE_ME", "secret_jwt", "change_me") {
		return nil, fmt.Errorf("JWT_SECRET must be set to a non-default value")
	}

	if cfg.App.Env == "production" {
		if err := validateProductionSecrets(cfg); err != nil {
			return nil, err
		}
	}

	cfg.App.PasswordCost = passwordCost()

	cfg.RateLimit = rateLimitConfig(cfg.App.Env)

	cfg.App.SwaggerEnabled = swaggerEnabled(cfg.App.Env)

	cfg.App.AllowedOrigins = allowedOrigins(cfg.App.Env)

	trustedProxies, err := trustedProxies()
	if err != nil {
		return nil, err
	}
	cfg.App.TrustedProxies = trustedProxies

	return cfg, nil
}

func isDefault(value string, defaults ...string) bool {
	if value == "" {
		return true
	}
	for _, d := range defaults {
		if value == d {
			return true
		}
	}
	return false
}

func validateProductionSecrets(cfg *Config) error {
	checks := []struct {
		name string
		val  string
		bad  []string
	}{
		{"POSTGRES_PASSWORD", cfg.Database.Password, []string{"change_me", "secret_postgres"}},
		{"REDIS_PASSWORD", cfg.Redis.Password, []string{"secret_redis"}},
		{"MINIO_ROOT_PASSWORD", cfg.MinIO.SecretKey, []string{"change_me_minio", "secret_minio"}},
	}
	for _, c := range checks {
		if isDefault(c.val, c.bad...) {
			return fmt.Errorf("%s must be set to a non-default value in production", c.name)
		}
	}
	if isDefault(cfg.App.AdminPassword, "secret_admin", "admin", "password") {
		return fmt.Errorf("ADMIN_PASSWORD must be set to a non-default value in production")
	}
	return nil
}

func trustedProxies() ([]string, error) {
	raw := getEnv("TRUSTED_PROXIES", "")
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	proxies := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p == "" {
			continue
		}
		_, _, err := net.ParseCIDR(p)
		if err != nil {
			if ip := net.ParseIP(p); ip != nil {
				proxies = append(proxies, p)
				continue
			}
			return nil, fmt.Errorf("invalid trusted proxy %q: must be an IP or CIDR", p)
		}
		proxies = append(proxies, p)
	}
	return proxies, nil
}

func passwordCost() int {
	v := getEnv("BCRYPT_COST", "12")
	cost, err := strconv.Atoi(v)
	if err != nil || cost < 10 || cost > 15 {
		return 12
	}
	return cost
}

const (
	defaultAuthLimit    = 15
	defaultAuthWindow   = 5 * time.Minute
	defaultReadLimit    = 1200
	defaultReadWindow   = time.Minute
	defaultWriteLimit   = 400
	defaultWriteWindow  = time.Minute
	defaultUploadLimit  = 120
	defaultUploadWindow = time.Minute
	defaultWsLimit      = 300
	defaultWsWindow     = 5 * time.Minute
)

func rateLimitConfig(env string) RateLimitConfig {
	return RateLimitConfig{
		Enabled: rateLimitEnabled(env),
		Auth: LimitConfig{
			Limit:  rateLimit("AUTH_RATE_LIMIT", defaultAuthLimit),
			Window: rateLimitWindow("AUTH_RATE_WINDOW", defaultAuthWindow),
		},
		Read: LimitConfig{
			Limit:  rateLimit("READ_RATE_LIMIT", defaultReadLimit),
			Window: rateLimitWindow("READ_RATE_WINDOW", defaultReadWindow),
		},
		Write: LimitConfig{
			Limit:  rateLimit("WRITE_RATE_LIMIT", defaultWriteLimit),
			Window: rateLimitWindow("WRITE_RATE_WINDOW", defaultWriteWindow),
		},
		Upload: LimitConfig{
			Limit:  rateLimit("UPLOAD_RATE_LIMIT", defaultUploadLimit),
			Window: rateLimitWindow("UPLOAD_RATE_WINDOW", defaultUploadWindow),
		},
		WS: LimitConfig{
			Limit:  rateLimit("WS_RATE_LIMIT", defaultWsLimit),
			Window: rateLimitWindow("WS_RATE_WINDOW", defaultWsWindow),
		},
	}
}

func rateLimitEnabled(env string) bool {
	switch getEnv("RATE_LIMIT_ENABLED", "") {
	case "true":
		return true
	case "false":
		return false
	}
	return env != "testing"
}

func rateLimit(key string, def int64) int64 {
	v := getEnv(key, "")
	if v == "" {
		return def
	}
	limit, err := strconv.ParseInt(v, 10, 64)
	if err != nil || limit < 1 {
		return def
	}
	return limit
}

func rateLimitWindow(key string, def time.Duration) time.Duration {
	return getEnvDuration(key, def)
}

func allowedOrigins(env string) []string {
	if v := os.Getenv("CORS_ALLOWED_ORIGINS"); v != "" {
		parts := strings.Split(v, ",")
		origins := make([]string, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				origins = append(origins, p)
			}
		}
		return origins
	}
	if env == "production" {
		return nil
	}
	return []string{"http://localhost:5173", "http://127.0.0.1:5173"}
}

func swaggerEnabled(env string) bool {
	raw := getEnv("SWAGGER_ENABLED", "")
	if raw != "" {
		return raw == "true"
	}
	return env != "production"
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
