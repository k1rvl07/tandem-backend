package tree

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	dtree "github.com/tandem/tandem/internal/http/dto/tree"
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
	ws.AddMemberFixture(wsID, user, mworkspace.RoleOwner)
	board := boards.AddBoardFixture(testutil.NewUUID(), wsID, "Board")
	col := cols.AddColumnFixture(testutil.NewUUID(), board.ID, "Backlog", 0)
	tasks.RegisterColumn(col.ID, board.ID)
	cols.RegisterBoard(wsID, board.ID)
	tasks.RegisterColumnWorkspace(col.ID, wsID)
	tasks.AddTaskFixture(testutil.NewUUID(), col.ID, "Alpha", 0)

	svc := NewService(ws, boards, cols, tasks, users, favorites, testutil.NewFakeCache())
	resp, err := svc.List(context.Background(), user, dtree.TreeQuery{Tasks: "all"})
	require.NoError(t, err)
	require.Len(t, resp, 1)
	require.Len(t, resp[0].Boards, 1)
	require.Len(t, resp[0].Boards[0].Tasks, 1)
	require.Equal(t, "WS", resp[0].Workspace.Prefix)
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
	ws.AddMemberFixture(wsID, me, mworkspace.RoleOwner)
	board := boards.AddBoardFixture(testutil.NewUUID(), wsID, "Board")
	col := cols.AddColumnFixture(testutil.NewUUID(), board.ID, "Backlog", 0)
	tasks.RegisterColumn(col.ID, board.ID)
	cols.RegisterBoard(wsID, board.ID)
	tasks.RegisterColumnWorkspace(col.ID, wsID)
	mine := tasks.AddTaskFixture(testutil.NewUUID(), col.ID, "Mine", 0)
	mine.AssigneeID = me
	tasks.AddTaskFixture(testutil.NewUUID(), col.ID, "Other", 1).AssigneeID = other

	svc := NewService(ws, boards, cols, tasks, users, favorites, testutil.NewFakeCache())
	resp, err := svc.List(context.Background(), me, dtree.TreeQuery{Tasks: "for_me"})
	require.NoError(t, err)
	got := resp[0].Boards[0].Tasks
	require.Len(t, got, 1)
	require.Equal(t, "Mine", got[0].Title)
}
