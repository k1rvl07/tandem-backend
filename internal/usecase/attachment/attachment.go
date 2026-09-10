package attachment

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"
	mattachment "github.com/tandem/tandem/internal/domain/models/attachment"
	mtask "github.com/tandem/tandem/internal/domain/models/task"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	dattachment "github.com/tandem/tandem/internal/http/dto/attachment"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
	"github.com/tandem/tandem/internal/usecase/cacheutil"
	file "github.com/tandem/tandem/internal/usecase/file"
)

const (
	eventAttachmentCreated = "attachment.created"
	eventAttachmentDeleted = "attachment.deleted"
)

type UseCase interface {
	Create(ctx context.Context, actorID, workspaceID, taskID, filename, contentType string, reader io.Reader, size int64) (*dattachment.AttachmentResponse, error)
	List(ctx context.Context, actorID, workspaceID, taskID string) ([]dattachment.AttachmentResponse, error)
	Download(ctx context.Context, actorID, workspaceID, taskID, attachmentID string) (*dattachment.AttachmentResponse, io.ReadCloser, error)
	Delete(ctx context.Context, actorID, workspaceID, taskID, attachmentID string) error
}

type Service struct {
	attachments repository.AttachmentRepository
	tasks       repository.TaskRepository
	columns     repository.ColumnRepository
	boards      repository.BoardRepository
	workspaces  repository.WorkspaceRepository
	files       *file.Service
	hub         ws.Hub
	cache       cache.Cache
}

func NewService(
	attachments repository.AttachmentRepository,
	tasks repository.TaskRepository,
	columns repository.ColumnRepository,
	boards repository.BoardRepository,
	workspaces repository.WorkspaceRepository,
	files *file.Service,
	hub ws.Hub,
	cache cache.Cache,
) *Service {
	return &Service{
		attachments: attachments,
		tasks:       tasks,
		columns:     columns,
		boards:      boards,
		workspaces:  workspaces,
		files:       files,
		hub:         hub,
		cache:       cache,
	}
}

func (s *Service) bumpWorkspace(ctx context.Context, workspaceID string) {
	cacheutil.Bump(ctx, s.cache, cacheutil.WSVerKey+workspaceID)
}

func (s *Service) Create(ctx context.Context, actorID, workspaceID, taskID, filename, contentType string, reader io.Reader, size int64) (*dattachment.AttachmentResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	if err := validate.UUID(taskID); err != nil {
		return nil, err
	}
	if _, err := s.memberOf(ctx, actorID, workspaceID); err != nil {
		return nil, err
	}
	if _, err := s.workspaceTask(ctx, workspaceID, taskID); err != nil {
		return nil, err
	}
	key, err := s.files.PrepareAttachment(workspaceID, actorID, filename, size)
	if err != nil {
		return nil, err
	}
	attachment := &mattachment.TaskAttachment{
		ID:          uuid.New().String(),
		TaskID:      taskID,
		Filename:    filename,
		ObjectKey:   key,
		Size:        size,
		ContentType: contentType,
		UploadedBy:  actorID,
	}
	if err := s.attachments.CreateAttachment(ctx, attachment); err != nil {
		return nil, err
	}
	if err := s.files.PutAttachment(ctx, key, reader, size, contentType); err != nil {
		_ = s.attachments.DeleteAttachment(ctx, attachment.ID)
		s.files.RemoveMany(ctx, []string{key})
		return nil, err
	}
	response := attachmentToResponse(attachment, workspaceID, taskID)
	s.bumpWorkspace(ctx, workspaceID)
	s.hub.BroadcastToRoom(boardRoom(workspaceID), &ws.Message{Type: eventAttachmentCreated, Data: response})
	return response, nil
}

