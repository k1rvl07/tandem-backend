package task

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tandem/tandem/internal/domain/models"
	"github.com/tandem/tandem/internal/http/dto"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	file "github.com/tandem/tandem/internal/usecase/file"
	"github.com/tandem/tandem/internal/usecase/testutil"
	"go.uber.org/zap"
)

type noopStore struct{}

func (noopStore) Put(context.Context, string, io.Reader, int64, string) error { return nil }
func (noopStore) Get(context.Context, string) (io.ReadCloser, error)          { return nil, errors.New("noop") }
func (noopStore) Delete(context.Context, string) error                        { return nil }
func (noopStore) Exists(context.Context, string) (bool, error)                { return false, nil }
func (noopStore) PresignGet(context.Context, string, time.Duration) (string, error) {
	return "", nil
}

type env struct {
	svc    *Service
	tasks  *testutil.FakeTaskRepo
	cols   *testutil.FakeColumnRepo
	ws     *testutil.FakeWorkspaceRepo
	users  *testutil.FakeUserRepo
	hub    *testutil.FakeHub
	actorO string
	actorE string
	actorV string
	wsA    string
	boardA string
	boardB string
	colA   string
	colB   string
	colC   string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	boards := testutil.NewFakeBoardRepo()
	cols := testutil.NewFakeColumnRepo()
	tasks := testutil.NewFakeTaskRepo()
	ws := testutil.NewFakeWorkspaceRepo()
	users := testutil.NewFakeUserRepo()
	hub := testutil.NewFakeHub()

	actorO := testutil.NewUUID()
	actorE := testutil.NewUUID()
	actorV := testutil.NewUUID()
	wsA := testutil.NewUUID()
	ws.AddWorkspaceFixture(wsA, "Team A")
	ws.AddMemberFixture(wsA, actorO, models.RoleOwner)
	ws.AddMemberFixture(wsA, actorE, models.RoleEditor)
	ws.AddMemberFixture(wsA, actorV, models.RoleMember)
	boardA := testutil.NewUUID()
	boardB := testutil.NewUUID()
	boards.AddBoardFixture(boardA, wsA, "Sprint")
	boards.AddBoardFixture(boardB, wsA, "Second")
	colA := testutil.NewUUID()
	colB := testutil.NewUUID()
	colC := testutil.NewUUID()
	cols.AddColumnFixture(colA, boardA, "Backlog", 0)
	cols.AddColumnFixture(colB, boardA, "Done", 1)
	cols.AddColumnFixture(colC, boardB, "Backlog", 0)
	tasks.RegisterColumn(colA, boardA)
	tasks.RegisterColumn(colB, boardA)
	tasks.RegisterColumn(colC, boardB)

	return &env{
		svc:    NewService(tasks, cols, boards, ws, users, file.NewService(noopStore{}, zap.NewNop()), hub, testutil.NewFakeCache()),
		tasks:  tasks,
		cols:   cols,
		ws:     ws,
		users:  users,
		hub:    hub,
		actorO: actorO,
		actorE: actorE,
		actorV: actorV,
		wsA:    wsA,
		boardA: boardA,
		boardB: boardB,
		colA:   colA,
		colB:   colB,
		colC:   colC,
	}
}

func TestCreateTask(t *testing.T) {
	e := newEnv(t)
	assignee := e.users.AddUserFixture(testutil.NewUUID(), "dev")
	e.ws.AddMemberFixture(e.wsA, assignee.ID, models.RoleEditor)

	task, err := e.svc.Create(context.Background(), e.actorE, e.wsA, e.boardA, dto.CreateTaskRequest{
		ColumnID:    e.colA,
		Title:       "Implement login",
		Description: "Do it",
		AssigneeID:  assignee.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, "Implement login", task.Title)
	assert.Equal(t, 0, task.Position)
	require.NotNil(t, task.Assignee)
	assert.Equal(t, "dev", task.Assignee.Login)
	assert.Equal(t, eventTaskCreated, e.hub.Messages[0].Type)
}

func TestCreateTaskDefaultPosition(t *testing.T) {
	e := newEnv(t)
	first := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colA, "First", 0)
	task, err := e.svc.Create(context.Background(), e.actorE, e.wsA, e.boardA, dto.CreateTaskRequest{
		ColumnID: e.colA,
		Title:    "Second",
	})
	require.NoError(t, err)
	assert.Equal(t, 0, task.Position)
	stored, _ := e.tasks.FindTaskByID(context.Background(), first.ID)
	assert.Equal(t, 1, stored.Position)
}

func TestCreateTaskDefaultFields(t *testing.T) {
	e := newEnv(t)
	task, err := e.svc.Create(context.Background(), e.actorE, e.wsA, e.boardA, dto.CreateTaskRequest{
		ColumnID: e.colA,
		Title:    "Default fields",
	})
	require.NoError(t, err)
	assert.Nil(t, task.DueDate)
	assert.Nil(t, task.Assignee)
}

