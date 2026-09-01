package app

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/tandem/tandem/docs"
	"github.com/tandem/tandem/internal/delivery/router"
	"github.com/tandem/tandem/internal/infrastructure/database"
	miniofs "github.com/tandem/tandem/internal/infrastructure/minio"
	rediscache "github.com/tandem/tandem/internal/infrastructure/redis"
	wshub "github.com/tandem/tandem/internal/infrastructure/ws"
	"github.com/tandem/tandem/internal/pkg/config"
	"go.uber.org/zap"
)

type App struct {
	config *config.Config
	logger *zap.Logger

	postgres  *database.Postgres
	redis     *rediscache.Redis
	fileStore *miniofs.MinIO
	hub       *wshub.Hub

	server *http.Server
}

func New(cfg *config.Config, logger *zap.Logger) (*App, error) {
	postgres, err := database.New(cfg.Database, database.WithConnectTimeout(10*time.Second))
	if err != nil {
		return nil, err
	}
	logger.Info("connected to postgres")

	redis, err := rediscache.New(cfg.Redis)
	if err != nil {
		_ = postgres.Close()
		return nil, err
	}
	logger.Info("connected to redis")

	fileStore, err := miniofs.New(cfg.MinIO)
	if err != nil {
		_ = postgres.Close()
		_ = redis.Close()
		return nil, err
	}
	logger.Info("connected to minio", zap.String("bucket", cfg.MinIO.Bucket))

	hub := wshub.New()
	logger.Info("websocket hub initialized")

	return &App{
		config:    cfg,
		logger:    logger,
		postgres:  postgres,
		redis:     redis,
		fileStore: fileStore,
		hub:       hub,
	}, nil
}

func (a *App) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	docs.SwaggerInfo.Host = "localhost:" + a.config.App.Port

	r := router.New(router.Dependencies{
		Logger:        a.logger,
		AllowedOrigin: []string{"*"},
		FileStore:     a.fileStore,
		Hub:           a.hub,
		EnableSwagger: true,
	})

	a.server = &http.Server{
		Addr:         ":" + a.config.App.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		a.logger.Info("http server starting", zap.String("addr", a.server.Addr))
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		a.logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := a.server.Shutdown(shutdownCtx); err != nil {
		return err
	}

	a.close()
	a.logger.Info("server stopped gracefully")
	return nil
}

func (a *App) close() {
	a.hub.Close()
	if err := a.redis.Close(); err != nil {
		a.logger.Warn("close redis", zap.Error(err))
	}
	if err := a.postgres.Close(); err != nil {
		a.logger.Warn("close postgres", zap.Error(err))
	}
}
