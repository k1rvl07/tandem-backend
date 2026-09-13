package base

import (
	"time"

	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/service"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	file "github.com/tandem/tandem/internal/usecase/file"
	"go.uber.org/zap"
)

type Deps struct {
	Users       repository.UserRepository
	Workspaces  repository.WorkspaceRepository
	Boards      repository.BoardRepository
	Columns     repository.ColumnRepository
	Tasks       repository.TaskRepository
	Attachments repository.AttachmentRepository
	Favorites   repository.FavoriteRepository
	Files       *file.Service
	Hub         ws.Hub
	Cache       cache.Cache
	Tokens      service.TokenService
	Hasher      service.PasswordHasher
	Logger      *zap.Logger
	TokenTTL    time.Duration
	RefreshTTL  time.Duration
}
