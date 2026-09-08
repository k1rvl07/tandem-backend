package repository

import (
	"testing"
	"time"

	"github.com/tandem/tandem/internal/domain/models"
)

func newWorkspace(id, name string) *models.Workspace {
	return &models.Workspace{
		ID:          id,
		Name:        name,
		Description: "desc",
		Prefix:      "TA",
		Theme:       "blue",
	}
}

func TestWorkspaceRepoCRUD(t *testing.T) {
	db := newTestDB(t)
	repo := NewWorkspaceRepo(db)

	ws := newWorkspace(newID(), "Team")
	must(t, repo.CreateWorkspace(testCtx, ws))
	if ws.CreatedAt.IsZero() {
		t.Fatalf("created_at not set")
	}

	got, err := repo.FindWorkspaceByID(testCtx, ws.ID)
	must(t, err)
	if got.Name != "Team" || got.Prefix != "TA" {
		t.Fatalf("unexpected workspace: %+v", got)
	}

	mustNotFound(t, errOf(repo.FindWorkspaceByID(testCtx, newID())))
	mustConflict(t, repo.CreateWorkspace(testCtx, newWorkspace(ws.ID, "Other")))
}

func TestWorkspaceRepoInvite(t *testing.T) {
	db := newTestDB(t)
	repo := NewWorkspaceRepo(db)

	ws := newWorkspace(newID(), "Team")
	must(t, repo.CreateWorkspace(testCtx, ws))

	expires := time.Now().Add(24 * time.Hour)
	must(t, repo.UpdateInvite(testCtx, ws.ID, "tok123", &expires))
	mustNotFound(t, repo.UpdateInvite(testCtx, newID(), "tok", &expires))

	found, err := repo.FindWorkspaceByInvite(testCtx, "tok123")
	must(t, err)
	if found.ID != ws.ID {
		t.Fatalf("wrong workspace by invite")
	}
	reloaded, err := repo.FindWorkspaceByID(testCtx, ws.ID)
	must(t, err)
	if reloaded.InviteToken != "tok123" {
		t.Fatalf("invite token not persisted")
	}
	if reloaded.InviteExpiresAt == nil || reloaded.InviteExpiresAt.Sub(expires) > time.Second {
		t.Fatalf("invite expiry not persisted: %v", reloaded.InviteExpiresAt)
	}

	must(t, repo.UpdateInvite(testCtx, ws.ID, "", nil))
	mustNotFound(t, errOf(repo.FindWorkspaceByInvite(testCtx, "tok123")))

	ws.Name = "Renamed"
	ws.Description = "new"
	ws.Prefix = "TB"
	ws.Theme = "red"
	must(t, repo.UpdateWorkspace(testCtx, ws))
	got, err := repo.FindWorkspaceByID(testCtx, ws.ID)
	must(t, err)
	if got.Name != "Renamed" || got.Theme != "red" {
		t.Fatalf("workspace update not applied: %+v", got)
	}
}