func TestCreateTaskMemberAllowed(t *testing.T) {
	e := newEnv(t)
	task, err := e.svc.Create(context.Background(), e.actorV, e.wsA, e.boardA, dto.CreateTaskRequest{
		ColumnID: e.colA,
		Title:    "Create by member",
	})
	require.NoError(t, err)
	assert.Equal(t, "Create by member", task.Title)
}

func TestCreateTaskAssigneeNotMember(t *testing.T) {
	e := newEnv(t)
	outsider := testutil.NewUUID()
	_, err := e.svc.Create(context.Background(), e.actorE, e.wsA, e.boardA, dto.CreateTaskRequest{
		ColumnID:   e.colA,
		Title:      "Assign",
		AssigneeID: outsider,
	})
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
}

func TestCreateTaskColumnFromOtherBoard(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.Create(context.Background(), e.actorE, e.wsA, e.boardA, dto.CreateTaskRequest{
		ColumnID: e.cols.AddColumnFixture(testutil.NewUUID(), e.boardB, "Backlog", 0).ID,
		Title:    "Wrong board",
	})
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
}

func TestUpdateTaskFields(t *testing.T) {
	e := newEnv(t)
	assignee := e.users.AddUserFixture(testutil.NewUUID(), "dev")
	e.ws.AddMemberFixture(e.wsA, assignee.ID, models.RoleEditor)
	task := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colA, "Old", 0)

	title := "New title"
	description := "Longer"
	dueDate := "2026-12-31"
	updated, err := e.svc.Update(context.Background(), e.actorE, e.wsA, e.boardA, task.ID, dto.UpdateTaskRequest{
		Title:       &title,
		Description: &description,
		DueDate:     &dueDate,
		AssigneeID:  &assignee.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, "New title", updated.Title)
	assert.Equal(t, "Longer", updated.Description)
	require.NotNil(t, updated.DueDate)
	assert.Equal(t, "2026-12-31", updated.DueDate.Format("2006-01-02"))
	require.NotNil(t, updated.Assignee)
	assert.Equal(t, assignee.ID, updated.Assignee.ID)
	assert.Equal(t, eventTaskUpdated, e.hub.Messages[0].Type)
}

func TestUpdateTaskClearDueDateAndAssignee(t *testing.T) {
	e := newEnv(t)
	due := mustDate(t, "2026-01-15")
	task := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colA, "Timed", 0)
	task.DueDate = &due
	task.AssigneeID = e.actorO

	clear := ""
	_, err := e.svc.Update(context.Background(), e.actorE, e.wsA, e.boardA, task.ID, dto.UpdateTaskRequest{
		DueDate:    &clear,
		AssigneeID: &clear,
	})
	require.NoError(t, err)
	assert.Nil(t, task.DueDate)
	assert.Empty(t, task.AssigneeID)
}

func TestUpdateTaskMoveToColumn(t *testing.T) {
	e := newEnv(t)
	src1 := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colA, "A1", 0)
	src2 := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colA, "A2", 1)
	tgt1 := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colB, "B1", 0)
	tgt2 := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colB, "B2", 1)

	position := 1
	updated, err := e.svc.Update(context.Background(), e.actorE, e.wsA, e.boardA, src1.ID, dto.UpdateTaskRequest{
		ColumnID: &e.colB,
		Position: &position,
	})
	require.NoError(t, err)
	assert.Equal(t, e.colB, updated.ColumnID)
	assert.Equal(t, 1, updated.Position)

	stored, _ := e.tasks.FindTaskByID(context.Background(), src1.ID)
	assert.Equal(t, e.colB, stored.ColumnID)
	assert.Equal(t, 1, stored.Position)

	stored, _ = e.tasks.FindTaskByID(context.Background(), tgt1.ID)
	assert.Equal(t, e.colB, stored.ColumnID)
	assert.Equal(t, 0, stored.Position)

	stored, _ = e.tasks.FindTaskByID(context.Background(), tgt2.ID)
	assert.Equal(t, 2, stored.Position)

	stored, _ = e.tasks.FindTaskByID(context.Background(), src2.ID)
	assert.Equal(t, e.colA, stored.ColumnID)
	assert.Equal(t, 0, stored.Position)
}

func TestUpdateTaskMoveWithinColumn(t *testing.T) {
	e := newEnv(t)
	t1 := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colA, "T1", 0)
	t2 := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colA, "T2", 1)
	t3 := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colA, "T3", 2)

	position := 2
	_, err := e.svc.Update(context.Background(), e.actorE, e.wsA, e.boardA, t1.ID, dto.UpdateTaskRequest{
		Position: &position,
	})
	require.NoError(t, err)

	stored, _ := e.tasks.FindTaskByID(context.Background(), t1.ID)
	assert.Equal(t, 2, stored.Position)

	stored, _ = e.tasks.FindTaskByID(context.Background(), t2.ID)
	assert.Equal(t, 0, stored.Position)

	stored, _ = e.tasks.FindTaskByID(context.Background(), t3.ID)
	assert.Equal(t, 1, stored.Position)
}

