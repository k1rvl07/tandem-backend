package app

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/tandem/tandem/docs"
	muser "github.com/tandem/tandem/internal/domain/models/user"
	repoport "github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/service"
	hadmin "github.com/tandem/tandem/internal/http/handler/admin"
	hattachment "github.com/tandem/tandem/internal/http/handler/attachment"
	hauth "github.com/tandem/tandem/internal/http/handler/auth"
	hboard "github.com/tandem/tandem/internal/http/handler/board"
	hfavorite "github.com/tandem/tandem/internal/http/handler/favorite"
	hprofile "github.com/tandem/tandem/internal/http/handler/profile"
	htask "github.com/tandem/tandem/internal/http/handler/task"
	htree "github.com/tandem/tandem/internal/http/handler/tree"
	hworkspace "github.com/tandem/tandem/internal/http/handler/workspace"
	"github.com/tandem/tandem/internal/http/router"
	"github.com/tandem/tandem/internal/infrastructure/ratelimit"
	"gorm.io/gorm"

	miniofs "github.com/tandem/tandem/internal/infrastructure/minio"
	"github.com/tandem/tandem/internal/infrastructure/password"
	rediscache "github.com/tandem/tandem/internal/infrastructure/redis"
	"github.com/tandem/tandem/internal/infrastructure/token"
	wshub "github.com/tandem/tandem/internal/infrastructure/ws"
	"github.com/tandem/tandem/internal/pkg/config"
	fkprep "github.com/tandem/tandem/internal/pkg/fkprep"
	"github.com/tandem/tandem/internal/pkg/validate"
	rboard "github.com/tandem/tandem/internal/repository/board"
	rcolumn "github.com/tandem/tandem/internal/repository/column"
	eboard "github.com/tandem/tandem/internal/repository/entity/board"
	efavorite "github.com/tandem/tandem/internal/repository/entity/favorite"
	etask "github.com/tandem/tandem/internal/repository/entity/task"
	euser "github.com/tandem/tandem/internal/repository/entity/user"
	eworkspace "github.com/tandem/tandem/internal/repository/entity/workspace"
	postgres "github.com/tandem/tandem/internal/repository/postgres"
	rtask "github.com/tandem/tandem/internal/repository/task"
	rtaskext "github.com/tandem/tandem/internal/repository/taskext"
	ruser "github.com/tandem/tandem/internal/repository/user"
	rworkspace "github.com/tandem/tandem/internal/repository/workspace"
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

	postgres  *postgres.Postgres
	redis     *rediscache.Redis
	fileStore *miniofs.MinIO
	hub       *wshub.Hub

	server *http.Server
}

