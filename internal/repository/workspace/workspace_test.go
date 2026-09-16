package workspace

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	mattachment "github.com/tandem/tandem/internal/domain/models/attachment"
	mboard "github.com/tandem/tandem/internal/domain/models/board"
	mcolumn "github.com/tandem/tandem/internal/domain/models/column"
	mfavorite "github.com/tandem/tandem/internal/domain/models/favorite"
	mtask "github.com/tandem/tandem/internal/domain/models/task"
	muser "github.com/tandem/tandem/internal/domain/models/user"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	"github.com/tandem/tandem/internal/repository/board"
	"github.com/tandem/tandem/internal/repository/column"
	"github.com/tandem/tandem/internal/repository/task"
	"github.com/tandem/tandem/internal/repository/taskext"
	"github.com/tandem/tandem/internal/repository/testutil"
	"github.com/tandem/tandem/internal/repository/user"
)

func TestWorkspaceRepoCRUD(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewWorkspaceRepo(db)

	ws := testutil.NewWorkspace(testutil.NewID(), "Team")
	testutil.Must(t, repo.CreateWorkspace(testutil.TestCtx, ws))
	if ws.CreatedAt.IsZero() {
		t.Fatalf("created_at not set")
	}

	got, err := repo.FindWorkspaceByID(testutil.TestCtx, ws.ID)
	testutil.Must(t, err)
	if got.Name != "Team" || got.Prefix != "TA" {
		t.Fatalf("unexpected workspace: %+v", got)
	}

	testutil.MustNotFound(t, testutil.ErrOf(repo.FindWorkspaceByID(testutil.TestCtx, testutil.NewID())))
	testutil.MustConflict(t, repo.CreateWorkspace(testutil.TestCtx, testutil.NewWorkspace(ws.ID, "Other")))
}

func TestWorkspaceRepoInvite(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewWorkspaceRepo(db)

	ws := testutil.NewWorkspace(testutil.NewID(), "Team")
	testutil.Must(t, repo.CreateWorkspace(testutil.TestCtx, ws))

	expires := time.Now().Add(24 * time.Hour)
	testutil.Must(t, repo.UpdateInvite(testutil.TestCtx, ws.ID, "tok123", &expires))
	testutil.MustNotFound(t, repo.UpdateInvite(testutil.TestCtx, testutil.NewID(), "tok", &expires))

	found, err := repo.FindWorkspaceByInvite(testutil.TestCtx, "tok123")
	testutil.Must(t, err)
	if found.ID != ws.ID {
		t.Fatalf("wrong workspace by invite")
	}
	reloaded, err := repo.FindWorkspaceByID(testutil.TestCtx, ws.ID)
	testutil.Must(t, err)
	if reloaded.InviteToken != "tok123" {
		t.Fatalf("invite token not persisted")
	}
	if reloaded.InviteExpiresAt == nil || reloaded.InviteExpiresAt.Sub(expires) > time.Second {
		t.Fatalf("invite expiry not persisted: %v", reloaded.InviteExpiresAt)
	}

	testutil.Must(t, repo.UpdateInvite(testutil.TestCtx, ws.ID, "", nil))
	testutil.MustNotFound(t, testutil.ErrOf(repo.FindWorkspaceByInvite(testutil.TestCtx, "tok123")))

	ws.Name = "Renamed"
	ws.Description = "new"
	ws.Prefix = "TB"
	ws.Theme = "red"
	testutil.Must(t, repo.UpdateWorkspace(testutil.TestCtx, ws))
	got, err := repo.FindWorkspaceByID(testutil.TestCtx, ws.ID)
	testutil.Must(t, err)
	if got.Name != "Renamed" || got.Theme != "red" {
		t.Fatalf("workspace update not applied: %+v", got)
	}
}

