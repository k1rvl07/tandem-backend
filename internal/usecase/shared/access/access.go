package access

import (
	"context"
	"errors"

	mboard "github.com/tandem/tandem/internal/domain/models/board"
	mtask "github.com/tandem/tandem/internal/domain/models/task"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
)

func MemberOrForbidden(ctx context.Context, repo repository.WorkspaceRepository, workspaceID, actorID string) (*mworkspace.WorkspaceMember, error) {
	member, err := repo.FindMember(ctx, workspaceID, actorID)
	if err != nil {
		if errors.Is(err, pkgerrors.ErrNotFound) {
			return nil, pkgerrors.ErrForbidden
		}
		return nil, err
	}
	return member, nil
}

func BoardInWorkspace(ctx context.Context, boards repository.BoardRepository, workspaceID, boardID string) (*mboard.Board, error) {
	board, err := boards.FindBoardByID(ctx, boardID)
	if err != nil {
		return nil, err
	}
	if board.WorkspaceID != workspaceID {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, errors.New("board not in workspace"))
	}
	return board, nil
}

func TaskInWorkspace(ctx context.Context, tasks repository.TaskRepository, columns repository.ColumnRepository, boards repository.BoardRepository, workspaceID, taskID string) (*mtask.Task, error) {
	task, err := tasks.FindTaskByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	column, err := columns.FindColumnByID(ctx, task.ColumnID)
	if err != nil {
		return nil, err
	}
	board, err := boards.FindBoardByID(ctx, column.BoardID)
	if err != nil {
		return nil, err
	}
	if board.WorkspaceID != workspaceID {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, errors.New("task not in workspace"))
	}
	return task, nil
}

func RequireEditor(role mworkspace.WorkspaceRole) error {
	if role != mworkspace.RoleOwner && role != mworkspace.RoleEditor {
		return pkgerrors.ErrForbidden
	}
	return nil
}

func RequireOwner(role mworkspace.WorkspaceRole) error {
	if role != mworkspace.RoleOwner {
		return pkgerrors.ErrForbidden
	}
	return nil
}
