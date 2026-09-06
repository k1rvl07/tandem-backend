package board

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
	ws     *testutil.FakeWorkspaceRepo
	cols   *testutil.FakeColumnRepo
	tasks  *testutil.FakeTaskRepo
	users  *testutil.FakeUserRepo
	hub    *testutil.FakeHub
	actorA string
	actorB string
	wsA    string
	wsB    string
}

func newEnv(t *testing.T) *env {
	t.Helper()
	boards := testutil.NewFakeBoardRepo()
	cols := testutil.NewFakeColumnRepo()
	tasks := testutil.NewFakeTaskRepo()
	ws := testutil.NewFakeWorkspaceRepo()
	users := testutil.NewFakeUserRepo()
	favorites := testutil.NewFakeFavoriteRepo()
	hub := testutil.NewFakeHub()

	actorA := testutil.NewUUID()
	actorB := testutil.NewUUID()
	wsA := testutil.NewUUID()
	wsB := testutil.NewUUID()
	ws.AddWorkspaceFixture(wsA, "Team A")
	ws.AddWorkspaceFixture(wsB, "Team B")
	ws.AddMemberFixture(wsA, actorA, models.RoleOwner)
	ws.AddMemberFixture(wsB, actorA, models.RoleOwner)
	ws.AddMemberFixture(wsA, actorB, models.RoleViewer)

	return &env{
		svc:    NewService(boards, cols, tasks, ws, users, favorites, hub, testutil.NewFakeCache()),
		ws:     ws,
		cols:   cols,
		tasks:  tasks,
		users:  users,
		hub:    hub,
		actorA: actorA,
		actorB: actorB,
		wsA:    wsA,
		wsB:    wsB,
	}
}

