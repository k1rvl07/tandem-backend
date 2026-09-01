package router

import (
	"context"
	"io"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/tandem/tandem/internal/delivery/middleware"
	"github.com/tandem/tandem/internal/interfaces/filestore"
	"github.com/tandem/tandem/internal/interfaces/ws"
	"go.uber.org/zap"
)

type Dependencies struct {
	Logger        *zap.Logger
	AllowedOrigin []string
	FileStore     filestore.FileStore
	Hub           ws.Hub
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

	if deps.EnableSwagger {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
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
func (nopHub) BroadcastToRoom(string, *ws.Message) {
}
func (nopHub) Broadcast(*ws.Message) {}
func (nopHub) Close()                {}
