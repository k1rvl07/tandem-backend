package core

import (
	"context"

	dattachment "github.com/tandem/tandem/internal/contracts/attachment"
	mattachment "github.com/tandem/tandem/internal/domain/models/attachment"
	mtask "github.com/tandem/tandem/internal/domain/models/task"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	"github.com/tandem/tandem/internal/usecase/attachment/mapping"
	file "github.com/tandem/tandem/internal/usecase/file"
	"github.com/tandem/tandem/internal/usecase/shared/access"
	sharedbase "github.com/tandem/tandem/internal/usecase/shared/base"
	"go.uber.org/zap"
)

const (
	EventAttachmentCreated = "attachment.created"
	EventAttachmentDeleted = "attachment.deleted"
)

type Core struct {
	sharedbase.Base
	Attachments repository.AttachmentRepository
	Tasks       repository.TaskRepository
	Columns     repository.ColumnRepository
	Boards      repository.BoardRepository
	Files       *file.Service
	Hub         ws.Hub
	Logger      *zap.Logger
}

func New(d sharedbase.Deps) *Core {
	return &Core{
		Base:        sharedbase.Base{Workspaces: d.Workspaces, Cache: d.Cache},
		Attachments: d.Attachments,
		Tasks:       d.Tasks,
		Columns:     d.Columns,
		Boards:      d.Boards,
		Files:       d.Files,
		Hub:         d.Hub,
		Logger:      d.Logger,
	}
}

func (c *Core) WorkspaceTask(ctx context.Context, workspaceID, taskID string) (*mtask.Task, error) {
	return access.TaskInWorkspace(ctx, c.Tasks, c.Columns, c.Boards, workspaceID, taskID)
}

func (c *Core) AttachmentResponse(attachment *mattachment.TaskAttachment, workspaceID, taskID string) *dattachment.AttachmentResponse {
	return mapping.AttachmentResponse(attachment, workspaceID, taskID)
}
