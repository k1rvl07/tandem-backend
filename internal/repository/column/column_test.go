package column

import (
	"testing"

	mcolumn "github.com/tandem/tandem/internal/domain/models/column"
	"github.com/tandem/tandem/internal/repository/board"
	"github.com/tandem/tandem/internal/repository/testutil"
	"github.com/tandem/tandem/internal/repository/workspace"
)

func TestColumnRepoCRUD(t *testing.T) {
	db := testutil.NewTestDB(t)
	columnRepo := NewColumnRepo(db)
	boardRepo := board.NewBoardRepo(db)

	ws := testutil.NewWorkspace(testutil.NewID(), "Team")
	testutil.Must(t, workspace.NewWorkspaceRepo(db).CreateWorkspace(testutil.TestCtx, ws))
	board := testutil.NewBoard(ws.ID, "Board", 0, true)
	testutil.Must(t, boardRepo.CreateBoard(testutil.TestCtx, board))

	c := &mcolumn.Column{ID: testutil.NewID(), BoardID: board.ID, Name: "Backlog", Position: 0}
	testutil.Must(t, columnRepo.CreateColumn(testutil.TestCtx, c))
	if c.CreatedAt.IsZero() {
		t.Fatalf("created_at not set")
	}

	got, err := columnRepo.FindColumnByID(testutil.TestCtx, c.ID)
	testutil.Must(t, err)
	if got.Name != "Backlog" {
		t.Fatalf("unexpected column: %+v", got)
	}
	testutil.MustNotFound(t, testutil.ErrOf(columnRepo.FindColumnByID(testutil.TestCtx, testutil.NewID())))

	c2 := &mcolumn.Column{ID: testutil.NewID(), BoardID: board.ID, Name: "Done", Position: 1}
	testutil.Must(t, columnRepo.CreateColumn(testutil.TestCtx, c2))

	listed, err := columnRepo.ListColumns(testutil.TestCtx, board.ID)
	testutil.Must(t, err)
	if len(listed) != 2 || listed[0].Name != "Backlog" {
		t.Fatalf("unexpected columns: %v", []string{listed[0].Name, listed[1].Name})
	}

	forWorkspace, err := columnRepo.ListColumnsForWorkspace(testutil.TestCtx, ws.ID)
	testutil.Must(t, err)
	if len(forWorkspace) != 2 {
		t.Fatalf("expected 2 columns for workspace, got %d", len(forWorkspace))
	}
}
