package tree

import (
	"context"
	"fmt"
	"sort"

	mboard "github.com/tandem/tandem/internal/domain/models/board"
	mfavorite "github.com/tandem/tandem/internal/domain/models/favorite"
	mtask "github.com/tandem/tandem/internal/domain/models/task"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	dboard "github.com/tandem/tandem/internal/http/dto/board"
	dtask "github.com/tandem/tandem/internal/http/dto/task"
	dtree "github.com/tandem/tandem/internal/http/dto/tree"
	dworkspace "github.com/tandem/tandem/internal/http/dto/workspace"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	cacheutil "github.com/tandem/tandem/internal/usecase/shared/cache"
	"github.com/tandem/tandem/internal/usecase/shared/errutil"
	"github.com/tandem/tandem/internal/usecase/shared/taskmap"
)

const taskFilterAll = "all"

type UseCase interface {
	List(ctx context.Context, actorID string, query dtree.TreeQuery) ([]dtree.TreeWorkspaceResponse, error)
}

type Service struct {
	workspaces repository.WorkspaceRepository
	boards     repository.BoardRepository
	columns    repository.ColumnRepository
	tasks      repository.TaskRepository
	users      repository.UserRepository
	favorites  repository.FavoriteRepository
	cache      cache.Cache
}

type Deps struct {
	Workspaces repository.WorkspaceRepository
	Boards     repository.BoardRepository
	Columns    repository.ColumnRepository
	Tasks      repository.TaskRepository
	Users      repository.UserRepository
	Favorites  repository.FavoriteRepository
	Cache      cache.Cache
}

func NewService(deps Deps) *Service {
	return &Service{
		workspaces: deps.Workspaces,
		boards:     deps.Boards,
		columns:    deps.Columns,
		tasks:      deps.Tasks,
		users:      deps.Users,
		favorites:  deps.Favorites,
		cache:      deps.Cache,
	}
}

type treeBrick struct {
	Workspace dworkspace.WorkspaceResponse `json:"workspace"`
	Boards    []dtree.TreeBoardResponse    `json:"boards"`
}

type favFragment struct {
	Workspaces map[string]bool `json:"workspaces"`
	Boards     map[string]bool `json:"boards"`
}

