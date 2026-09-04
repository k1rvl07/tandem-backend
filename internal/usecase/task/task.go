package task

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/tandem/tandem/internal/domain/models"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	"github.com/tandem/tandem/internal/http/dto"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
)

const (
	eventTaskCreated = "task.created"
	eventTaskUpdated = "task.updated"
	eventTaskDeleted = "task.deleted"

	maxDescriptionLen = 1500
)

type UseCase interface {
	Create(ctx context.Context, actorID, workspaceID, boardID string, req dto.CreateTaskRequest) (*dto.TaskResponse, error)
	Update(ctx context.Context, actorID, workspaceID, boardID, taskID string, req dto.UpdateTaskRequest) (*dto.TaskResponse, error)
	Delete(ctx context.Context, actorID, workspaceID, boardID, taskID string) error
}

type Service struct {
	tasks      repository.TaskRepository
	columns    repository.ColumnRepository
	boards     repository.BoardRepository
	workspaces repository.WorkspaceRepository
	users      repository.UserRepository
	hub        ws.Hub
}

func NewService(
	tasks repository.TaskRepository,
	columns repository.ColumnRepository,
	boards repository.BoardRepository,
	workspaces repository.WorkspaceRepository,
	users repository.UserRepository,
	hub ws.Hub,
) *Service {
	return &Service{
		tasks:      tasks,
		columns:    columns,
		boards:     boards,
		workspaces: workspaces,
		users:      users,
		hub:        hub,
	}
}

