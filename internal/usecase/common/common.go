package common

import (
	"context"
	"errors"

	mtask "github.com/tandem/tandem/internal/domain/models/task"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	dtask "github.com/tandem/tandem/internal/http/dto/task"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/usecase/cacheutil"
)

func BumpWorkspace(ctx context.Context, c cache.Cache, workspaceID string) {
	cacheutil.Bump(ctx, c, cacheutil.WSVerKey+workspaceID)
}

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

func IsNotFound(err error) bool {
	return errors.Is(err, pkgerrors.ErrNotFound)
}
