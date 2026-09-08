package repository

import (
	"testing"

	"github.com/tandem/tandem/internal/domain/models"
)

func TestAttachmentRepoCRUD(t *testing.T) {
	db := newTestDB(t)
	attachmentRepo := NewAttachmentRepo(db)
	taskRepo := NewTaskRepo(db)
	columnRepo := NewColumnRepo(db)
	boardRepo := NewBoardRepo(db)
	userRepo := NewUserRepo(db)

	ws := newWorkspace(newID(), "Team")
	must(t, NewWorkspaceRepo(db).CreateWorkspace(testCtx, ws))
	board := newBoard(ws.ID, "Board", 0, true)
	must(t, boardRepo.CreateBoard(testCtx, board))
	column := &models.Column{ID: newID(), BoardID: board.ID, Name: "Backlog"}
	must(t, columnRepo.CreateColumn(testCtx, column))
	task := &models.Task{ID: newID(), ColumnID: column.ID, Title: "Task"}
	must(t, taskRepo.CreateTask(testCtx, task))
	u := newUser(newID(), "uploader")
	must(t, userRepo.Create(testCtx, u))

	att := &models.TaskAttachment{ID: newID(), TaskID: task.ID, Filename: "a.txt", ObjectKey: "attachments/a.txt", Size: 12, ContentType: "text/plain", UploadedBy: u.ID}
	must(t, attachmentRepo.CreateAttachment(testCtx, att))
	if att.CreatedAt.IsZero() {
		t.Fatalf("created_at not set")
	}

	got, err := attachmentRepo.FindAttachmentByID(testCtx, att.ID)
	must(t, err)
	if got.Filename != "a.txt" || got.Size != 12 {
		t.Fatalf("unexpected attachment: %+v", got)
	}
	mustNotFound(t, errOf(attachmentRepo.FindAttachmentByID(testCtx, newID())))

	att2 := &models.TaskAttachment{ID: newID(), TaskID: task.ID, Filename: "b.txt", ObjectKey: "attachments/b.txt", Size: 3, UploadedBy: u.ID}
	must(t, attachmentRepo.CreateAttachment(testCtx, att2))

	listed, err := attachmentRepo.ListAttachmentsByTask(testCtx, task.ID)
	must(t, err)
	if len(listed) != 2 || listed[0].ID != att.ID {
		t.Fatalf("unexpected attachments: %v", listed)
	}

	must(t, attachmentRepo.DeleteAttachment(testCtx, att.ID))
	mustNotFound(t, errOf(attachmentRepo.FindAttachmentByID(testCtx, att.ID)))
	mustNotFound(t, attachmentRepo.DeleteAttachment(testCtx, att.ID))
}

func TestFavoriteRepoCRUD(t *testing.T) {
	db := newTestDB(t)
	favoriteRepo := NewFavoriteRepo(db)

	userID := newID()
	ws := newID()

	must(t, favoriteRepo.AddFavorite(testCtx, &models.Favorite{ID: newID(), UserID: userID, TargetType: models.FavoriteWorkspace, TargetID: ws}))
	must(t, favoriteRepo.AddFavorite(testCtx, &models.Favorite{ID: newID(), UserID: userID, TargetType: models.FavoriteWorkspace, TargetID: newID()}))
	must(t, favoriteRepo.AddFavorite(testCtx, &models.Favorite{ID: newID(), UserID: userID, TargetType: models.FavoriteBoard, TargetID: newID()}))

	ok, err := favoriteRepo.IsFavorite(testCtx, userID, models.FavoriteWorkspace, ws)
	must(t, err)
	if !ok {
		t.Fatalf("expected favorite present")
	}
	ok, err = favoriteRepo.IsFavorite(testCtx, newID(), models.FavoriteWorkspace, ws)
	must(t, err)
	if ok {
		t.Fatalf("expected favorite absent for other user")
	}

	targets, err := favoriteRepo.ListFavoriteTargets(testCtx, userID, models.FavoriteWorkspace)
	must(t, err)
	if len(targets) != 2 || !targets[ws] {
		t.Fatalf("unexpected targets: %v", targets)
	}

	must(t, favoriteRepo.RemoveFavorite(testCtx, userID, models.FavoriteWorkspace, ws))
	ok, err = favoriteRepo.IsFavorite(testCtx, userID, models.FavoriteWorkspace, ws)
	must(t, err)
	if ok {
		t.Fatalf("expected favorite removed")
	}
}

func TestFavoriteRepoDeleteUserFavorites(t *testing.T) {
	db := newTestDB(t)
	favoriteRepo := NewFavoriteRepo(db)

	userID := newID()
	other := newID()
	for i := 0; i < 3; i++ {
		must(t, favoriteRepo.AddFavorite(testCtx, &models.Favorite{ID: newID(), UserID: userID, TargetType: models.FavoriteWorkspace, TargetID: newID()}))
	}
	must(t, favoriteRepo.AddFavorite(testCtx, &models.Favorite{ID: newID(), UserID: other, TargetType: models.FavoriteBoard, TargetID: newID()}))

	must(t, favoriteRepo.DeleteUserFavorites(testCtx, userID))

	targets, err := favoriteRepo.ListFavoriteTargets(testCtx, userID, models.FavoriteWorkspace)
	must(t, err)
	if len(targets) != 0 {
		t.Fatalf("user favorites not cleaned: %v", targets)
	}
	others, err := favoriteRepo.ListFavoriteTargets(testCtx, other, models.FavoriteBoard)
	must(t, err)
	if len(others) != 1 {
		t.Fatalf("other user favorites removed: %v", others)
	}
}
