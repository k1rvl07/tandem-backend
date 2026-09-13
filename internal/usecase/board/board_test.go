package board

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	dboard "github.com/tandem/tandem/internal/http/dto/board"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/usecase/board/core"
	file "github.com/tandem/tandem/internal/usecase/file"
	"github.com/tandem/tandem/internal/usecase/testutil"
	"go.uber.org/zap"
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
	ws.AddMemberFixture(wsA, actorA, mworkspace.RoleOwner)
	ws.AddMemberFixture(wsB, actorA, mworkspace.RoleOwner)
	ws.AddMemberFixture(wsA, actorB, mworkspace.RoleMember)

	return &env{
		svc:    NewService(Deps{Boards: boards, Columns: cols, Tasks: tasks, Workspaces: ws, Users: users, Favorites: favorites, Files: file.NewService(noopBoardStore{}, zap.NewNop()), Hub: hub, Cache: testutil.NewFakeCache()}),
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

type noopBoardStore struct{}

func (noopBoardStore) Put(context.Context, string, io.Reader, int64, string) error {
	return nil
}

func (noopBoardStore) Get(context.Context, string) (io.ReadCloser, error) {
	return nil, errors.New("noop")
}

func (noopBoardStore) Delete(context.Context, string) error {
	return nil
}

func (noopBoardStore) Exists(context.Context, string) (bool, error) {
	return false, nil
}

func (noopBoardStore) PresignGet(context.Context, string, time.Duration) (string, error) {
	return "", nil
}

func TestCreateBoard(t *testing.T) {
	e := newEnv(t)
	board, err := e.svc.Create(context.Background(), e.actorA, e.wsA, dboard.CreateBoardRequest{Name: "Sprint 1"})
	require.NoError(t, err)
	assert.Equal(t, "Sprint 1", board.Name)
	assert.Equal(t, e.wsA, board.WorkspaceID)
	columns, _ := e.cols.ListColumns(context.Background(), board.ID)
	require.Len(t, columns, 4)
	for i, want := range []string{"Backlog", "To Do", "In Progress", "Done"} {
		assert.Equal(t, want, columns[i].Name)
		assert.Equal(t, i, columns[i].Position)
	}
	require.Len(t, e.hub.Messages, 1)
	assert.Equal(t, core.EventBoardCreated, e.hub.Messages[0].Type)
}

func TestCreateBoardMemberForbidden(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.Create(context.Background(), e.actorB, e.wsA, dboard.CreateBoardRequest{Name: "Sprint 1"})
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}

func TestCreateBoardInvalidName(t *testing.T) {
	e := newEnv(t)
	_, err := e.svc.Create(context.Background(), e.actorA, e.wsA, dboard.CreateBoardRequest{Name: "  "})
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
}

func TestListBoards(t *testing.T) {
	e := newEnv(t)
	boardsRepo := e.svc.Boards.Boards
	b1 := boardsRepo.(*testutil.FakeBoardRepo).AddBoardFixture(testutil.NewUUID(), e.wsA, "Alpha")
	b2 := boardsRepo.(*testutil.FakeBoardRepo).AddBoardFixture(testutil.NewUUID(), e.wsA, "Beta")
	boardsRepo.(*testutil.FakeBoardRepo).AddBoardFixture(testutil.NewUUID(), e.wsB, "Other")

	list, err := e.svc.List(context.Background(), e.actorA, e.wsA)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, b1.ID, list[0].ID)
	assert.Equal(t, b2.ID, list[1].ID)
}

func TestGetBoard(t *testing.T) {
	e := newEnv(t)
	boards := e.svc.Boards.Boards.(*testutil.FakeBoardRepo)
	board := boards.AddBoardFixture(testutil.NewUUID(), e.wsA, "Sprint")
	col1 := e.cols.AddColumnFixture(testutil.NewUUID(), board.ID, "Backlog", 0)
	col2 := e.cols.AddColumnFixture(testutil.NewUUID(), board.ID, "Done", 1)
	e.tasks.RegisterColumn(col1.ID, board.ID)
	e.tasks.RegisterColumn(col2.ID, board.ID)
	assignee := e.users.AddUserFixture(testutil.NewUUID(), "dev")
	task1 := e.tasks.AddTaskFixture(testutil.NewUUID(), col1.ID, "Implement", 0)
	task1.AssigneeID = assignee.ID

	detail, err := e.svc.Get(context.Background(), e.actorA, e.wsA, board.ID)
	require.NoError(t, err)
	require.Len(t, detail.Columns, 2)
	require.Equal(t, 1, detail.Columns[0].TaskCount)
	require.Len(t, detail.Columns[0].Tasks, 1)
	require.NotNil(t, detail.Columns[0].Tasks[0].Assignee)
	assert.Equal(t, "dev", detail.Columns[0].Tasks[0].Assignee.Login)
}

