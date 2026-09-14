package file

import (
	"github.com/tandem/tandem/internal/domain/ports/filestore"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/usecase/file/core"
	"github.com/tandem/tandem/internal/usecase/file/features/attachments"
	"github.com/tandem/tandem/internal/usecase/file/features/images"
	"go.uber.org/zap"
)

type Service struct {
	*images.Images
	*attachments.Attachments
}

func NewService(store filestore.FileStore, logger *zap.Logger) *Service {
	c := core.New(store, logger)
	return &Service{
		Images:      images.New(c),
		Attachments: attachments.New(c),
	}
}

func (s *Service) WithAccessCheck(tasks repository.TaskRepository, workspaces repository.WorkspaceRepository) *Service {
	s.Images.WithAccessCheck(tasks, workspaces)
	return s
}
