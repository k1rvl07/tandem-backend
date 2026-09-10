package router

import (
	"context"
	"io"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tandem/tandem/internal/domain/ports/filestore"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/service"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	hadmin "github.com/tandem/tandem/internal/http/handler/admin"
	hattachment "github.com/tandem/tandem/internal/http/handler/attachment"
	hauth "github.com/tandem/tandem/internal/http/handler/auth"
	hboard "github.com/tandem/tandem/internal/http/handler/board"
	hfavorite "github.com/tandem/tandem/internal/http/handler/favorite"
	hfile "github.com/tandem/tandem/internal/http/handler/file"
	hprofile "github.com/tandem/tandem/internal/http/handler/profile"
	htask "github.com/tandem/tandem/internal/http/handler/task"
	htree "github.com/tandem/tandem/internal/http/handler/tree"
	hworkspace "github.com/tandem/tandem/internal/http/handler/workspace"
	hws "github.com/tandem/tandem/internal/http/handler/ws"
	mwauth "github.com/tandem/tandem/internal/http/middleware/auth"
	mwcors "github.com/tandem/tandem/internal/http/middleware/cors"
	mwlog "github.com/tandem/tandem/internal/http/middleware/logger"
	mwstaff "github.com/tandem/tandem/internal/http/middleware/require_staff"
	mwws "github.com/tandem/tandem/internal/http/middleware/ws_identity"
	"github.com/tandem/tandem/internal/http/openapi"
	file "github.com/tandem/tandem/internal/usecase/file"
	"go.uber.org/zap"
)

type RateLimiter interface {
	Middleware() gin.HandlerFunc
}

type Dependencies struct {
	Logger              *zap.Logger
	AllowedOrigin       []string
	FileStore           filestore.FileStore
	Hub                 ws.Hub
	TokenService        service.TokenService
	UserRepository      repository.UserRepository
	WorkspaceRepository repository.WorkspaceRepository
	AuthHandler         *hauth.AuthHandler
	ProfileHandler      *hprofile.ProfileHandler
	AdminHandler        *hadmin.AdminHandler
	WorkspaceHandler    *hworkspace.WorkspaceHandler
	BoardHandler        *hboard.BoardHandler
	TaskHandler         *htask.TaskHandler
	AttachmentHandler   *hattachment.AttachmentHandler
	FavoriteHandler     *hfavorite.FavoriteHandler
	TreeHandler         *htree.TreeHandler
	Files               *file.Service
	EnableSwagger       bool

	AuthLimiter   RateLimiter
	ReadLimiter   RateLimiter
	WriteLimiter  RateLimiter
	UploadLimiter RateLimiter
	WSLimiter     RateLimiter
}

