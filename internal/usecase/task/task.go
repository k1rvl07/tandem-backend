package task

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	mboard "github.com/tandem/tandem/internal/domain/models/board"
	mcolumn "github.com/tandem/tandem/internal/domain/models/column"
	mtask "github.com/tandem/tandem/internal/domain/models/task"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	"github.com/tandem/tandem/internal/domain/ports/cache"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	dtask "github.com/tandem/tandem/internal/http/dto/task"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/pkg/validate"
	"github.com/tandem/tandem/internal/usecase/cacheutil"
	"github.com/tandem/tandem/internal/usecase/common"
	file "github.com/tandem/tandem/internal/usecase/file"
)

const (
	eventTaskCreated = "task.created"
	eventTaskUpdated = "task.updated"
	eventTaskDeleted = "task.deleted"

	maxDescriptionLen = 1500
)

type UseCase interface {
	Create(ctx context.Context, actorID, workspaceID, boardID string, req dtask.CreateTaskRequest) (*dtask.TaskResponse, error)
	Update(ctx context.Context, actorID, workspaceID, boardID, taskID string, req dtask.UpdateTaskRequest) (*dtask.TaskResponse, error)
	Delete(ctx context.Context, actorID, workspaceID, boardID, taskID string) error
	Get(ctx context.Context, actorID, workspaceID, taskID string) (*dtask.TaskDetailResponse, error)
	List(ctx context.Context, actorID, workspaceID string, query dtask.ListWorkspaceTasksQuery) ([]dtask.TaskResponse, error)
}

type Service struct {
	tasks      repository.TaskRepository
	columns    repository.ColumnRepository
	boards     repository.BoardRepository
	workspaces repository.WorkspaceRepository
	users      repository.UserRepository
	files      *file.Service
	hub        ws.Hub
	cache      cache.Cache
}

type Deps struct {
	Tasks      repository.TaskRepository
	Columns    repository.ColumnRepository
	Boards     repository.BoardRepository
	Workspaces repository.WorkspaceRepository
	Users      repository.UserRepository
	Files      *file.Service
	Hub        ws.Hub
	Cache      cache.Cache
}

func NewService(deps Deps) *Service {
	return &Service{
		tasks:      deps.Tasks,
		columns:    deps.Columns,
		boards:     deps.Boards,
		workspaces: deps.Workspaces,
		users:      deps.Users,
		files:      deps.Files,
		hub:        deps.Hub,
		cache:      deps.Cache,
	}
}

func (s *Service) bumpWorkspace(ctx context.Context, workspaceID string) {
	common.BumpWorkspace(ctx, s.cache, workspaceID)
}

