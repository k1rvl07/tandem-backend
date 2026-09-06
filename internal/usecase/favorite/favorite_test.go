package favorite

import (
	"context"
	"errors"
	"testing"

	"github.com/tandem/tandem/internal/domain/models"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/usecase/testutil"
)

func TestAddAndRemoveFavorite(t *testing.T) {
	favorites := testutil.NewFakeFavoriteRepo()
	ws := testutil.NewFakeWorkspaceRepo()
	boards := testutil.NewFakeBoardRepo()
	svc := NewService(favorites, ws, boards, testutil.NewFakeHub(), testutil.NewFakeCache())

	user := testutil.NewUUID()
	wsID := testutil.NewUUID()
	ws.AddWorkspaceFixture(wsID, "Team")
	ws.AddMemberFixture(wsID, user, models.RoleOwner)
	boardID := testutil.NewUUID()
	boards.AddBoardFixture(boardID, wsID, "Board")

	if err := svc.Add(context.Background(), user, "workspace", wsID); err != nil {
		t.Fatalf("add: %v", err)
	}
	ok, _ := favorites.IsFavorite(context.Background(), user, "workspace", wsID)
	if !ok {
		t.Fatalf("expected favorite present")
	}
	if err := svc.Add(context.Background(), user, "board", boardID); err != nil {
		t.Fatalf("add board: %v", err)
	}
	if err := svc.Remove(context.Background(), user, "workspace", wsID); err != nil {
		t.Fatalf("remove: %v", err)
	}
	ok, _ = favorites.IsFavorite(context.Background(), user, "workspace", wsID)
	if ok {
		t.Fatalf("expected favorite removed")
	}
}

func TestAddFavoriteNonMember(t *testing.T) {
	favorites := testutil.NewFakeFavoriteRepo()
	ws := testutil.NewFakeWorkspaceRepo()
	boards := testutil.NewFakeBoardRepo()
	svc := NewService(favorites, ws, boards, testutil.NewFakeHub(), testutil.NewFakeCache())

	user := testutil.NewUUID()
	wsID := testutil.NewUUID()
	ws.AddWorkspaceFixture(wsID, "Team")
	err := svc.Add(context.Background(), user, "workspace", wsID)
	if !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestAddFavoriteInvalidTarget(t *testing.T) {
	favorites := testutil.NewFakeFavoriteRepo()
	svc := NewService(favorites, testutil.NewFakeWorkspaceRepo(), testutil.NewFakeBoardRepo(), testutil.NewFakeHub(), testutil.NewFakeCache())
	err := svc.Add(context.Background(), testutil.NewUUID(), "unknown", testutil.NewUUID())
	if !errors.Is(err, pkgerrors.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}
