package repository

import (
	"testing"

	"github.com/tandem/tandem/internal/domain/models"
)

func newUser(id, login string) *models.User {
	return &models.User{
		ID:           id,
		Login:        login,
		PasswordHash: "hash",
		Role:         models.RoleUser,
		DisplayName:  "User " + login,
		Bio:          "bio",
		AvatarKey:    "",
	}
}

func TestUserRepoCRUD(t *testing.T) {
	db := newTestDB(t)
	repo := NewUserRepo(db)

	u := newUser(newID(), "alice")
	must(t, repo.Create(testCtx, u))
	if u.CreatedAt.IsZero() {
		t.Fatalf("created_at not set")
	}

	got, err := repo.FindByID(testCtx, u.ID)
	must(t, err)
	if got.Login != "alice" || got.Role != models.RoleUser {
		t.Fatalf("unexpected user: %+v", got)
	}

	mustConflict(t, repo.Create(testCtx, newUser(newID(), "alice")))

	byLogin, err := repo.FindByLogin(testCtx, "alice")
	must(t, err)
	if byLogin.ID != u.ID {
		t.Fatalf("wrong user by login")
	}

	ok, err := repo.ExistsByLogin(testCtx, "alice")
	must(t, err)
	if !ok {
		t.Fatalf("expected login to exist")
	}
	ok, err = repo.ExistsByLogin(testCtx, "missing")
	must(t, err)
	if ok {
		t.Fatalf("expected login to be absent")
	}

	mustNotFound(t, errOf(repo.FindByID(testCtx, newID())))
	mustNotFound(t, errOf(repo.FindByLogin(testCtx, "nobody")))
}

func TestUserRepoListAndPage(t *testing.T) {
	db := newTestDB(t)
	repo := NewUserRepo(db)

	must(t, repo.Create(testCtx, newUser(newID(), "zorro")))
	must(t, repo.Create(testCtx, newUser(newID(), "amber")))
	must(t, repo.Create(testCtx, newUser(newID(), "ann")))
	must(t, repo.Create(testCtx, newUser(newID(), "bob")))

	all, err := repo.List(testCtx)
	must(t, err)
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

	page, err := repo.ListPage(testCtx, "a", 10, 0)
	must(t, err)
	if len(page) != 2 {
		t.Fatalf("expected 2 matches for 'a', got %d", len(page))
	}

	total, err := repo.Count(testCtx, "a")
	must(t, err)
	if total != 2 {
		t.Fatalf("expected count 2, got %d", total)
	}
	allCount, err := repo.Count(testCtx, "")
	must(t, err)
	if allCount != 4 {
		t.Fatalf("expected count 4, got %d", allCount)
	}

	escaped, err := repo.ListPage(testCtx, "%", 10, 0)
	must(t, err)
	if len(escaped) != 0 {
		t.Fatalf("expected no matches for escaped wildcard, got %d", len(escaped))
	}
}

func TestUserRepoUpdateAndDelete(t *testing.T) {
	db := newTestDB(t)
	repo := NewUserRepo(db)

	u := newUser(newID(), "carol")
	must(t, repo.Create(testCtx, u))

	u.DisplayName = "Caroline"
	u.Role = models.RoleModerator
	u.AvatarKey = "avatars/carol.png"
	must(t, repo.Update(testCtx, u))

	got, err := repo.FindByID(testCtx, u.ID)
	must(t, err)
	if got.DisplayName != "Caroline" || got.Role != models.RoleModerator || got.AvatarKey != "avatars/carol.png" {
		t.Fatalf("update not applied: %+v", got)
	}

	must(t, repo.Delete(testCtx, u.ID))
	mustNotFound(t, errOf(repo.FindByID(testCtx, u.ID)))
	mustNotFound(t, repo.Delete(testCtx, u.ID))
}
