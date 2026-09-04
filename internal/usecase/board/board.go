package board

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/tandem/tandem/internal/domain/models"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	"github.com/tandem/tandem/internal/http/dto"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
)

const (
	eventBoardCreated = "board.created"
	eventBoardUpdated = "board.updated"
	eventBoardDeleted = "board.deleted"
)

var defaultColumns = []string{"Backlog", "To Do", "In Progress", "Done"}

type UseCase interface {
	Create(ctx context.Context, actorID, workspaceID string, req dto.CreateBoardRequest) (*dto.BoardResponse, error)
	List(ctx context.Context, actorID, workspaceID string) ([]dto.BoardResponse, error)
	Get(ctx context.Context, actorID, workspaceID, boardID string) (*dto.BoardDetailResponse, error)
	Update(ctx context.Context, actorID, workspaceID, boardID string, req dto.UpdateBoardRequest) (*dto.BoardResponse, error)
	Delete(ctx context.Context, actorID, workspaceID, boardID string) error
}

type Service struct {
	boards     repository.BoardRepository
	columns    repository.ColumnRepository
	tasks      repository.TaskRepository
	workspaces repository.WorkspaceRepository
	users      repository.UserRepository
	hub        ws.Hub
}

func NewService(
	boards repository.BoardRepository,
	columns repository.ColumnRepository,
	tasks repository.TaskRepository,
	workspaces repository.WorkspaceRepository,
	users repository.UserRepository,
	hub ws.Hub,
) *Service {
	return &Service{
		boards:     boards,
		columns:    columns,
		tasks:      tasks,
		workspaces: workspaces,
		users:      users,
		hub:        hub,
	}
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
	boards, err := s.boards.ListBoards(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	responses := make([]dto.BoardResponse, 0, len(boards))
	for i := range boards {
		responses = append(responses, *boardToResponse(boards[i]))
	}
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
	columns, err := s.columns.ListColumns(ctx, board.ID)
	if err != nil {
		return nil, err
	}
	tasks, err := s.tasks.ListTasksForBoard(ctx, board.ID)
	if err != nil {
		return nil, err
	}
	assignees, err := s.resolveAssignees(ctx, tasks)
	if err != nil {
		return nil, err
	}

	columnDetails := make([]dto.ColumnDetailResponse, 0, len(columns))
	for i := range columns {
		columnTasks := make([]dto.TaskResponse, 0)
		for _, task := range tasks {
			if task.ColumnID == columns[i].ID {
				columnTasks = append(columnTasks, *taskToResponse(task, assignees[task.AssigneeID]))
			}
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
	return &dto.BoardDetailResponse{
		ID:          board.ID,
		WorkspaceID: board.WorkspaceID,
		Name:        board.Name,
		Position:    board.Position,
		Columns:     columnDetails,
		CreatedAt:   board.CreatedAt,
		UpdatedAt:   board.UpdatedAt,
	}, nil
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
	if err := s.boards.DeleteBoard(ctx, board.ID); err != nil {
		return err
	}
	s.hub.BroadcastToRoom(boardRoom(workspaceID), &ws.Message{
		Type: eventBoardDeleted,
		Data: map[string]string{"id": board.ID},
	})
	return nil
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

func (s *Service) resolveAssignees(ctx context.Context, tasks []*models.Task) (map[string]*dto.TaskAssigneeResponse, error) {
	assignees := make(map[string]*dto.TaskAssigneeResponse)
	seen := make(map[string]bool)
	for _, task := range tasks {
		if task.AssigneeID == "" || seen[task.AssigneeID] {
			continue
		}
		seen[task.AssigneeID] = true
		user, err := s.users.FindByID(ctx, task.AssigneeID)
		if err != nil {
			if errors.Is(err, pkgerrors.ErrNotFound) {
				continue
			}
			return nil, err
		}
		assignees[user.ID] = &dto.TaskAssigneeResponse{
			ID:          user.ID,
			Login:       user.Login,
			DisplayName: user.DisplayName,
			AvatarKey:   user.AvatarKey,
		}
	}
	return assignees, nil
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
		CreatedAt:   board.CreatedAt,
		UpdatedAt:   board.UpdatedAt,
	}
}

func taskToResponse(task *models.Task, assignee *dto.TaskAssigneeResponse) *dto.TaskResponse {
	return &dto.TaskResponse{
		ID:          task.ID,
		ColumnID:    task.ColumnID,
		Title:       task.Title,
		Description: task.Description,
		Priority:    task.Priority,
		Assignee:    assignee,
		DueDate:     task.DueDate,
		Position:    task.Position,
		CreatedAt:   task.CreatedAt,
		UpdatedAt:   task.UpdatedAt,
	}
}

var _ UseCase = (*Service)(nil)