func TestWorkspaceRepoClaimInviteSingleWinner(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewWorkspaceRepo(db)

	ws := testutil.NewWorkspace(testutil.NewID(), "Team")
	testutil.Must(t, repo.CreateWorkspace(testutil.TestCtx, ws))
	testutil.Must(t, repo.UpdateInvite(testutil.TestCtx, ws.ID, "tok-single", nil))

	claimed, err := repo.ClaimInvite(testutil.TestCtx, ws.ID, "tok-single")
	testutil.Must(t, err)
	if !claimed {
		t.Fatalf("first claim must win")
	}
	claimed, err = repo.ClaimInvite(testutil.TestCtx, ws.ID, "tok-single")
	testutil.Must(t, err)
	if claimed {
		t.Fatalf("second claim must lose")
	}
	testutil.MustNotFound(t, testutil.ErrOf(repo.FindWorkspaceByInvite(testutil.TestCtx, "tok-single")))

	wrong, err := repo.ClaimInvite(testutil.TestCtx, ws.ID, "tok-wrong")
	testutil.Must(t, err)
	if wrong {
		t.Fatalf("wrong token must not claim")
	}
	reloaded, err := repo.FindWorkspaceByID(testutil.TestCtx, ws.ID)
	testutil.Must(t, err)
	if reloaded.InviteToken != "" {
		t.Fatalf("wrong token must not clear invite: %q", reloaded.InviteToken)
	}
	missing, err := repo.ClaimInvite(testutil.TestCtx, testutil.NewID(), "tok-single")
	testutil.Must(t, err)
	if missing {
		t.Fatalf("missing workspace must not claim")
	}
}

func TestWorkspaceRepoClaimInviteConcurrent(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewWorkspaceRepo(db)

	ws := testutil.NewWorkspace(testutil.NewID(), "Team")
	testutil.Must(t, repo.CreateWorkspace(testutil.TestCtx, ws))
	testutil.Must(t, repo.UpdateInvite(testutil.TestCtx, ws.ID, "tok-race", nil))

	const n = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	errs := 0
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := repo.ClaimInvite(context.Background(), ws.ID, "tok-race")
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs++
				return
			}
			if ok {
				wins++
			}
		}()
	}
	wg.Wait()
	if errs != 0 {
		t.Fatalf("unexpected claim errors: %d", errs)
	}
	if wins != 1 {
		t.Fatalf("expected exactly 1 winner, got %d", wins)
	}
	testutil.MustNotFound(t, testutil.ErrOf(repo.FindWorkspaceByInvite(testutil.TestCtx, "tok-race")))
}

func TestWorkspaceRepoClaimInviteAndAddMemberAtomic(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewWorkspaceRepo(db)
	userRepo := user.NewUserRepo(db)

	ws := testutil.NewWorkspace(testutil.NewID(), "Team")
	testutil.Must(t, repo.CreateWorkspace(testutil.TestCtx, ws))
	testutil.Must(t, repo.UpdateInvite(testutil.TestCtx, ws.ID, "tok-atomic", nil))

	userA := testutil.NewUser(testutil.NewID(), "joiner-a")
	userB := testutil.NewUser(testutil.NewID(), "joiner-b")
	testutil.Must(t, userRepo.Create(testutil.TestCtx, userA))
	testutil.Must(t, userRepo.Create(testutil.TestCtx, userB))

	claimed, added, err := repo.ClaimInviteAndAddMember(testutil.TestCtx, ws.ID, "tok-atomic", userA.ID, mworkspace.RoleMember)
	testutil.Must(t, err)
	if !claimed || !added {
		t.Fatalf("first join must claim and add, got claimed=%v added=%v", claimed, added)
	}
	claimed, added, err = repo.ClaimInviteAndAddMember(testutil.TestCtx, ws.ID, "tok-atomic", userA.ID, mworkspace.RoleMember)
	testutil.Must(t, err)
	if claimed || added {
		t.Fatalf("reused token must not claim/add again, got claimed=%v added=%v", claimed, added)
	}

	testutil.Must(t, repo.UpdateInvite(testutil.TestCtx, ws.ID, "tok-existing", nil))
	claimed, added, err = repo.ClaimInviteAndAddMember(testutil.TestCtx, ws.ID, "tok-existing", userA.ID, mworkspace.RoleMember)
	testutil.Must(t, err)
	if !claimed || added {
		t.Fatalf("existing member must be claimed but not added, got claimed=%v added=%v", claimed, added)
	}
	member, err := repo.FindMember(testutil.TestCtx, ws.ID, userA.ID)
	testutil.Must(t, err)
	if member.Role != mworkspace.RoleMember {
		t.Fatalf("unexpected role: %v", member.Role)
	}

	testutil.Must(t, repo.UpdateInvite(testutil.TestCtx, ws.ID, "tok-b", nil))
	claimed, added, err = repo.ClaimInviteAndAddMember(testutil.TestCtx, ws.ID, "tok-b", userB.ID, mworkspace.RoleMember)
	testutil.Must(t, err)
	if !claimed || !added {
		t.Fatalf("second user must be added, got claimed=%v added=%v", claimed, added)
	}
}

