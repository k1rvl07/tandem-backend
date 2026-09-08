package board

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/tandem/tandem/internal/domain/models"
	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	"github.com/tandem/tandem/internal/http/dto"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
	"github.com/tandem/tandem/internal/usecase/cacheutil"
	file "github.com/tandem/tandem/internal/usecase/file"
)

const (
	eventBoardCreated    = "board.created"
	eventBoardUpdated    = "board.updated"
	eventBoardDeleted    = "board.deleted"
	eventBoardsReordered = "boards.reordered"
)

var defaultColumns = models.DefaultColumnNames

type UseCase interface {
	Create(ctx context.Context, actorID, workspaceID string, req dto.CreateBoardRequest) (*dto.BoardResponse, error)
	List(ctx context.Context, actorID, workspaceID string) ([]dto.BoardResponse, error)
	Get(ctx context.Context, actorID, workspaceID, boardID string) (*dto.BoardDetailResponse, error)
	Update(ctx context.Context, actorID, workspaceID, boardID string, req dto.UpdateBoardRequest) (*dto.BoardResponse, error)
	Delete(ctx context.Context, actorID, workspaceID, boardID string) error
	SetMain(ctx context.Context, actorID, workspaceID, boardID string) (*dto.BoardResponse, error)
	Reorder(ctx context.Context, actorID, workspaceID string, req dto.ReorderBoardsRequest) ([]dto.BoardResponse, error)
}

type Service struct {
	boards     repository.BoardRepository
	columns    repository.ColumnRepository
	tasks      repository.TaskRepository
	workspaces repository.WorkspaceRepository
	users      repository.UserRepository
	favorites  repository.FavoriteRepository
	files      *file.Service
	hub        ws.Hub
	cache      cache.Cache
}

func NewService(
	boards repository.BoardRepository,
	columns repository.ColumnRepository,
	tasks repository.TaskRepository,
	workspaces repository.WorkspaceRepository,
	users repository.UserRepository,
	favorites repository.FavoriteRepository,
	files *file.Service,
	hub ws.Hub,
	cache cache.Cache,
) *Service {
	return &Service{
		boards:     boards,
		columns:    columns,
		tasks:      tasks,
		workspaces: workspaces,
		users:      users,
		favorites:  favorites,
		files:      files,
		hub:        hub,
		cache:      cache,
	}
}

func (s *Service) bumpWorkspace(ctx context.Context, workspaceID string) {
	cacheutil.Bump(ctx, s.cache, cacheutil.WSVerKey+workspaceID)
}

