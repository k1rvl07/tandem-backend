package core

import (
	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	dtree "github.com/tandem/tandem/internal/http/dto/tree"
	dworkspace "github.com/tandem/tandem/internal/http/dto/workspace"
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

func New(workspaces repository.WorkspaceRepository, boards repository.BoardRepository, columns repository.ColumnRepository, tasks repository.TaskRepository, users repository.UserRepository, favorites repository.FavoriteRepository, cache cache.Cache) *Core {
	return &Core{Workspaces: workspaces, Boards: boards, Columns: columns, Tasks: tasks, Users: users, Favorites: favorites, Cache: cache}
}