func TestWorkspaceRepoMembers(t *testing.T) {
	db := newTestDB(t)
	repo := NewWorkspaceRepo(db)
	userRepo := NewUserRepo(db)

	ws := newWorkspace(newID(), "Team")
	must(t, repo.CreateWorkspace(testCtx, ws))

	owner := newUser(newID(), "owner")
	member := newUser(newID(), "member")
	must(t, userRepo.Create(testCtx, owner))
	must(t, userRepo.Create(testCtx, member))

	must(t, repo.AddMember(testCtx, ws.ID, owner.ID, models.RoleOwner))
	must(t, repo.AddMember(testCtx, ws.ID, member.ID, models.RoleMember))
	mustConflict(t, repo.AddMember(testCtx, ws.ID, member.ID, models.RoleMember))

	got, err := repo.FindMember(testCtx, ws.ID, member.ID)
	must(t, err)
	if got.Role != models.RoleMember {
		t.Fatalf("wrong member role: %v", got.Role)
	}
	mustNotFound(t, errOf(repo.FindMember(testCtx, ws.ID, newID())))

	memberships, err := repo.ListWorkspacesForUser(testCtx, member.ID)
	must(t, err)
	if len(memberships) != 1 || memberships[0].Workspace.ID != ws.ID || memberships[0].Role != models.RoleMember {
		t.Fatalf("unexpected memberships: %+v", memberships)
	}

	all, err := repo.ListMembers(testCtx, ws.ID)
	must(t, err)
	if len(all) != 2 {
		t.Fatalf("expected 2 members, got %d", len(all))
	}

	must(t, repo.UpdateMemberRole(testCtx, ws.ID, member.ID, models.RoleEditor))
	got, err = repo.FindMember(testCtx, ws.ID, member.ID)
	must(t, err)
	if got.Role != models.RoleEditor {
		t.Fatalf("role update not applied: %v", got.Role)
	}
	mustNotFound(t, repo.UpdateMemberRole(testCtx, ws.ID, newID(), models.RoleEditor))
}

func TestWorkspaceRepoTransferOwnership(t *testing.T) {
	db := newTestDB(t)
	repo := NewWorkspaceRepo(db)
	userRepo := NewUserRepo(db)

	ws := newWorkspace(newID(), "Team")
	must(t, repo.CreateWorkspace(testCtx, ws))
	oldOwner := newUser(newID(), "oldo")
	newOwner := newUser(newID(), "newo")
	must(t, userRepo.Create(testCtx, oldOwner))
	must(t, userRepo.Create(testCtx, newOwner))
	must(t, repo.AddMember(testCtx, ws.ID, oldOwner.ID, models.RoleOwner))
	must(t, repo.AddMember(testCtx, ws.ID, newOwner.ID, models.RoleEditor))

	must(t, repo.TransferOwnership(testCtx, ws.ID, oldOwner.ID, newOwner.ID))
	old, _ := repo.FindMember(testCtx, ws.ID, oldOwner.ID)
	neu, _ := repo.FindMember(testCtx, ws.ID, newOwner.ID)
	if old.Role != models.RoleEditor || neu.Role != models.RoleOwner {
		t.Fatalf("ownership not transferred: old=%v new=%v", old.Role, neu.Role)
	}

	mustNotFound(t, repo.TransferOwnership(testCtx, ws.ID, newID(), newOwner.ID))
	mustNotFound(t, repo.TransferOwnership(testCtx, ws.ID, oldOwner.ID, newID()))
}

func TestWorkspaceRepoRemoveAndDeleteMembers(t *testing.T) {
	db := newTestDB(t)
	repo := NewWorkspaceRepo(db)
	userRepo := NewUserRepo(db)

	wsA := newWorkspace(newID(), "A")
	wsB := newWorkspace(newID(), "B")
	must(t, repo.CreateWorkspace(testCtx, wsA))
	must(t, repo.CreateWorkspace(testCtx, wsB))
	u := newUser(newID(), "u")
	other := newID()
	must(t, userRepo.Create(testCtx, u))

	must(t, repo.AddMember(testCtx, wsA.ID, u.ID, models.RoleMember))
	must(t, repo.AddMember(testCtx, wsB.ID, u.ID, models.RoleEditor))
	must(t, repo.AddMember(testCtx, wsB.ID, other, models.RoleMember))

	must(t, repo.RemoveMember(testCtx, wsA.ID, u.ID))
	mustNotFound(t, repo.RemoveMember(testCtx, wsA.ID, u.ID))
	mustNotFound(t, errOf(repo.FindMember(testCtx, wsA.ID, u.ID)))

	must(t, repo.DeleteMembersByUser(testCtx, u.ID))
	mustNotFound(t, errOf(repo.FindMember(testCtx, wsB.ID, u.ID)))
	leftover, err := repo.ListMembers(testCtx, wsB.ID)
	must(t, err)
	if len(leftover) != 1 || leftover[0].UserID != other {
		t.Fatalf("expected unrelated member to remain, got %v", leftover)
	}

	must(t, repo.DeleteMembersByWorkspace(testCtx, wsB.ID))
	empty, err := repo.ListMembers(testCtx, wsB.ID)
	must(t, err)
	if len(empty) != 0 {
		t.Fatalf("expected no members after workspace cleanup, got %d", len(empty))
	}
}

