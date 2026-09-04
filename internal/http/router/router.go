package router

import (
	"context"
	"io"

	"github.com/gin-gonic/gin"
	"github.com/tandem/tandem/internal/domain/ports/filestore"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/service"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	"github.com/tandem/tandem/internal/http/handler"
	"github.com/tandem/tandem/internal/http/middleware"
	"github.com/tandem/tandem/internal/http/openapi"
	file "github.com/tandem/tandem/internal/usecase/file"
	"go.uber.org/zap"
)

type Dependencies struct {
	Logger              *zap.Logger
	AllowedOrigin       []string
	FileStore           filestore.FileStore
	Hub                 ws.Hub
	TokenService        service.TokenService
	UserRepository      repository.UserRepository
	WorkspaceRepository repository.WorkspaceRepository
	AuthHandler         *handler.AuthHandler
	ProfileHandler      *handler.ProfileHandler
	AdminHandler        *handler.AdminHandler
	WorkspaceHandler    *handler.WorkspaceHandler
	BoardHandler        *handler.BoardHandler
	ColumnHandler       *handler.ColumnHandler
	TaskHandler         *handler.TaskHandler
	Files               *file.Service
	EnableSwagger       bool
}

func New(deps Dependencies) *gin.Engine {
	if deps.AllowedOrigin == nil {
		deps.AllowedOrigin = []string{"*"}
	}

	if deps.FileStore == nil {
		deps.FileStore = nopFileStore{}
	}
	if deps.Hub == nil {
		deps.Hub = nopHub{}
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.Logger(deps.Logger))
	r.Use(middleware.CORS(deps.AllowedOrigin))

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1")
	if deps.AuthHandler != nil {
		api.POST("/auth/login", deps.AuthHandler.Login)
	}

	if deps.TokenService != nil && deps.Hub != nil && deps.AuthHandler != nil {
		wsHandler := handler.NewWSHandler(deps.Hub, deps.TokenService, deps.WorkspaceRepository)
		r.GET("/ws", wsHandler.Connect)
	}

	if deps.TokenService != nil {
		protected := api.Group("", middleware.Auth(deps.TokenService))
		if deps.ProfileHandler != nil {
			protected.GET("/me", deps.ProfileHandler.GetProfile)
			protected.PATCH("/me", deps.ProfileHandler.UpdateProfile)
			protected.POST("/me/avatar", deps.ProfileHandler.UploadAvatar)
			protected.POST("/me/password", deps.ProfileHandler.ChangePassword)
		}
		if deps.AdminHandler != nil && deps.UserRepository != nil {
			admin := protected.Group("", middleware.RequireStaff(deps.UserRepository))
			admin.POST("/admin/users", deps.AdminHandler.CreateUser)
			admin.GET("/admin/users", deps.AdminHandler.ListUsers)
			admin.DELETE("/admin/users/:id", deps.AdminHandler.DeleteUser)
		}
		if deps.WorkspaceHandler != nil {
			protected.POST("/workspaces", deps.WorkspaceHandler.Create)
			protected.GET("/workspaces", deps.WorkspaceHandler.List)
			protected.GET("/workspaces/:id", deps.WorkspaceHandler.Get)
			protected.PATCH("/workspaces/:id", deps.WorkspaceHandler.Update)
			protected.DELETE("/workspaces/:id", deps.WorkspaceHandler.Delete)
			protected.POST("/workspaces/:id/members", deps.WorkspaceHandler.AddMember)
			protected.DELETE("/workspaces/:id/members/:userId", deps.WorkspaceHandler.RemoveMember)
			protected.POST("/workspaces/:id/owner", deps.WorkspaceHandler.TransferOwner)
		}
		if deps.BoardHandler != nil {
			protected.GET("/workspaces/:id/boards", deps.BoardHandler.List)
			protected.POST("/workspaces/:id/boards", deps.BoardHandler.Create)
			protected.GET("/workspaces/:id/boards/:boardId", deps.BoardHandler.Get)
			protected.PATCH("/workspaces/:id/boards/:boardId", deps.BoardHandler.Update)
			protected.DELETE("/workspaces/:id/boards/:boardId", deps.BoardHandler.Delete)
		}
		if deps.ColumnHandler != nil {
			protected.POST("/workspaces/:id/boards/:boardId/columns", deps.ColumnHandler.Create)
			protected.PATCH("/workspaces/:id/boards/:boardId/columns/:columnId", deps.ColumnHandler.Update)
			protected.DELETE("/workspaces/:id/boards/:boardId/columns/:columnId", deps.ColumnHandler.Delete)
		}
		if deps.TaskHandler != nil {
			protected.POST("/workspaces/:id/boards/:boardId/tasks", deps.TaskHandler.Create)
			protected.PATCH("/workspaces/:id/boards/:boardId/tasks/:taskId", deps.TaskHandler.Update)
			protected.DELETE("/workspaces/:id/boards/:boardId/tasks/:taskId", deps.TaskHandler.Delete)
		}
		if deps.Files != nil {
			filesHandler := handler.NewFileHandler(deps.Files)
			protected.POST("/files/images", filesHandler.UploadImage)
			api.GET("/files/*key", filesHandler.GetImage)
		}
	}

	if deps.EnableSwagger {
		openapi.Register(r, "/swagger")
	}

	return r
}

type nopFileStore struct{}

func (nopFileStore) Put(context.Context, string, io.Reader, int64, string) error {
	return nil
}

func (nopFileStore) Get(context.Context, string) (io.ReadCloser, error) {
	return nil, nil
}

func (nopFileStore) Delete(context.Context, string) error {
	return nil
}

func (nopFileStore) Exists(context.Context, string) (bool, error) {
	return false, nil
}

type nopHub struct{}

func (nopHub) Register(ws.Client)          {}
func (nopHub) Unregister(ws.Client)        {}
func (nopHub) JoinRoom(string, ws.Client)  {}
func (nopHub) LeaveRoom(string, ws.Client) {}
func (nopHub) RoomMembers(string) []string { return nil }
func (nopHub) BroadcastToRoom(string, *ws.Message) {
}
func (nopHub) Broadcast(*ws.Message) {}
func (nopHub) Close()                {}

var _ filestore.FileStore = nopFileStore{}
var _ ws.Hub = nopHub{}