func New(cfg *config.Config, logger *zap.Logger) (*App, error) {
	postgres, err := postgres.NewPostgres(cfg.Database, postgres.WithConnectTimeout(10*time.Second))
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

	if err := fkprep.Prepare(postgres.DB); err != nil {
		_ = redis.Close()
		_ = postgres.Close()
		return nil, err
	}

	if err := postgres.AutoMigrate(&euser.User{}, &eworkspace.Workspace{}, &eworkspace.WorkspaceMember{}, &eboard.Board{}, &eboard.Column{}, &eboard.Task{}, &etask.TaskAttachment{}, &efavorite.Favorite{}); err != nil {
		_ = redis.Close()
		_ = postgres.Close()
		return nil, err
	}
	if err := postgres.DB.Exec("DROP TABLE IF EXISTS task_comments").Error; err != nil {
		_ = redis.Close()
		_ = postgres.Close()
		return nil, err
	}
	if err := postgres.DB.Exec("UPDATE workspace_members SET role = 'member' WHERE role = 'viewer'").Error; err != nil {
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

func (a *App) BuildRouter(ctx context.Context) (*gin.Engine, error) {
	userRepo := ruser.NewUserRepo(a.postgres.DB)
	tokenManager := token.NewJWTManager(a.config.JWT.Secret, a.redis)
	hasher := password.NewBCryptHasher(a.config.App.PasswordCost)
	authService := auth.NewService(userRepo, tokenManager, hasher, a.config.JWT.TokenTTL, a.config.JWT.RefreshTTL)
	authHandler := hauth.NewAuthHandler(authService)
	fileService := file.NewService(a.fileStore, a.logger)
	profileService := profile.NewService(userRepo, hasher, fileService, a.redis, tokenManager, a.config.JWT.TokenTTL, a.config.JWT.RefreshTTL, a.logger)
	profileHandler := hprofile.NewProfileHandler(profileService)
	workspaceRepo := rworkspace.NewWorkspaceRepo(a.postgres.DB)
	favoriteRepo := rtaskext.NewFavoriteRepo(a.postgres.DB)
	adminService := admin.NewService(userRepo, hasher, a.redis, tokenManager, workspaceRepo, favoriteRepo, fileService)
	adminHandler := hadmin.NewAdminHandler(adminService)
	boardRepo := rboard.NewBoardRepo(a.postgres.DB)
	columnRepo := rcolumn.NewColumnRepo(a.postgres.DB)
	taskRepo := rtask.NewTaskRepo(a.postgres.DB)
	attachmentRepo := rtaskext.NewAttachmentRepo(a.postgres.DB)
	workspaceService := workspace.NewService(workspaceRepo, userRepo, favoriteRepo, boardRepo, columnRepo, taskRepo, fileService, a.hub, a.redis, a.logger)
	workspaceHandler := hworkspace.NewWorkspaceHandler(workspaceService)
	boardService := board.NewService(boardRepo, columnRepo, taskRepo, workspaceRepo, userRepo, favoriteRepo, fileService, a.hub, a.redis)
	boardHandler := hboard.NewBoardHandler(boardService)
	taskService := task.NewService(taskRepo, columnRepo, boardRepo, workspaceRepo, userRepo, fileService, a.hub, a.redis)
	taskHandler := htask.NewTaskHandler(taskService)
	attachmentService := attachment.NewService(attachmentRepo, taskRepo, columnRepo, boardRepo, workspaceRepo, fileService, a.hub, a.redis)
	attachmentHandler := hattachment.NewAttachmentHandler(attachmentService)
	favoriteService := favorite.NewService(favoriteRepo, workspaceRepo, boardRepo, a.hub, a.redis)
	favoriteHandler := hfavorite.NewFavoriteHandler(favoriteService)
	treeService := tree.NewService(workspaceRepo, boardRepo, columnRepo, taskRepo, userRepo, favoriteRepo, a.redis)
	treeHandler := htree.NewTreeHandler(treeService)

	var authLimiter, readLimiter, writeLimiter, uploadLimiter, wsLimiter router.RateLimiter
	if a.config.App.Env != "testing" {
		authLimiter = ratelimit.New(a.redis.Raw(), "auth", ratelimit.AuthLimit, ratelimit.AuthWindow, ratelimit.ByIP())
		readLimiter = ratelimit.New(a.redis.Raw(), "read", ratelimit.ReadLimit, ratelimit.ReadWindow, ratelimit.ByUser())
		writeLimiter = ratelimit.New(a.redis.Raw(), "write", ratelimit.WriteLimit, ratelimit.WriteWindow, ratelimit.ByUser())
		uploadLimiter = ratelimit.New(a.redis.Raw(), "upload", ratelimit.UploadLimit, ratelimit.UploadWindow, ratelimit.ByUser())
		wsLimiter = ratelimit.New(a.redis.Raw(), "ws", ratelimit.WsLimit, ratelimit.WsWindow, ratelimit.ByUser())
	}

	if err := a.seedAdmin(ctx, userRepo, hasher); err != nil {
		return nil, err
	}

	return router.New(router.Dependencies{
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
		AuthLimiter:         authLimiter,
		ReadLimiter:         readLimiter,
		WriteLimiter:        writeLimiter,
		UploadLimiter:       uploadLimiter,
		WSLimiter:           wsLimiter,
	}), nil
}

func (a *App) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	r, err := a.BuildRouter(ctx)
	if err != nil {
		return err
	}

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

	a.Close()
	a.logger.Info("server stopped gracefully")
	return nil
}

func (a *App) Postgres() *gorm.DB {
	return a.postgres.DB
}

func (a *App) Close() {
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
	user := &muser.User{
		ID:           uuid.New().String(),
		Login:        login,
		PasswordHash: passwordHash,
		Role:         muser.RoleAdmin,
		DisplayName:  login,
	}
	if err := repo.Create(ctx, user); err != nil {
		return err
	}
	a.logger.Info("admin seeded", zap.String("login", login))
	return nil
}