func TestWorkspaceRepoClaimInviteAndAddMemberConcurrent(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewWorkspaceRepo(db)
	userRepo := user.NewUserRepo(db)

	ws := testutil.NewWorkspace(testutil.NewID(), "Team")
	testutil.Must(t, repo.CreateWorkspace(testutil.TestCtx, ws))
	testutil.Must(t, repo.UpdateInvite(testutil.TestCtx, ws.ID, "tok-race2", nil))

	users := make([]*muser.User, 8)
	for i := range users {
		users[i] = testutil.NewUser(testutil.NewID(), fmt.Sprintf("race-joiner-%d", i))
		testutil.Must(t, userRepo.Create(testutil.TestCtx, users[i]))
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	addedTotal := 0
	claimedTotal := 0
	errs := 0
	for _, u := range users {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			claimed, added, err := repo.ClaimInviteAndAddMember(context.Background(), ws.ID, "tok-race2", id, mworkspace.RoleMember)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs++
				return
			}
			if claimed {
				claimedTotal++
			}
			if added {
				addedTotal++
			}
		}(u.ID)
	}
	wg.Wait()
	if errs != 0 {
		t.Fatalf("unexpected join errors: %d", errs)
	}
	if claimedTotal != 1 {
		t.Fatalf("expected exactly 1 claim winner, got %d", claimedTotal)
	}
	if addedTotal != 1 {
		t.Fatalf("expected exactly 1 member added, got %d", addedTotal)
	}
	testutil.MustNotFound(t, testutil.ErrOf(repo.FindWorkspaceByInvite(testutil.TestCtx, "tok-race2")))
}

func TestWorkspaceRepoMembers(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewWorkspaceRepo(db)
	userRepo := user.NewUserRepo(db)

	ws := testutil.NewWorkspace(testutil.NewID(), "Team")
	testutil.Must(t, repo.CreateWorkspace(testutil.TestCtx, ws))

	owner := testutil.NewUser(testutil.NewID(), "owner")
	member := testutil.NewUser(testutil.NewID(), "member")
	testutil.Must(t, userRepo.Create(testutil.TestCtx, owner))
	testutil.Must(t, userRepo.Create(testutil.TestCtx, member))

	testutil.Must(t, repo.AddMember(testutil.TestCtx, ws.ID, owner.ID, mworkspace.RoleOwner))
	testutil.Must(t, repo.AddMember(testutil.TestCtx, ws.ID, member.ID, mworkspace.RoleMember))
	testutil.MustConflict(t, repo.AddMember(testutil.TestCtx, ws.ID, member.ID, mworkspace.RoleMember))

	got, err := repo.FindMember(testutil.TestCtx, ws.ID, member.ID)
	testutil.Must(t, err)
	if got.Role != mworkspace.RoleMember {
		t.Fatalf("wrong member role: %v", got.Role)
	}
	testutil.MustNotFound(t, testutil.ErrOf(repo.FindMember(testutil.TestCtx, ws.ID, testutil.NewID())))

	memberships, err := repo.ListWorkspacesForUser(testutil.TestCtx, member.ID)
	testutil.Must(t, err)
	if len(memberships) != 1 || memberships[0].Workspace.ID != ws.ID || memberships[0].Role != mworkspace.RoleMember {
		t.Fatalf("unexpected memberships: %+v", memberships)
	}

	all, err := repo.ListMembers(testutil.TestCtx, ws.ID)
	testutil.Must(t, err)
	if len(all) != 2 {
		t.Fatalf("expected 2 members, got %d", len(all))
	}

	testutil.Must(t, repo.UpdateMemberRole(testutil.TestCtx, ws.ID, member.ID, mworkspace.RoleEditor))
	got, err = repo.FindMember(testutil.TestCtx, ws.ID, member.ID)
	testutil.Must(t, err)
	if got.Role != mworkspace.RoleEditor {
		t.Fatalf("role update not applied: %v", got.Role)
	}
	testutil.MustNotFound(t, repo.UpdateMemberRole(testutil.TestCtx, ws.ID, testutil.NewID(), mworkspace.RoleEditor))
}

