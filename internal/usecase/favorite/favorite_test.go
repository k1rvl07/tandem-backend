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