func TestUpdateTaskInsertToColumnTop(t *testing.T) {
	e := newEnv(t)
	src := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colA, "A1", 0)
	tgt1 := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colB, "B1", 0)
	tgt2 := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colB, "B2", 1)

	updated, err := e.svc.Update(context.Background(), e.actorE, e.wsA, e.boardA, src.ID, dto.UpdateTaskRequest{
		ColumnID: &e.colB,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, updated.Position)

	stored, _ := e.tasks.FindTaskByID(context.Background(), tgt1.ID)
	assert.Equal(t, 1, stored.Position)

	stored, _ = e.tasks.FindTaskByID(context.Background(), tgt2.ID)
	assert.Equal(t, 2, stored.Position)
}

func TestUpdateTaskInPlaceKeepsPosition(t *testing.T) {
	e := newEnv(t)
	task := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colA, "T1", 1)
	e.tasks.AddTaskFixture(testutil.NewUUID(), e.colA, "T2", 0)
	title := "Renamed in place"
	updated, err := e.svc.Update(context.Background(), e.actorE, e.wsA, e.boardA, task.ID, dto.UpdateTaskRequest{
		Title:    &title,
		ColumnID: &e.colA,
	})
	require.NoError(t, err)
	assert.Equal(t, title, updated.Title)
	assert.Equal(t, 1, updated.Position)

	stored, _ := e.tasks.FindTaskByID(context.Background(), task.ID)
	assert.Equal(t, 1, stored.Position)
}

func TestUpdateTaskMemberAllowed(t *testing.T) {
	e := newEnv(t)
	task := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colA, "T1", 0)
	title := "Edited by member"
	updated, err := e.svc.Update(context.Background(), e.actorV, e.wsA, e.boardA, task.ID, dto.UpdateTaskRequest{
		Title:    &title,
		ColumnID: &e.colB,
	})
	require.NoError(t, err)
	assert.Equal(t, title, updated.Title)
	assert.Equal(t, e.colB, updated.ColumnID)
}

func TestDeleteTaskMemberAllowed(t *testing.T) {
	e := newEnv(t)
	task := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colA, "T1", 0)
	err := e.svc.Delete(context.Background(), e.actorV, e.wsA, e.boardA, task.ID)
	require.NoError(t, err)
	assert.NotContains(t, e.tasks.Tasks, task.ID)
	assert.Equal(t, eventTaskDeleted, e.hub.Messages[0].Type)
}

func TestDeleteTask(t *testing.T) {
	e := newEnv(t)
	task := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colA, "T1", 0)
	err := e.svc.Delete(context.Background(), e.actorE, e.wsA, e.boardA, task.ID)
	require.NoError(t, err)
	assert.NotContains(t, e.tasks.Tasks, task.ID)
	assert.Equal(t, eventTaskDeleted, e.hub.Messages[0].Type)
}

func TestDeleteTaskFromOtherBoard(t *testing.T) {
	e := newEnv(t)
	task := e.tasks.AddTaskFixture(testutil.NewUUID(), e.cols.AddColumnFixture(testutil.NewUUID(), e.boardB, "Backlog", 0).ID, "T1", 0)
	err := e.svc.Delete(context.Background(), e.actorE, e.wsA, e.boardA, task.ID)
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
}

func mustDate(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := parseDueDate(value)
	if err != nil {
		t.Fatalf("parse date: %v", err)
	}
	return *parsed
}

func TestGetTaskDetailWithParentAndSubtasks(t *testing.T) {
	e := newEnv(t)
	task, err := e.svc.Create(context.Background(), e.actorE, e.wsA, e.boardA, dto.CreateTaskRequest{
		ColumnID: e.colA,
		Title:    "Parent",
	})
	require.NoError(t, err)
	child, err := e.svc.Create(context.Background(), e.actorE, e.wsA, e.boardA, dto.CreateTaskRequest{
		ColumnID: e.colB,
		Title:    "Child",
		ParentID: task.ID,
	})
	require.NoError(t, err)
	require.NotEmpty(t, child.DisplayID)
	detail, err := e.svc.Get(context.Background(), e.actorO, e.wsA, task.ID)
	require.NoError(t, err)
	assert.Nil(t, detail.Parent)
	require.Len(t, detail.Subtasks, 1)
	assert.Equal(t, child.ID, detail.Subtasks[0].ID)
	childDetail, err := e.svc.Get(context.Background(), e.actorO, e.wsA, child.ID)
	require.NoError(t, err)
	require.NotNil(t, childDetail.Parent)
	assert.Equal(t, task.ID, childDetail.Parent.ID)
}

