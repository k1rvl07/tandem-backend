package router

import (
	"context"
	"io"

	"github.com/gin-gonic/gin"
	"github.com/tandem/tandem/internal/domain/ports/filestore"
	"github.com/tandem/tandem/internal/domain/ports/service"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	"github.com/tandem/tandem/internal/http/handler"
	"github.com/tandem/tandem/internal/http/middleware"
	"github.com/tandem/tandem/internal/http/openapi"
	file "github.com/tandem/tandem/internal/usecase/file"
	"go.uber.org/zap"
)

type Dependencies struct {
	Logger        *zap.Logger
	AllowedOrigin []string
	FileStore     filestore.FileStore
	Hub           ws.Hub
	TokenService  service.TokenService
	AuthHandler   *handler.AuthHandler
	Files         *file.Service
	EnableSwagger bool
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
		api.POST("/auth/register", deps.AuthHandler.Register)
		api.POST("/auth/login", deps.AuthHandler.Login)
	}

	if deps.TokenService != nil && deps.Hub != nil && deps.AuthHandler != nil {
		wsHandler := handler.NewWSHandler(deps.Hub, deps.TokenService)
		r.GET("/ws", wsHandler.Connect)
	}

	if deps.Files != nil && deps.TokenService != nil {
		filesHandler := handler.NewFileHandler(deps.Files)
		protected := api.Group("", middleware.Auth(deps.TokenService))
		protected.POST("/files/images", filesHandler.UploadImage)
		api.GET("/files/*key", filesHandler.GetImage)
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
