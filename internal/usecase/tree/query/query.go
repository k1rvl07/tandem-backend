package query

import (
	"context"
	"fmt"

	mfavorite "github.com/tandem/tandem/internal/domain/models/favorite"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	dtree "github.com/tandem/tandem/internal/http/dto/tree"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	cacheutil "github.com/tandem/tandem/internal/usecase/shared/cache"
	"github.com/tandem/tandem/internal/usecase/tree/build"
	"github.com/tandem/tandem/internal/usecase/tree/core"
)

const taskFilterAll = "all"

type Query struct {
	*core.Core
	*build.Build
}

func New(c *core.Core, b *build.Build) *Query {
	return &Query{Core: c, Build: b}
}

func (s *Query) List(ctx context.Context, actorID string, query dtree.TreeQuery) ([]dtree.TreeWorkspaceResponse, error) {
	tasksFilter := query.Tasks
	if tasksFilter == "" {
		tasksFilter = taskFilterAll
	}
	switch tasksFilter {
	case taskFilterAll, "mine", "for_me":
	default:
		return nil, pkgerrors.NewValidationError("tasks filter must be one of all, mine, for_me")
	}
	switch query.Workspaces {
	case "", "all", "fav":
	default:
		return nil, pkgerrors.NewValidationError("workspaces filter must be one of all, fav")
	}
	favWsOnly := query.Workspaces == "fav"
	favBoardOnly := query.Boards == "fav"

	uver := cacheutil.Version(ctx, s.Cache, cacheutil.UVerKey+actorID)
	fav, err := s.loadFavFragment(ctx, actorID, uver)
	if err != nil {
		return nil, err
	}

	memberships, err := s.Workspaces.ListWorkspacesForUser(ctx, actorID)
	if err != nil {
		return nil, err
	}
	brickKeys := s.brickKeys(ctx, memberships, actorID, uver, tasksFilter)
	values, err := s.Cache.MGet(ctx, brickKeys...)
	if err != nil {
		values = make([]string, len(brickKeys))
	}

	result := make([]dtree.TreeWorkspaceResponse, 0, len(memberships))
	for i := range memberships {
		workspace := &memberships[i].Workspace
		if favWsOnly && !fav.Workspaces[workspace.ID] {
			continue
		}
		brick, err := s.loadBrick(ctx, values, i, actorID, workspace, memberships[i].Role, tasksFilter, brickKeys[i])
		if err != nil {
			return nil, err
		}
		treeBoards := make([]dtree.TreeBoardResponse, 0, len(brick.Boards))
		for j := range brick.Boards {
			if favBoardOnly && !fav.Boards[brick.Boards[j].Board.ID] {
				continue
			}
			tb := brick.Boards[j]
			tb.Board.IsFavorite = fav.Boards[tb.Board.ID]
			treeBoards = append(treeBoards, tb)
		}
		if len(treeBoards) == 0 {
			continue
		}
		wsResp := brick.Workspace
		wsResp.IsFavorite = fav.Workspaces[workspace.ID]
		result = append(result, dtree.TreeWorkspaceResponse{Workspace: wsResp, Boards: treeBoards})
	}
	return result, nil
}

func (s *Query) loadFavFragment(ctx context.Context, actorID, uver string) (core.FavFragment, error) {
	favKey := fmt.Sprintf("u:%s:t:v1:fav:%s", actorID, uver)
	var fav core.FavFragment
	if cacheutil.Load(ctx, s.Cache, favKey, &fav) {
		return fav, nil
	}
	favWorkspaces, err := s.Favorites.ListFavoriteTargets(ctx, actorID, mfavorite.FavoriteWorkspace)
	if err != nil {
		return fav, err
	}
	favBoards, err := s.Favorites.ListFavoriteTargets(ctx, actorID, mfavorite.FavoriteBoard)
	if err != nil {
		return fav, err
	}
	fav = core.FavFragment{Workspaces: favWorkspaces, Boards: favBoards}
	cacheutil.Store(ctx, s.Cache, favKey, &fav, cacheutil.TTL)
	return fav, nil
}

func (s *Query) brickKeys(ctx context.Context, memberships []mworkspace.WorkspaceMembership, actorID, uver, tasksFilter string) []string {
	keys := make([]string, len(memberships))
	for i := range memberships {
		wsver := cacheutil.Version(ctx, s.Cache, cacheutil.WSVerKey+memberships[i].Workspace.ID)
		keys[i] = fmt.Sprintf("u:%s:t:v1:tree:%s:%s:%s:%s", actorID, memberships[i].Workspace.ID, wsver, uver, tasksFilter)
	}
	return keys
}

func (s *Query) loadBrick(ctx context.Context, values []string, index int, actorID string, workspace *mworkspace.Workspace, role mworkspace.WorkspaceRole, tasksFilter, key string) (core.TreeBrick, error) {
	var brick core.TreeBrick
	if index < len(values) && values[index] != "" && cacheutil.Unmarshal(values[index], &brick) {
		return brick, nil
	}
	brick, err := s.BuildBrick(ctx, actorID, workspace, role, tasksFilter)
	if err != nil {
		return core.TreeBrick{}, err
	}
	cacheutil.Store(ctx, s.Cache, key, &brick, cacheutil.TTL)
	return brick, nil
}
