package board

import (
	"testing"

	mattachment "github.com/tandem/tandem/internal/domain/models/attachment"
	mcolumn "github.com/tandem/tandem/internal/domain/models/column"
	mfavorite "github.com/tandem/tandem/internal/domain/models/favorite"
	mtask "github.com/tandem/tandem/internal/domain/models/task"
	"github.com/tandem/tandem/internal/repository/column"
	"github.com/tandem/tandem/internal/repository/task"
	"github.com/tandem/tandem/internal/repository/taskext"
	"github.com/tandem/tandem/internal/repository/testutil"
	"github.com/tandem/tandem/internal/repository/user"
	"github.com/tandem/tandem/internal/repository/workspace"
)

func TestBoardRepoCRUD(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewBoardRepo(db)

	ws := testutil.NewWorkspace(testutil.NewID(), "Team")
	testutil.Must(t, workspace.NewWorkspaceRepo(db).CreateWorkspace(testutil.TestCtx, ws))

	b := testutil.NewBoard(ws.ID, "Board", 1, true)
	testutil.Must(t, repo.CreateBoard(testutil.TestCtx, b))
	if b.CreatedAt.IsZero() {
		t.Fatalf("created_at not set")
	}

	got, err := repo.FindBoardByID(testutil.TestCtx, b.ID)
	testutil.Must(t, err)
	if got.Name != "Board" || !got.IsMain {
		t.Fatalf("unexpected board: %+v", got)
	}
	testutil.MustNotFound(t, testutil.ErrOf(repo.FindBoardByID(testutil.TestCtx, testutil.NewID())))

	b.Name = "Renamed"
	b.Position = 2
	b.IsMain = false
	testutil.Must(t, repo.UpdateBoard(testutil.TestCtx, b))
	got, err = repo.FindBoardByID(testutil.TestCtx, b.ID)
	testutil.Must(t, err)
	if got.Name != "Renamed" || got.Position != 2 || got.IsMain {
		t.Fatalf("update not applied: %+v", got)
	}

	testutil.Must(t, repo.ClearMainBoards(testutil.TestCtx, ws.ID))
	cleared, _ := repo.FindBoardByID(testutil.TestCtx, b.ID)
	if cleared.IsMain {
		t.Fatalf("ClearMainBoards did not clear is_main")
	}
}

func TestBoardRepoListAndReorder(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewBoardRepo(db)

	ws := testutil.NewWorkspace(testutil.NewID(), "Team")
	testutil.Must(t, workspace.NewWorkspaceRepo(db).CreateWorkspace(testutil.TestCtx, ws))

	b1 := testutil.NewBoard(ws.ID, "One", 0, true)
	b2 := testutil.NewBoard(ws.ID, "Two", 1, false)
	b3 := testutil.NewBoard(ws.ID, "Three", 2, false)
	testutil.Must(t, repo.CreateBoard(testutil.TestCtx, b1))
	testutil.Must(t, repo.CreateBoard(testutil.TestCtx, b2))
	testutil.Must(t, repo.CreateBoard(testutil.TestCtx, b3))

	listed, err := repo.ListBoards(testutil.TestCtx, ws.ID)
	testutil.Must(t, err)
	if len(listed) != 3 || listed[0].Name != "One" || listed[2].Name != "Three" {
		t.Fatalf("unexpected order: %v", []string{listed[0].Name, listed[1].Name, listed[2].Name})
	}

	testutil.Must(t, repo.ReorderBoards(testutil.TestCtx, ws.ID, []string{b3.ID, b1.ID, b2.ID}))
	listed, err = repo.ListBoards(testutil.TestCtx, ws.ID)
	testutil.Must(t, err)
	if listed[0].Name != "Three" || listed[1].Name != "One" || listed[2].Name != "Two" {
		t.Fatalf("reorder not applied")
	}
}

