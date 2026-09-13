package taskmap

import (
	"context"
	"errors"

	mtask "github.com/tandem/tandem/internal/domain/models/task"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	dtask "github.com/tandem/tandem/internal/http/dto/task"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
)

func ResolveUsers(ctx context.Context, repo repository.UserRepository, tasks []*mtask.Task) (map[string]*dtask.TaskUserResponse, error) {
	ids := make(map[string]bool)
	for _, task := range tasks {
		if task.AuthorID != "" {
			ids[task.AuthorID] = true
		}
		if task.AssigneeID != "" {
			ids[task.AssigneeID] = true
		}
		if task.CuratorID != "" {
			ids[task.CuratorID] = true
		}
	}
	result := make(map[string]*dtask.TaskUserResponse)
	for id := range ids {
		user, err := repo.FindByID(ctx, id)
		if err != nil {
			if errors.Is(err, pkgerrors.ErrNotFound) {
				continue
			}
			return nil, err
		}
		result[id] = &dtask.TaskUserResponse{
			ID:          user.ID,
			Login:       user.Login,
			DisplayName: user.DisplayName,
			AvatarKey:   user.AvatarKey,
		}
	}
	return result, nil
}

func DisplayID(prefix, taskID string) string {
	if prefix == "" {
		prefix = "T"
	}
	short := taskID
	if len(short) > 8 {
		short = short[:8]
	}
	return prefix + "-" + short
}

type TaskResponseCtx struct {
	WorkspaceID string
	BoardID     string
	BoardName   string
	ColumnName  string
	DisplayID   string
}

func BuildTaskResponse(task *mtask.Task, users map[string]*dtask.TaskUserResponse, ctx TaskResponseCtx) dtask.TaskResponse {
	return dtask.TaskResponse{
		ID:          task.ID,
		DisplayID:   ctx.DisplayID,
		WorkspaceID: ctx.WorkspaceID,
		BoardID:     ctx.BoardID,
		BoardName:   ctx.BoardName,
		ColumnID:    task.ColumnID,
		ColumnName:  ctx.ColumnName,
		Title:       task.Title,
		Description: task.Description,
		Author:      users[task.AuthorID],
		Assignee:    users[task.AssigneeID],
		Curator:     users[task.CuratorID],
		ParentID:    task.ParentID,
		DueDate:     task.DueDate,
		Position:    task.Position,
		IsUrgent:    task.IsUrgent,
		IsHidden:    task.IsHidden,
		ImageKey:    task.ImageKey,
		CreatedAt:   task.CreatedAt,
		UpdatedAt:   task.UpdatedAt,
	}
}
