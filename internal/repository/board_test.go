package repository

import (
	"testing"

	"github.com/tandem/tandem/internal/domain/models"
)

func newBoard(wsID, name string, position int, main bool) *models.Board {
	return &models.Board{ID: newID(), WorkspaceID: wsID, Name: name, Position: position, IsMain: main}
}

func TestBoardRepoCRUD(t *testing.T) {
	db := newTestDB(t)
	repo := NewBoardRepo(db)

	ws := newWorkspace(newID(), "Team")
	must(t, NewWorkspaceRepo(db).CreateWorkspace(testCtx, ws))

	b := newBoard(ws.ID, "Board", 1, true)
	must(t, repo.CreateBoard(testCtx, b))
	if b.CreatedAt.IsZero() {
		t.Fatalf("created_at not set")
	}

	got, err := repo.FindBoardByID(testCtx, b.ID)
	must(t, err)
	if got.Name != "Board" || !got.IsMain {
		t.Fatalf("unexpected board: %+v", got)
	}
	mustNotFound(t, errOf(repo.FindBoardByID(testCtx, newID())))

	b.Name = "Renamed"
	b.Position = 2
	b.IsMain = false
	must(t, repo.UpdateBoard(testCtx, b))
	got, err = repo.FindBoardByID(testCtx, b.ID)
	must(t, err)
	if got.Name != "Renamed" || got.Position != 2 || got.IsMain {
		t.Fatalf("update not applied: %+v", got)
	}

	must(t, repo.ClearMainBoards(testCtx, ws.ID))
	cleared, _ := repo.FindBoardByID(testCtx, b.ID)
	if cleared.IsMain {
		t.Fatalf("ClearMainBoards did not clear is_main")
	}
}

func TestBoardRepoListAndReorder(t *testing.T) {
	db := newTestDB(t)
	repo := NewBoardRepo(db)

	ws := newWorkspace(newID(), "Team")
	must(t, NewWorkspaceRepo(db).CreateWorkspace(testCtx, ws))

	b1 := newBoard(ws.ID, "One", 0, true)
	b2 := newBoard(ws.ID, "Two", 1, false)
	b3 := newBoard(ws.ID, "Three", 2, false)
	must(t, repo.CreateBoard(testCtx, b1))
	must(t, repo.CreateBoard(testCtx, b2))
	must(t, repo.CreateBoard(testCtx, b3))

	listed, err := repo.ListBoards(testCtx, ws.ID)
	must(t, err)
	if len(listed) != 3 || listed[0].Name != "One" || listed[2].Name != "Three" {
		t.Fatalf("unexpected order: %v", []string{listed[0].Name, listed[1].Name, listed[2].Name})
	}

	must(t, repo.ReorderBoards(testCtx, ws.ID, []string{b3.ID, b1.ID, b2.ID}))
	listed, err = repo.ListBoards(testCtx, ws.ID)
	must(t, err)
	if listed[0].Name != "Three" || listed[1].Name != "One" || listed[2].Name != "Two" {
		t.Fatalf("reorder not applied")
	}
}

func TestBoardRepoCountTasks(t *testing.T) {
	db := newTestDB(t)
	boardRepo := NewBoardRepo(db)
	columnRepo := NewColumnRepo(db)
	taskRepo := NewTaskRepo(db)

	ws := newWorkspace(newID(), "Team")
	must(t, NewWorkspaceRepo(db).CreateWorkspace(testCtx, ws))

	b1 := newBoard(ws.ID, "One", 0, true)
	b2 := newBoard(ws.ID, "Two", 1, false)
	must(t, boardRepo.CreateBoard(testCtx, b1))
	must(t, boardRepo.CreateBoard(testCtx, b2))

	var cols []*models.Column
	for i, name := range []string{"Backlog", "Done"} {
		c := &models.Column{ID: newID(), BoardID: b1.ID, Name: name, Position: i}
		must(t, columnRepo.CreateColumn(testCtx, c))
		cols = append(cols, c)
	}
	for i := 0; i < 3; i++ {
		col := cols[i%2]
		must(t, taskRepo.CreateTask(testCtx, &models.Task{ID: newID(), ColumnID: col.ID, Title: "T"}))
	}

	counts, err := boardRepo.CountTasksByBoard(testCtx, ws.ID)
	must(t, err)
	if counts[b1.ID] != 3 {
		t.Fatalf("expected 3 tasks for board, got %v", counts)
	}
	if _, ok := counts[b2.ID]; ok {
		t.Fatalf("unexpected board in counts: %v", counts)
	}
}

func TestBoardRepoDeleteCascades(t *testing.T) {
	db := newTestDB(t)
	boardRepo := NewBoardRepo(db)
	columnRepo := NewColumnRepo(db)
	taskRepo := NewTaskRepo(db)
	attachmentRepo := NewAttachmentRepo(db)
	favoriteRepo := NewFavoriteRepo(db)
	userRepo := NewUserRepo(db)

	ws := newWorkspace(newID(), "Team")
	must(t, NewWorkspaceRepo(db).CreateWorkspace(testCtx, ws))
	u := newUser(newID(), "author")
	must(t, userRepo.Create(testCtx, u))

	board := newBoard(ws.ID, "Board", 0, true)
	must(t, boardRepo.CreateBoard(testCtx, board))
	column := &models.Column{ID: newID(), BoardID: board.ID, Name: "Done"}
	must(t, columnRepo.CreateColumn(testCtx, column))
	task := &models.Task{ID: newID(), ColumnID: column.ID, Title: "Task"}
	must(t, taskRepo.CreateTask(testCtx, task))
	must(t, attachmentRepo.CreateAttachment(testCtx, &models.TaskAttachment{ID: newID(), TaskID: task.ID, Filename: "f", ObjectKey: "attachments/x", Size: 1, UploadedBy: u.ID}))
	must(t, favoriteRepo.AddFavorite(testCtx, &models.Favorite{ID: newID(), UserID: u.ID, TargetType: models.FavoriteBoard, TargetID: board.ID}))

	must(t, boardRepo.DeleteBoard(testCtx, board.ID))

	mustNotFound(t, errOf(boardRepo.FindBoardByID(testCtx, board.ID)))
	mustNotFound(t, errOf(columnRepo.FindColumnByID(testCtx, column.ID)))
	mustNotFound(t, errOf(taskRepo.FindTaskByID(testCtx, task.ID)))
	keys, err := taskRepo.CollectBoardKeys(testCtx, board.ID)
	must(t, err)
	if len(keys) != 0 {
		t.Fatalf("board attachments not cleaned: %v", keys)
	}
	fav, err := favoriteRepo.IsFavorite(testCtx, u.ID, models.FavoriteBoard, board.ID)
	must(t, err)
	if fav {
		t.Fatalf("board favorite not cleaned")
	}

	mustNotFound(t, boardRepo.DeleteBoard(testCtx, board.ID))
}
