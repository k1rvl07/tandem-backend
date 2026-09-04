package column

import (
	"context"
	"errors"
	"testing"

	"github.com/tandem/tandem/internal/domain/models"
	"github.com/tandem/tandem/internal/http/dto"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/usecase/testutil"
)

type env struct {
	svc    *Service
	cols   *testutil.FakeColumnRepo
	ws     *testutil.FakeWorkspaceRepo
	hub    *testutil.FakeHub
	actorO string
	actorV string
	wsA    string
	wsB    string
	boardA string
	boardB string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	boards := testutil.NewFakeBoardRepo()
	cols := testutil.NewFakeColumnRepo()
	ws := testutil.NewFakeWorkspaceRepo()
	hub := testutil.NewFakeHub()

	actorO := testutil.NewUUID()
	actorV := testutil.NewUUID()
	actorE := testutil.NewUUID()
	wsA := testutil.NewUUID()
	wsB := testutil.NewUUID()
	ws.AddWorkspaceFixture(wsA, "Team A")
	ws.AddWorkspaceFixture(wsB, "Team B")
	ws.AddMemberFixture(wsA, actorO, models.RoleOwner)
	ws.AddMemberFixture(wsA, actorE, models.RoleEditor)
	ws.AddMemberFixture(wsA, actorV, models.RoleViewer)
	boardA := testutil.NewUUID()
	boardB := testutil.NewUUID()
	boards.AddBoardFixture(boardA, wsA, "Sprint")
	boards.AddBoardFixture(boardB, wsB, "Other")

	return &env{
		svc:    NewService(cols, boards, ws, hub),
		cols:   cols,
		ws:     ws,
		hub:    hub,
		actorO: actorO,
		actorV: actorV,
		wsA:    wsA,
		wsB:    wsB,
		boardA: boardA,
		boardB: boardB,
	}
}

func TestCreateColumn(t *testing.T) {
	e := newEnv(t)
	e.cols.AddColumnFixture(testutil.NewUUID(), e.boardA, "Backlog", 0)
	e.cols.AddColumnFixture(testutil.NewUUID(), e.boardA, "To Do", 1)

	column, err := e.svc.Create(context.Background(), e.actorO, e.wsA, e.boardA, dto.CreateColumnRequest{Name: "Testing"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if column.Name != "Testing" || column.Position != 2 {
		t.Fatalf("unexpected column: %+v", column)
	}
	if e.hub.Messages[0].Type != eventColumnCreated {
		t.Fatalf("expected column.created, got %s", e.hub.Messages[0].Type)
	}
}

func TestCreateColumnViewerForbidden(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.Create(context.Background(), e.actorV, e.wsA, e.boardA, dto.CreateColumnRequest{Name: "Testing"})
	if !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestCreateColumnBoardInOtherWorkspace(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.Create(context.Background(), e.actorO, e.wsB, e.boardB, dto.CreateColumnRequest{Name: "Testing"})
	if !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("expected forbidden (not a member of B), got %v", err)
	}
}

func TestUpdateColumnRename(t *testing.T) {
	e := newEnv(t)
	column := e.cols.AddColumnFixture(testutil.NewUUID(), e.boardA, "Backlog", 0)
	name := "Ready"
	updated, err := e.svc.Update(context.Background(), e.actorO, e.wsA, e.boardA, column.ID, dto.UpdateColumnRequest{Name: &name})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "Ready" {
		t.Fatalf("name not updated: %+v", updated)
	}
	if e.hub.Messages[0].Type != eventColumnUpdated {
		t.Fatalf("expected column.updated, got %s", e.hub.Messages[0].Type)
	}
}

func TestUpdateColumnReorder(t *testing.T) {
	e := newEnv(t)
	collapsed := testutil.NewUUID()
	columns := []struct {
		id   string
		name string
	}{
		{collapsed, "Backlog"},
		{testutil.NewUUID(), "To Do"},
		{testutil.NewUUID(), "Done"},
	}
	for i, c := range columns {
		e.cols.AddColumnFixture(c.id, e.boardA, c.name, i)
	}
	position := 2
	if _, err := e.svc.Update(context.Background(), e.actorO, e.wsA, e.boardA, collapsed, dto.UpdateColumnRequest{Position: &position}); err != nil {
		t.Fatalf("update: %v", err)
	}
	cols, _ := e.cols.ListColumns(context.Background(), e.boardA)
	want := []string{"To Do", "Done", "Backlog"}
	for i := range want {
		if cols[i].Name != want[i] || cols[i].Position != i {
			t.Fatalf("column %d = %s pos %d, want %s pos %d", i, cols[i].Name, cols[i].Position, want[i], i)
		}
	}
}

func TestUpdateColumnViewerForbidden(t *testing.T) {
	e := newEnv(t)
	column := e.cols.AddColumnFixture(testutil.NewUUID(), e.boardA, "Backlog", 0)
	name := "Ready"
	_, err := e.svc.Update(context.Background(), e.actorV, e.wsA, e.boardA, column.ID, dto.UpdateColumnRequest{Name: &name})
	if !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestDeleteColumn(t *testing.T) {
	e := newEnv(t)
	column := e.cols.AddColumnFixture(testutil.NewUUID(), e.boardA, "Backlog", 0)
	if err := e.svc.Delete(context.Background(), e.actorO, e.wsA, e.boardA, column.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := e.cols.Columns[column.ID]; ok {
		t.Fatal("column still exists after delete")
	}
	if e.hub.Messages[0].Type != eventColumnDeleted {
		t.Fatalf("expected column.deleted, got %s", e.hub.Messages[0].Type)
	}
}

func TestDeleteColumnColumnFromOtherBoard(t *testing.T) {
	e := newEnv(t)
	column := e.cols.AddColumnFixture(testutil.NewUUID(), e.boardB, "Backlog", 0)
	err := e.svc.Delete(context.Background(), e.actorO, e.wsA, e.boardB, column.ID)
	if !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}
