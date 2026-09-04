package task

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tandem/tandem/internal/domain/models"
	"github.com/tandem/tandem/internal/http/dto"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/usecase/testutil"
)

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
	ws.AddMemberFixture(wsA, actorV, models.RoleViewer)
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
		svc:    NewService(tasks, cols, boards, ws, users, hub),
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
		Priority:    models.PriorityHigh,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if task.Title != "Implement login" || task.Priority != models.PriorityHigh || task.Position != 0 {
		t.Fatalf("unexpected task: %+v", task)
	}
	if task.Assignee == nil || task.Assignee.Login != "dev" {
		t.Fatalf("assignee not resolved: %+v", task.Assignee)
	}
	if e.hub.Messages[0].Type != eventTaskCreated {
		t.Fatalf("expected task.created, got %s", e.hub.Messages[0].Type)
	}
}

func TestCreateTaskDefaultPosition(t *testing.T) {
	e := newEnv(t)
	e.tasks.AddTaskFixture(testutil.NewUUID(), e.colA, "First", 0)
	task, err := e.svc.Create(context.Background(), e.actorE, e.wsA, e.boardA, dto.CreateTaskRequest{
		ColumnID: e.colA,
		Title:    "Second",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if task.Position != 1 {
		t.Fatalf("expected position 1, got %d", task.Position)
	}
}

func TestCreateTaskDefaultFields(t *testing.T) {
	e := newEnv(t)
	task, err := e.svc.Create(context.Background(), e.actorE, e.wsA, e.boardA, dto.CreateTaskRequest{
		ColumnID: e.colA,
		Title:    "Default fields",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if task.Priority != models.PriorityMedium || task.DueDate != nil || task.Assignee != nil {
		t.Fatalf("unexpected defaults: %+v", task)
	}
}

func TestCreateTaskViewerForbidden(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.Create(context.Background(), e.actorV, e.wsA, e.boardA, dto.CreateTaskRequest{
		ColumnID: e.colA,
		Title:    "Nope",
	})
	if !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestCreateTaskInvalidPriority(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.Create(context.Background(), e.actorE, e.wsA, e.boardA, dto.CreateTaskRequest{
		ColumnID: e.colA,
		Title:    "Invalid",
		Priority: "urgent",
	})
	if !errors.Is(err, pkgerrors.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestCreateTaskAssigneeNotMember(t *testing.T) {
	e := newEnv(t)
	outsider := testutil.NewUUID()
	_, err := e.svc.Create(context.Background(), e.actorE, e.wsA, e.boardA, dto.CreateTaskRequest{
		ColumnID:   e.colA,
		Title:      "Assign",
		AssigneeID: outsider,
	})
	if !errors.Is(err, pkgerrors.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestCreateTaskColumnFromOtherBoard(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.Create(context.Background(), e.actorE, e.wsA, e.boardA, dto.CreateTaskRequest{
		ColumnID: e.cols.AddColumnFixture(testutil.NewUUID(), e.boardB, "Backlog", 0).ID,
		Title:    "Wrong board",
	})
	if !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestUpdateTaskFields(t *testing.T) {
	e := newEnv(t)
	assignee := e.users.AddUserFixture(testutil.NewUUID(), "dev")
	e.ws.AddMemberFixture(e.wsA, assignee.ID, models.RoleEditor)
	task := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colA, "Old", 0)

	title := "New title"
	description := "Longer"
	priority := models.PriorityLow
	dueDate := "2026-12-31"
	updated, err := e.svc.Update(context.Background(), e.actorE, e.wsA, e.boardA, task.ID, dto.UpdateTaskRequest{
		Title:       &title,
		Description: &description,
		Priority:    &priority,
		DueDate:     &dueDate,
		AssigneeID:  &assignee.ID,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Title != "New title" || updated.Priority != models.PriorityLow || updated.Description != "Longer" {
		t.Fatalf("fields not updated: %+v", updated)
	}
	if updated.DueDate == nil || updated.DueDate.Format("2006-01-02") != "2026-12-31" {
		t.Fatalf("due date not updated: %+v", updated.DueDate)
	}
	if updated.Assignee == nil || updated.Assignee.ID != assignee.ID {
		t.Fatalf("assignee not updated: %+v", updated.Assignee)
	}
	if e.hub.Messages[0].Type != eventTaskUpdated {
		t.Fatalf("expected task.updated, got %s", e.hub.Messages[0].Type)
	}
}

func TestUpdateTaskClearDueDateAndAssignee(t *testing.T) {
	e := newEnv(t)
	due := mustDate(t, "2026-01-15")
	task := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colA, "Timed", 0)
	task.DueDate = &due
	task.AssigneeID = e.actorO

	clear := ""
	if _, err := e.svc.Update(context.Background(), e.actorE, e.wsA, e.boardA, task.ID, dto.UpdateTaskRequest{
		DueDate:    &clear,
		AssigneeID: &clear,
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if task.DueDate != nil || task.AssigneeID != "" {
		t.Fatalf("fields not cleared: %+v", task)
	}
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
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.ColumnID != e.colB || updated.Position != 1 {
		t.Fatalf("unexpected move result: %+v", updated)
	}
	if stored, _ := e.tasks.FindTaskByID(context.Background(), src1.ID); stored.ColumnID != e.colB || stored.Position != 1 {
		t.Fatalf("stored task not moved: %+v", stored)
	}
	if stored, _ := e.tasks.FindTaskByID(context.Background(), tgt1.ID); stored.ColumnID != e.colB || stored.Position != 0 {
		t.Fatalf("target first sibling position = %d, want 0", stored.Position)
	}
	if stored, _ := e.tasks.FindTaskByID(context.Background(), tgt2.ID); stored.Position != 2 {
		t.Fatalf("target sibling not pushed: %+v", stored)
	}
	if stored, _ := e.tasks.FindTaskByID(context.Background(), src2.ID); stored.ColumnID != e.colA || stored.Position != 0 {
		t.Fatalf("source sibling not renumbered: %+v", stored)
	}
}

func TestUpdateTaskMoveWithinColumn(t *testing.T) {
	e := newEnv(t)
	t1 := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colA, "T1", 0)
	t2 := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colA, "T2", 1)
	t3 := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colA, "T3", 2)

	position := 2
	if _, err := e.svc.Update(context.Background(), e.actorE, e.wsA, e.boardA, t1.ID, dto.UpdateTaskRequest{
		Position: &position,
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if stored, _ := e.tasks.FindTaskByID(context.Background(), t1.ID); stored.Position != 2 {
		t.Fatalf("t1 position = %d, want 2", stored.Position)
	}
	if stored, _ := e.tasks.FindTaskByID(context.Background(), t2.ID); stored.Position != 0 {
		t.Fatalf("t2 position = %d, want 0", stored.Position)
	}
	if stored, _ := e.tasks.FindTaskByID(context.Background(), t3.ID); stored.Position != 1 {
		t.Fatalf("t3 position = %d, want 1", stored.Position)
	}
}

func TestUpdateTaskAppendToColumn(t *testing.T) {
	e := newEnv(t)
	src := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colA, "A1", 0)
	e.tasks.AddTaskFixture(testutil.NewUUID(), e.colB, "B1", 0)
	e.tasks.AddTaskFixture(testutil.NewUUID(), e.colB, "B2", 1)

	updated, err := e.svc.Update(context.Background(), e.actorE, e.wsA, e.boardA, src.ID, dto.UpdateTaskRequest{
		ColumnID: &e.colB,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Position != 2 {
		t.Fatalf("expected append at position 2, got %d", updated.Position)
	}
}

func TestUpdateTaskViewerForbidden(t *testing.T) {
	e := newEnv(t)
	task := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colA, "T1", 0)
	title := "Nope"
	_, err := e.svc.Update(context.Background(), e.actorV, e.wsA, e.boardA, task.ID, dto.UpdateTaskRequest{Title: &title})
	if !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestDeleteTask(t *testing.T) {
	e := newEnv(t)
	task := e.tasks.AddTaskFixture(testutil.NewUUID(), e.colA, "T1", 0)
	if err := e.svc.Delete(context.Background(), e.actorE, e.wsA, e.boardA, task.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := e.tasks.Tasks[task.ID]; ok {
		t.Fatal("task still exists after delete")
	}
	if e.hub.Messages[0].Type != eventTaskDeleted {
		t.Fatalf("expected task.deleted, got %s", e.hub.Messages[0].Type)
	}
}

func TestDeleteTaskFromOtherBoard(t *testing.T) {
	e := newEnv(t)
	task := e.tasks.AddTaskFixture(testutil.NewUUID(), e.cols.AddColumnFixture(testutil.NewUUID(), e.boardB, "Backlog", 0).ID, "T1", 0)
	err := e.svc.Delete(context.Background(), e.actorE, e.wsA, e.boardA, task.ID)
	if !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func mustDate(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := parseDueDate(value)
	if err != nil {
		t.Fatalf("parse date: %v", err)
	}
	return *parsed
}
