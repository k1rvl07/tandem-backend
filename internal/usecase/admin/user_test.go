package admin

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tandem/tandem/internal/domain/models"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/http/dto"
	"github.com/tandem/tandem/internal/infrastructure/password"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/usecase/testutil"
)

type fakeRepo struct {
	users map[string]*models.User
	seq   int
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{users: make(map[string]*models.User)}
}

func (f *fakeRepo) Create(_ context.Context, user *models.User) error {
	if _, ok := f.users[user.Login]; ok {
		return errors.New("unique constraint")
	}
	f.users[user.Login] = user
	return nil
}

func (f *fakeRepo) FindByID(_ context.Context, id string) (*models.User, error) {
	for _, u := range f.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, pkgerrors.ErrNotFound
}

func (f *fakeRepo) FindByLogin(_ context.Context, login string) (*models.User, error) {
	u, ok := f.users[login]
	if !ok {
		return nil, pkgerrors.ErrNotFound
	}
	return u, nil
}

func (f *fakeRepo) ExistsByLogin(_ context.Context, login string) (bool, error) {
	_, ok := f.users[login]
	return ok, nil
}

func (f *fakeRepo) List(_ context.Context) ([]*models.User, error) {
	users := make([]*models.User, 0, len(f.users))
	for _, u := range f.users {
		users = append(users, u)
	}
	return users, nil
}

func (f *fakeRepo) ListPage(_ context.Context, query string, limit, offset int) ([]*models.User, error) {
	users := make([]*models.User, 0, len(f.users))
	for _, u := range f.users {
		if query != "" && !strings.Contains(u.Login, query) {
			continue
		}
		users = append(users, u)
	}
	sort.Slice(users, func(i, j int) bool {
		return users[i].CreatedAt.Before(users[j].CreatedAt)
	})
	if offset > len(users) {
		offset = len(users)
	}
	end := offset + limit
	if end > len(users) {
		end = len(users)
	}
	return users[offset:end], nil
}

func (f *fakeRepo) Count(_ context.Context, query string) (int, error) {
	total := 0
	for login := range f.users {
		if query == "" || strings.Contains(login, query) {
			total++
		}
	}
	return total, nil
}

func (f *fakeRepo) Update(_ context.Context, user *models.User) error {
	f.users[user.Login] = user
	return nil
}

func (f *fakeRepo) Delete(_ context.Context, id string) error {
	for login, u := range f.users {
		if u.ID == id {
			delete(f.users, login)
			return nil
		}
	}
	return pkgerrors.ErrNotFound
}

func (f *fakeRepo) seed(login, role string) string {
	id := uuid.New().String()
	f.seq++
	f.users[login] = &models.User{ID: id, Login: login, Role: role, DisplayName: login, CreatedAt: time.Unix(0, int64(f.seq))}
	return id
}

func newTestService(repo repository.UserRepository) *Service {
	return NewService(repo, password.NewBCryptHasher(12), testutil.NewFakeCache())
}

