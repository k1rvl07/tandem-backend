package main

import (
	"log"

	"github.com/tandem/tandem/internal/app"
	"github.com/tandem/tandem/internal/pkg/config"
	"go.uber.org/zap"
)

const envFile = ".env.dev"

// @title       Tandem API
// @version     1.0
// @description Collaborative task management API for development teams.
// @host        localhost:8080
// @BasePath    /
func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("init logger: %v", err)
	}
	defer func() { _ = logger.Sync() }()

	cfg, err := config.Load(envFile)
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