func New(deps Dependencies) *gin.Engine {
	if deps.FileStore == nil {
		deps.FileStore = nopFileStore{}
	}
	if deps.Hub == nil {
		deps.Hub = nopHub{}
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(mwlog.Logger(deps.Logger))
	r.Use(mwcors.CORS(deps.AllowedOrigin))

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api := r.Group("/api/v1")
	if deps.AuthHandler != nil {
		authGroup := api.Group("/auth")
		if deps.AuthLimiter != nil {
			authGroup.Use(deps.AuthLimiter.Middleware())
		}
		authGroup.POST("/login", deps.AuthHandler.Login)
	}

	if deps.TokenService != nil && deps.Hub != nil && deps.AuthHandler != nil {
		wsHandler := hws.NewWSHandler(deps.Hub, deps.TokenService, deps.WorkspaceRepository, deps.AllowedOrigin)
		wsHandlers := []gin.HandlerFunc{mwws.WSIdentity(deps.TokenService)}
		if deps.WSLimiter != nil {
			wsHandlers = append(wsHandlers, deps.WSLimiter.Middleware())
		}
		wsHandlers = append(wsHandlers, wsHandler.Connect)
		r.GET("/ws", wsHandlers...)
	}

	if deps.TokenService != nil {
		authMw := mwauth.Auth(deps.TokenService)
		readHandlers := []gin.HandlerFunc{authMw}
		if deps.ReadLimiter != nil {
			readHandlers = append(readHandlers, deps.ReadLimiter.Middleware())
		}
		writeHandlers := []gin.HandlerFunc{authMw}
		if deps.WriteLimiter != nil {
			writeHandlers = append(writeHandlers, deps.WriteLimiter.Middleware())
		}
		read := api.Group("", readHandlers...)
		write := api.Group("", writeHandlers...)
		if deps.AuthHandler != nil {
			write.POST("/auth/logout", deps.AuthHandler.Logout)
		}
		if deps.ProfileHandler != nil {
			read.GET("/me", deps.ProfileHandler.GetProfile)
			write.PATCH("/me", deps.ProfileHandler.UpdateProfile)
			write.POST("/me/avatar", deps.ProfileHandler.UploadAvatar)
			write.DELETE("/me/avatar", deps.ProfileHandler.DeleteAvatar)
			write.POST("/me/password", deps.ProfileHandler.ChangePassword)
		}
		if deps.AdminHandler != nil && deps.UserRepository != nil {
			adminRead := read.Group("", mwstaff.RequireStaff(deps.UserRepository))
			adminWrite := write.Group("", mwstaff.RequireStaff(deps.UserRepository))
			adminWrite.POST("/admin/users", deps.AdminHandler.CreateUser)
			adminRead.GET("/admin/users", deps.AdminHandler.ListUsers)
			adminWrite.PATCH("/admin/users/:id/role", deps.AdminHandler.UpdateUserRole)
			adminWrite.DELETE("/admin/users/:id", deps.AdminHandler.DeleteUser)
		}
		if deps.WorkspaceHandler != nil {
			write.POST("/workspaces", deps.WorkspaceHandler.Create)
			read.GET("/workspaces", deps.WorkspaceHandler.List)
			read.GET("/workspaces/:id", deps.WorkspaceHandler.Get)
			write.PATCH("/workspaces/:id", deps.WorkspaceHandler.Update)
			write.PUT("/workspaces/:id/theme", deps.WorkspaceHandler.SetTheme)
			write.DELETE("/workspaces/:id", deps.WorkspaceHandler.Delete)
			write.POST("/workspaces/:id/members", deps.WorkspaceHandler.AddMember)
			write.PATCH("/workspaces/:id/members/:userId", deps.WorkspaceHandler.UpdateRole)
			write.DELETE("/workspaces/:id/members/:userId", deps.WorkspaceHandler.RemoveMember)
			write.POST("/workspaces/:id/owner", deps.WorkspaceHandler.TransferOwner)
			read.GET("/workspaces/:id/invite", deps.WorkspaceHandler.GetInvite)
			write.DELETE("/workspaces/:id/invite", deps.WorkspaceHandler.DisableInvite)
			write.POST("/invite/:token/join", deps.WorkspaceHandler.JoinByInvite)
		}
		if deps.BoardHandler != nil {
			read.GET("/workspaces/:id/boards", deps.BoardHandler.List)
			write.POST("/workspaces/:id/boards", deps.BoardHandler.Create)
			write.PUT("/workspaces/:id/boards/reorder", deps.BoardHandler.Reorder)
			read.GET("/workspaces/:id/boards/:boardId", deps.BoardHandler.Get)
			write.PATCH("/workspaces/:id/boards/:boardId", deps.BoardHandler.Update)
			write.PUT("/workspaces/:id/boards/:boardId/main", deps.BoardHandler.SetMain)
			write.DELETE("/workspaces/:id/boards/:boardId", deps.BoardHandler.Delete)
		}
		if deps.TaskHandler != nil {
			read.GET("/workspaces/:id/tasks", deps.TaskHandler.List)
			read.GET("/workspaces/:id/tasks/:taskId", deps.TaskHandler.Get)
			write.POST("/workspaces/:id/boards/:boardId/tasks", deps.TaskHandler.Create)
			write.PATCH("/workspaces/:id/boards/:boardId/tasks/:taskId", deps.TaskHandler.Update)
			write.DELETE("/workspaces/:id/boards/:boardId/tasks/:taskId", deps.TaskHandler.Delete)
		}
		if deps.AttachmentHandler != nil {
			attachmentCreate := write.Group("/workspaces/:id/tasks/:taskId/attachments")
			if deps.UploadLimiter != nil {
				attachmentCreate.Use(deps.UploadLimiter.Middleware())
			}
			attachmentCreate.POST("", deps.AttachmentHandler.Create)
			read.GET("/workspaces/:id/tasks/:taskId/attachments", deps.AttachmentHandler.List)
			read.GET("/workspaces/:id/tasks/:taskId/attachments/:attachmentId", deps.AttachmentHandler.Download)
			write.DELETE("/workspaces/:id/tasks/:taskId/attachments/:attachmentId", deps.AttachmentHandler.Delete)
		}
		if deps.FavoriteHandler != nil {
			write.PUT("/favorites/workspaces/:workspaceId", deps.FavoriteHandler.AddWorkspace)
			write.DELETE("/favorites/workspaces/:workspaceId", deps.FavoriteHandler.RemoveWorkspace)
			write.PUT("/favorites/boards/:boardId", deps.FavoriteHandler.AddBoard)
			write.DELETE("/favorites/boards/:boardId", deps.FavoriteHandler.RemoveBoard)
		}
		if deps.TreeHandler != nil {
			read.GET("/tasks/tree", deps.TreeHandler.List)
		}
		if deps.Files != nil {
			filesHandler := hfile.NewFileHandler(deps.Files)
			uploadImage := write.Group("/files/images")
			if deps.UploadLimiter != nil {
				uploadImage.Use(deps.UploadLimiter.Middleware())
			}
			uploadImage.POST("", filesHandler.UploadImage)
			uploadImage.DELETE("", filesHandler.DeleteImage)
			read.GET("/files/sign", filesHandler.Sign)
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

func (nopFileStore) PresignGet(context.Context, string, time.Duration) (string, error) {
	return "", nil
}

type nopHub struct{}

func (nopHub) Register(ws.Client)          {}
func (nopHub) Unregister(ws.Client)        {}
func (nopHub) JoinRoom(string, ws.Client)  {}
func (nopHub) LeaveRoom(string, ws.Client) {}
func (nopHub) RoomMembers(string) []string { return nil }
func (nopHub) BroadcastToRoom(string, *ws.Message) {
}
func (nopHub) Broadcast(*ws.Message)          {}
func (nopHub) SendToUser(string, *ws.Message) {}
func (nopHub) Close()                         {}

var _ filestore.FileStore = nopFileStore{}
var _ ws.Hub = nopHub{}
