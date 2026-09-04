package column

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
	eventColumnCreated = "column.created"
	eventColumnUpdated = "column.updated"
	eventColumnDeleted = "column.deleted"
)

type UseCase interface {
	Create(ctx context.Context, actorID, workspaceID, boardID string, req dto.CreateColumnRequest) (*dto.ColumnDetailResponse, error)
	Update(ctx context.Context, actorID, workspaceID, boardID, columnID string, req dto.UpdateColumnRequest) (*dto.ColumnDetailResponse, error)
	Delete(ctx context.Context, actorID, workspaceID, boardID, columnID string) error
}

type Service struct {
	columns    repository.ColumnRepository
	boards     repository.BoardRepository
	workspaces repository.WorkspaceRepository
	hub        ws.Hub
}

func NewService(
	columns repository.ColumnRepository,
	boards repository.BoardRepository,
	workspaces repository.WorkspaceRepository,
	hub ws.Hub,
) *Service {
	return &Service{
		columns:    columns,
		boards:     boards,
		workspaces: workspaces,
		hub:        hub,
	}
}

func (s *Service) Create(ctx context.Context, actorID, workspaceID, boardID string, req dto.CreateColumnRequest) (*dto.ColumnDetailResponse, error) {
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
	if _, err := s.boardInWorkspace(ctx, workspaceID, boardID); err != nil {
		return nil, err
	}
	existing, err := s.columns.ListColumns(ctx, boardID)
	if err != nil {
		return nil, err
	}
	column := &models.Column{
		ID:       uuid.New().String(),
		BoardID:  boardID,
		Name:     strings.TrimSpace(req.Name),
		Position: len(existing),
	}
	if err := s.columns.CreateColumn(ctx, column); err != nil {
		return nil, err
	}
	response := columnToDetailResponse(column, nil)
	s.hub.BroadcastToRoom(boardRoom(workspaceID), &ws.Message{Type: eventColumnCreated, Data: response})
	return response, nil
}

func (s *Service) Update(ctx context.Context, actorID, workspaceID, boardID, columnID string, req dto.UpdateColumnRequest) (*dto.ColumnDetailResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	if err := validate.UUID(boardID); err != nil {
		return nil, err
	}
	if err := validate.UUID(columnID); err != nil {
		return nil, err
	}
	member, err := s.memberOf(ctx, actorID, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.requireEditor(member.Role); err != nil {
		return nil, err
	}
	if _, err := s.boardInWorkspace(ctx, workspaceID, boardID); err != nil {
		return nil, err
	}
	column, err := s.columnInBoard(ctx, boardID, columnID)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		if err := validate.Name(*req.Name); err != nil {
			return nil, err
		}
		column.Name = strings.TrimSpace(*req.Name)
	}
	if req.Position != nil {
		if *req.Position < 0 || *req.Position > 999 {
			return nil, pkgerrors.NewValidationError("position must be between 0 and 999")
		}
		if err := s.reorderColumns(ctx, boardID, column, *req.Position); err != nil {
			return nil, err
		}
	}
	if err := s.columns.UpdateColumn(ctx, column); err != nil {
		return nil, err
	}
	response := columnToDetailResponse(column, nil)
	s.hub.BroadcastToRoom(boardRoom(workspaceID), &ws.Message{Type: eventColumnUpdated, Data: response})
	return response, nil
}

func (s *Service) Delete(ctx context.Context, actorID, workspaceID, boardID, columnID string) error {
	if err := validate.UUID(workspaceID); err != nil {
		return err
	}
	if err := validate.UUID(boardID); err != nil {
		return err
	}
	if err := validate.UUID(columnID); err != nil {
		return err
	}
	member, err := s.memberOf(ctx, actorID, workspaceID)
	if err != nil {
		return err
	}
	if err := s.requireEditor(member.Role); err != nil {
		return err
	}
	if _, err := s.boardInWorkspace(ctx, workspaceID, boardID); err != nil {
		return err
	}
	column, err := s.columnInBoard(ctx, boardID, columnID)
	if err != nil {
		return err
	}
	if err := s.columns.DeleteColumn(ctx, column.ID); err != nil {
		return err
	}
	s.hub.BroadcastToRoom(boardRoom(workspaceID), &ws.Message{
		Type: eventColumnDeleted,
		Data: map[string]string{"id": column.ID},
	})
	return nil
}

func (s *Service) reorderColumns(ctx context.Context, boardID string, moved *models.Column, position int) error {
	columns, err := s.columns.ListColumns(ctx, boardID)
	if err != nil {
		return err
	}
	order := make([]*models.Column, 0, len(columns))
	for _, c := range columns {
		if c.ID != moved.ID {
			order = append(order, c)
		}
	}
	target := position
	if target > len(order) {
		target = len(order)
	}
	order = append(order[:target], append([]*models.Column{moved}, order[target:]...)...)
	for i, c := range order {
		if c.Position == i {
			continue
		}
		c.Position = i
		if err := s.columns.UpdateColumn(ctx, c); err != nil {
			return err
		}
	}
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

func (s *Service) columnInBoard(ctx context.Context, boardID, columnID string) (*models.Column, error) {
	column, err := s.columns.FindColumnByID(ctx, columnID)
	if err != nil {
		return nil, err
	}
	if column.BoardID != boardID {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, errors.New("column not in board"))
	}
	return column, nil
}

func boardRoom(workspaceID string) string {
	return "workspace:" + workspaceID
}

func columnToDetailResponse(column *models.Column, tasks []dto.TaskResponse) *dto.ColumnDetailResponse {
	if tasks == nil {
		tasks = []dto.TaskResponse{}
	}
	return &dto.ColumnDetailResponse{
		ID:        column.ID,
		BoardID:   column.BoardID,
		Name:      column.Name,
		Position:  column.Position,
		TaskCount: len(tasks),
		Tasks:     tasks,
		CreatedAt: column.CreatedAt,
		UpdatedAt: column.UpdatedAt,
	}
}

var _ UseCase = (*Service)(nil)