func (s *Service) Create(ctx context.Context, actorID, workspaceID string, req dto.CreateBoardRequest) (*dto.BoardResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	member, err := s.memberOf(ctx, actorID, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.requireEditor(member.Role); err != nil {
		return nil, err
	}
	if err := validate.Name(req.Name); err != nil {
		return nil, err
	}
	existing, err := s.boards.ListBoards(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	board := &models.Board{
		ID:          uuid.New().String(),
		WorkspaceID: workspaceID,
		Name:        strings.TrimSpace(req.Name),
		Position:    len(existing),
	}
	if err := s.boards.CreateBoard(ctx, board); err != nil {
		return nil, err
	}
	for i, name := range defaultColumns {
		column := &models.Column{
			ID:       uuid.New().String(),
			BoardID:  board.ID,
			Name:     name,
			Position: i,
		}
		if err := s.columns.CreateColumn(ctx, column); err != nil {
			return nil, err
		}
	}
	response := boardToResponse(board)
	s.bumpWorkspace(ctx, workspaceID)
	s.hub.BroadcastToRoom(boardRoom(workspaceID), &ws.Message{Type: eventBoardCreated, Data: response})
	return response, nil
}

func (s *Service) List(ctx context.Context, actorID, workspaceID string) ([]dto.BoardResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	member, err := s.memberOf(ctx, actorID, workspaceID)
	if err != nil {
		return nil, err
	}
	_ = member
	wsver := cacheutil.Version(ctx, s.cache, cacheutil.WSVerKey+workspaceID)
	uver := cacheutil.Version(ctx, s.cache, cacheutil.UVerKey+actorID)
	listKey := fmt.Sprintf("u:%s:t:v1:boards:%s:%s:%s", actorID, workspaceID, wsver, uver)
	var cached []dto.BoardResponse
	if cacheutil.Load(ctx, s.cache, listKey, &cached) {
		return cached, nil
	}
	boards, err := s.boards.ListBoards(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	counts, err := s.boards.CountTasksByBoard(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	responses := make([]dto.BoardResponse, 0, len(boards))
	favBoards, err := s.favorites.ListFavoriteTargets(ctx, actorID, models.FavoriteBoard)
	if err != nil {
		return nil, err
	}
	for i := range boards {
		resp := boardToResponse(boards[i])
		resp.TaskCount = counts[boards[i].ID]
		resp.IsFavorite = favBoards[boards[i].ID]
		responses = append(responses, *resp)
	}
	cacheutil.Store(ctx, s.cache, listKey, responses, cacheutil.TTL)
	return responses, nil
}

func (s *Service) Get(ctx context.Context, actorID, workspaceID, boardID string) (*dto.BoardDetailResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	if err := validate.UUID(boardID); err != nil {
		return nil, err
	}
	if _, err := s.memberOf(ctx, actorID, workspaceID); err != nil {
		return nil, err
	}
	board, err := s.boardInWorkspace(ctx, workspaceID, boardID)
	if err != nil {
		return nil, err
	}
	wsver := cacheutil.Version(ctx, s.cache, cacheutil.WSVerKey+workspaceID)
	uver := cacheutil.Version(ctx, s.cache, cacheutil.UVerKey+actorID)
	detailKey := fmt.Sprintf("u:%s:t:v1:board:%s:%s:%s", actorID, boardID, wsver, uver)
	var cached dto.BoardDetailResponse
	if cacheutil.Load(ctx, s.cache, detailKey, &cached) {
		return &cached, nil
	}
	columns, err := s.columns.ListColumns(ctx, board.ID)
	if err != nil {
		return nil, err
	}
	tasks, err := s.tasks.ListTasksForBoard(ctx, board.ID)
	if err != nil {
		return nil, err
	}
	workspace, err := s.workspaces.FindWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	users, err := s.resolveUsers(ctx, tasks)
	if err != nil {
		return nil, err
	}
	columnNames := make(map[string]string, len(columns))
	for i := range columns {
		columnNames[columns[i].ID] = columns[i].Name
	}

	columnDetails := make([]dto.ColumnDetailResponse, 0, len(columns))
	for i := range columns {
		columnTasks := make([]dto.TaskResponse, 0)
		for _, task := range tasks {
			if task.ColumnID != columns[i].ID {
				continue
			}
			if task.IsHidden {
				continue
			}
			columnTasks = append(columnTasks, taskToResponse(task, users, workspace.Prefix, boardID, board.Name, columns[i].Name))
		}
		columnDetails = append(columnDetails, dto.ColumnDetailResponse{
			ID:        columns[i].ID,
			BoardID:   columns[i].BoardID,
			Name:      columns[i].Name,
			Position:  columns[i].Position,
			TaskCount: len(columnTasks),
			Tasks:     columnTasks,
			CreatedAt: columns[i].CreatedAt,
			UpdatedAt: columns[i].UpdatedAt,
		})
	}
	detail := &dto.BoardDetailResponse{
		ID:          board.ID,
		WorkspaceID: board.WorkspaceID,
		Name:        board.Name,
		Position:    board.Position,
		IsMain:      board.IsMain,
		Columns:     columnDetails,
		CreatedAt:   board.CreatedAt,
		UpdatedAt:   board.UpdatedAt,
	}
	cacheutil.Store(ctx, s.cache, detailKey, detail, cacheutil.TTL)
	return detail, nil
}

func (s *Service) Update(ctx context.Context, actorID, workspaceID, boardID string, req dto.UpdateBoardRequest) (*dto.BoardResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	if err := validate.UUID(boardID); err != nil {
		return nil, err
	}
	member, err := s.memberOf(ctx, actorID, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.requireEditor(member.Role); err != nil {
		return nil, err
	}
	if err := validate.Name(req.Name); err != nil {
		return nil, err
	}
	board, err := s.boardInWorkspace(ctx, workspaceID, boardID)
	if err != nil {
		return nil, err
	}
	board.Name = strings.TrimSpace(req.Name)
	if err := s.boards.UpdateBoard(ctx, board); err != nil {
		return nil, err
	}
	response := boardToResponse(board)
	s.bumpWorkspace(ctx, workspaceID)
	s.hub.BroadcastToRoom(boardRoom(workspaceID), &ws.Message{Type: eventBoardUpdated, Data: response})
	return response, nil
}

func (s *Service) Delete(ctx context.Context, actorID, workspaceID, boardID string) error {
	if err := validate.UUID(workspaceID); err != nil {
		return err
	}
	if err := validate.UUID(boardID); err != nil {
		return err
	}
	member, err := s.memberOf(ctx, actorID, workspaceID)
	if err != nil {
		return err
	}
	if err := s.requireEditor(member.Role); err != nil {
		return err
	}
	board, err := s.boardInWorkspace(ctx, workspaceID, boardID)
	if err != nil {
		return err
	}
	if board.IsMain {
		return pkgerrors.NewValidationError("cannot delete the main board")
	}
	keys, err := s.tasks.CollectBoardKeys(ctx, board.ID)
	if err != nil {
		return err
	}
	s.files.RemoveMany(ctx, keys)
	if err := s.boards.DeleteBoard(ctx, board.ID); err != nil {
		return err
	}
	s.bumpWorkspace(ctx, workspaceID)
	s.hub.BroadcastToRoom(boardRoom(workspaceID), &ws.Message{
		Type: eventBoardDeleted,
		Data: map[string]string{"id": board.ID},
	})
	return nil
}

func (s *Service) SetMain(ctx context.Context, actorID, workspaceID, boardID string) (*dto.BoardResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	if err := validate.UUID(boardID); err != nil {
		return nil, err
	}
	member, err := s.memberOf(ctx, actorID, workspaceID)
	if err != nil {
		return nil, err
	}
	if member.Role != models.RoleOwner {
		return nil, pkgerrors.ErrForbidden
	}
	board, err := s.boardInWorkspace(ctx, workspaceID, boardID)
	if err != nil {
		return nil, err
	}
	if err := s.boards.ClearMainBoards(ctx, workspaceID); err != nil {
		return nil, err
	}
	board.IsMain = true
	if err := s.boards.UpdateBoard(ctx, board); err != nil {
		return nil, err
	}
	response := boardToResponse(board)
	s.bumpWorkspace(ctx, workspaceID)
	s.hub.BroadcastToRoom(boardRoom(workspaceID), &ws.Message{Type: eventBoardUpdated, Data: response})
	return response, nil
}

func (s *Service) Reorder(ctx context.Context, actorID, workspaceID string, req dto.ReorderBoardsRequest) ([]dto.BoardResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	member, err := s.memberOf(ctx, actorID, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.requireEditor(member.Role); err != nil {
		return nil, err
	}
	boards, err := s.boards.ListBoards(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	valid := make(map[string]bool, len(boards))
	for _, b := range boards {
		valid[b.ID] = true
	}
	if len(req.BoardIDs) == 0 {
		return nil, pkgerrors.NewValidationError("board_ids is required")
	}
	seen := make(map[string]bool, len(req.BoardIDs))
	for _, id := range req.BoardIDs {
		if !valid[id] {
			return nil, pkgerrors.NewValidationError("board_ids contains a board not in this workspace")
		}
		if seen[id] {
			return nil, pkgerrors.NewValidationError("board_ids contains duplicates")
		}
		seen[id] = true
	}
	if err := s.boards.ReorderBoards(ctx, workspaceID, req.BoardIDs); err != nil {
		return nil, err
	}
	counts, err := s.boards.CountTasksByBoard(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	ordered := make([]dto.BoardResponse, 0, len(req.BoardIDs))
	for _, id := range req.BoardIDs {
		for _, b := range boards {
			if b.ID == id {
				resp := boardToResponse(b)
				resp.Position = len(ordered)
				resp.TaskCount = counts[id]
				ordered = append(ordered, *resp)
				break
			}
		}
	}
	s.bumpWorkspace(ctx, workspaceID)
	s.hub.BroadcastToRoom(boardRoom(workspaceID), &ws.Message{Type: eventBoardsReordered, Data: ordered})
	return ordered, nil
}

func (s *Service) memberOf(ctx context.Context, actorID, workspaceID string) (*models.WorkspaceMember, error) {
	member, err := s.workspaces.FindMember(ctx, workspaceID, actorID)
	if err != nil {
		if errors.Is(err, pkgerrors.ErrNotFound) {
			return nil, pkgerrors.ErrForbidden
		}
		return nil, err
	}
	return member, nil
}

func (s *Service) requireEditor(role models.WorkspaceRole) error {
	if role != models.RoleOwner && role != models.RoleEditor {
		return pkgerrors.ErrForbidden
	}
	return nil
}

func (s *Service) boardInWorkspace(ctx context.Context, workspaceID, boardID string) (*models.Board, error) {
	board, err := s.boards.FindBoardByID(ctx, boardID)
	if err != nil {
		return nil, err
	}
	if board.WorkspaceID != workspaceID {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, errors.New("board not in workspace"))
	}
	return board, nil
}

func (s *Service) resolveUsers(ctx context.Context, tasks []*models.Task) (map[string]*dto.TaskUserResponse, error) {
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
	result := make(map[string]*dto.TaskUserResponse)
	for id := range ids {
		user, err := s.users.FindByID(ctx, id)
		if err != nil {
			if errors.Is(err, pkgerrors.ErrNotFound) {
				continue
			}
			return nil, err
		}
		result[id] = &dto.TaskUserResponse{
			ID:          user.ID,
			Login:       user.Login,
			DisplayName: user.DisplayName,
			AvatarKey:   user.AvatarKey,
		}
	}
	return result, nil
}

func boardRoom(workspaceID string) string {
	return "workspace:" + workspaceID
}

func boardToResponse(board *models.Board) *dto.BoardResponse {
	return &dto.BoardResponse{
		ID:          board.ID,
		WorkspaceID: board.WorkspaceID,
		Name:        board.Name,
		Position:    board.Position,
		IsMain:      board.IsMain,
		CreatedAt:   board.CreatedAt,
		UpdatedAt:   board.UpdatedAt,
	}
}

func taskToResponse(task *models.Task, users map[string]*dto.TaskUserResponse, prefix, boardID, boardName, columnName string) dto.TaskResponse {
	short := task.ID
	if len(short) > 8 {
		short = short[:8]
	}
	if prefix == "" {
		prefix = "T"
	}
	return dto.TaskResponse{
		ID:          task.ID,
		DisplayID:   prefix + "-" + short,
		BoardID:     boardID,
		BoardName:   boardName,
		ColumnID:    task.ColumnID,
		ColumnName:  columnName,
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

var _ UseCase = (*Service)(nil)
