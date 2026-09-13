package attachment

import (
	"context"
	"io"

	dattachment "github.com/tandem/tandem/internal/http/dto/attachment"
	attcore "github.com/tandem/tandem/internal/usecase/attachment/core"
	"github.com/tandem/tandem/internal/usecase/attachment/features/crud"
	"github.com/tandem/tandem/internal/usecase/shared/base"
)

type UseCase interface {
	Create(ctx context.Context, actorID, workspaceID, taskID, filename, contentType string, reader io.Reader, size int64) (*dattachment.AttachmentResponse, error)
	List(ctx context.Context, actorID, workspaceID, taskID string) ([]dattachment.AttachmentResponse, error)
	Download(ctx context.Context, actorID, workspaceID, taskID, attachmentID string) (*dattachment.AttachmentResponse, io.ReadCloser, error)
	Delete(ctx context.Context, actorID, workspaceID, taskID, attachmentID string) error
}

type Deps struct {
	base.Deps
}

type Service struct {
	*crud.Crud
}

func NewService(deps Deps) *Service {
	c := attcore.New(deps.Attachments, deps.Tasks, deps.Columns, deps.Boards, deps.Workspaces, deps.Files, deps.Hub, deps.Cache)
	return &Service{Crud: crud.New(c)}
}

var _ UseCase = (*Service)(nil)
