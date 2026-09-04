package app

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/tandem/tandem/docs"
	"github.com/tandem/tandem/internal/http/handler"
	"github.com/tandem/tandem/internal/http/router"
	miniofs "github.com/tandem/tandem/internal/infrastructure/minio"
	"github.com/tandem/tandem/internal/infrastructure/password"
	rediscache "github.com/tandem/tandem/internal/infrastructure/redis"
	"github.com/tandem/tandem/internal/infrastructure/token"
	wshub "github.com/tandem/tandem/internal/infrastructure/ws"
	"github.com/tandem/tandem/internal/pkg/config"
	"github.com/tandem/tandem/internal/repository"
	"github.com/tandem/tandem/internal/repository/entity"
	"github.com/tandem/tandem/internal/usecase/auth"
	file "github.com/tandem/tandem/internal/usecase/file"
	"go.uber.org/zap"
)

type App struct {
	config *config.Config
	logger *zap.Logger

	postgres  *repository.Postgres
	redis     *rediscache.Redis
	fileStore *miniofs.MinIO
	hub       *wshub.Hub

	server *http.Server
}

func New(cfg *config.Config, logger *zap.Logger) (*App, error) {
	postgres, err := repository.NewPostgres(cfg.Database, repository.WithConnectTimeout(10*time.Second))
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

	if err := postgres.AutoMigrate(&entity.User{}); err != nil {
		_ = redis.Close()
		_ = postgres.Close()
		return nil, err
	}
	logger.Info("database migrated")

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

	userRepo := repository.NewUserRepo(a.postgres.DB)
	tokenManager := token.NewJWTManager(a.config.JWT.Secret)
	hasher := password.NewBCryptHasher()
	authService := auth.NewService(userRepo, tokenManager, hasher, a.config.JWT.TokenTTL)
	authHandler := handler.NewAuthHandler(authService)
	fileService := file.NewService(a.fileStore)

	r := router.New(router.Dependencies{
		Logger:        a.logger,
		AllowedOrigin: a.config.App.AllowedOrigins,
		FileStore:     a.fileStore,
		Hub:           a.hub,
		TokenService:  tokenManager,
		AuthHandler:   authHandler,
		Files:         fileService,
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