func TestWorkspaceRepoTransferOwnership(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewWorkspaceRepo(db)
	userRepo := user.NewUserRepo(db)

	ws := testutil.NewWorkspace(testutil.NewID(), "Team")
	testutil.Must(t, repo.CreateWorkspace(testutil.TestCtx, ws))
	oldOwner := testutil.NewUser(testutil.NewID(), "oldo")
	newOwner := testutil.NewUser(testutil.NewID(), "newo")
	testutil.Must(t, userRepo.Create(testutil.TestCtx, oldOwner))
	testutil.Must(t, userRepo.Create(testutil.TestCtx, newOwner))
	testutil.Must(t, repo.AddMember(testutil.TestCtx, ws.ID, oldOwner.ID, mworkspace.RoleOwner))
	testutil.Must(t, repo.AddMember(testutil.TestCtx, ws.ID, newOwner.ID, mworkspace.RoleEditor))

	testutil.Must(t, repo.TransferOwnership(testutil.TestCtx, ws.ID, oldOwner.ID, newOwner.ID))
	old, _ := repo.FindMember(testutil.TestCtx, ws.ID, oldOwner.ID)
	neu, _ := repo.FindMember(testutil.TestCtx, ws.ID, newOwner.ID)
	if old.Role != mworkspace.RoleEditor || neu.Role != mworkspace.RoleOwner {
		t.Fatalf("ownership not transferred: old=%v new=%v", old.Role, neu.Role)
	}

	testutil.MustNotFound(t, repo.TransferOwnership(testutil.TestCtx, ws.ID, testutil.NewID(), newOwner.ID))
	testutil.MustNotFound(t, repo.TransferOwnership(testutil.TestCtx, ws.ID, oldOwner.ID, testutil.NewID()))
}

func TestWorkspaceRepoRemoveAndDeleteMembers(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewWorkspaceRepo(db)
	userRepo := user.NewUserRepo(db)

	wsA := testutil.NewWorkspace(testutil.NewID(), "A")
	wsB := testutil.NewWorkspace(testutil.NewID(), "B")
	testutil.Must(t, repo.CreateWorkspace(testutil.TestCtx, wsA))
	testutil.Must(t, repo.CreateWorkspace(testutil.TestCtx, wsB))
	u := testutil.NewUser(testutil.NewID(), "u")
	other := testutil.NewUser(testutil.NewID(), "other")
	testutil.Must(t, userRepo.Create(testutil.TestCtx, u))
	testutil.Must(t, userRepo.Create(testutil.TestCtx, other))

	testutil.Must(t, repo.AddMember(testutil.TestCtx, wsA.ID, u.ID, mworkspace.RoleMember))
	testutil.Must(t, repo.AddMember(testutil.TestCtx, wsB.ID, u.ID, mworkspace.RoleEditor))
	testutil.Must(t, repo.AddMember(testutil.TestCtx, wsB.ID, other.ID, mworkspace.RoleMember))

	testutil.Must(t, repo.RemoveMember(testutil.TestCtx, wsA.ID, u.ID))
	testutil.MustNotFound(t, repo.RemoveMember(testutil.TestCtx, wsA.ID, u.ID))
	testutil.MustNotFound(t, testutil.ErrOf(repo.FindMember(testutil.TestCtx, wsA.ID, u.ID)))

	testutil.Must(t, repo.DeleteMembersByUser(testutil.TestCtx, u.ID))
	testutil.MustNotFound(t, testutil.ErrOf(repo.FindMember(testutil.TestCtx, wsB.ID, u.ID)))
	leftover, err := repo.ListMembers(testutil.TestCtx, wsB.ID)
	testutil.Must(t, err)
	if len(leftover) != 1 || leftover[0].UserID != other.ID {
		t.Fatalf("expected unrelated member to remain, got %v", leftover)
	}

	testutil.Must(t, repo.DeleteMembersByWorkspace(testutil.TestCtx, wsB.ID))
	empty, err := repo.ListMembers(testutil.TestCtx, wsB.ID)
	testutil.Must(t, err)
	if len(empty) != 0 {
		t.Fatalf("expected no members after workspace cleanup, got %d", len(empty))
	}
}

