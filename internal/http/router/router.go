package router

import (
	"context"
	"io"

	"github.com/gin-gonic/gin"
	"github.com/tandem/tandem/internal/domain/ports/filestore"
	"github.com/tandem/tandem/internal/domain/ports/service"
	"github.com/tandem/tandem/internal/http/handler"
	"github.com/tandem/tandem/internal/http/middleware"
	"github.com/tandem/tandem/internal/http/openapi"
	"go.uber.org/zap"
)

type Dependencies struct {
	Logger        *zap.Logger
	AllowedOrigin []string
	FileStore     filestore.FileStore
	TokenService  service.TokenService
	AuthHandler   *handler.AuthHandler
	EnableSwagger bool
}

func New(deps Dependencies) *gin.Engine {
	if deps.AllowedOrigin == nil {
		deps.AllowedOrigin = []string{"*"}
	}

	if deps.FileStore == nil {
		deps.FileStore = nopFileStore{}
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

var _ filestore.FileStore = nopFileStore{}