func (s *Service) Create(ctx context.Context, actorID, workspaceID, boardID string, req dto.CreateTaskRequest) (*dto.TaskResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	if err := validate.UUID(boardID); err != nil {
		return nil, err
	}
	if err := validate.UUID(req.ColumnID); err != nil {
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
	if _, err := s.columnInBoard(ctx, boardID, req.ColumnID); err != nil {
		return nil, err
	}
	if err := validate.Title(req.Title); err != nil {
		return nil, err
	}
	if utf8.RuneCountInString(strings.TrimSpace(req.Description)) > maxDescriptionLen {
		return nil, pkgerrors.NewValidationError("description must be at most %d characters", maxDescriptionLen)
	}
	priority, err := normalizePriority(req.Priority)
	if err != nil {
		return nil, err
	}
	dueDate, err := parseDueDate(req.DueDate)
	if err != nil {
		return nil, err
	}
	assigneeID, err := s.validateAssignee(ctx, workspaceID, strings.TrimSpace(req.AssigneeID))
	if err != nil {
		return nil, err
	}
	columnTasks, err := s.tasks.ListTasksForColumn(ctx, req.ColumnID)
	if err != nil {
		return nil, err
	}

	task := &models.Task{
		ID:          uuid.New().String(),
		ColumnID:    req.ColumnID,
		Title:       strings.TrimSpace(req.Title),
		Description: strings.TrimSpace(req.Description),
		AssigneeID:  assigneeID,
		Priority:    priority,
		DueDate:     dueDate,
		Position:    len(columnTasks),
	}
	if err := s.tasks.CreateTask(ctx, task); err != nil {
		return nil, err
	}
	assignee, err := s.resolveAssignee(ctx, assigneeID)
	if err != nil {
		return nil, err
	}
	response := taskToResponse(task, assignee)
	s.hub.BroadcastToRoom(boardRoom(workspaceID), &ws.Message{Type: eventTaskCreated, Data: response})
	return response, nil
}

func (s *Service) Update(ctx context.Context, actorID, workspaceID, boardID, taskID string, req dto.UpdateTaskRequest) (*dto.TaskResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	if err := validate.UUID(boardID); err != nil {
		return nil, err
	}
	if err := validate.UUID(taskID); err != nil {
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
	task, err := s.taskInBoard(ctx, boardID, taskID)
	if err != nil {
		return nil, err
	}

	if req.Title != nil {
		if err := validate.Title(*req.Title); err != nil {
			return nil, err
		}
		task.Title = strings.TrimSpace(*req.Title)
	}
	if req.Description != nil {
		if utf8.RuneCountInString(strings.TrimSpace(*req.Description)) > maxDescriptionLen {
			return nil, pkgerrors.NewValidationError("description must be at most %d characters", maxDescriptionLen)
		}
		task.Description = strings.TrimSpace(*req.Description)
	}
	if req.Priority != nil {
		priority, err := normalizePriority(*req.Priority)
		if err != nil {
			return nil, err
		}
		task.Priority = priority
	}
	if req.DueDate != nil {
		dueDate, err := parseDueDate(*req.DueDate)
		if err != nil {
			return nil, err
		}
		task.DueDate = dueDate
	}
	if req.AssigneeID != nil {
		assigneeID, err := s.validateAssignee(ctx, workspaceID, strings.TrimSpace(*req.AssigneeID))
		if err != nil {
			return nil, err
		}
		task.AssigneeID = assigneeID
	}

	targetColumnID := task.ColumnID
	if req.ColumnID != nil {
		if err := validate.UUID(*req.ColumnID); err != nil {
			return nil, err
		}
		if _, err := s.columnInBoard(ctx, boardID, *req.ColumnID); err != nil {
			return nil, err
		}
		targetColumnID = *req.ColumnID
	}
	if req.ColumnID != nil || req.Position != nil {
		position := -1
		if req.Position != nil {
			position = *req.Position
		}
		if err := s.moveTask(ctx, task, targetColumnID, position); err != nil {
			return nil, err
		}
	}

	if err := s.tasks.UpdateTask(ctx, task); err != nil {
		return nil, err
	}
	assignee, err := s.resolveAssignee(ctx, task.AssigneeID)
	if err != nil {
		return nil, err
	}
	response := taskToResponse(task, assignee)
	s.hub.BroadcastToRoom(boardRoom(workspaceID), &ws.Message{Type: eventTaskUpdated, Data: response})
	return response, nil
}

func (s *Service) Delete(ctx context.Context, actorID, workspaceID, boardID, taskID string) error {
	if err := validate.UUID(workspaceID); err != nil {
		return err
	}
	if err := validate.UUID(boardID); err != nil {
		return err
	}
	if err := validate.UUID(taskID); err != nil {
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
	task, err := s.taskInBoard(ctx, boardID, taskID)
	if err != nil {
		return err
	}
	if err := s.tasks.DeleteTask(ctx, task.ID); err != nil {
		return err
	}
	s.hub.BroadcastToRoom(boardRoom(workspaceID), &ws.Message{
		Type: eventTaskDeleted,
		Data: map[string]string{"id": task.ID},
	})
	return nil
}

func (s *Service) moveTask(ctx context.Context, task *models.Task, targetColumnID string, position int) error {
	if task.ColumnID == targetColumnID {
		order, err := s.tasks.ListTasksForColumn(ctx, targetColumnID)
		if err != nil {
			return err
		}
		order = removeTask(order, task.ID)
		order = insertTask(order, position, task)
		return s.rewritePositions(ctx, order)
	}

	source, err := s.tasks.ListTasksForColumn(ctx, task.ColumnID)
	if err != nil {
		return err
	}
	source = removeTask(source, task.ID)
	if err := s.rewritePositions(ctx, source); err != nil {
		return err
	}

	target, err := s.tasks.ListTasksForColumn(ctx, targetColumnID)
	if err != nil {
		return err
	}
	target = insertTask(target, position, task)
	if err := s.rewritePositions(ctx, target); err != nil {
		return err
	}
	task.ColumnID = targetColumnID
	return nil
}

func (s *Service) rewritePositions(ctx context.Context, order []*models.Task) error {
	for i, t := range order {
		if t.Position == i {
			continue
		}
		t.Position = i
		if err := s.tasks.UpdateTask(ctx, t); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) validateAssignee(ctx context.Context, workspaceID, assigneeID string) (string, error) {
	if assigneeID == "" {
		return "", nil
	}
	if err := validate.UUID(assigneeID); err != nil {
		return "", err
	}
	if _, err := s.workspaces.FindMember(ctx, workspaceID, assigneeID); err != nil {
		if errors.Is(err, pkgerrors.ErrNotFound) {
			return "", pkgerrors.NewValidationError("assignee must be a workspace member")
		}
		return "", err
	}
	return assigneeID, nil
}

func (s *Service) resolveAssignee(ctx context.Context, assigneeID string) (*dto.TaskAssigneeResponse, error) {
	if assigneeID == "" {
		return nil, nil
	}
	user, err := s.users.FindByID(ctx, assigneeID)
	if err != nil {
		if errors.Is(err, pkgerrors.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &dto.TaskAssigneeResponse{
		ID:          user.ID,
		Login:       user.Login,
		DisplayName: user.DisplayName,
		AvatarKey:   user.AvatarKey,
	}, nil
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

func (s *Service) taskInBoard(ctx context.Context, boardID, taskID string) (*models.Task, error) {
	task, err := s.tasks.FindTaskByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	column, err := s.columns.FindColumnByID(ctx, task.ColumnID)
	if err != nil {
		return nil, err
	}
	if column.BoardID != boardID {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, errors.New("task not in board"))
	}
	return task, nil
}

func removeTask(tasks []*models.Task, id string) []*models.Task {
	filtered := make([]*models.Task, 0, len(tasks))
	for _, t := range tasks {
		if t.ID != id {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func insertTask(tasks []*models.Task, position int, task *models.Task) []*models.Task {
	if position < 0 || position > len(tasks) {
		position = len(tasks)
	}
	tasks = append(tasks, nil)
	copy(tasks[position+1:], tasks[position:])
	tasks[position] = task
	return tasks
}

func normalizePriority(priority string) (string, error) {
	switch priority {
	case "", models.PriorityMedium:
		return models.PriorityMedium, nil
	case models.PriorityLow, models.PriorityHigh:
		return priority, nil
	default:
		return "", pkgerrors.NewValidationError("invalid priority")
	}
}

func parseDueDate(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, pkgerrors.NewValidationError("due_date must be in YYYY-MM-DD format")
	}
	return &parsed, nil
}

func boardRoom(workspaceID string) string {
	return "workspace:" + workspaceID
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
