package favorite

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/usecase/testutil"
)

func TestAddAndRemoveFavorite(t *testing.T) {
	favorites := testutil.NewFakeFavoriteRepo()
	ws := testutil.NewFakeWorkspaceRepo()
	boards := testutil.NewFakeBoardRepo()
	svc := NewService(Deps{Favorites: favorites, Workspaces: ws, Boards: boards, Hub: testutil.NewFakeHub(), Cache: testutil.NewFakeCache()})

	user := testutil.NewUUID()
	wsID := testutil.NewUUID()
	ws.AddWorkspaceFixture(wsID, "Team")
	ws.AddMemberFixture(wsID, user, mworkspace.RoleOwner)
	boardID := testutil.NewUUID()
	boards.AddBoardFixture(boardID, wsID, "Board")

	err := svc.Add(context.Background(), user, "workspace", wsID)
	require.NoError(t, err)
	ok, _ := favorites.IsFavorite(context.Background(), user, "workspace", wsID)
	require.True(t, ok)
	err = svc.Add(context.Background(), user, "board", boardID)
	require.NoError(t, err)
	err = svc.Remove(context.Background(), user, "workspace", wsID)
	require.NoError(t, err)
	ok, _ = favorites.IsFavorite(context.Background(), user, "workspace", wsID)
	require.False(t, ok)
}

func TestAddFavoriteNonMember(t *testing.T) {
	favorites := testutil.NewFakeFavoriteRepo()
	ws := testutil.NewFakeWorkspaceRepo()
	boards := testutil.NewFakeBoardRepo()
	svc := NewService(Deps{Favorites: favorites, Workspaces: ws, Boards: boards, Hub: testutil.NewFakeHub(), Cache: testutil.NewFakeCache()})

	user := testutil.NewUUID()
	wsID := testutil.NewUUID()
	ws.AddWorkspaceFixture(wsID, "Team")
	err := svc.Add(context.Background(), user, "workspace", wsID)
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}

func TestAddFavoriteInvalidTarget(t *testing.T) {
	favorites := testutil.NewFakeFavoriteRepo()
	svc := NewService(Deps{Favorites: favorites, Workspaces: testutil.NewFakeWorkspaceRepo(), Boards: testutil.NewFakeBoardRepo(), Hub: testutil.NewFakeHub(), Cache: testutil.NewFakeCache()})
	err := svc.Add(context.Background(), testutil.NewUUID(), "unknown", testutil.NewUUID())
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
}

func TestAddFavoriteInvalidUUID(t *testing.T) {
	favorites := testutil.NewFakeFavoriteRepo()
	svc := NewService(Deps{Favorites: favorites, Workspaces: testutil.NewFakeWorkspaceRepo(), Boards: testutil.NewFakeBoardRepo(), Hub: testutil.NewFakeHub(), Cache: testutil.NewFakeCache()})
	err := svc.Add(context.Background(), testutil.NewUUID(), "workspace", "not-a-uuid")
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
	err = svc.Remove(context.Background(), testutil.NewUUID(), "workspace", "not-a-uuid")
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
}

func TestRemoveFavoriteInvalidTarget(t *testing.T) {
	svc := NewService(Deps{Favorites: testutil.NewFakeFavoriteRepo(), Workspaces: testutil.NewFakeWorkspaceRepo(), Boards: testutil.NewFakeBoardRepo(), Hub: testutil.NewFakeHub(), Cache: testutil.NewFakeCache()})
	err := svc.Remove(context.Background(), testutil.NewUUID(), "unknown", testutil.NewUUID())
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
}

func TestAddFavoriteWorkspaceRepoError(t *testing.T) {
	ws := testutil.NewFakeWorkspaceRepo()
	ws.Err = pkgerrors.NewValidationError("db")
	svc := NewService(Deps{Favorites: testutil.NewFakeFavoriteRepo(), Workspaces: ws, Boards: testutil.NewFakeBoardRepo(), Hub: testutil.NewFakeHub(), Cache: testutil.NewFakeCache()})
	err := svc.Add(context.Background(), testutil.NewUUID(), "workspace", testutil.NewUUID())
	require.Error(t, err)
}

func TestAddFavoriteBoardRepoError(t *testing.T) {
	boards := testutil.NewFakeBoardRepo()
	boards.Err = pkgerrors.NewValidationError("db")
	svc := NewService(Deps{Favorites: testutil.NewFakeFavoriteRepo(), Workspaces: testutil.NewFakeWorkspaceRepo(), Boards: boards, Hub: testutil.NewFakeHub(), Cache: testutil.NewFakeCache()})
	err := svc.Add(context.Background(), testutil.NewUUID(), "board", testutil.NewUUID())
	require.Error(t, err)
}

func TestAddFavoriteStoreError(t *testing.T) {
	ws := testutil.NewFakeWorkspaceRepo()
	user := testutil.NewUUID()
	wsID := testutil.NewUUID()
	ws.AddWorkspaceFixture(wsID, "Team")
	ws.AddMemberFixture(wsID, user, mworkspace.RoleOwner)
	favorites := testutil.NewFakeFavoriteRepo()
	favorites.Err = pkgerrors.NewValidationError("db")
	svc := NewService(Deps{Favorites: favorites, Workspaces: ws, Boards: testutil.NewFakeBoardRepo(), Hub: testutil.NewFakeHub(), Cache: testutil.NewFakeCache()})
	err := svc.Add(context.Background(), user, "workspace", wsID)
	require.Error(t, err)
}

func TestRemoveFavoriteStoreError(t *testing.T) {
	favorites := testutil.NewFakeFavoriteRepo()
	favorites.Err = pkgerrors.NewValidationError("db")
	svc := NewService(Deps{Favorites: favorites, Workspaces: testutil.NewFakeWorkspaceRepo(), Boards: testutil.NewFakeBoardRepo(), Hub: testutil.NewFakeHub(), Cache: testutil.NewFakeCache()})
	err := svc.Remove(context.Background(), testutil.NewUUID(), "workspace", testutil.NewUUID())
	require.Error(t, err)
}

func TestAddFavoriteNilHub(t *testing.T) {
	ws := testutil.NewFakeWorkspaceRepo()
	user := testutil.NewUUID()
	wsID := testutil.NewUUID()
	ws.AddWorkspaceFixture(wsID, "Team")
	ws.AddMemberFixture(wsID, user, mworkspace.RoleOwner)
	svc := NewService(Deps{Favorites: testutil.NewFakeFavoriteRepo(), Workspaces: ws, Boards: testutil.NewFakeBoardRepo(), Cache: testutil.NewFakeCache()})
	err := svc.Add(context.Background(), user, "workspace", wsID)
	require.NoError(t, err)
}