func TestBoardRepoCountTasks(t *testing.T) {
	db := testutil.NewTestDB(t)
	boardRepo := NewBoardRepo(db)
	columnRepo := column.NewColumnRepo(db)
	taskRepo := task.NewTaskRepo(db)

	ws := testutil.NewWorkspace(testutil.NewID(), "Team")
	testutil.Must(t, workspace.NewWorkspaceRepo(db).CreateWorkspace(testutil.TestCtx, ws))

	b1 := testutil.NewBoard(ws.ID, "One", 0, true)
	b2 := testutil.NewBoard(ws.ID, "Two", 1, false)
	testutil.Must(t, boardRepo.CreateBoard(testutil.TestCtx, b1))
	testutil.Must(t, boardRepo.CreateBoard(testutil.TestCtx, b2))

	var cols []*mcolumn.Column
	for i, name := range []string{"Backlog", "Done"} {
		c := &mcolumn.Column{ID: testutil.NewID(), BoardID: b1.ID, Name: name, Position: i}
		testutil.Must(t, columnRepo.CreateColumn(testutil.TestCtx, c))
		cols = append(cols, c)
	}
	for i := 0; i < 3; i++ {
		col := cols[i%2]
		testutil.Must(t, taskRepo.CreateTask(testutil.TestCtx, &mtask.Task{ID: testutil.NewID(), ColumnID: col.ID, Title: "T"}))
	}

	counts, err := boardRepo.CountTasksByBoard(testutil.TestCtx, ws.ID)
	testutil.Must(t, err)
	if counts[b1.ID] != 3 {
		t.Fatalf("expected 3 tasks for board, got %v", counts)
	}
	if _, ok := counts[b2.ID]; ok {
		t.Fatalf("unexpected board in counts: %v", counts)
	}
}

func TestBoardRepoDeleteCascades(t *testing.T) {
	db := testutil.NewTestDB(t)
	boardRepo := NewBoardRepo(db)
	columnRepo := column.NewColumnRepo(db)
	taskRepo := task.NewTaskRepo(db)
	attachmentRepo := taskext.NewAttachmentRepo(db)
	favoriteRepo := taskext.NewFavoriteRepo(db)
	userRepo := user.NewUserRepo(db)

	ws := testutil.NewWorkspace(testutil.NewID(), "Team")
	testutil.Must(t, workspace.NewWorkspaceRepo(db).CreateWorkspace(testutil.TestCtx, ws))
	u := testutil.NewUser(testutil.NewID(), "author")
	testutil.Must(t, userRepo.Create(testutil.TestCtx, u))

	board := testutil.NewBoard(ws.ID, "Board", 0, true)
	testutil.Must(t, boardRepo.CreateBoard(testutil.TestCtx, board))
	column := &mcolumn.Column{ID: testutil.NewID(), BoardID: board.ID, Name: "Done"}
	testutil.Must(t, columnRepo.CreateColumn(testutil.TestCtx, column))
	task := &mtask.Task{ID: testutil.NewID(), ColumnID: column.ID, Title: "Task"}
	testutil.Must(t, taskRepo.CreateTask(testutil.TestCtx, task))
	testutil.Must(t, attachmentRepo.CreateAttachment(testutil.TestCtx, &mattachment.TaskAttachment{ID: testutil.NewID(), TaskID: task.ID, Filename: "f", ObjectKey: "attachments/x", Size: 1, UploadedBy: u.ID}))
	testutil.Must(t, favoriteRepo.AddFavorite(testutil.TestCtx, &mfavorite.Favorite{ID: testutil.NewID(), UserID: u.ID, TargetType: mfavorite.FavoriteBoard, TargetID: board.ID}))

	testutil.Must(t, boardRepo.DeleteBoard(testutil.TestCtx, board.ID))

	testutil.MustNotFound(t, testutil.ErrOf(boardRepo.FindBoardByID(testutil.TestCtx, board.ID)))
	testutil.MustNotFound(t, testutil.ErrOf(columnRepo.FindColumnByID(testutil.TestCtx, column.ID)))
	testutil.MustNotFound(t, testutil.ErrOf(taskRepo.FindTaskByID(testutil.TestCtx, task.ID)))
	keys, err := taskRepo.CollectBoardKeys(testutil.TestCtx, board.ID)
	testutil.Must(t, err)
	if len(keys) != 0 {
		t.Fatalf("board attachments not cleaned: %v", keys)
	}
	fav, err := favoriteRepo.IsFavorite(testutil.TestCtx, u.ID, mfavorite.FavoriteBoard, board.ID)
	testutil.Must(t, err)
	if fav {
		t.Fatalf("board favorite not cleaned")
	}

	testutil.MustNotFound(t, boardRepo.DeleteBoard(testutil.TestCtx, board.ID))
}
