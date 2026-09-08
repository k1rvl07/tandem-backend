package repository

import (
	"testing"

	"github.com/tandem/tandem/internal/domain/models"
)

func TestColumnRepoCRUD(t *testing.T) {
	db := newTestDB(t)
	columnRepo := NewColumnRepo(db)
	boardRepo := NewBoardRepo(db)

	ws := newWorkspace(newID(), "Team")
	must(t, NewWorkspaceRepo(db).CreateWorkspace(testCtx, ws))
	board := newBoard(ws.ID, "Board", 0, true)
	must(t, boardRepo.CreateBoard(testCtx, board))

	c := &models.Column{ID: newID(), BoardID: board.ID, Name: "Backlog", Position: 0}
	must(t, columnRepo.CreateColumn(testCtx, c))
	if c.CreatedAt.IsZero() {
		t.Fatalf("created_at not set")
	}

	got, err := columnRepo.FindColumnByID(testCtx, c.ID)
	must(t, err)
	if got.Name != "Backlog" {
		t.Fatalf("unexpected column: %+v", got)
	}
	mustNotFound(t, errOf(columnRepo.FindColumnByID(testCtx, newID())))

	c2 := &models.Column{ID: newID(), BoardID: board.ID, Name: "Done", Position: 1}
	must(t, columnRepo.CreateColumn(testCtx, c2))

	listed, err := columnRepo.ListColumns(testCtx, board.ID)
	must(t, err)
	if len(listed) != 2 || listed[0].Name != "Backlog" {
		t.Fatalf("unexpected columns: %v", []string{listed[0].Name, listed[1].Name})
	}

	forWorkspace, err := columnRepo.ListColumnsForWorkspace(testCtx, ws.ID)
	must(t, err)
	if len(forWorkspace) != 2 {
		t.Fatalf("expected 2 columns for workspace, got %d", len(forWorkspace))
	}
}