func (s *Service) List(ctx context.Context, actorID string, query dtree.TreeQuery) ([]dtree.TreeWorkspaceResponse, error) {
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

	uver := cacheutil.Version(ctx, s.cache, cacheutil.UVerKey+actorID)
	fav, err := s.loadFavFragment(ctx, actorID, uver)
	if err != nil {
		return nil, err
	}

	memberships, err := s.workspaces.ListWorkspacesForUser(ctx, actorID)
	if err != nil {
		return nil, err
	}
	brickKeys := s.brickKeys(ctx, memberships, actorID, uver, tasksFilter)
	values, err := s.cache.MGet(ctx, brickKeys...)
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

func (s *Service) loadFavFragment(ctx context.Context, actorID, uver string) (favFragment, error) {
	favKey := fmt.Sprintf("u:%s:t:v1:fav:%s", actorID, uver)
	var fav favFragment
	if cacheutil.Load(ctx, s.cache, favKey, &fav) {
		return fav, nil
	}
	favWorkspaces, err := s.favorites.ListFavoriteTargets(ctx, actorID, mfavorite.FavoriteWorkspace)
	if err != nil {
		return fav, err
	}
	favBoards, err := s.favorites.ListFavoriteTargets(ctx, actorID, mfavorite.FavoriteBoard)
	if err != nil {
		return fav, err
	}
	fav = favFragment{Workspaces: favWorkspaces, Boards: favBoards}
	cacheutil.Store(ctx, s.cache, favKey, &fav, cacheutil.TTL)
	return fav, nil
}

func (s *Service) brickKeys(ctx context.Context, memberships []mworkspace.WorkspaceMembership, actorID, uver, tasksFilter string) []string {
	keys := make([]string, len(memberships))
	for i := range memberships {
		wsver := cacheutil.Version(ctx, s.cache, cacheutil.WSVerKey+memberships[i].Workspace.ID)
		keys[i] = fmt.Sprintf("u:%s:t:v1:tree:%s:%s:%s:%s", actorID, memberships[i].Workspace.ID, wsver, uver, tasksFilter)
	}
	return keys
}

func (s *Service) loadBrick(ctx context.Context, values []string, index int, actorID string, workspace *mworkspace.Workspace, role mworkspace.WorkspaceRole, tasksFilter, key string) (treeBrick, error) {
	var brick treeBrick
	if index < len(values) && values[index] != "" && cacheutil.Unmarshal(values[index], &brick) {
		return brick, nil
	}
	brick, err := s.buildBrick(ctx, actorID, workspace, role, tasksFilter)
	if err != nil {
		return treeBrick{}, err
	}
	cacheutil.Store(ctx, s.cache, key, &brick, cacheutil.TTL)
	return brick, nil
}

func (s *Service) buildBrick(ctx context.Context, actorID string, workspace *mworkspace.Workspace, role mworkspace.WorkspaceRole, tasksFilter string) (treeBrick, error) {
	boards, err := s.boards.ListBoards(ctx, workspace.ID)
	if err != nil {
		return treeBrick{}, err
	}
	tasks, err := s.tasks.ListTasksForWorkspace(ctx, workspace.ID)
	if err != nil {
		return treeBrick{}, err
	}
	columns, err := s.columns.ListColumnsForWorkspace(ctx, workspace.ID)
	if err != nil {
		return treeBrick{}, err
	}
	columnBoard := make(map[string]string, len(columns))
	columnOrder := make(map[string]int, len(columns))
	for j := range columns {
		columnBoard[columns[j].ID] = columns[j].BoardID
		columnOrder[columns[j].ID] = columns[j].Position
	}
	treeBoards := make([]dtree.TreeBoardResponse, 0, len(boards))
	for _, board := range boards {
		matching := s.matchingTasks(tasks, board, columnBoard, tasksFilter, actorID)
		if len(matching) == 0 {
			continue
		}
		sortTasksByColumnOrder(matching, columnOrder)
		responses, err := s.taskResponses(ctx, workspace, board, matching)
		if err != nil {
			return treeBrick{}, err
		}
		treeBoards = append(treeBoards, dtree.TreeBoardResponse{
			Board: dboard.BoardResponse{
				ID:          board.ID,
				WorkspaceID: board.WorkspaceID,
				Name:        board.Name,
				Position:    board.Position,
				IsMain:      board.IsMain,
				IsFavorite:  false,
				CreatedAt:   board.CreatedAt,
				UpdatedAt:   board.UpdatedAt,
			},
			Tasks: responses,
		})
	}
	return treeBrick{
		Workspace: dworkspace.WorkspaceResponse{
			ID:         workspace.ID,
			Name:       workspace.Name,
			Prefix:     workspace.Prefix,
			Theme:      workspace.Theme,
			Role:       string(role),
			IsFavorite: false,
			CreatedAt:  workspace.CreatedAt,
			UpdatedAt:  workspace.UpdatedAt,
		},
		Boards: treeBoards,
	}, nil
}

func (s *Service) matchingTasks(tasks []*mtask.Task, board *mboard.Board, columnBoard map[string]string, tasksFilter string, actorID string) []*mtask.Task {
	matching := make([]*mtask.Task, 0, 4)
	for _, task := range tasks {
		if task.IsHidden {
			continue
		}
		if columnBoard[task.ColumnID] != board.ID {
			continue
		}
		if !s.taskMatches(tasksFilter, task, actorID) {
			continue
		}
		matching = append(matching, task)
	}
	return matching
}

func sortTasksByColumnOrder(tasks []*mtask.Task, columnOrder map[string]int) {
	sort.SliceStable(tasks, func(i, j int) bool {
		if columnOrder[tasks[i].ColumnID] != columnOrder[tasks[j].ColumnID] {
			return columnOrder[tasks[i].ColumnID] < columnOrder[tasks[j].ColumnID]
		}
		if tasks[i].Position != tasks[j].Position {
			return tasks[i].Position < tasks[j].Position
		}
		return tasks[i].CreatedAt.Before(tasks[j].CreatedAt)
	})
}

func (s *Service) taskMatches(filter string, task *mtask.Task, actorID string) bool {
	switch filter {
	case "mine":
		return task.AuthorID == actorID || task.CuratorID == actorID || task.AssigneeID == actorID
	case "for_me":
		return task.AssigneeID == actorID
	default:
		return true
	}
}

func (s *Service) taskResponses(ctx context.Context, workspace *mworkspace.Workspace, board *mboard.Board, tasks []*mtask.Task) ([]dtask.TaskResponse, error) {
	columnNames := make(map[string]string)
	ids := make(map[string]bool)
	for _, task := range tasks {
		columnNames[task.ColumnID] = ""
		for _, id := range []string{task.AuthorID, task.AssigneeID, task.CuratorID} {
			if id != "" {
				ids[id] = true
			}
		}
	}
	columns, err := s.columns.ListColumnsForWorkspace(ctx, workspace.ID)
	if err != nil {
		return nil, err
	}
	for i := range columns {
		columnNames[columns[i].ID] = columns[i].Name
	}
	users := make(map[string]*dtask.TaskUserResponse)
	for id := range ids {
		user, err := s.users.FindByID(ctx, id)
		if err != nil {
			if errutil.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		users[id] = &dtask.TaskUserResponse{
			ID:          user.ID,
			Login:       user.Login,
			DisplayName: user.DisplayName,
			AvatarKey:   user.AvatarKey,
		}
	}
	responses := make([]dtask.TaskResponse, 0, len(tasks))
	for _, task := range tasks {
		responses = append(responses, dtask.TaskResponse{
			ID:          task.ID,
			DisplayID:   displayID(workspace.Prefix, task.ID),
			WorkspaceID: workspace.ID,
			BoardID:     board.ID,
			BoardName:   board.Name,
			ColumnID:    task.ColumnID,
			ColumnName:  columnNames[task.ColumnID],
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
		})
	}
	return responses, nil
}

func displayID(prefix, taskID string) string {
	return taskmap.DisplayID(prefix, taskID)
}

var _ UseCase = (*Service)(nil)