func TestListFilters(t *testing.T) {
	e := newEnv(t)
	e.cols.RegisterBoard(e.wsA, e.boardA)
	e.cols.RegisterBoard(e.wsA, e.boardB)
	e.tasks.RegisterColumnWorkspace(e.colA, e.wsA)
	e.tasks.RegisterColumnWorkspace(e.colB, e.wsA)
	e.tasks.RegisterColumnWorkspace(e.colC, e.wsA)
	t1 := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colA, "Alpha urgent", 0)
	t1.AssigneeID = e.actorE
	t2 := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colB, "Beta", 0)
	t2.AssigneeID = e.actorO
	child := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colA, "Gamma child", 1)
	child.ParentID = t1.ID

	all, err := e.svc.List(context.Background(), e.actorO, e.wsA, dto.ListWorkspaceTasksQuery{})
	require.NoError(t, err)
	require.Len(t, all, 3)
	withQ, err := e.svc.List(context.Background(), e.actorO, e.wsA, dto.ListWorkspaceTasksQuery{Q: "alpha"})
	require.NoError(t, err)
	require.Len(t, withQ, 1)
	assert.Equal(t, "Alpha urgent", withQ[0].Title)
	mine, err := e.svc.List(context.Background(), e.actorO, e.wsA, dto.ListWorkspaceTasksQuery{Only: "for_me"})
	require.NoError(t, err)
	require.Len(t, mine, 1)
	assert.Equal(t, "Beta", mine[0].Title)
	noSub, err := e.svc.List(context.Background(), e.actorO, e.wsA, dto.ListWorkspaceTasksQuery{ExcludeSubtasks: true})
	require.NoError(t, err)
	require.Len(t, noSub, 2)
}

func TestUpdateBoardMoveLandsFirstColumn(t *testing.T) {
	e := newEnv(t)
	task := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colC, "Moved", 0)
	e.tasks.RegisterColumn(e.colC, e.boardB)
	resp, err := e.svc.Update(context.Background(), e.actorE, e.wsA, e.boardB, task.ID, dto.UpdateTaskRequest{
		BoardID: &e.boardA,
	})
	require.NoError(t, err)
	assert.Equal(t, e.colA, resp.ColumnID)
}

func TestUpdateHidden(t *testing.T) {
	e := newEnv(t)
	task := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colA, "T1", 0)
	hidden := true
	resp, err := e.svc.Update(context.Background(), e.actorE, e.wsA, e.boardA, task.ID, dto.UpdateTaskRequest{
		IsHidden: &hidden,
	})
	require.NoError(t, err)
	assert.True(t, resp.IsHidden)
}

func TestUpdateImageKey(t *testing.T) {
	e := newEnv(t)
	task := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colA, "T1", 0)
	valid := "images/" + e.actorE + "/" + testutil.NewUUID() + ".png"
	resp, err := e.svc.Update(context.Background(), e.actorE, e.wsA, e.boardA, task.ID, dto.UpdateTaskRequest{ImageKey: &valid})
	require.NoError(t, err)
	assert.Equal(t, valid, resp.ImageKey)

	foreign := "images/" + e.actorO + "/" + testutil.NewUUID() + ".png"
	_, err = e.svc.Update(context.Background(), e.actorE, e.wsA, e.boardA, task.ID, dto.UpdateTaskRequest{ImageKey: &foreign})
	require.ErrorIs(t, err, pkgerrors.ErrValidation)

	empty := ""
	_, err = e.svc.Update(context.Background(), e.actorE, e.wsA, e.boardA, task.ID, dto.UpdateTaskRequest{ImageKey: &empty})
	require.NoError(t, err)
}

func TestParentCycleRejected(t *testing.T) {
	e := newEnv(t)
	t1, err := e.svc.Create(context.Background(), e.actorE, e.wsA, e.boardA, dto.CreateTaskRequest{ColumnID: e.colA, Title: "One"})
	require.NoError(t, err)
	t2, err := e.svc.Create(context.Background(), e.actorE, e.wsA, e.boardA, dto.CreateTaskRequest{ColumnID: e.colA, Title: "Two", ParentID: t1.ID})
	require.NoError(t, err)
	_, err = e.svc.Update(context.Background(), e.actorE, e.wsA, e.boardA, t1.ID, dto.UpdateTaskRequest{ParentID: &t2.ID})
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
}

func TestCuratorMustBeMember(t *testing.T) {
	e := newEnv(t)
	outsider := testutil.NewUUID()
	_, err := e.svc.Create(context.Background(), e.actorE, e.wsA, e.boardA, dto.CreateTaskRequest{
		ColumnID:  e.colA,
		Title:     "No",
		CuratorID: outsider,
	})
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
}