func (s *Service) List(ctx context.Context, actorID, workspaceID, taskID string) ([]dattachment.AttachmentResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	if err := validate.UUID(taskID); err != nil {
		return nil, err
	}
	if _, err := s.memberOf(ctx, actorID, workspaceID); err != nil {
		return nil, err
	}
	if _, err := s.workspaceTask(ctx, workspaceID, taskID); err != nil {
		return nil, err
	}
	wsver := cacheutil.Version(ctx, s.cache, cacheutil.WSVerKey+workspaceID)
	uver := cacheutil.Version(ctx, s.cache, cacheutil.UVerKey+actorID)
	listKey := fmt.Sprintf("u:%s:t:v1:attach:%s:%s:%s", actorID, taskID, wsver, uver)
	var cached []dattachment.AttachmentResponse
	if cacheutil.Load(ctx, s.cache, listKey, &cached) {
		return cached, nil
	}
	attachments, err := s.attachments.ListAttachmentsByTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	responses := make([]dattachment.AttachmentResponse, 0, len(attachments))
	for i := range attachments {
		responses = append(responses, *attachmentToResponse(&attachments[i], workspaceID, taskID))
	}
	cacheutil.Store(ctx, s.cache, listKey, responses, cacheutil.TTL)
	return responses, nil
}

func (s *Service) Download(ctx context.Context, actorID, workspaceID, taskID, attachmentID string) (*dattachment.AttachmentResponse, io.ReadCloser, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, nil, err
	}
	if err := validate.UUID(taskID); err != nil {
		return nil, nil, err
	}
	if err := validate.UUID(attachmentID); err != nil {
		return nil, nil, err
	}
	if _, err := s.memberOf(ctx, actorID, workspaceID); err != nil {
		return nil, nil, err
	}
	if _, err := s.workspaceTask(ctx, workspaceID, taskID); err != nil {
		return nil, nil, err
	}
	attachment, err := s.attachments.FindAttachmentByID(ctx, attachmentID)
	if err != nil {
		return nil, nil, err
	}
	if attachment.TaskID != taskID {
		return nil, nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, errors.New("attachment not in task"))
	}
	rc, err := s.files.OpenFile(ctx, attachment.ObjectKey)
	if err != nil {
		return nil, nil, err
	}
	return attachmentToResponse(attachment, workspaceID, taskID), rc, nil
}

func (s *Service) Delete(ctx context.Context, actorID, workspaceID, taskID, attachmentID string) error {
	if err := validate.UUID(workspaceID); err != nil {
		return err
	}
	if err := validate.UUID(taskID); err != nil {
		return err
	}
	if err := validate.UUID(attachmentID); err != nil {
		return err
	}
	if _, err := s.memberOf(ctx, actorID, workspaceID); err != nil {
		return err
	}
	if _, err := s.workspaceTask(ctx, workspaceID, taskID); err != nil {
		return err
	}
	attachment, err := s.attachments.FindAttachmentByID(ctx, attachmentID)
	if err != nil {
		return err
	}
	if attachment.TaskID != taskID {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, errors.New("attachment not in task"))
	}
	if err := s.attachments.DeleteAttachment(ctx, attachmentID); err != nil {
		return err
	}
	s.files.RemoveMany(ctx, []string{attachment.ObjectKey})
	s.bumpWorkspace(ctx, workspaceID)
	s.hub.BroadcastToRoom(boardRoom(workspaceID), &ws.Message{
		Type: eventAttachmentDeleted,
		Data: map[string]string{"id": attachment.ID},
	})
	return nil
}

func (s *Service) memberOf(ctx context.Context, actorID, workspaceID string) (*mworkspace.WorkspaceMember, error) {
	member, err := s.workspaces.FindMember(ctx, workspaceID, actorID)
	if err != nil {
		if errors.Is(err, pkgerrors.ErrNotFound) {
			return nil, pkgerrors.ErrForbidden
		}
		return nil, err
	}
	return member, nil
}

func (s *Service) workspaceTask(ctx context.Context, workspaceID, taskID string) (*mtask.Task, error) {
	task, err := s.tasks.FindTaskByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	column, err := s.columns.FindColumnByID(ctx, task.ColumnID)
	if err != nil {
		return nil, err
	}
	board, err := s.boards.FindBoardByID(ctx, column.BoardID)
	if err != nil {
		return nil, err
	}
	if board.WorkspaceID != workspaceID {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, errors.New("task not in workspace"))
	}
	return task, nil
}

func attachmentToResponse(attachment *mattachment.TaskAttachment, workspaceID, taskID string) *dattachment.AttachmentResponse {
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

func boardRoom(workspaceID string) string {
	return "workspace:" + workspaceID
}

var _ UseCase = (*Service)(nil)