func TestWorkspaceRepoDeleteCascades(t *testing.T) {
	db := newTestDB(t)
	repo := NewWorkspaceRepo(db)
	userRepo := NewUserRepo(db)
	boardRepo := NewBoardRepo(db)
	columnRepo := NewColumnRepo(db)
	taskRepo := NewTaskRepo(db)
	attachmentRepo := NewAttachmentRepo(db)
	favoriteRepo := NewFavoriteRepo(db)

	ws := newWorkspace(newID(), "Team")
	must(t, repo.CreateWorkspace(testCtx, ws))
	u := newUser(newID(), "author")
	must(t, userRepo.Create(testCtx, u))
	must(t, repo.AddMember(testCtx, ws.ID, u.ID, models.RoleOwner))

	board := &models.Board{ID: newID(), WorkspaceID: ws.ID, Name: "Board", IsMain: true}
	must(t, boardRepo.CreateBoard(testCtx, board))
	column := &models.Column{ID: newID(), BoardID: board.ID, Name: "Done", Position: 0}
	must(t, columnRepo.CreateColumn(testCtx, column))
	task := &models.Task{ID: newID(), ColumnID: column.ID, Title: "Task", ImageKey: "tasks/task.png"}
	must(t, taskRepo.CreateTask(testCtx, task))
	attachment := &models.TaskAttachment{ID: newID(), TaskID: task.ID, Filename: "f.txt", ObjectKey: "attachments/a.txt", Size: 1, ContentType: "text/plain", UploadedBy: u.ID}
	must(t, attachmentRepo.CreateAttachment(testCtx, attachment))
	must(t, favoriteRepo.AddFavorite(testCtx, &models.Favorite{ID: newID(), UserID: u.ID, TargetType: models.FavoriteWorkspace, TargetID: ws.ID}))
	must(t, favoriteRepo.AddFavorite(testCtx, &models.Favorite{ID: newID(), UserID: u.ID, TargetType: models.FavoriteBoard, TargetID: board.ID}))

	must(t, repo.DeleteWorkspace(testCtx, ws.ID))

	mustNotFound(t, errOf(repo.FindWorkspaceByID(testCtx, ws.ID)))
	mustNotFound(t, errOf(boardRepo.FindBoardByID(testCtx, board.ID)))
	mustNotFound(t, errOf(columnRepo.FindColumnByID(testCtx, column.ID)))
	mustNotFound(t, errOf(taskRepo.FindTaskByID(testCtx, task.ID)))
	keys, err := taskRepo.CollectTaskKeys(testCtx, task.ID)
	must(t, err)
	if len(keys) != 0 {
		t.Fatalf("attachments not cleaned: %v", keys)
	}
	var memberCount int64
	must(t, db.Table("workspace_members").Where("workspace_id = ?", ws.ID).Count(&memberCount).Error)
	if memberCount != 0 {
		t.Fatalf("members not cleaned: %d", memberCount)
	}
	favWS, err := favoriteRepo.IsFavorite(testCtx, u.ID, models.FavoriteWorkspace, ws.ID)
	must(t, err)
	favBoard, err := favoriteRepo.IsFavorite(testCtx, u.ID, models.FavoriteBoard, board.ID)
	must(t, err)
	if favWS || favBoard {
		t.Fatalf("favorites not cleaned")
	}

	mustNotFound(t, repo.DeleteWorkspace(testCtx, ws.ID))
}