func TestGetBoardFromOtherWorkspace(t *testing.T) {
	e := newEnv(t)
	board := e.svc.Boards.Boards.(*testutil.FakeBoardRepo).AddBoardFixture(testutil.NewUUID(), e.wsB, "Other")
	_, err := e.svc.Get(context.Background(), e.actorA, e.wsA, board.ID)
	if !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestGetBoardNonMember(t *testing.T) {
	e := newEnv(t)
	board := e.svc.Boards.Boards.(*testutil.FakeBoardRepo).AddBoardFixture(testutil.NewUUID(), e.wsA, "Sprint")
	outsider := testutil.NewUUID()
	_, err := e.svc.Get(context.Background(), outsider, e.wsA, board.ID)
	if !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestUpdateBoard(t *testing.T) {
	e := newEnv(t)
	board := e.svc.Boards.Boards.(*testutil.FakeBoardRepo).AddBoardFixture(testutil.NewUUID(), e.wsA, "Sprint")
	updated, err := e.svc.Update(context.Background(), e.actorA, e.wsA, board.ID, dboard.UpdateBoardRequest{Name: "Release"})
	require.NoError(t, err)
	assert.Equal(t, "Release", updated.Name)
	assert.Equal(t, core.EventBoardUpdated, e.hub.Messages[0].Type)
}

func TestUpdateBoardMemberForbidden(t *testing.T) {
	e := newEnv(t)
	board := e.svc.Boards.Boards.(*testutil.FakeBoardRepo).AddBoardFixture(testutil.NewUUID(), e.wsA, "Sprint")
	_, err := e.svc.Update(context.Background(), e.actorB, e.wsA, board.ID, dboard.UpdateBoardRequest{Name: "Release"})
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}

func TestDeleteBoard(t *testing.T) {
	e := newEnv(t)
	board := e.svc.Boards.Boards.(*testutil.FakeBoardRepo).AddBoardFixture(testutil.NewUUID(), e.wsA, "Sprint")
	require.NoError(t, e.svc.Delete(context.Background(), e.actorA, e.wsA, board.ID))
	assert.NotContains(t, e.svc.Boards.Boards.(*testutil.FakeBoardRepo).Boards, board.ID)
	assert.Equal(t, core.EventBoardDeleted, e.hub.Messages[0].Type)
}

func TestDeleteBoardFromOtherWorkspace(t *testing.T) {
	e := newEnv(t)
	board := e.svc.Boards.Boards.(*testutil.FakeBoardRepo).AddBoardFixture(testutil.NewUUID(), e.wsB, "Other")
	err := e.svc.Delete(context.Background(), e.actorA, e.wsA, board.ID)
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
}

func TestReorderBoards(t *testing.T) {
	e := newEnv(t)
	boards := e.svc.Boards.Boards.(*testutil.FakeBoardRepo)
	b1 := boards.AddBoardFixture(testutil.NewUUID(), e.wsA, "Alpha")
	b2 := boards.AddBoardFixture(testutil.NewUUID(), e.wsA, "Beta")
	b3 := boards.AddBoardFixture(testutil.NewUUID(), e.wsA, "Gamma")

	ordered, err := e.svc.Reorder(context.Background(), e.actorA, e.wsA, dboard.ReorderBoardsRequest{BoardIDs: []string{b3.ID, b1.ID, b2.ID}})
	require.NoError(t, err)
	require.Len(t, ordered, 3)
	assert.Equal(t, b3.ID, ordered[0].ID)
	assert.Equal(t, b1.ID, ordered[1].ID)
	assert.Equal(t, b2.ID, ordered[2].ID)
	assert.Equal(t, 0, ordered[0].Position)
	assert.Equal(t, 1, ordered[1].Position)
	assert.Equal(t, 2, ordered[2].Position)

	list, err := e.svc.List(context.Background(), e.actorA, e.wsA)
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.Equal(t, b3.ID, list[0].ID)
	assert.Equal(t, b2.ID, list[2].ID)
}

func TestReorderBoardsValidation(t *testing.T) {
	e := newEnv(t)
	boards := e.svc.Boards.Boards.(*testutil.FakeBoardRepo)
	b1 := boards.AddBoardFixture(testutil.NewUUID(), e.wsA, "Alpha")
	boards.AddBoardFixture(testutil.NewUUID(), e.wsA, "Beta")

	_, err := e.svc.Reorder(context.Background(), e.actorA, e.wsA, dboard.ReorderBoardsRequest{BoardIDs: []string{}})
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
	_, err = e.svc.Reorder(context.Background(), e.actorA, e.wsA, dboard.ReorderBoardsRequest{BoardIDs: []string{testutil.NewUUID()}})
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
	_, err = e.svc.Reorder(context.Background(), e.actorA, e.wsA, dboard.ReorderBoardsRequest{BoardIDs: []string{b1.ID, b1.ID}})
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
}

func TestReorderBoardsMemberForbidden(t *testing.T) {
	e := newEnv(t)
	boards := e.svc.Boards.Boards.(*testutil.FakeBoardRepo)
	b1 := boards.AddBoardFixture(testutil.NewUUID(), e.wsA, "Alpha")

	_, err := e.svc.Reorder(context.Background(), e.actorB, e.wsA, dboard.ReorderBoardsRequest{BoardIDs: []string{b1.ID}})
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}