func (s *Service) Create(ctx context.Context, actorID, workspaceID, boardID string, req dtask.CreateTaskRequest) (*dtask.TaskResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	if err := validate.UUID(boardID); err != nil {
		return nil, err
	}
	if err := validate.UUID(req.ColumnID); err != nil {
		return nil, err
	}
	if _, err := s.memberOf(ctx, actorID, workspaceID); err != nil {
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
	dueDate, err := parseDueDate(req.DueDate)
	if err != nil {
		return nil, err
	}
	assigneeID, err := s.validateAssignee(ctx, workspaceID, strings.TrimSpace(req.AssigneeID))
	if err != nil {
		return nil, err
	}
	curatorID, err := s.validateCurator(ctx, workspaceID, strings.TrimSpace(req.CuratorID))
	if err != nil {
		return nil, err
	}
	parentID, err := s.validateParent(ctx, workspaceID, strings.TrimSpace(req.ParentID), "")
	if err != nil {
		return nil, err
	}
	columnTasks, err := s.tasks.ListTasksForColumn(ctx, req.ColumnID)
	if err != nil {
		return nil, err
	}
	if err := s.shiftPositions(ctx, columnTasks); err != nil {
		return nil, err
	}

	imageKey := strings.TrimSpace(req.ImageKey)
	if err := s.files.ValidateImageKey(imageKey, actorID); err != nil {
		return nil, err
	}

	task := &mtask.Task{
		ID:          uuid.New().String(),
		ColumnID:    req.ColumnID,
		Title:       strings.TrimSpace(req.Title),
		Description: strings.TrimSpace(req.Description),
		AuthorID:    actorID,
		AssigneeID:  assigneeID,
		CuratorID:   curatorID,
		ParentID:    parentID,
		DueDate:     dueDate,
		Position:    0,
		IsUrgent:    req.IsUrgent,
		IsHidden:    req.IsHidden,
		ImageKey:    imageKey,
	}
	if err := s.tasks.CreateTask(ctx, task); err != nil {
		return nil, err
	}
	response, err := s.responseFor(ctx, workspaceID, task)
	if err != nil {
		return nil, err
	}
	s.bumpWorkspace(ctx, workspaceID)
	s.hub.BroadcastToRoom(boardRoom(workspaceID), &ws.Message{Type: eventTaskCreated, Data: response})
	return response, nil
}

func (s *Service) Update(ctx context.Context, actorID, workspaceID, boardID, taskID string, req dtask.UpdateTaskRequest) (*dtask.TaskResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	if err := validate.UUID(boardID); err != nil {
		return nil, err
	}
	if err := validate.UUID(taskID); err != nil {
		return nil, err
	}
	if _, err := s.memberOf(ctx, actorID, workspaceID); err != nil {
		return nil, err
	}
	if _, err := s.boardInWorkspace(ctx, workspaceID, boardID); err != nil {
		return nil, err
	}
	task, err := s.taskInBoard(ctx, boardID, taskID)
	if err != nil {
		return nil, err
	}

	oldImageKey, err := s.applyUpdateFields(ctx, actorID, workspaceID, task, req)
	if err != nil {
		return nil, err
	}
	if err := s.applyTaskMoves(ctx, workspaceID, boardID, task, req); err != nil {
		return nil, err
	}
	if err := s.tasks.UpdateTask(ctx, task); err != nil {
		return nil, err
	}
	if oldImageKey != "" && oldImageKey != task.ImageKey {
		s.files.RemoveMany(ctx, []string{oldImageKey})
	}
	return s.finishUpdate(ctx, workspaceID, task)
}

func (s *Service) applyUpdateFields(ctx context.Context, actorID, workspaceID string, task *mtask.Task, req dtask.UpdateTaskRequest) (string, error) {
	oldImageKey := task.ImageKey
	if req.Title != nil {
		if err := validate.Title(*req.Title); err != nil {
			return "", err
		}
		next := strings.TrimSpace(*req.Title)
		task.Title = next
	}
	if req.Description != nil {
		if utf8.RuneCountInString(strings.TrimSpace(*req.Description)) > maxDescriptionLen {
			return "", pkgerrors.NewValidationError("description must be at most %d characters", maxDescriptionLen)
		}
		next := strings.TrimSpace(*req.Description)
		task.Description = next
	}
	if req.DueDate != nil {
		dueDate, err := parseDueDate(*req.DueDate)
		if err != nil {
			return "", err
		}
		task.DueDate = dueDate
	}
	if req.AssigneeID != nil {
		assigneeID, err := s.validateAssignee(ctx, workspaceID, strings.TrimSpace(*req.AssigneeID))
		if err != nil {
			return "", err
		}
		task.AssigneeID = assigneeID
	}
	if req.CuratorID != nil {
		curatorID, err := s.validateCurator(ctx, workspaceID, strings.TrimSpace(*req.CuratorID))
		if err != nil {
			return "", err
		}
		task.CuratorID = curatorID
	}
	if req.ParentID != nil {
		parentID, err := s.validateParent(ctx, workspaceID, strings.TrimSpace(*req.ParentID), task.ID)
		if err != nil {
			return "", err
		}
		task.ParentID = parentID
	}
	if req.IsUrgent != nil {
		task.IsUrgent = *req.IsUrgent
	}
	if req.IsHidden != nil {
		task.IsHidden = *req.IsHidden
	}
	if req.ImageKey != nil {
		key := strings.TrimSpace(*req.ImageKey)
		if err := s.files.ValidateImageKey(key, actorID); err != nil {
			return "", err
		}
		task.ImageKey = key
	}
	return oldImageKey, nil
}

func (s *Service) applyTaskMoves(ctx context.Context, workspaceID, boardID string, task *mtask.Task, req dtask.UpdateTaskRequest) error {
	if req.BoardID != nil && *req.BoardID != boardID {
		target, err := s.boardInWorkspace(ctx, workspaceID, *req.BoardID)
		if err != nil {
			return err
		}
		columns, err := s.columns.ListColumns(ctx, target.ID)
		if err != nil {
			return err
		}
		if len(columns) == 0 {
			return pkgerrors.NewValidationError("target board has no columns")
		}
		first := columns[0]
		targetTasks, err := s.tasks.ListTasksForColumn(ctx, first.ID)
		if err != nil {
			return err
		}
		if err := s.leaveColumn(ctx, task); err != nil {
			return err
		}
		task.ColumnID = first.ID
		if err := s.shiftPositions(ctx, targetTasks); err != nil {
			return err
		}
		task.Position = 0
	} else if req.ColumnID != nil || req.Position != nil {
		targetColumnID := task.ColumnID
		if req.ColumnID != nil {
			if err := validate.UUID(*req.ColumnID); err != nil {
				return err
			}
			if _, err := s.columnInBoard(ctx, boardID, *req.ColumnID); err != nil {
				return err
			}
			targetColumnID = *req.ColumnID
		}
		position := -1
		if req.Position != nil {
			position = *req.Position
		}
		if position < 0 {
			position = 0
		}
		shouldMove := targetColumnID != task.ColumnID || req.Position != nil
		if shouldMove {
			if err := s.moveTask(ctx, task, targetColumnID, position); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) finishUpdate(ctx context.Context, workspaceID string, task *mtask.Task) (*dtask.TaskResponse, error) {
	response, err := s.responseFor(ctx, workspaceID, task)
	if err != nil {
		return nil, err
	}
	s.bumpWorkspace(ctx, workspaceID)
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
	if _, err := s.memberOf(ctx, actorID, workspaceID); err != nil {
		return err
	}
	if _, err := s.boardInWorkspace(ctx, workspaceID, boardID); err != nil {
		return err
	}
	task, err := s.taskInBoard(ctx, boardID, taskID)
	if err != nil {
		return err
	}
	keys, err := s.tasks.CollectTaskKeys(ctx, task.ID)
	if err != nil {
		return err
	}
	s.files.RemoveMany(ctx, keys)
	if err := s.tasks.DeleteTask(ctx, task.ID); err != nil {
		return err
	}
	s.bumpWorkspace(ctx, workspaceID)
	s.hub.BroadcastToRoom(boardRoom(workspaceID), &ws.Message{
		Type: eventTaskDeleted,
		Data: map[string]string{"id": task.ID},
	})
	return nil
}

func (s *Service) Get(ctx context.Context, actorID, workspaceID, taskID string) (*dtask.TaskDetailResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	if err := validate.UUID(taskID); err != nil {
		return nil, err
	}
	if _, err := s.memberOf(ctx, actorID, workspaceID); err != nil {
		return nil, err
	}
	if _, err := s.workspaceTask(ctx, workspaceID, taskID); err != nil {
		return nil, err
	}
	wsver := cacheutil.Version(ctx, s.cache, cacheutil.WSVerKey+workspaceID)
	uver := cacheutil.Version(ctx, s.cache, cacheutil.UVerKey+actorID)
	detailKey := fmt.Sprintf("u:%s:t:v1:task:%s:%s:%s", actorID, taskID, wsver, uver)
	var cached dtask.TaskDetailResponse
	if cacheutil.Load(ctx, s.cache, detailKey, &cached) {
		return &cached, nil
	}
	task, err := s.tasks.FindTaskByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	response, err := s.responseFor(ctx, workspaceID, task)
	if err != nil {
		return nil, err
	}
	detail := &dtask.TaskDetailResponse{TaskResponse: *response}
	if task.ParentID != "" {
		parent, err := s.tasks.FindTaskByID(ctx, task.ParentID)
		if err != nil {
			if !common.IsNotFound(err) {
				return nil, err
			}
		} else if parent != nil {
			ref, err := s.taskReference(ctx, parent)
			if err != nil {
				return nil, err
			}
			detail.Parent = ref
		}
	}
	children, err := s.tasks.ListChildTasks(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	subtasks := make([]dtask.TaskReference, 0, len(children))
	for _, child := range children {
		ref, err := s.taskReference(ctx, child)
		if err != nil {
			return nil, err
		}
		subtasks = append(subtasks, *ref)
	}
	detail.Subtasks = subtasks
	cacheutil.Store(ctx, s.cache, detailKey, detail, cacheutil.TTL)
	return detail, nil
}

func (s *Service) List(ctx context.Context, actorID, workspaceID string, query dtask.ListWorkspaceTasksQuery) ([]dtask.TaskResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	if _, err := s.memberOf(ctx, actorID, workspaceID); err != nil {
		return nil, err
	}
	wsver := cacheutil.Version(ctx, s.cache, cacheutil.WSVerKey+workspaceID)
	uver := cacheutil.Version(ctx, s.cache, cacheutil.UVerKey+actorID)
	limit, offset := normalizePagination(query.Limit, query.Offset)
	queryHash := cacheutil.QueryHash(query.Q, query.BoardID, query.AssigneeID, query.Status, query.Only, strconv.FormatBool(query.ExcludeSubtasks), strconv.Itoa(limit), strconv.Itoa(offset))
	listKey := fmt.Sprintf("u:%s:t:v1:tasks:%s:%s:%s:%s", actorID, workspaceID, wsver, uver, queryHash)
	var cached []dtask.TaskResponse
	if cacheutil.Load(ctx, s.cache, listKey, &cached) {
		return cached, nil
	}
	tasks, err := s.tasks.ListTasksForWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	columnInfo, err := s.buildColumnInfo(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	filtered := s.filterTasks(tasks, columnInfo, query, actorID)
	if offset > len(filtered) {
		offset = len(filtered)
	}
	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	filtered = filtered[offset:end]
	responses, err := s.responsesFor(ctx, workspaceID, filtered)
	if err != nil {
		return nil, err
	}
	cacheutil.Store(ctx, s.cache, listKey, responses, cacheutil.TTL)
	return responses, nil
}

func (s *Service) buildColumnInfo(ctx context.Context, workspaceID string) (map[string]workspaceColumn, error) {
	columns, err := s.columns.ListColumnsForWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	columnInfo := make(map[string]workspaceColumn, len(columns))
	for i := range columns {
		columnInfo[columns[i].ID] = workspaceColumn{name: columns[i].Name, boardID: columns[i].BoardID}
	}
	return columnInfo, nil
}

func (s *Service) filterTasks(tasks []*mtask.Task, columnInfo map[string]workspaceColumn, query dtask.ListWorkspaceTasksQuery, actorID string) []*mtask.Task {
	filtered := make([]*mtask.Task, 0, len(tasks))
	for _, task := range tasks {
		info, ok := columnInfo[task.ColumnID]
		if !ok {
			continue
		}
		if matchesTaskQuery(task, info, query, actorID) {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

func matchesTaskQuery(task *mtask.Task, info workspaceColumn, query dtask.ListWorkspaceTasksQuery, actorID string) bool {
	if !strings.Contains(strings.ToLower(task.Title), strings.ToLower(query.Q)) {
		return false
	}
	if query.BoardID != "" && info.boardID != query.BoardID {
		return false
	}
	if query.AssigneeID != "" && task.AssigneeID != query.AssigneeID {
		return false
	}
	if query.Status != "" && !strings.EqualFold(strings.TrimSpace(query.Status), info.name) {
		return false
	}
	if query.ExcludeSubtasks && task.ParentID != "" {
		return false
	}
	switch query.Only {
	case "mine":
		if task.AuthorID != actorID && task.CuratorID != actorID && task.AssigneeID != actorID {
			return false
		}
	case "for_me":
		if task.AssigneeID != actorID {
			return false
		}
	}
	return true
}

func (s *Service) moveTask(ctx context.Context, task *mtask.Task, targetColumnID string, position int) error {
	if task.ColumnID == targetColumnID {
		order, err := s.tasks.ListTasksForColumn(ctx, targetColumnID)
		if err != nil {
			return err
		}
		order = removeTask(order, task.ID)
		order = insertTask(order, position, task)
		return s.rewritePositions(ctx, order)
	}

	if err := s.leaveColumn(ctx, task); err != nil {
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

func (s *Service) leaveColumn(ctx context.Context, task *mtask.Task) error {
	source, err := s.tasks.ListTasksForColumn(ctx, task.ColumnID)
	if err != nil {
		return err
	}
	source = removeTask(source, task.ID)
	return s.rewritePositions(ctx, source)
}

func (s *Service) shiftPositions(ctx context.Context, order []*mtask.Task) error {
	for _, t := range order {
		t.Position++
		if err := s.tasks.UpdateTask(ctx, t); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) rewritePositions(ctx context.Context, order []*mtask.Task) error {
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
		if common.IsNotFound(err) {
			return "", pkgerrors.NewValidationError("assignee must be a workspace member")
		}
		return "", err
	}
	return assigneeID, nil
}

func (s *Service) validateCurator(ctx context.Context, workspaceID, curatorID string) (string, error) {
	if curatorID == "" {
		return "", nil
	}
	if err := validate.UUID(curatorID); err != nil {
		return "", err
	}
	if _, err := s.workspaces.FindMember(ctx, workspaceID, curatorID); err != nil {
		if common.IsNotFound(err) {
			return "", pkgerrors.NewValidationError("curator must be a workspace member")
		}
		return "", err
	}
	return curatorID, nil
}

func (s *Service) validateParent(ctx context.Context, workspaceID, parentID, selfID string) (string, error) {
	if parentID == "" {
		return "", nil
	}
	if err := validate.UUID(parentID); err != nil {
		return "", err
	}
	if selfID != "" && parentID == selfID {
		return "", pkgerrors.NewValidationError("task cannot be its own parent")
	}
	if _, err := s.workspaceTask(ctx, workspaceID, parentID); err != nil {
		return "", err
	}
	seen := map[string]bool{selfID: true}
	current := parentID
	for current != "" {
		parent, err := s.tasks.FindTaskByID(ctx, current)
		if err != nil {
			return "", err
		}
		if seen[parent.ID] {
			return "", pkgerrors.NewValidationError("parent creates a circular dependency")
		}
		seen[parent.ID] = true
		if _, err := s.workspaceTask(ctx, workspaceID, parent.ID); err != nil {
			return "", err
		}
		current = parent.ParentID
	}
	return parentID, nil
}

func (s *Service) memberOf(ctx context.Context, actorID, workspaceID string) (*mworkspace.WorkspaceMember, error) {
	return common.MemberOrForbidden(ctx, s.workspaces, workspaceID, actorID)
}

func (s *Service) boardInWorkspace(ctx context.Context, workspaceID, boardID string) (*mboard.Board, error) {
	return common.BoardInWorkspace(ctx, s.boards, workspaceID, boardID)
}

func (s *Service) columnInBoard(ctx context.Context, boardID, columnID string) (*mcolumn.Column, error) {
	column, err := s.columns.FindColumnByID(ctx, columnID)
	if err != nil {
		return nil, err
	}
	if column.BoardID != boardID {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, errors.New("column not in board"))
	}
	return column, nil
}

func (s *Service) taskInBoard(ctx context.Context, boardID, taskID string) (*mtask.Task, error) {
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

func (s *Service) workspaceTask(ctx context.Context, workspaceID, taskID string) (*mtask.Task, error) {
	task, err := s.tasks.FindTaskByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	column, err := s.columns.FindColumnByID(ctx, task.ColumnID)
	if err != nil {
		return nil, err
	}
	board, err := s.boards.FindBoardByID(ctx, column.BoardID)
	if err != nil {
		return nil, err
	}
	if board.WorkspaceID != workspaceID {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, errors.New("task not in workspace"))
	}
	return task, nil
}

func (s *Service) responseFor(ctx context.Context, workspaceID string, task *mtask.Task) (*dtask.TaskResponse, error) {
	responses, err := s.responsesFor(ctx, workspaceID, []*mtask.Task{task})
	if err != nil {
		return nil, err
	}
	return &responses[0], nil
}

func (s *Service) responsesFor(ctx context.Context, workspaceID string, tasks []*mtask.Task) ([]dtask.TaskResponse, error) {
	if len(tasks) == 0 {
		return []dtask.TaskResponse{}, nil
	}
	workspace, err := s.workspaces.FindWorkspaceByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	columns, err := s.columns.ListColumnsForWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	boards, err := s.boards.ListBoards(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	boardNameByID := make(map[string]string, len(boards))
	for i := range boards {
		boardNameByID[boards[i].ID] = boards[i].Name
	}
	columnInfo := make(map[string]workspaceColumn, len(columns))
	for i := range columns {
		columnInfo[columns[i].ID] = workspaceColumn{name: columns[i].Name, boardID: columns[i].BoardID, boardName: boardNameByID[columns[i].BoardID]}
	}
	users, err := s.resolveUsers(ctx, tasks)
	if err != nil {
		return nil, err
	}
	responses := make([]dtask.TaskResponse, 0, len(tasks))
	for _, task := range tasks {
		info, ok := columnInfo[task.ColumnID]
		if !ok {
			info = workspaceColumn{boardID: ""}
		}
		responses = append(responses, common.BuildTaskResponse(task, users, common.TaskResponseCtx{
			WorkspaceID: workspaceID,
			BoardID:     info.boardID,
			BoardName:   info.boardName,
			ColumnName:  info.name,
			DisplayID:   displayID(workspace.Prefix, task.ID),
		}))
	}
	return responses, nil
}

func (s *Service) taskReference(ctx context.Context, task *mtask.Task) (*dtask.TaskReference, error) {
	column, err := s.columns.FindColumnByID(ctx, task.ColumnID)
	if err != nil {
		return nil, err
	}
	board, err := s.boards.FindBoardByID(ctx, column.BoardID)
	if err != nil {
		return nil, err
	}
	workspace, err := s.workspaces.FindWorkspaceByID(ctx, board.WorkspaceID)
	if err != nil {
		return nil, err
	}
	return &dtask.TaskReference{
		ID:          task.ID,
		DisplayID:   displayID(workspace.Prefix, task.ID),
		Title:       task.Title,
		WorkspaceID: board.WorkspaceID,
		BoardID:     board.ID,
		BoardName:   board.Name,
		ColumnID:    column.ID,
		ColumnName:  column.Name,
		IsUrgent:    task.IsUrgent,
		IsHidden:    task.IsHidden,
	}, nil
}

func (s *Service) resolveUsers(ctx context.Context, tasks []*mtask.Task) (map[string]*dtask.TaskUserResponse, error) {
	return common.ResolveUsers(ctx, s.users, tasks)
}

type workspaceColumn struct {
	name      string
	boardID   string
	boardName string
}

func displayID(prefix, taskID string) string {
	return common.DisplayID(prefix, taskID)
}

func removeTask(tasks []*mtask.Task, id string) []*mtask.Task {
	filtered := make([]*mtask.Task, 0, len(tasks))
	for _, t := range tasks {
		if t.ID != id {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func insertTask(tasks []*mtask.Task, position int, task *mtask.Task) []*mtask.Task {
	if position < 0 || position > len(tasks) {
		position = len(tasks)
	}
	tasks = append(tasks, nil)
	copy(tasks[position+1:], tasks[position:])
	tasks[position] = task
	return tasks
}

func normalizePagination(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
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

var _ UseCase = (*Service)(nil)