func TestWorkspaceRepoDeleteCascades(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewWorkspaceRepo(db)
	userRepo := user.NewUserRepo(db)
	boardRepo := board.NewBoardRepo(db)
	columnRepo := column.NewColumnRepo(db)
	taskRepo := task.NewTaskRepo(db)
	attachmentRepo := taskext.NewAttachmentRepo(db)
	favoriteRepo := taskext.NewFavoriteRepo(db)

	ws := testutil.NewWorkspace(testutil.NewID(), "Team")
	testutil.Must(t, repo.CreateWorkspace(testutil.TestCtx, ws))
	u := testutil.NewUser(testutil.NewID(), "author")
	testutil.Must(t, userRepo.Create(testutil.TestCtx, u))
	testutil.Must(t, repo.AddMember(testutil.TestCtx, ws.ID, u.ID, mworkspace.RoleOwner))

	board := &mboard.Board{ID: testutil.NewID(), WorkspaceID: ws.ID, Name: "Board", IsMain: true}
	testutil.Must(t, boardRepo.CreateBoard(testutil.TestCtx, board))
	column := &mcolumn.Column{ID: testutil.NewID(), BoardID: board.ID, Name: "Done", Position: 0}
	testutil.Must(t, columnRepo.CreateColumn(testutil.TestCtx, column))
	task := &mtask.Task{ID: testutil.NewID(), ColumnID: column.ID, Title: "Task", ImageKey: "tasks/task.png"}
	testutil.Must(t, taskRepo.CreateTask(testutil.TestCtx, task))
	attachment := &mattachment.TaskAttachment{ID: testutil.NewID(), TaskID: task.ID, Filename: "f.txt", ObjectKey: "attachments/a.txt", Size: 1, ContentType: "text/plain", UploadedBy: u.ID}
	testutil.Must(t, attachmentRepo.CreateAttachment(testutil.TestCtx, attachment))
	testutil.Must(t, favoriteRepo.AddFavorite(testutil.TestCtx, &mfavorite.Favorite{ID: testutil.NewID(), UserID: u.ID, TargetType: mfavorite.FavoriteWorkspace, TargetID: ws.ID}))
	testutil.Must(t, favoriteRepo.AddFavorite(testutil.TestCtx, &mfavorite.Favorite{ID: testutil.NewID(), UserID: u.ID, TargetType: mfavorite.FavoriteBoard, TargetID: board.ID}))

	testutil.Must(t, repo.DeleteWorkspace(testutil.TestCtx, ws.ID))

	testutil.MustNotFound(t, testutil.ErrOf(repo.FindWorkspaceByID(testutil.TestCtx, ws.ID)))
	testutil.MustNotFound(t, testutil.ErrOf(boardRepo.FindBoardByID(testutil.TestCtx, board.ID)))
	testutil.MustNotFound(t, testutil.ErrOf(columnRepo.FindColumnByID(testutil.TestCtx, column.ID)))
	testutil.MustNotFound(t, testutil.ErrOf(taskRepo.FindTaskByID(testutil.TestCtx, task.ID)))
	keys, err := taskRepo.CollectTaskKeys(testutil.TestCtx, task.ID)
	testutil.Must(t, err)
	if len(keys) != 0 {
		t.Fatalf("attachments not cleaned: %v", keys)
	}
	var memberCount int64
	testutil.Must(t, db.Table("workspace_members").Where("workspace_id = ?", ws.ID).Count(&memberCount).Error)
	if memberCount != 0 {
		t.Fatalf("members not cleaned: %d", memberCount)
	}
	favWS, err := favoriteRepo.IsFavorite(testutil.TestCtx, u.ID, mfavorite.FavoriteWorkspace, ws.ID)
	testutil.Must(t, err)
	favBoard, err := favoriteRepo.IsFavorite(testutil.TestCtx, u.ID, mfavorite.FavoriteBoard, board.ID)
	testutil.Must(t, err)
	if favWS || favBoard {
		t.Fatalf("favorites not cleaned")
	}

	testutil.MustNotFound(t, repo.DeleteWorkspace(testutil.TestCtx, ws.ID))
}
