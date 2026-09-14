package tree

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dtree "github.com/tandem/tandem/internal/contracts/tree"
	mcolumn "github.com/tandem/tandem/internal/domain/models/column"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
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

	svc := NewService(Deps{Workspaces: ws, Boards: boards, Columns: cols, Tasks: tasks, Users: users, Favorites: favorites, Cache: testutil.NewFakeCache()})
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

	svc := NewService(Deps{Workspaces: ws, Boards: boards, Columns: cols, Tasks: tasks, Users: users, Favorites: favorites, Cache: testutil.NewFakeCache()})
	resp, err := svc.List(context.Background(), me, dtree.TreeQuery{Tasks: "for_me"})
	require.NoError(t, err)
	got := resp[0].Boards[0].Tasks
	require.Len(t, got, 1)
	require.Equal(t, "Mine", got[0].Title)
}

func TestTreeSortsByColumnOrder(t *testing.T) {
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
	first := cols.AddColumnFixture(testutil.NewUUID(), board.ID, "To Do", 2)
	second := cols.AddColumnFixture(testutil.NewUUID(), board.ID, "Backlog", 0)
	current := cols.AddColumnFixture(testutil.NewUUID(), board.ID, "Done", 1)
	for _, c := range []*mcolumn.Column{first, second, current} {
		tasks.RegisterColumn(c.ID, board.ID)
		tasks.RegisterColumnWorkspace(c.ID, wsID)
	}
	cols.RegisterBoard(wsID, board.ID)
	a := tasks.AddTaskFixture(testutil.NewUUID(), current.ID, "A", 0)
	b := tasks.AddTaskFixture(testutil.NewUUID(), current.ID, "B", 1)
	c1 := tasks.AddTaskFixture(testutil.NewUUID(), first.ID, "C", 0)
	d := tasks.AddTaskFixture(testutil.NewUUID(), second.ID, "D", 1)
	e := tasks.AddTaskFixture(testutil.NewUUID(), second.ID, "E", 0)
	a.CreatedAt = time.Now()
	b.CreatedAt = a.CreatedAt.Add(-time.Hour)
	c1.CreatedAt = time.Now()
	d.CreatedAt = time.Now()
	e.CreatedAt = time.Now()

	svc := NewService(Deps{Workspaces: ws, Boards: boards, Columns: cols, Tasks: tasks, Users: users, Favorites: favorites, Cache: testutil.NewFakeCache()})
	resp, err := svc.List(context.Background(), user, dtree.TreeQuery{Tasks: "all"})
	require.NoError(t, err)
	got := resp[0].Boards[0].Tasks
	require.Len(t, got, 5)
	require.Equal(t, []string{"E", "D", "A", "B", "C"}, []string{got[0].Title, got[1].Title, got[2].Title, got[3].Title, got[4].Title})
}

func TestTreeSkipsHiddenAndMineByCurator(t *testing.T) {
	ws := testutil.NewFakeWorkspaceRepo()
	boards := testutil.NewFakeBoardRepo()
	cols := testutil.NewFakeColumnRepo()
	tasks := testutil.NewFakeTaskRepo()
	users := testutil.NewFakeUserRepo()
	favorites := testutil.NewFakeFavoriteRepo()

	me := testutil.NewUUID()
	wsID := testutil.NewUUID()
	ws.AddWorkspaceFixture(wsID, "Team")
	ws.AddMemberFixture(wsID, me, mworkspace.RoleOwner)
	board := boards.AddBoardFixture(testutil.NewUUID(), wsID, "Board")
	col := cols.AddColumnFixture(testutil.NewUUID(), board.ID, "Backlog", 0)
	tasks.RegisterColumn(col.ID, board.ID)
	cols.RegisterBoard(wsID, board.ID)
	tasks.RegisterColumnWorkspace(col.ID, wsID)
	curated := tasks.AddTaskFixture(testutil.NewUUID(), col.ID, "Curated", 0)
	curated.CuratorID = me
	hidden := tasks.AddTaskFixture(testutil.NewUUID(), col.ID, "Hidden", 1)
	hidden.IsHidden = true

	svc := NewService(Deps{Workspaces: ws, Boards: boards, Columns: cols, Tasks: tasks, Users: users, Favorites: favorites, Cache: testutil.NewFakeCache()})
	resp, err := svc.List(context.Background(), me, dtree.TreeQuery{Tasks: "mine"})
	require.NoError(t, err)
	got := resp[0].Boards[0].Tasks
	require.Len(t, got, 1)
	require.Equal(t, "Curated", got[0].Title)
}
