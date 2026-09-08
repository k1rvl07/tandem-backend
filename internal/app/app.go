package app

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	_ "github.com/tandem/tandem/docs"
	"github.com/tandem/tandem/internal/domain/models"
	repoport "github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/service"
	"github.com/tandem/tandem/internal/http/handler"
	"github.com/tandem/tandem/internal/http/router"
	miniofs "github.com/tandem/tandem/internal/infrastructure/minio"
	"github.com/tandem/tandem/internal/infrastructure/password"
	rediscache "github.com/tandem/tandem/internal/infrastructure/redis"
	"github.com/tandem/tandem/internal/infrastructure/token"
	wshub "github.com/tandem/tandem/internal/infrastructure/ws"
	"github.com/tandem/tandem/internal/pkg/config"
	"github.com/tandem/tandem/internal/pkg/validate"
	"github.com/tandem/tandem/internal/repository"
	"github.com/tandem/tandem/internal/repository/entity"
	"github.com/tandem/tandem/internal/usecase/admin"
	"github.com/tandem/tandem/internal/usecase/attachment"
	"github.com/tandem/tandem/internal/usecase/auth"
	"github.com/tandem/tandem/internal/usecase/board"
	"github.com/tandem/tandem/internal/usecase/favorite"
	file "github.com/tandem/tandem/internal/usecase/file"
	"github.com/tandem/tandem/internal/usecase/profile"
	"github.com/tandem/tandem/internal/usecase/task"
	"github.com/tandem/tandem/internal/usecase/tree"
	"github.com/tandem/tandem/internal/usecase/workspace"
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

	if err := postgres.AutoMigrate(&entity.User{}, &entity.Workspace{}, &entity.WorkspaceMember{}, &entity.Board{}, &entity.Column{}, &entity.Task{}, &entity.TaskAttachment{}, &entity.Favorite{}); err != nil {
		_ = redis.Close()
		_ = postgres.Close()
		return nil, err
	}
	if err := postgres.DB.Exec("DROP TABLE IF EXISTS task_comments").Error; err != nil {
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
	tokenManager := token.NewJWTManager(a.config.JWT.Secret, a.redis)
	hasher := password.NewBCryptHasher(a.config.App.PasswordCost)
	authService := auth.NewService(userRepo, tokenManager, hasher, a.config.JWT.TokenTTL)
	authHandler := handler.NewAuthHandler(authService)
	fileService := file.NewService(a.fileStore, a.logger)
	profileService := profile.NewService(userRepo, hasher, fileService, a.redis, tokenManager, a.config.JWT.TokenTTL, a.logger)
	profileHandler := handler.NewProfileHandler(profileService)
	adminService := admin.NewService(userRepo, hasher, a.redis)
	adminHandler := handler.NewAdminHandler(adminService)
	workspaceRepo := repository.NewWorkspaceRepo(a.postgres.DB)
	favoriteRepo := repository.NewFavoriteRepo(a.postgres.DB)
	boardRepo := repository.NewBoardRepo(a.postgres.DB)
	columnRepo := repository.NewColumnRepo(a.postgres.DB)
	taskRepo := repository.NewTaskRepo(a.postgres.DB)
	attachmentRepo := repository.NewAttachmentRepo(a.postgres.DB)
	workspaceService := workspace.NewService(workspaceRepo, userRepo, favoriteRepo, boardRepo, columnRepo, a.hub, a.redis)
	workspaceHandler := handler.NewWorkspaceHandler(workspaceService)
	boardService := board.NewService(boardRepo, columnRepo, taskRepo, workspaceRepo, userRepo, favoriteRepo, fileService, a.hub, a.redis)
	boardHandler := handler.NewBoardHandler(boardService)
	taskService := task.NewService(taskRepo, columnRepo, boardRepo, workspaceRepo, userRepo, fileService, a.hub, a.redis)
	taskHandler := handler.NewTaskHandler(taskService)
	attachmentService := attachment.NewService(attachmentRepo, taskRepo, columnRepo, boardRepo, workspaceRepo, fileService, a.hub, a.redis)
	attachmentHandler := handler.NewAttachmentHandler(attachmentService)
	favoriteService := favorite.NewService(favoriteRepo, workspaceRepo, boardRepo, a.hub, a.redis)
	favoriteHandler := handler.NewFavoriteHandler(favoriteService)
	treeService := tree.NewService(workspaceRepo, boardRepo, columnRepo, taskRepo, userRepo, favoriteRepo, a.redis)
	treeHandler := handler.NewTreeHandler(treeService)

	if err := a.seedAdmin(ctx, userRepo, hasher); err != nil {
		return err
	}

	r := router.New(router.Dependencies{
		Logger:              a.logger,
		AllowedOrigin:       a.config.App.AllowedOrigins,
		FileStore:           a.fileStore,
		Hub:                 a.hub,
		TokenService:        tokenManager,
		UserRepository:      userRepo,
		WorkspaceRepository: workspaceRepo,
		AuthHandler:         authHandler,
		ProfileHandler:      profileHandler,
		AdminHandler:        adminHandler,
		WorkspaceHandler:    workspaceHandler,
		BoardHandler:        boardHandler,
		TaskHandler:         taskHandler,
		AttachmentHandler:   attachmentHandler,
		FavoriteHandler:     favoriteHandler,
		TreeHandler:         treeHandler,
		Files:               fileService,
		EnableSwagger:       true,
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

func (a *App) seedAdmin(ctx context.Context, repo repoport.UserRepository, hasher service.PasswordHasher) error {
	login := validate.NormalizeLogin(a.config.App.AdminLogin)
	if login == "" && a.config.App.AdminPassword == "" {
		a.logger.Info("admin seed skipped: ADMIN_LOGIN and ADMIN_PASSWORD not set")
		return nil
	}
	if err := validate.Login(login); err != nil {
		return err
	}
	if err := validate.Password(a.config.App.AdminPassword); err != nil {
		return err
	}

	exists, err := repo.ExistsByLogin(ctx, login)
	if err != nil {
		return err
	}
	if exists {
		a.logger.Info("admin seed skipped: login already exists", zap.String("login", login))
		return nil
	}

	passwordHash, err := hasher.Hash(a.config.App.AdminPassword)
	if err != nil {
		return err
	}
	user := &models.User{
		ID:           uuid.New().String(),
		Login:        login,
		PasswordHash: passwordHash,
		Role:         models.RoleAdmin,
		DisplayName:  login,
	}
	if err := repo.Create(ctx, user); err != nil {
		return err
	}
	a.logger.Info("admin seeded", zap.String("login", login))
	return nil
}
