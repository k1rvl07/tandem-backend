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
	b := &routerBuilder{deps: deps}
	b.setup()
	return b.r
}

type routerBuilder struct {
	deps  Dependencies
	r     *gin.Engine
	api   *gin.RouterGroup
	read  *gin.RouterGroup
	write *gin.RouterGroup
}

func (b *routerBuilder) setup() {
	b.r = gin.New()
	b.r.Use(gin.Recovery())
	b.r.Use(mwlog.Logger(b.deps.Logger))
	b.r.Use(mwcors.CORS(b.deps.AllowedOrigin))

	b.r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	b.api = b.r.Group("/api/v1")
	b.registerAuth()
	b.registerWS()
	b.registerAuthed()

	if b.deps.EnableSwagger {
		openapi.Register(b.r, "/swagger")
	}
}

func (b *routerBuilder) registerAuth() {
	if b.deps.AuthHandler == nil {
		return
	}
	authGroup := b.api.Group("/auth")
	if b.deps.AuthLimiter != nil {
		authGroup.Use(b.deps.AuthLimiter.Middleware())
	}
	authGroup.POST("/login", b.deps.AuthHandler.Login)
	authGroup.POST("/refresh", b.deps.AuthHandler.Refresh)
}

func (b *routerBuilder) registerWS() {
	if b.deps.TokenService == nil || b.deps.Hub == nil || b.deps.AuthHandler == nil {
		return
	}
	wsHandler := hws.NewWSHandler(b.deps.Hub, b.deps.TokenService, b.deps.WorkspaceRepository, b.deps.AllowedOrigin)
	wsHandlers := []gin.HandlerFunc{mwws.WSIdentity(b.deps.TokenService)}
	if b.deps.WSLimiter != nil {
		wsHandlers = append(wsHandlers, b.deps.WSLimiter.Middleware())
	}
	wsHandlers = append(wsHandlers, wsHandler.Connect)
	b.r.GET("/ws", wsHandlers...)
}

func (b *routerBuilder) registerAuthed() {
	if b.deps.TokenService == nil {
		return
	}
	authMw := mwauth.Auth(b.deps.TokenService)
	readHandlers := []gin.HandlerFunc{authMw}
	if b.deps.ReadLimiter != nil {
		readHandlers = append(readHandlers, b.deps.ReadLimiter.Middleware())
	}
	writeHandlers := []gin.HandlerFunc{authMw}
	if b.deps.WriteLimiter != nil {
		writeHandlers = append(writeHandlers, b.deps.WriteLimiter.Middleware())
	}
	b.read = b.api.Group("", readHandlers...)
	b.write = b.api.Group("", writeHandlers...)
	b.registerProfile()
	b.registerAdmin()
	b.registerWorkspace()
	b.registerBoard()
	b.registerTask()
	b.registerAttachment()
	b.registerFavorite()
	b.registerTree()
	b.registerFiles()
}

func (b *routerBuilder) registerProfile() {
	if b.deps.AuthHandler != nil {
		b.write.POST("/auth/logout", b.deps.AuthHandler.Logout)
	}
	if b.deps.ProfileHandler == nil {
		return
	}
	b.read.GET("/me", b.deps.ProfileHandler.GetProfile)
	b.write.PATCH("/me", b.deps.ProfileHandler.UpdateProfile)
	b.write.POST("/me/avatar", b.deps.ProfileHandler.UploadAvatar)
	b.write.DELETE("/me/avatar", b.deps.ProfileHandler.DeleteAvatar)
	b.write.POST("/me/password", b.deps.ProfileHandler.ChangePassword)
}

func (b *routerBuilder) registerAdmin() {
	if b.deps.AdminHandler == nil || b.deps.UserRepository == nil {
		return
	}
	adminRead := b.read.Group("", mwstaff.RequireStaff(b.deps.UserRepository))
	adminWrite := b.write.Group("", mwstaff.RequireStaff(b.deps.UserRepository))
	adminWrite.POST("/admin/users", b.deps.AdminHandler.CreateUser)
	adminRead.GET("/admin/users", b.deps.AdminHandler.ListUsers)
	adminWrite.PATCH("/admin/users/:id/role", b.deps.AdminHandler.UpdateUserRole)
	adminWrite.DELETE("/admin/users/:id", b.deps.AdminHandler.DeleteUser)
}

