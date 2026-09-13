package core

import (
	"context"

	mattachment "github.com/tandem/tandem/internal/domain/models/attachment"
	mtask "github.com/tandem/tandem/internal/domain/models/task"
	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	dattachment "github.com/tandem/tandem/internal/http/dto/attachment"
	"github.com/tandem/tandem/internal/usecase/attachment/mapping"
	file "github.com/tandem/tandem/internal/usecase/file"
	"github.com/tandem/tandem/internal/usecase/shared/access"
	sharedbase "github.com/tandem/tandem/internal/usecase/shared/base"
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
}

func New(attachments repository.AttachmentRepository, tasks repository.TaskRepository, columns repository.ColumnRepository, boards repository.BoardRepository, workspaces repository.WorkspaceRepository, files *file.Service, hub ws.Hub, cache cache.Cache) *Core {
	return &Core{Base: sharedbase.Base{Workspaces: workspaces, Cache: cache}, Attachments: attachments, Tasks: tasks, Columns: columns, Boards: boards, Files: files, Hub: hub}
}

func (c *Core) WorkspaceTask(ctx context.Context, workspaceID, taskID string) (*mtask.Task, error) {
	return access.TaskInWorkspace(ctx, c.Tasks, c.Columns, c.Boards, workspaceID, taskID)
}

func (c *Core) AttachmentResponse(attachment *mattachment.TaskAttachment, workspaceID, taskID string) *dattachment.AttachmentResponse {
	return mapping.AttachmentResponse(attachment, workspaceID, taskID)
}
