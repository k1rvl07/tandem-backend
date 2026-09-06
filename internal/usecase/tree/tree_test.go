package tree

import (
	"context"
	"testing"

	"github.com/tandem/tandem/internal/domain/models"
	"github.com/tandem/tandem/internal/http/dto"
	"github.com/tandem/tandem/internal/usecase/testutil"
)

func TestTreeAllTasks(t *testing.T) {
	ws := testutil.NewFakeWorkspaceRepo()
	boards := testutil.NewFakeBoardRepo()
	cols := testutil.NewFakeColumnRepo()
	tasks := testutil.NewFakeTaskRepo()
	users := testutil.NewFakeUserRepo()
	favorites := testutil.NewFakeFavoriteRepo()

	user := testutil.NewUUID()
	wsID := testutil.NewUUID()
	ws.AddWorkspaceFixture(wsID, "Team")
	ws.AddMemberFixture(wsID, user, models.RoleOwner)
	board := boards.AddBoardFixture(testutil.NewUUID(), wsID, "Board")
	col := cols.AddColumnFixture(testutil.NewUUID(), board.ID, "Backlog", 0)
	tasks.RegisterColumn(col.ID, board.ID)
	cols.RegisterBoard(wsID, board.ID)
	tasks.RegisterColumnWorkspace(col.ID, wsID)
	tasks.AddTaskFixture(testutil.NewUUID(), col.ID, "Alpha", 0)

	svc := NewService(ws, boards, cols, tasks, users, favorites, testutil.NewFakeCache())
	resp, err := svc.List(context.Background(), user, dto.TreeQuery{Tasks: "all"})
	if err != nil {
		t.Fatalf("tree: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(resp))
	}
	if len(resp[0].Boards) != 1 || len(resp[0].Boards[0].Tasks) != 1 {
		t.Fatalf("unexpected tree: %+v", resp)
	}
	if resp[0].Workspace.Prefix != "WS" {
		t.Fatalf("unexpected prefix %q", resp[0].Workspace.Prefix)
	}
}

func TestTreeMineExcludesOtherTasks(t *testing.T) {
	ws := testutil.NewFakeWorkspaceRepo()
	boards := testutil.NewFakeBoardRepo()
	cols := testutil.NewFakeColumnRepo()
	tasks := testutil.NewFakeTaskRepo()
	users := testutil.NewFakeUserRepo()
	favorites := testutil.NewFakeFavoriteRepo()

	me := testutil.NewUUID()
	other := testutil.NewUUID()
	wsID := testutil.NewUUID()
	ws.AddWorkspaceFixture(wsID, "Team")
	ws.AddMemberFixture(wsID, me, models.RoleOwner)
	board := boards.AddBoardFixture(testutil.NewUUID(), wsID, "Board")
	col := cols.AddColumnFixture(testutil.NewUUID(), board.ID, "Backlog", 0)
	tasks.RegisterColumn(col.ID, board.ID)
	cols.RegisterBoard(wsID, board.ID)
	tasks.RegisterColumnWorkspace(col.ID, wsID)
	mine := tasks.AddTaskFixture(testutil.NewUUID(), col.ID, "Mine", 0)
	mine.AssigneeID = me
	tasks.AddTaskFixture(testutil.NewUUID(), col.ID, "Other", 1).AssigneeID = other

	svc := NewService(ws, boards, cols, tasks, users, favorites, testutil.NewFakeCache())
	resp, err := svc.List(context.Background(), me, dto.TreeQuery{Tasks: "for_me"})
	if err != nil {
		t.Fatalf("tree: %v", err)
	}
	got := resp[0].Boards[0].Tasks
	if len(got) != 1 || got[0].Title != "Mine" {
		t.Fatalf("unexpected filtered tasks: %+v", got)
	}
}
