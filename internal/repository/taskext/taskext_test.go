package taskext

import (
	"testing"

	mattachment "github.com/tandem/tandem/internal/domain/models/attachment"
	mcolumn "github.com/tandem/tandem/internal/domain/models/column"
	mfavorite "github.com/tandem/tandem/internal/domain/models/favorite"
	mtask "github.com/tandem/tandem/internal/domain/models/task"
	"github.com/tandem/tandem/internal/repository/board"
	"github.com/tandem/tandem/internal/repository/column"
	"github.com/tandem/tandem/internal/repository/task"
	"github.com/tandem/tandem/internal/repository/testutil"
	"github.com/tandem/tandem/internal/repository/user"
	"github.com/tandem/tandem/internal/repository/workspace"
)

func TestAttachmentRepoCRUD(t *testing.T) {
	db := testutil.NewTestDB(t)
	attachmentRepo := NewAttachmentRepo(db)
	taskRepo := task.NewTaskRepo(db)
	columnRepo := column.NewColumnRepo(db)
	boardRepo := board.NewBoardRepo(db)
	userRepo := user.NewUserRepo(db)

	ws := testutil.NewWorkspace(testutil.NewID(), "Team")
	testutil.Must(t, workspace.NewWorkspaceRepo(db).CreateWorkspace(testutil.TestCtx, ws))
	board := testutil.NewBoard(ws.ID, "Board", 0, true)
	testutil.Must(t, boardRepo.CreateBoard(testutil.TestCtx, board))
	column := &mcolumn.Column{ID: testutil.NewID(), BoardID: board.ID, Name: "Backlog"}
	testutil.Must(t, columnRepo.CreateColumn(testutil.TestCtx, column))
	task := &mtask.Task{ID: testutil.NewID(), ColumnID: column.ID, Title: "Task"}
	testutil.Must(t, taskRepo.CreateTask(testutil.TestCtx, task))
	u := testutil.NewUser(testutil.NewID(), "uploader")
	testutil.Must(t, userRepo.Create(testutil.TestCtx, u))

	att := &mattachment.TaskAttachment{ID: testutil.NewID(), TaskID: task.ID, Filename: "a.txt", ObjectKey: "attachments/a.txt", Size: 12, ContentType: "text/plain", UploadedBy: u.ID}
	testutil.Must(t, attachmentRepo.CreateAttachment(testutil.TestCtx, att))
	if att.CreatedAt.IsZero() {
		t.Fatalf("created_at not set")
	}

	got, err := attachmentRepo.FindAttachmentByID(testutil.TestCtx, att.ID)
	testutil.Must(t, err)
	if got.Filename != "a.txt" || got.Size != 12 {
		t.Fatalf("unexpected attachment: %+v", got)
	}
	testutil.MustNotFound(t, testutil.ErrOf(attachmentRepo.FindAttachmentByID(testutil.TestCtx, testutil.NewID())))

	att2 := &mattachment.TaskAttachment{ID: testutil.NewID(), TaskID: task.ID, Filename: "b.txt", ObjectKey: "attachments/b.txt", Size: 3, UploadedBy: u.ID}
	testutil.Must(t, attachmentRepo.CreateAttachment(testutil.TestCtx, att2))

	listed, err := attachmentRepo.ListAttachmentsByTask(testutil.TestCtx, task.ID)
	testutil.Must(t, err)
	if len(listed) != 2 || listed[0].ID != att.ID {
		t.Fatalf("unexpected attachments: %v", listed)
	}

	testutil.Must(t, attachmentRepo.DeleteAttachment(testutil.TestCtx, att.ID))
	testutil.MustNotFound(t, testutil.ErrOf(attachmentRepo.FindAttachmentByID(testutil.TestCtx, att.ID)))
	testutil.MustNotFound(t, attachmentRepo.DeleteAttachment(testutil.TestCtx, att.ID))
}

func TestFavoriteRepoCRUD(t *testing.T) {
	db := testutil.NewTestDB(t)
	favoriteRepo := NewFavoriteRepo(db)

	userID := testutil.NewID()
	ws := testutil.NewID()

	testutil.Must(t, favoriteRepo.AddFavorite(testutil.TestCtx, &mfavorite.Favorite{ID: testutil.NewID(), UserID: userID, TargetType: mfavorite.FavoriteWorkspace, TargetID: ws}))
	testutil.Must(t, favoriteRepo.AddFavorite(testutil.TestCtx, &mfavorite.Favorite{ID: testutil.NewID(), UserID: userID, TargetType: mfavorite.FavoriteWorkspace, TargetID: testutil.NewID()}))
	testutil.Must(t, favoriteRepo.AddFavorite(testutil.TestCtx, &mfavorite.Favorite{ID: testutil.NewID(), UserID: userID, TargetType: mfavorite.FavoriteBoard, TargetID: testutil.NewID()}))

	ok, err := favoriteRepo.IsFavorite(testutil.TestCtx, userID, mfavorite.FavoriteWorkspace, ws)
	testutil.Must(t, err)
	if !ok {
		t.Fatalf("expected favorite present")
	}
	ok, err = favoriteRepo.IsFavorite(testutil.TestCtx, testutil.NewID(), mfavorite.FavoriteWorkspace, ws)
	testutil.Must(t, err)
	if ok {
		t.Fatalf("expected favorite absent for other user")
	}

	targets, err := favoriteRepo.ListFavoriteTargets(testutil.TestCtx, userID, mfavorite.FavoriteWorkspace)
	testutil.Must(t, err)
	if len(targets) != 2 || !targets[ws] {
		t.Fatalf("unexpected targets: %v", targets)
	}

	testutil.Must(t, favoriteRepo.RemoveFavorite(testutil.TestCtx, userID, mfavorite.FavoriteWorkspace, ws))
	ok, err = favoriteRepo.IsFavorite(testutil.TestCtx, userID, mfavorite.FavoriteWorkspace, ws)
	testutil.Must(t, err)
	if ok {
		t.Fatalf("expected favorite removed")
	}
}

func TestFavoriteRepoDeleteUserFavorites(t *testing.T) {
	db := testutil.NewTestDB(t)
	favoriteRepo := NewFavoriteRepo(db)

	userID := testutil.NewID()
	other := testutil.NewID()
	for i := 0; i < 3; i++ {
		testutil.Must(t, favoriteRepo.AddFavorite(testutil.TestCtx, &mfavorite.Favorite{ID: testutil.NewID(), UserID: userID, TargetType: mfavorite.FavoriteWorkspace, TargetID: testutil.NewID()}))
	}
	testutil.Must(t, favoriteRepo.AddFavorite(testutil.TestCtx, &mfavorite.Favorite{ID: testutil.NewID(), UserID: other, TargetType: mfavorite.FavoriteBoard, TargetID: testutil.NewID()}))

	testutil.Must(t, favoriteRepo.DeleteUserFavorites(testutil.TestCtx, userID))

	targets, err := favoriteRepo.ListFavoriteTargets(testutil.TestCtx, userID, mfavorite.FavoriteWorkspace)
	testutil.Must(t, err)
	if len(targets) != 0 {
		t.Fatalf("user favorites not cleaned: %v", targets)
	}
	others, err := favoriteRepo.ListFavoriteTargets(testutil.TestCtx, other, mfavorite.FavoriteBoard)
	testutil.Must(t, err)
	if len(others) != 1 {
		t.Fatalf("other user favorites removed: %v", others)
	}
}
