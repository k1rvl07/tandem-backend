package config

import (
	"fmt"
	"os"
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
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
	Bucket    string
}

type JWTConfig struct {
	Secret   string
	TokenTTL time.Duration
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
			Port: getEnv("APP_PORT", "8080"),
			Env:  getEnv("APP_ENV", "dev"),
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
			Endpoint:  getEnv("MINIO_ENDPOINT", "localhost:9000"),
			AccessKey: getEnv("MINIO_ROOT_USER", "tandem"),
			SecretKey: getEnv("MINIO_ROOT_PASSWORD", "change_me_minio"),
			UseSSL:    getEnv("MINIO_USE_SSL", "false") == "true",
			Bucket:    getEnv("MINIO_BUCKET", "tandem-files"),
		},
		JWT: JWTConfig{
			Secret:   getEnv("JWT_SECRET", ""),
			TokenTTL: getEnvDuration("JWT_TTL", 24*time.Hour),
		},
	}

	if cfg.JWT.Secret == "" || cfg.JWT.Secret == "CHANGE_ME" {
		return nil, fmt.Errorf("JWT_SECRET must be set to a non-default value")
	}

	cfg.App.AllowedOrigins = allowedOrigins(cfg.App.Env)

	return cfg, nil
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
