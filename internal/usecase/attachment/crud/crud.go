package crud

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/google/uuid"
	mattachment "github.com/tandem/tandem/internal/domain/models/attachment"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	dattachment "github.com/tandem/tandem/internal/http/dto/attachment"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
	attcore "github.com/tandem/tandem/internal/usecase/attachment/core"
	cacheutil "github.com/tandem/tandem/internal/usecase/shared/cache"
)

type Crud struct {
	*attcore.Core
}

func New(c *attcore.Core) *Crud {
	return &Crud{Core: c}
}

func (s *Crud) Create(ctx context.Context, actorID, workspaceID, taskID, filename, contentType string, reader io.Reader, size int64) (*dattachment.AttachmentResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	if err := validate.UUID(taskID); err != nil {
		return nil, err
	}
	if _, err := s.MemberOf(ctx, actorID, workspaceID); err != nil {
		return nil, err
	}
	if _, err := s.WorkspaceTask(ctx, workspaceID, taskID); err != nil {
		return nil, err
	}
	key, err := s.Files.PrepareAttachment(workspaceID, actorID, filename, size)
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
	if err := s.Attachments.CreateAttachment(ctx, attachment); err != nil {
		return nil, err
	}
	if err := s.Files.PutAttachment(ctx, key, reader, size, contentType); err != nil {
		_ = s.Attachments.DeleteAttachment(ctx, attachment.ID)
		s.Files.RemoveMany(ctx, []string{key})
		return nil, err
	}
	response := s.AttachmentResponse(attachment, workspaceID, taskID)
	s.BumpWorkspace(ctx, workspaceID)
	s.Hub.BroadcastToRoom(s.Room(workspaceID), &ws.Message{Type: attcore.EventAttachmentCreated, Data: response})
	return response, nil
}

func (s *Crud) List(ctx context.Context, actorID, workspaceID, taskID string) ([]dattachment.AttachmentResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	if err := validate.UUID(taskID); err != nil {
		return nil, err
	}
	if _, err := s.MemberOf(ctx, actorID, workspaceID); err != nil {
		return nil, err
	}
	if _, err := s.WorkspaceTask(ctx, workspaceID, taskID); err != nil {
		return nil, err
	}
	wsver := cacheutil.Version(ctx, s.Cache, cacheutil.WSVerKey+workspaceID)
	uver := cacheutil.Version(ctx, s.Cache, cacheutil.UVerKey+actorID)
	listKey := fmt.Sprintf("u:%s:t:v1:attach:%s:%s:%s", actorID, taskID, wsver, uver)
	var cached []dattachment.AttachmentResponse
	if cacheutil.Load(ctx, s.Cache, listKey, &cached) {
		return cached, nil
	}
	attachments, err := s.Attachments.ListAttachmentsByTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	responses := make([]dattachment.AttachmentResponse, 0, len(attachments))
	for i := range attachments {
		responses = append(responses, *s.AttachmentResponse(&attachments[i], workspaceID, taskID))
	}
	cacheutil.Store(ctx, s.Cache, listKey, responses, cacheutil.TTL)
	return responses, nil
}

func (s *Crud) Download(ctx context.Context, actorID, workspaceID, taskID, attachmentID string) (*dattachment.AttachmentResponse, io.ReadCloser, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, nil, err
	}
	if err := validate.UUID(taskID); err != nil {
		return nil, nil, err
	}
	if err := validate.UUID(attachmentID); err != nil {
		return nil, nil, err
	}
	if _, err := s.MemberOf(ctx, actorID, workspaceID); err != nil {
		return nil, nil, err
	}
	if _, err := s.WorkspaceTask(ctx, workspaceID, taskID); err != nil {
		return nil, nil, err
	}
	attachment, err := s.Attachments.FindAttachmentByID(ctx, attachmentID)
	if err != nil {
		return nil, nil, err
	}
	if attachment.TaskID != taskID {
		return nil, nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, errors.New("attachment not in task"))
	}
	rc, err := s.Files.OpenFile(ctx, attachment.ObjectKey)
	if err != nil {
		return nil, nil, err
	}
	return s.AttachmentResponse(attachment, workspaceID, taskID), rc, nil
}

func (s *Crud) Delete(ctx context.Context, actorID, workspaceID, taskID, attachmentID string) error {
	if err := validate.UUID(workspaceID); err != nil {
		return err
	}
	if err := validate.UUID(taskID); err != nil {
		return err
	}
	if err := validate.UUID(attachmentID); err != nil {
		return err
	}
	if _, err := s.MemberOf(ctx, actorID, workspaceID); err != nil {
		return err
	}
	if _, err := s.WorkspaceTask(ctx, workspaceID, taskID); err != nil {
		return err
	}
	attachment, err := s.Attachments.FindAttachmentByID(ctx, attachmentID)
	if err != nil {
		return err
	}
	if attachment.TaskID != taskID {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, errors.New("attachment not in task"))
	}
	if err := s.Attachments.DeleteAttachment(ctx, attachmentID); err != nil {
		return err
	}
	s.Files.RemoveMany(ctx, []string{attachment.ObjectKey})
	s.BumpWorkspace(ctx, workspaceID)
	s.Hub.BroadcastToRoom(s.Room(workspaceID), &ws.Message{
		Type: attcore.EventAttachmentDeleted,
		Data: map[string]string{"id": attachment.ID},
	})
	return nil
}
