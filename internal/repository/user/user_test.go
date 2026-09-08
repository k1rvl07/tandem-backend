package user

import (
	"testing"

	muser "github.com/tandem/tandem/internal/domain/models/user"
	"github.com/tandem/tandem/internal/repository/testutil"
)

func TestUserRepoCRUD(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewUserRepo(db)

	u := testutil.NewUser(testutil.NewID(), "alice")
	testutil.Must(t, repo.Create(testutil.TestCtx, u))
	if u.CreatedAt.IsZero() {
		t.Fatalf("created_at not set")
	}

	got, err := repo.FindByID(testutil.TestCtx, u.ID)
	testutil.Must(t, err)
	if got.Login != "alice" || got.Role != muser.RoleUser {
		t.Fatalf("unexpected user: %+v", got)
	}

	testutil.MustConflict(t, repo.Create(testutil.TestCtx, testutil.NewUser(testutil.NewID(), "alice")))

	byLogin, err := repo.FindByLogin(testutil.TestCtx, "alice")
	testutil.Must(t, err)
	if byLogin.ID != u.ID {
		t.Fatalf("wrong user by login")
	}

	ok, err := repo.ExistsByLogin(testutil.TestCtx, "alice")
	testutil.Must(t, err)
	if !ok {
		t.Fatalf("expected login to exist")
	}
	ok, err = repo.ExistsByLogin(testutil.TestCtx, "missing")
	testutil.Must(t, err)
	if ok {
		t.Fatalf("expected login to be absent")
	}

	testutil.MustNotFound(t, testutil.ErrOf(repo.FindByID(testutil.TestCtx, testutil.NewID())))
	testutil.MustNotFound(t, testutil.ErrOf(repo.FindByLogin(testutil.TestCtx, "nobody")))
}

func TestUserRepoListAndPage(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewUserRepo(db)

	testutil.Must(t, repo.Create(testutil.TestCtx, testutil.NewUser(testutil.NewID(), "zorro")))
	testutil.Must(t, repo.Create(testutil.TestCtx, testutil.NewUser(testutil.NewID(), "amber")))
	testutil.Must(t, repo.Create(testutil.TestCtx, testutil.NewUser(testutil.NewID(), "ann")))
	testutil.Must(t, repo.Create(testutil.TestCtx, testutil.NewUser(testutil.NewID(), "bob")))

	all, err := repo.List(testutil.TestCtx)
	testutil.Must(t, err)
	if len(all) != 4 {
		t.Fatalf("expected 4 users, got %d", len(all))
	}
	seen := make(map[string]bool, len(all))
	for _, u := range all {
		seen[u.Login] = true
	}
	for _, login := range []string{"amber", "ann", "bob", "zorro"} {
		if !seen[login] {
			t.Fatalf("expected user %q in list", login)
		}
	}

	page, err := repo.ListPage(testutil.TestCtx, "a", 10, 0)
	testutil.Must(t, err)
	if len(page) != 2 {
		t.Fatalf("expected 2 matches for 'a', got %d", len(page))
	}

	total, err := repo.Count(testutil.TestCtx, "a")
	testutil.Must(t, err)
	if total != 2 {
		t.Fatalf("expected count 2, got %d", total)
	}
	allCount, err := repo.Count(testutil.TestCtx, "")
	testutil.Must(t, err)
	if allCount != 4 {
		t.Fatalf("expected count 4, got %d", allCount)
	}

	escaped, err := repo.ListPage(testutil.TestCtx, "%", 10, 0)
	testutil.Must(t, err)
	if len(escaped) != 0 {
		t.Fatalf("expected no matches for escaped wildcard, got %d", len(escaped))
	}
}

func TestUserRepoUpdateAndDelete(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewUserRepo(db)

	u := testutil.NewUser(testutil.NewID(), "carol")
	testutil.Must(t, repo.Create(testutil.TestCtx, u))

	u.DisplayName = "Caroline"
	u.Role = muser.RoleModerator
	u.AvatarKey = "avatars/carol.png"
	testutil.Must(t, repo.Update(testutil.TestCtx, u))

	got, err := repo.FindByID(testutil.TestCtx, u.ID)
	testutil.Must(t, err)
	if got.DisplayName != "Caroline" || got.Role != muser.RoleModerator || got.AvatarKey != "avatars/carol.png" {
		t.Fatalf("update not applied: %+v", got)
	}

	testutil.Must(t, repo.Delete(testutil.TestCtx, u.ID))
	testutil.MustNotFound(t, testutil.ErrOf(repo.FindByID(testutil.TestCtx, u.ID)))
	testutil.MustNotFound(t, repo.Delete(testutil.TestCtx, u.ID))
}
