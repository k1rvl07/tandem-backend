package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	App      AppConfig
	Database DatabaseConfig
	Redis    RedisConfig
	MinIO    MinIOConfig
	JWT      JWTConfig
}

type AppConfig struct {
	Port           string
	Env            string
	AllowedOrigins []string
	AdminLogin     string
	AdminPassword  string
	PasswordCost   int
	SwaggerEnabled bool
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

	if cfg.JWT.Secret == "" || cfg.JWT.Secret == "CHANGE_ME" {
		return nil, fmt.Errorf("JWT_SECRET must be set to a non-default value")
	}

	if cfg.App.Env == "production" {
		if cfg.MinIO.SecretKey == "" || cfg.MinIO.SecretKey == "change_me_minio" {
			return nil, fmt.Errorf("MINIO_ROOT_PASSWORD must be set to a non-default value in production")
		}
	}

	cfg.App.PasswordCost = passwordCost()

	cfg.App.SwaggerEnabled = swaggerEnabled(cfg.App.Env)

	cfg.App.AllowedOrigins = allowedOrigins(cfg.App.Env)

	return cfg, nil
}

func passwordCost() int {
	v := getEnv("BCRYPT_COST", "12")
	cost, err := strconv.Atoi(v)
	if err != nil || cost < 10 || cost > 15 {
		return 12
	}
	return cost
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
