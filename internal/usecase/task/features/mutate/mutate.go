package mutate

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	mtask "github.com/tandem/tandem/internal/domain/models/task"
	dtask "github.com/tandem/tandem/internal/http/dto/task"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
	"github.com/tandem/tandem/internal/usecase/shared/errutil"
	"github.com/tandem/tandem/internal/usecase/task/core"
)

const maxDescriptionLen = 1500

type Mutate struct {
	*core.Core
}

func New(c *core.Core) *Mutate {
	return &Mutate{Core: c}
}

func (s *Mutate) ValidatePayload(title, description string) error {
	if err := validate.Title(title); err != nil {
		return err
	}
	if utf8.RuneCountInString(strings.TrimSpace(description)) > maxDescriptionLen {
		return pkgerrors.NewValidationError("description must be at most %d characters", maxDescriptionLen)
	}
	return nil
}

func (s *Mutate) ApplyUpdateFields(ctx context.Context, actorID, workspaceID string, task *mtask.Task, req dtask.UpdateTaskRequest) (string, error) {
	oldImageKey := task.ImageKey
	if req.Title != nil {
		if err := validate.Title(*req.Title); err != nil {
			return "", err
		}
		next := strings.TrimSpace(*req.Title)
		task.Title = next
	}
	if req.Description != nil {
		if utf8.RuneCountInString(strings.TrimSpace(*req.Description)) > maxDescriptionLen {
			return "", pkgerrors.NewValidationError("description must be at most %d characters", maxDescriptionLen)
		}
		next := strings.TrimSpace(*req.Description)
		task.Description = next
	}
	if req.DueDate != nil {
		dueDate, err := s.ParseDueDate(*req.DueDate)
		if err != nil {
			return "", err
		}
		task.DueDate = dueDate
	}
	if req.AssigneeID != nil {
		assigneeID, err := s.ValidateAssignee(ctx, workspaceID, strings.TrimSpace(*req.AssigneeID))
		if err != nil {
			return "", err
		}
		task.AssigneeID = assigneeID
	}
	if req.CuratorID != nil {
		curatorID, err := s.ValidateCurator(ctx, workspaceID, strings.TrimSpace(*req.CuratorID))
		if err != nil {
			return "", err
		}
		task.CuratorID = curatorID
	}
	if req.ParentID != nil {
		parentID, err := s.ValidateParent(ctx, workspaceID, strings.TrimSpace(*req.ParentID), task.ID)
		if err != nil {
			return "", err
		}
		task.ParentID = parentID
	}
	if req.IsUrgent != nil {
		task.IsUrgent = *req.IsUrgent
	}
	if req.IsHidden != nil {
		task.IsHidden = *req.IsHidden
	}
	if req.ImageKey != nil {
		key := strings.TrimSpace(*req.ImageKey)
		if err := s.Files.ValidateImageKey(key, actorID); err != nil {
			return "", err
		}
		task.ImageKey = key
	}
	return oldImageKey, nil
}

func (s *Mutate) ValidateAssignee(ctx context.Context, workspaceID, assigneeID string) (string, error) {
	if assigneeID == "" {
		return "", nil
	}
	if err := validate.UUID(assigneeID); err != nil {
		return "", err
	}
	if _, err := s.Workspaces.FindMember(ctx, workspaceID, assigneeID); err != nil {
		if errutil.IsNotFound(err) {
			return "", pkgerrors.NewValidationError("assignee must be a workspace member")
		}
		return "", err
	}
	return assigneeID, nil
}

func (s *Mutate) ValidateCurator(ctx context.Context, workspaceID, curatorID string) (string, error) {
	if curatorID == "" {
		return "", nil
	}
	if err := validate.UUID(curatorID); err != nil {
		return "", err
	}
	if _, err := s.Workspaces.FindMember(ctx, workspaceID, curatorID); err != nil {
		if errutil.IsNotFound(err) {
			return "", pkgerrors.NewValidationError("curator must be a workspace member")
		}
		return "", err
	}
	return curatorID, nil
}

func (s *Mutate) ValidateParent(ctx context.Context, workspaceID, parentID, selfID string) (string, error) {
	if parentID == "" {
		return "", nil
	}
	if err := validate.UUID(parentID); err != nil {
		return "", err
	}
	if selfID != "" && parentID == selfID {
		return "", pkgerrors.NewValidationError("task cannot be its own parent")
	}
	if _, err := s.WorkspaceTask(ctx, workspaceID, parentID); err != nil {
		return "", err
	}
	seen := map[string]bool{selfID: true}
	current := parentID
	for current != "" {
		parent, err := s.Tasks.FindTaskByID(ctx, current)
		if err != nil {
			return "", err
		}
		if seen[parent.ID] {
			return "", pkgerrors.NewValidationError("parent creates a circular dependency")
		}
		seen[parent.ID] = true
		if _, err := s.WorkspaceTask(ctx, workspaceID, parent.ID); err != nil {
			return "", err
		}
		current = parent.ParentID
	}
	return parentID, nil
}

func (s *Mutate) ParseDueDate(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, pkgerrors.NewValidationError("due_date must be in YYYY-MM-DD format")
	}
	return &parsed, nil
}