func TestCreateBoard(t *testing.T) {
	e := newEnv(t)
	board, err := e.svc.Create(context.Background(), e.actorA, e.wsA, dto.CreateBoardRequest{Name: "Sprint 1"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if board.Name != "Sprint 1" || board.WorkspaceID != e.wsA {
		t.Fatalf("unexpected board: %+v", board)
	}
	columns, _ := e.cols.ListColumns(context.Background(), board.ID)
	if len(columns) != 4 {
		t.Fatalf("expected 4 default columns, got %d", len(columns))
	}
	for i, want := range []string{"Backlog", "To Do", "In Progress", "Done"} {
		if columns[i].Name != want || columns[i].Position != i {
			t.Fatalf("column %d = %s pos %d, want %s pos %d", i, columns[i].Name, columns[i].Position, want, i)
		}
	}
	if len(e.hub.Messages) != 1 {
		t.Fatalf("expected 1 board event, got %d", len(e.hub.Messages))
	}
	if e.hub.Messages[0].Type != eventBoardCreated {
		t.Fatalf("expected board.created, got %s", e.hub.Messages[0].Type)
	}
}

func TestCreateBoardViewerForbidden(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.Create(context.Background(), e.actorB, e.wsA, dto.CreateBoardRequest{Name: "Sprint 1"})
	if !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestCreateBoardInvalidName(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.Create(context.Background(), e.actorA, e.wsA, dto.CreateBoardRequest{Name: "  "})
	if !errors.Is(err, pkgerrors.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestListBoards(t *testing.T) {
	e := newEnv(t)
	boardsRepo := e.svc.boards
	b1 := boardsRepo.(*testutil.FakeBoardRepo).AddBoardFixture(testutil.NewUUID(), e.wsA, "Alpha")
	b2 := boardsRepo.(*testutil.FakeBoardRepo).AddBoardFixture(testutil.NewUUID(), e.wsA, "Beta")
	boardsRepo.(*testutil.FakeBoardRepo).AddBoardFixture(testutil.NewUUID(), e.wsB, "Other")

	list, err := e.svc.List(context.Background(), e.actorA, e.wsA, false)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 || list[0].ID != b1.ID || list[1].ID != b2.ID {
		t.Fatalf("unexpected list: %+v", list)
	}
}

func TestGetBoard(t *testing.T) {
	e := newEnv(t)
	boards := e.svc.boards.(*testutil.FakeBoardRepo)
	board := boards.AddBoardFixture(testutil.NewUUID(), e.wsA, "Sprint")
	col1 := e.cols.AddColumnFixture(testutil.NewUUID(), board.ID, "Backlog", 0)
	col2 := e.cols.AddColumnFixture(testutil.NewUUID(), board.ID, "Done", 1)
	e.tasks.RegisterColumn(col1.ID, board.ID)
	e.tasks.RegisterColumn(col2.ID, board.ID)
	assignee := e.users.AddUserFixture(testutil.NewUUID(), "dev")
	task1 := e.tasks.AddTaskFixture(testutil.NewUUID(), col1.ID, "Implement", 0)
	task1.AssigneeID = assignee.ID

	detail, err := e.svc.Get(context.Background(), e.actorA, e.wsA, board.ID, false)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(detail.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(detail.Columns))
	}
	if detail.Columns[0].TaskCount != 1 || len(detail.Columns[0].Tasks) != 1 {
		t.Fatalf("unexpected column 0 tasks: %+v", detail.Columns[0])
	}
	if detail.Columns[0].Tasks[0].Assignee == nil || detail.Columns[0].Tasks[0].Assignee.Login != "dev" {
		t.Fatalf("assignee not resolved: %+v", detail.Columns[0].Tasks[0].Assignee)
	}
}

func TestGetBoardFromOtherWorkspace(t *testing.T) {
	e := newEnv(t)
	board := e.svc.boards.(*testutil.FakeBoardRepo).AddBoardFixture(testutil.NewUUID(), e.wsB, "Other")
	_, err := e.svc.Get(context.Background(), e.actorA, e.wsA, board.ID, false)
	if !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestGetBoardNonMember(t *testing.T) {
	e := newEnv(t)
	board := e.svc.boards.(*testutil.FakeBoardRepo).AddBoardFixture(testutil.NewUUID(), e.wsA, "Sprint")
	outsider := testutil.NewUUID()
	_, err := e.svc.Get(context.Background(), outsider, e.wsA, board.ID, false)
	if !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestUpdateBoard(t *testing.T) {
	e := newEnv(t)
	board := e.svc.boards.(*testutil.FakeBoardRepo).AddBoardFixture(testutil.NewUUID(), e.wsA, "Sprint")
	updated, err := e.svc.Update(context.Background(), e.actorA, e.wsA, board.ID, dto.UpdateBoardRequest{Name: "Release"})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "Release" {
		t.Fatalf("name not updated: %+v", updated)
	}
	if e.hub.Messages[0].Type != eventBoardUpdated {
		t.Fatalf("expected board.updated, got %s", e.hub.Messages[0].Type)
	}
}

func TestUpdateBoardViewerForbidden(t *testing.T) {
	e := newEnv(t)
	board := e.svc.boards.(*testutil.FakeBoardRepo).AddBoardFixture(testutil.NewUUID(), e.wsA, "Sprint")
	_, err := e.svc.Update(context.Background(), e.actorB, e.wsA, board.ID, dto.UpdateBoardRequest{Name: "Release"})
	if !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestDeleteBoard(t *testing.T) {
	e := newEnv(t)
	board := e.svc.boards.(*testutil.FakeBoardRepo).AddBoardFixture(testutil.NewUUID(), e.wsA, "Sprint")
	if err := e.svc.Delete(context.Background(), e.actorA, e.wsA, board.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := e.svc.boards.(*testutil.FakeBoardRepo).Boards[board.ID]; ok {
		t.Fatal("board still exists after delete")
	}
	if e.hub.Messages[0].Type != eventBoardDeleted {
		t.Fatalf("expected board.deleted, got %s", e.hub.Messages[0].Type)
	}
}

func TestDeleteBoardFromOtherWorkspace(t *testing.T) {
	e := newEnv(t)
	board := e.svc.boards.(*testutil.FakeBoardRepo).AddBoardFixture(testutil.NewUUID(), e.wsB, "Other")
	err := e.svc.Delete(context.Background(), e.actorA, e.wsA, board.ID)
	if !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestArchiveBoard(t *testing.T) {
	e := newEnv(t)
	boards := e.svc.boards.(*testutil.FakeBoardRepo)
	board := boards.AddBoardFixture(testutil.NewUUID(), e.wsA, "Sprint")

	archived, err := e.svc.Archive(context.Background(), e.actorA, e.wsA, board.ID, true)
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if !archived.Archived || archived.ArchivedAt == nil {
		t.Fatalf("expected archived board, got %+v", archived)
	}

	list, err := e.svc.List(context.Background(), e.actorA, e.wsA, false)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected archive-hidden board, got %+v", list)
	}

	list, err = e.svc.List(context.Background(), e.actorA, e.wsA, true)
	if err != nil {
		t.Fatalf("list with archived: %v", err)
	}
	if len(list) != 1 || !list[0].Archived {
		t.Fatalf("expected archived board in list, got %+v", list)
	}

	restored, err := e.svc.Archive(context.Background(), e.actorA, e.wsA, board.ID, false)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if restored.Archived || restored.ArchivedAt != nil {
		t.Fatalf("expected restored board, got %+v", restored)
	}
}

func TestArchiveBoardMainForbidden(t *testing.T) {
	e := newEnv(t)
	boards := e.svc.boards.(*testutil.FakeBoardRepo)
	board := boards.AddBoardFixture(testutil.NewUUID(), e.wsA, "Main")
	board.IsMain = true

	_, err := e.svc.Archive(context.Background(), e.actorA, e.wsA, board.ID, true)
	if !errors.Is(err, pkgerrors.ErrValidation) {
		t.Fatalf("expected validation error archiving main board, got %v", err)
	}
}

func TestArchiveBoardViewerForbidden(t *testing.T) {
	e := newEnv(t)
	boards := e.svc.boards.(*testutil.FakeBoardRepo)
	board := boards.AddBoardFixture(testutil.NewUUID(), e.wsA, "Sprint")

	_, err := e.svc.Archive(context.Background(), e.actorB, e.wsA, board.ID, true)
	if !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestReorderBoards(t *testing.T) {
	e := newEnv(t)
	boards := e.svc.boards.(*testutil.FakeBoardRepo)
	b1 := boards.AddBoardFixture(testutil.NewUUID(), e.wsA, "Alpha")
	b2 := boards.AddBoardFixture(testutil.NewUUID(), e.wsA, "Beta")
	b3 := boards.AddBoardFixture(testutil.NewUUID(), e.wsA, "Gamma")

	ordered, err := e.svc.Reorder(context.Background(), e.actorA, e.wsA, dto.ReorderBoardsRequest{BoardIDs: []string{b3.ID, b1.ID, b2.ID}})
	if err != nil {
		t.Fatalf("reorder: %v", err)
	}
	if len(ordered) != 3 || ordered[0].ID != b3.ID || ordered[1].ID != b1.ID || ordered[2].ID != b2.ID {
		t.Fatalf("unexpected order: %+v", ordered)
	}
	if ordered[0].Position != 0 || ordered[1].Position != 1 || ordered[2].Position != 2 {
		t.Fatalf("positions not sequential: %+v", ordered)
	}

	list, err := e.svc.List(context.Background(), e.actorA, e.wsA, false)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 3 || list[0].ID != b3.ID || list[2].ID != b2.ID {
		t.Fatalf("stored order not reordered: %+v", list)
	}
}

func TestReorderBoardsValidation(t *testing.T) {
	e := newEnv(t)
	boards := e.svc.boards.(*testutil.FakeBoardRepo)
	b1 := boards.AddBoardFixture(testutil.NewUUID(), e.wsA, "Alpha")
	boards.AddBoardFixture(testutil.NewUUID(), e.wsA, "Beta")

	if _, err := e.svc.Reorder(context.Background(), e.actorA, e.wsA, dto.ReorderBoardsRequest{BoardIDs: []string{}}); !errors.Is(err, pkgerrors.ErrValidation) {
		t.Fatalf("expected validation error for empty list, got %v", err)
	}
	if _, err := e.svc.Reorder(context.Background(), e.actorA, e.wsA, dto.ReorderBoardsRequest{BoardIDs: []string{testutil.NewUUID()}}); !errors.Is(err, pkgerrors.ErrValidation) {
		t.Fatalf("expected validation error for foreign board, got %v", err)
	}
	if _, err := e.svc.Reorder(context.Background(), e.actorA, e.wsA, dto.ReorderBoardsRequest{BoardIDs: []string{b1.ID, b1.ID}}); !errors.Is(err, pkgerrors.ErrValidation) {
		t.Fatalf("expected validation error for duplicates, got %v", err)
	}
}

func TestReorderBoardsViewerForbidden(t *testing.T) {
	e := newEnv(t)
	boards := e.svc.boards.(*testutil.FakeBoardRepo)
	b1 := boards.AddBoardFixture(testutil.NewUUID(), e.wsA, "Alpha")

	_, err := e.svc.Reorder(context.Background(), e.actorB, e.wsA, dto.ReorderBoardsRequest{BoardIDs: []string{b1.ID}})
	if !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestListBoardsIncludeArchived(t *testing.T) {
	e := newEnv(t)
	board := e.svc.boards.(*testutil.FakeBoardRepo).AddBoardFixture(testutil.NewUUID(), e.wsA, "Sprint")
	now := time.Now()
	board.ArchivedAt = &now

	list, err := e.svc.List(context.Background(), e.actorA, e.wsA, false)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected archived hidden, got %+v", list)
	}

	list, err = e.svc.List(context.Background(), e.actorA, e.wsA, true)
	if err != nil {
		t.Fatalf("list with archived: %v", err)
	}
	if len(list) != 1 || !list[0].Archived {
		t.Fatalf("expected archived board, got %+v", list)
	}
}