func TestCreateUserSuccess(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)
	actor := Actor{ID: "admin-1", Role: models.RoleAdmin}

	resp, err := svc.CreateUser(context.Background(), actor, dto.CreateUserRequest{
		Login:    " Ivanov.II ",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Login != "ivanov.ii" {
		t.Errorf("expected normalized login, got %q", resp.Login)
	}
	if resp.DisplayName != "ivanov.ii" {
		t.Errorf("expected default display name = login, got %q", resp.DisplayName)
	}
	if resp.Role != models.RoleUser {
		t.Errorf("expected role user, got %q", resp.Role)
	}
	stored, err := repo.FindByLogin(context.Background(), "ivanov.ii")
	if err != nil {
		t.Fatalf("find failed: %v", err)
	}
	if stored.PasswordHash == "password123" {
		t.Error("password must be hashed")
	}
	if !password.NewBCryptHasher(12).Check(stored.PasswordHash, "password123") {
		t.Error("stored hash should verify against the plain password")
	}
}

func TestCreateUserAdminCreatesModerator(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)

	resp, err := svc.CreateUser(context.Background(), Actor{ID: "admin-1", Role: models.RoleAdmin}, dto.CreateUserRequest{
		Login:       "mod.one",
		Password:    "password123",
		DisplayName: "Mod One",
		Role:        models.RoleModerator,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Role != models.RoleModerator {
		t.Errorf("expected role moderator, got %q", resp.Role)
	}
}

func TestCreateUserModeratorCreatesUser(t *testing.T) {
	repo := newFakeRepo()
	repo.seed("mod.one", models.RoleModerator)
	svc := newTestService(repo)

	resp, err := svc.CreateUser(context.Background(), Actor{ID: "mod-1", Role: models.RoleModerator}, dto.CreateUserRequest{
		Login:    "user.one",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Role != models.RoleUser {
		t.Errorf("expected role user, got %q", resp.Role)
	}
}

func TestCreateUserModeratorCannotCreateModerator(t *testing.T) {
	repo := newFakeRepo()
	repo.seed("mod.one", models.RoleModerator)
	svc := newTestService(repo)

	_, err := svc.CreateUser(context.Background(), Actor{ID: "mod-1", Role: models.RoleModerator}, dto.CreateUserRequest{
		Login:    "mod.two",
		Password: "password123",
		Role:     models.RoleModerator,
	})
	if !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestCreateUserInvalidRole(t *testing.T) {
	svc := newTestService(newFakeRepo())

	_, err := svc.CreateUser(context.Background(), Actor{ID: "admin-1", Role: models.RoleAdmin}, dto.CreateUserRequest{
		Login:    "user.one",
		Password: "password123",
		Role:     models.RoleAdmin,
	})
	if !errors.Is(err, pkgerrors.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestCreateUserCustomDisplayName(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)
	actor := Actor{ID: "admin-1", Role: models.RoleAdmin}

	resp, err := svc.CreateUser(context.Background(), actor, dto.CreateUserRequest{
		Login:       "ivanov.ii",
		Password:    "password123",
		DisplayName: "Ivan Ivanov",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.DisplayName != "Ivan Ivanov" {
		t.Errorf("expected custom display name, got %q", resp.DisplayName)
	}
}

func TestCreateUserDuplicate(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)
	actor := Actor{ID: "admin-1", Role: models.RoleAdmin}

	req := dto.CreateUserRequest{Login: "ivanov.ii", Password: "password123"}
	if _, err := svc.CreateUser(context.Background(), actor, req); err != nil {
		t.Fatalf("first create failed: %v", err)
	}
	if _, err := svc.CreateUser(context.Background(), actor, req); err == nil {
		t.Fatal("expected conflict on duplicate login")
	}
}

func TestCreateUserValidation(t *testing.T) {
	svc := newTestService(newFakeRepo())
	actor := Actor{ID: "admin-1", Role: models.RoleAdmin}

	cases := []dto.CreateUserRequest{
		{Login: "", Password: "password123"},
		{Login: "ab", Password: "password123"},
		{Login: "bad login!", Password: "password123"},
		{Login: "valid.login", Password: "short"},
		{Login: "valid.login", Password: ""},
	}
	for _, req := range cases {
		if _, err := svc.CreateUser(context.Background(), actor, req); err == nil {
			t.Errorf("expected validation error for %+v", req)
		}
	}
}

func TestListUsers(t *testing.T) {
	repo := newFakeRepo()
	repo.seed("ivanov.ii", models.RoleUser)
	repo.seed("petrov.ii", models.RoleUser)
	svc := newTestService(repo)

	page, err := svc.ListUsers(context.Background(), "actor-id", dto.AdminListQuery{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.Total != 2 {
		t.Errorf("expected total 2, got %d", page.Total)
	}
	if len(page.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(page.Items))
	}
	if page.Page != 1 || page.PageSize != 20 {
		t.Errorf("expected default page 1 / size 20, got %d / %d", page.Page, page.PageSize)
	}
}

func TestListUsersPagination(t *testing.T) {
	repo := newFakeRepo()
	for i := 1; i <= 5; i++ {
		repo.seed(fmt.Sprintf("user.%02d", i), models.RoleUser)
	}
	svc := newTestService(repo)

	page1, err := svc.ListUsers(context.Background(), "actor-id", dto.AdminListQuery{Page: 1, PageSize: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page1.Total != 5 || len(page1.Items) != 2 {
		t.Fatalf("expected total 5 and 2 items on page 1, got total %d len %d", page1.Total, len(page1.Items))
	}
	if page1.Items[0].Login != "user.01" || page1.Items[1].Login != "user.02" {
		t.Errorf("unexpected page 1 order: %v", []string{page1.Items[0].Login, page1.Items[1].Login})
	}

	page3, err := svc.ListUsers(context.Background(), "actor-id", dto.AdminListQuery{Page: 3, PageSize: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page3.Items) != 1 || page3.Items[0].Login != "user.05" {
		t.Errorf("expected single user.05 on page 3, got %v", page3.Items)
	}

	offPage, err := svc.ListUsers(context.Background(), "actor-id", dto.AdminListQuery{Page: 99, PageSize: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(offPage.Items) != 0 {
		t.Errorf("expected empty items beyond last page, got %d", len(offPage.Items))
	}
}

func TestListUsersSearch(t *testing.T) {
	repo := newFakeRepo()
	repo.seed("ivanov.ii", models.RoleUser)
	repo.seed("petrov.ii", models.RoleUser)
	repo.seed("ivanova.p", models.RoleUser)
	svc := newTestService(repo)

	page, err := svc.ListUsers(context.Background(), "actor-id", dto.AdminListQuery{Query: "ivan"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.Total != 2 {
		t.Errorf("expected 2 matches, got %d", page.Total)
	}
	got := []string{page.Items[0].Login, page.Items[1].Login}
	if got[0] != "ivanov.ii" || got[1] != "ivanova.p" {
		t.Errorf("unexpected search results: %v", got)
	}
}

func TestListUsersPageSizeCap(t *testing.T) {
	repo := newFakeRepo()
	for i := 1; i <= 150; i++ {
		repo.seed(fmt.Sprintf("user.%03d", i), models.RoleUser)
	}
	svc := newTestService(repo)

	page, err := svc.ListUsers(context.Background(), "actor-id", dto.AdminListQuery{Page: 1, PageSize: 5000})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.PageSize != 100 {
		t.Errorf("expected page size capped at 100, got %d", page.PageSize)
	}
	if len(page.Items) != 100 {
		t.Errorf("expected 100 items, got %d", len(page.Items))
	}
}

func TestUpdateUserRoleAdminPromotesToModerator(t *testing.T) {
	repo := newFakeRepo()
	targetID := repo.seed("user.one", models.RoleUser)
	svc := newTestService(repo)

	resp, err := svc.UpdateUserRole(context.Background(), Actor{ID: "admin-1", Role: models.RoleAdmin}, targetID, dto.UpdateUserRoleRequest{Role: models.RoleModerator})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Role != models.RoleModerator {
		t.Errorf("expected moderator, got %q", resp.Role)
	}
}

func TestUpdateUserRoleAdminDemotesToUser(t *testing.T) {
	repo := newFakeRepo()
	targetID := repo.seed("mod.one", models.RoleModerator)
	svc := newTestService(repo)

	resp, err := svc.UpdateUserRole(context.Background(), Actor{ID: "admin-1", Role: models.RoleAdmin}, targetID, dto.UpdateUserRoleRequest{Role: models.RoleUser})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Role != models.RoleUser {
		t.Errorf("expected user, got %q", resp.Role)
	}
}

func TestUpdateUserRoleModeratorForbidden(t *testing.T) {
	repo := newFakeRepo()
	targetID := repo.seed("user.one", models.RoleUser)
	svc := newTestService(repo)

	_, err := svc.UpdateUserRole(context.Background(), Actor{ID: "mod-1", Role: models.RoleModerator}, targetID, dto.UpdateUserRoleRequest{Role: models.RoleModerator})
	if !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestUpdateUserRoleAdminProtected(t *testing.T) {
	repo := newFakeRepo()
	otherAdminID := repo.seed("admin.other", models.RoleAdmin)
	svc := newTestService(repo)

	_, err := svc.UpdateUserRole(context.Background(), Actor{ID: "admin-2", Role: models.RoleAdmin}, otherAdminID, dto.UpdateUserRoleRequest{Role: models.RoleUser})
	if !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestUpdateUserRoleSelf(t *testing.T) {
	repo := newFakeRepo()
	selfID := repo.seed("admin", models.RoleAdmin)
	svc := newTestService(repo)

	_, err := svc.UpdateUserRole(context.Background(), Actor{ID: selfID, Role: models.RoleAdmin}, selfID, dto.UpdateUserRoleRequest{Role: models.RoleUser})
	if !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestUpdateUserRoleInvalidRole(t *testing.T) {
	repo := newFakeRepo()
	targetID := repo.seed("user.one", models.RoleUser)
	svc := newTestService(repo)

	_, err := svc.UpdateUserRole(context.Background(), Actor{ID: "admin-1", Role: models.RoleAdmin}, targetID, dto.UpdateUserRoleRequest{Role: models.RoleAdmin})
	if !errors.Is(err, pkgerrors.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestUpdateUserRoleNotFound(t *testing.T) {
	svc := newTestService(newFakeRepo())

	_, err := svc.UpdateUserRole(context.Background(), Actor{ID: "admin-1", Role: models.RoleAdmin}, "9dfc4d6c-1783-48bd-b8d0-a2dfb59d1f8e", dto.UpdateUserRoleRequest{Role: models.RoleUser})
	if !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestDeleteUserAdminDeletesUser(t *testing.T) {
	repo := newFakeRepo()
	targetID := repo.seed("user.one", models.RoleUser)
	svc := newTestService(repo)

	err := svc.DeleteUser(context.Background(), Actor{ID: "admin-1", Role: models.RoleAdmin}, targetID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := repo.users["user.one"]; ok {
		t.Error("user should be deleted")
	}
}

func TestDeleteUserAdminDeletesModerator(t *testing.T) {
	repo := newFakeRepo()
	targetID := repo.seed("mod.one", models.RoleModerator)
	svc := newTestService(repo)

	err := svc.DeleteUser(context.Background(), Actor{ID: "admin-1", Role: models.RoleAdmin}, targetID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := repo.users["mod.one"]; ok {
		t.Error("moderator should be deleted")
	}
}

func TestDeleteUserModeratorDeletesUser(t *testing.T) {
	repo := newFakeRepo()
	targetID := repo.seed("user.one", models.RoleUser)
	svc := newTestService(repo)

	err := svc.DeleteUser(context.Background(), Actor{ID: "mod-1", Role: models.RoleModerator}, targetID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := repo.users["user.one"]; ok {
		t.Error("user should be deleted")
	}
}

func TestDeleteUserModeratorCannotDeleteModerator(t *testing.T) {
	repo := newFakeRepo()
	modID := repo.seed("mod.two", models.RoleModerator)
	svc := newTestService(repo)

	err := svc.DeleteUser(context.Background(), Actor{ID: "mod-1", Role: models.RoleModerator}, modID)
	if !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestDeleteUserSelf(t *testing.T) {
	repo := newFakeRepo()
	selfID := repo.seed("admin", models.RoleAdmin)
	svc := newTestService(repo)

	err := svc.DeleteUser(context.Background(), Actor{ID: selfID, Role: models.RoleAdmin}, selfID)
	if !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestDeleteUserAdminProtected(t *testing.T) {
	repo := newFakeRepo()
	otherAdminID := repo.seed("admin.other", models.RoleAdmin)
	svc := newTestService(repo)

	err := svc.DeleteUser(context.Background(), Actor{ID: "admin-2", Role: models.RoleAdmin}, otherAdminID)
	if !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestDeleteUserNotFound(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)

	err := svc.DeleteUser(context.Background(), Actor{ID: "admin-1", Role: models.RoleAdmin}, "9dfc4d6c-1783-48bd-b8d0-a2dfb59d1f8e")
	if !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestDeleteUserInvalidID(t *testing.T) {
	svc := newTestService(newFakeRepo())

	err := svc.DeleteUser(context.Background(), Actor{ID: "admin-1", Role: models.RoleAdmin}, "not-a-uuid")
	if !errors.Is(err, pkgerrors.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}