func (b *routerBuilder) registerWorkspace() {
	if b.deps.WorkspaceHandler == nil {
		return
	}
	b.write.POST("/workspaces", b.deps.WorkspaceHandler.Create)
	b.read.GET("/workspaces", b.deps.WorkspaceHandler.List)
	b.read.GET("/workspaces/:id", b.deps.WorkspaceHandler.Get)
	b.write.PATCH("/workspaces/:id", b.deps.WorkspaceHandler.Update)
	b.write.PUT("/workspaces/:id/theme", b.deps.WorkspaceHandler.SetTheme)
	b.write.DELETE("/workspaces/:id", b.deps.WorkspaceHandler.Delete)
	b.write.POST("/workspaces/:id/members", b.deps.WorkspaceHandler.AddMember)
	b.write.PATCH("/workspaces/:id/members/:userId", b.deps.WorkspaceHandler.UpdateRole)
	b.write.DELETE("/workspaces/:id/members/:userId", b.deps.WorkspaceHandler.RemoveMember)
	b.write.POST("/workspaces/:id/owner", b.deps.WorkspaceHandler.TransferOwner)
	b.read.GET("/workspaces/:id/invite", b.deps.WorkspaceHandler.GetInvite)
	b.write.DELETE("/workspaces/:id/invite", b.deps.WorkspaceHandler.DisableInvite)
	b.write.POST("/invite/:token/join", b.deps.WorkspaceHandler.JoinByInvite)
}

func (b *routerBuilder) registerBoard() {
	if b.deps.BoardHandler == nil {
		return
	}
	b.read.GET("/workspaces/:id/boards", b.deps.BoardHandler.List)
	b.write.POST("/workspaces/:id/boards", b.deps.BoardHandler.Create)
	b.write.PUT("/workspaces/:id/boards/reorder", b.deps.BoardHandler.Reorder)
	b.read.GET("/workspaces/:id/boards/:boardId", b.deps.BoardHandler.Get)
	b.write.PATCH("/workspaces/:id/boards/:boardId", b.deps.BoardHandler.Update)
	b.write.PUT("/workspaces/:id/boards/:boardId/main", b.deps.BoardHandler.SetMain)
	b.write.DELETE("/workspaces/:id/boards/:boardId", b.deps.BoardHandler.Delete)
}

func (b *routerBuilder) registerTask() {
	if b.deps.TaskHandler == nil {
		return
	}
	b.read.GET("/workspaces/:id/tasks", b.deps.TaskHandler.List)
	b.read.GET("/workspaces/:id/tasks/:taskId", b.deps.TaskHandler.Get)
	b.write.POST("/workspaces/:id/boards/:boardId/tasks", b.deps.TaskHandler.Create)
	b.write.PATCH("/workspaces/:id/boards/:boardId/tasks/:taskId", b.deps.TaskHandler.Update)
	b.write.DELETE("/workspaces/:id/boards/:boardId/tasks/:taskId", b.deps.TaskHandler.Delete)
}

func (b *routerBuilder) registerAttachment() {
	if b.deps.AttachmentHandler == nil {
		return
	}
	attachmentCreate := b.write.Group("/workspaces/:id/tasks/:taskId/attachments")
	if b.deps.UploadLimiter != nil {
		attachmentCreate.Use(b.deps.UploadLimiter.Middleware())
	}
	attachmentCreate.POST("", b.deps.AttachmentHandler.Create)
	b.read.GET("/workspaces/:id/tasks/:taskId/attachments", b.deps.AttachmentHandler.List)
	b.read.GET("/workspaces/:id/tasks/:taskId/attachments/:attachmentId", b.deps.AttachmentHandler.Download)
	b.write.DELETE("/workspaces/:id/tasks/:taskId/attachments/:attachmentId", b.deps.AttachmentHandler.Delete)
}

func (b *routerBuilder) registerFavorite() {
	if b.deps.FavoriteHandler == nil {
		return
	}
	b.write.PUT("/favorites/workspaces/:workspaceId", b.deps.FavoriteHandler.AddWorkspace)
	b.write.DELETE("/favorites/workspaces/:workspaceId", b.deps.FavoriteHandler.RemoveWorkspace)
	b.write.PUT("/favorites/boards/:boardId", b.deps.FavoriteHandler.AddBoard)
	b.write.DELETE("/favorites/boards/:boardId", b.deps.FavoriteHandler.RemoveBoard)
}

func (b *routerBuilder) registerTree() {
	if b.deps.TreeHandler == nil {
		return
	}
	b.read.GET("/tasks/tree", b.deps.TreeHandler.List)
}

func (b *routerBuilder) registerFiles() {
	if b.deps.Files == nil {
		return
	}
	filesHandler := hfile.NewFileHandler(b.deps.Files)
	uploadImage := b.write.Group("/files/images")
	if b.deps.UploadLimiter != nil {
		uploadImage.Use(b.deps.UploadLimiter.Middleware())
	}
	uploadImage.POST("", filesHandler.UploadImage)
	uploadImage.DELETE("", filesHandler.DeleteImage)
	b.read.GET("/files/sign", filesHandler.Sign)
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
