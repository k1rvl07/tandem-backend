package main

import (
	"log"
	"os"

	"github.com/tandem/tandem/internal/app"
	"github.com/tandem/tandem/internal/pkg/config"
	"go.uber.org/zap"
)

func envFile() string {
	if file := os.Getenv("ENV_FILE"); file != "" {
		return file
	}
	if os.Getenv("APP_ENV") == "production" {
		return ".env.prod"
	}
	return ".env.dev"
}

// @title       Tandem API
// @version     1.0
// @description Collaborative task management API for development teams.
// @servers.url http://localhost:8080
// @BasePath    /
// @securityDefinitions.apikey BearerAuth
// @in           header
// @name         Authorization
func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("init logger: %v", err)
	}
	defer func() { _ = logger.Sync() }()

	cfg, err := config.Load(envFile())
	if err != nil {
		logger.Fatal("load config", zap.Error(err))
	}

	a, err := app.New(cfg, logger)
	if err != nil {
		logger.Fatal("init app", zap.Error(err))
	}

	if err := a.Run(); err != nil {
		logger.Fatal("app stopped with error", zap.Error(err))
	}
}
