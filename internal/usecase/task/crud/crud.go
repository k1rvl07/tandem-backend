package crud

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	mtask "github.com/tandem/tandem/internal/domain/models/task"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	dtask "github.com/tandem/tandem/internal/http/dto/task"
	"github.com/tandem/tandem/internal/pkg/validate"
	cacheutil "github.com/tandem/tandem/internal/usecase/shared/cache"
	"github.com/tandem/tandem/internal/usecase/shared/errutil"
	"github.com/tandem/tandem/internal/usecase/shared/taskmap"
	"github.com/tandem/tandem/internal/usecase/task/core"
	"github.com/tandem/tandem/internal/usecase/task/mutate"
	"github.com/tandem/tandem/internal/usecase/task/ordering"
)

type Crud struct {
	*core.Core
	*mutate.Mutate
	*ordering.Ordering
}

func New(c *core.Core) *Crud {
	return &Crud{Core: c, Mutate: mutate.New(c), Ordering: ordering.New(c)}
}

func (s *Crud) Create(ctx context.Context, actorID, workspaceID, boardID string, req dtask.CreateTaskRequest) (*dtask.TaskResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	if err := validate.UUID(boardID); err != nil {
		return nil, err
	}
	if err := validate.UUID(req.ColumnID); err != nil {
		return nil, err
	}
	if _, err := s.MemberOf(ctx, actorID, workspaceID); err != nil {
		return nil, err
	}
	if _, err := s.BoardInWorkspace(ctx, workspaceID, boardID); err != nil {
		return nil, err
	}
	if _, err := s.ColumnInBoard(ctx, boardID, req.ColumnID); err != nil {
		return nil, err
	}
	if err := s.ValidatePayload(req.Title, req.Description); err != nil {
		return nil, err
	}
	dueDate, err := s.ParseDueDate(req.DueDate)
	if err != nil {
		return nil, err
	}
	assigneeID, err := s.ValidateAssignee(ctx, workspaceID, strings.TrimSpace(req.AssigneeID))
	if err != nil {
		return nil, err
	}
	curatorID, err := s.ValidateCurator(ctx, workspaceID, strings.TrimSpace(req.CuratorID))
	if err != nil {
		return nil, err
	}
	parentID, err := s.ValidateParent(ctx, workspaceID, strings.TrimSpace(req.ParentID), "")
	if err != nil {
		return nil, err
	}
	columnTasks, err := s.Tasks.ListTasksForColumn(ctx, req.ColumnID)
	if err != nil {
		return nil, err
	}
	if err := s.ShiftPositions(ctx, columnTasks); err != nil {
		return nil, err
	}

	imageKey := strings.TrimSpace(req.ImageKey)
	if err := s.Files.ValidateImageKey(imageKey, actorID); err != nil {
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
	if err := s.Tasks.CreateTask(ctx, task); err != nil {
		return nil, err
	}
	response, err := s.ResponseFor(ctx, workspaceID, task)
	if err != nil {
		return nil, err
	}
	s.BumpWorkspace(ctx, workspaceID)
	s.Hub.BroadcastToRoom(s.Room(workspaceID), &ws.Message{Type: core.EventTaskCreated, Data: response})
	return response, nil
}

func (s *Crud) Update(ctx context.Context, actorID, workspaceID, boardID, taskID string, req dtask.UpdateTaskRequest) (*dtask.TaskResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	if err := validate.UUID(boardID); err != nil {
		return nil, err
	}
	if err := validate.UUID(taskID); err != nil {
		return nil, err
	}
	if _, err := s.MemberOf(ctx, actorID, workspaceID); err != nil {
		return nil, err
	}
	if _, err := s.BoardInWorkspace(ctx, workspaceID, boardID); err != nil {
		return nil, err
	}
	task, err := s.TaskInBoard(ctx, boardID, taskID)
	if err != nil {
		return nil, err
	}

	oldImageKey, err := s.ApplyUpdateFields(ctx, actorID, workspaceID, task, req)
	if err != nil {
		return nil, err
	}
	if err := s.ApplyTaskMoves(ctx, workspaceID, boardID, task, req); err != nil {
		return nil, err
	}
	if err := s.Tasks.UpdateTask(ctx, task); err != nil {
		return nil, err
	}
	if oldImageKey != "" && oldImageKey != task.ImageKey {
		s.Files.RemoveMany(ctx, []string{oldImageKey})
	}
	return s.finishUpdate(ctx, workspaceID, task)
}

func (s *Crud) finishUpdate(ctx context.Context, workspaceID string, task *mtask.Task) (*dtask.TaskResponse, error) {
	response, err := s.ResponseFor(ctx, workspaceID, task)
	if err != nil {
		return nil, err
	}
	s.BumpWorkspace(ctx, workspaceID)
	s.Hub.BroadcastToRoom(s.Room(workspaceID), &ws.Message{Type: core.EventTaskUpdated, Data: response})
	return response, nil
}

func (s *Crud) Delete(ctx context.Context, actorID, workspaceID, boardID, taskID string) error {
	if err := validate.UUID(workspaceID); err != nil {
		return err
	}
	if err := validate.UUID(boardID); err != nil {
		return err
	}
	if err := validate.UUID(taskID); err != nil {
		return err
	}
	if _, err := s.MemberOf(ctx, actorID, workspaceID); err != nil {
		return err
	}
	if _, err := s.BoardInWorkspace(ctx, workspaceID, boardID); err != nil {
		return err
	}
	task, err := s.TaskInBoard(ctx, boardID, taskID)
	if err != nil {
		return err
	}
	keys, err := s.Tasks.CollectTaskKeys(ctx, task.ID)
	if err != nil {
		return err
	}
	s.Files.RemoveMany(ctx, keys)
	if err := s.Tasks.DeleteTask(ctx, task.ID); err != nil {
		return err
	}
	s.BumpWorkspace(ctx, workspaceID)
	s.Hub.BroadcastToRoom(s.Room(workspaceID), &ws.Message{
		Type: core.EventTaskDeleted,
		Data: map[string]string{"id": task.ID},
	})
	return nil
}

func (s *Crud) Get(ctx context.Context, actorID, workspaceID, taskID string) (*dtask.TaskDetailResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	if err := validate.UUID(taskID); err != nil {
		return nil, err
	}
	if _, err := s.MemberOf(ctx, actorID, workspaceID); err != nil {
		return nil, err
	}
	if _, err := s.WorkspaceTask(ctx, workspaceID, taskID); err != nil {
		return nil, err
	}
	wsver := cacheutil.Version(ctx, s.Cache, cacheutil.WSVerKey+workspaceID)
	uver := cacheutil.Version(ctx, s.Cache, cacheutil.UVerKey+actorID)
	detailKey := fmt.Sprintf("u:%s:t:v1:task:%s:%s:%s", actorID, taskID, wsver, uver)
	var cached dtask.TaskDetailResponse
	if cacheutil.Load(ctx, s.Cache, detailKey, &cached) {
		return &cached, nil
	}
	task, err := s.Tasks.FindTaskByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	response, err := s.ResponseFor(ctx, workspaceID, task)
	if err != nil {
		return nil, err
	}
	detail := &dtask.TaskDetailResponse{TaskResponse: *response}
	if task.ParentID != "" {
		parent, err := s.Tasks.FindTaskByID(ctx, task.ParentID)
		if err != nil {
			if !errutil.IsNotFound(err) {
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
	children, err := s.Tasks.ListChildTasks(ctx, task.ID)
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
	cacheutil.Store(ctx, s.Cache, detailKey, detail, cacheutil.TTL)
	return detail, nil
}

func (s *Crud) List(ctx context.Context, actorID, workspaceID string, query dtask.ListWorkspaceTasksQuery) ([]dtask.TaskResponse, error) {
	if err := validate.UUID(workspaceID); err != nil {
		return nil, err
	}
	if _, err := s.MemberOf(ctx, actorID, workspaceID); err != nil {
		return nil, err
	}
	wsver := cacheutil.Version(ctx, s.Cache, cacheutil.WSVerKey+workspaceID)
	uver := cacheutil.Version(ctx, s.Cache, cacheutil.UVerKey+actorID)
	limit, offset := s.NormalizePagination(query.Limit, query.Offset)
	queryHash := cacheutil.QueryHash(query.Q, query.BoardID, query.AssigneeID, query.Status, query.Only, strconv.FormatBool(query.ExcludeSubtasks), strconv.Itoa(limit), strconv.Itoa(offset))
	listKey := fmt.Sprintf("u:%s:t:v1:tasks:%s:%s:%s:%s", actorID, workspaceID, wsver, uver, queryHash)
	var cached []dtask.TaskResponse
	if cacheutil.Load(ctx, s.Cache, listKey, &cached) {
		return cached, nil
	}
	tasks, err := s.Tasks.ListTasksForWorkspace(ctx, workspaceID)
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
	responses, err := s.ResponsesFor(ctx, workspaceID, filtered)
	if err != nil {
		return nil, err
	}
	cacheutil.Store(ctx, s.Cache, listKey, responses, cacheutil.TTL)
	return responses, nil
}

func (s *Crud) buildColumnInfo(ctx context.Context, workspaceID string) (map[string]core.WorkspaceColumn, error) {
	columns, err := s.Columns.ListColumnsForWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	columnInfo := make(map[string]core.WorkspaceColumn, len(columns))
	for i := range columns {
		columnInfo[columns[i].ID] = core.WorkspaceColumn{Name: columns[i].Name, BoardID: columns[i].BoardID}
	}
	return columnInfo, nil
}

func (s *Crud) filterTasks(tasks []*mtask.Task, columnInfo map[string]core.WorkspaceColumn, query dtask.ListWorkspaceTasksQuery, actorID string) []*mtask.Task {
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

func matchesTaskQuery(task *mtask.Task, info core.WorkspaceColumn, query dtask.ListWorkspaceTasksQuery, actorID string) bool {
	if !strings.Contains(strings.ToLower(task.Title), strings.ToLower(query.Q)) {
		return false
	}
	if query.BoardID != "" && info.BoardID != query.BoardID {
		return false
	}
	if query.AssigneeID != "" && task.AssigneeID != query.AssigneeID {
		return false
	}
	if query.Status != "" && !strings.EqualFold(strings.TrimSpace(query.Status), info.Name) {
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

func (s *Crud) taskReference(ctx context.Context, task *mtask.Task) (*dtask.TaskReference, error) {
	column, err := s.Columns.FindColumnByID(ctx, task.ColumnID)
	if err != nil {
		return nil, err
	}
	board, err := s.Boards.FindBoardByID(ctx, column.BoardID)
	if err != nil {
		return nil, err
	}
	workspace, err := s.Workspaces.FindWorkspaceByID(ctx, board.WorkspaceID)
	if err != nil {
		return nil, err
	}
	return &dtask.TaskReference{
		ID:          task.ID,
		DisplayID:   taskmap.DisplayID(workspace.Prefix, task.ID),
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
