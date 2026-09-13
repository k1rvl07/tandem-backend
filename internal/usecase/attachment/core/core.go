package core

import (
	"context"

	mattachment "github.com/tandem/tandem/internal/domain/models/attachment"
	mtask "github.com/tandem/tandem/internal/domain/models/task"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	dattachment "github.com/tandem/tandem/internal/http/dto/attachment"
	file "github.com/tandem/tandem/internal/usecase/file"
	"github.com/tandem/tandem/internal/usecase/shared/access"
	cacheutil "github.com/tandem/tandem/internal/usecase/shared/cache"
)

const (
	EventAttachmentCreated = "attachment.created"
	EventAttachmentDeleted = "attachment.deleted"
)

type Core struct {
	Attachments repository.AttachmentRepository
	Tasks       repository.TaskRepository
	Columns     repository.ColumnRepository
	Boards      repository.BoardRepository
	Workspaces  repository.WorkspaceRepository
	Files       *file.Service
	Hub         ws.Hub
	Cache       cache.Cache
}

func New(attachments repository.AttachmentRepository, tasks repository.TaskRepository, columns repository.ColumnRepository, boards repository.BoardRepository, workspaces repository.WorkspaceRepository, files *file.Service, hub ws.Hub, cache cache.Cache) *Core {
	return &Core{Attachments: attachments, Tasks: tasks, Columns: columns, Boards: boards, Workspaces: workspaces, Files: files, Hub: hub, Cache: cache}
}

func (c *Core) BumpWorkspace(ctx context.Context, workspaceID string) {
	cacheutil.Bump(ctx, c.Cache, cacheutil.WSVerKey+workspaceID)
}

func (c *Core) MemberOf(ctx context.Context, actorID, workspaceID string) (*mworkspace.WorkspaceMember, error) {
	return access.MemberOrForbidden(ctx, c.Workspaces, workspaceID, actorID)
}

func (c *Core) WorkspaceTask(ctx context.Context, workspaceID, taskID string) (*mtask.Task, error) {
	return access.TaskInWorkspace(ctx, c.Tasks, c.Columns, c.Boards, workspaceID, taskID)
}

func (c *Core) Room(workspaceID string) string {
	return "workspace:" + workspaceID
}

func (c *Core) AttachmentResponse(attachment *mattachment.TaskAttachment, workspaceID, taskID string) *dattachment.AttachmentResponse {
	return &dattachment.AttachmentResponse{
		ID:          attachment.ID,
		TaskID:      attachment.TaskID,
		Filename:    attachment.Filename,
		ContentType: attachment.ContentType,
		Size:        attachment.Size,
		UploadedBy:  attachment.UploadedBy,
		CreatedAt:   attachment.CreatedAt,
		URL:         "/api/v1/workspaces/" + workspaceID + "/tasks/" + taskID + "/attachments/" + attachment.ID,
	}
}
