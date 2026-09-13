package core

import (
	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	dtree "github.com/tandem/tandem/internal/http/dto/tree"
	dworkspace "github.com/tandem/tandem/internal/http/dto/workspace"
	base "github.com/tandem/tandem/internal/usecase/shared/base"
)

type TreeBrick struct {
	Workspace dworkspace.WorkspaceResponse `json:"workspace"`
	Boards    []dtree.TreeBoardResponse    `json:"boards"`
}

type FavFragment struct {
	Workspaces map[string]bool `json:"workspaces"`
	Boards     map[string]bool `json:"boards"`
}

type Core struct {
	Workspaces repository.WorkspaceRepository
	Boards     repository.BoardRepository
	Columns    repository.ColumnRepository
	Tasks      repository.TaskRepository
	Users      repository.UserRepository
	Favorites  repository.FavoriteRepository
	Cache      cache.Cache
}

func New(d base.Deps) *Core {
	return &Core{
		Workspaces: d.Workspaces,
		Boards:     d.Boards,
		Columns:    d.Columns,
		Tasks:      d.Tasks,
		Users:      d.Users,
		Favorites:  d.Favorites,
		Cache:      d.Cache,
	}
}
