package admin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tandem/tandem/internal/domain/models"
	"github.com/tandem/tandem/internal/domain/ports/filestore"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/http/dto"
	"github.com/tandem/tandem/internal/infrastructure/password"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/usecase/file"
	"github.com/tandem/tandem/internal/usecase/testutil"
	"go.uber.org/zap"
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
	store := filestore.FileStore(noopStore{})
	return NewService(repo, password.NewBCryptHasher(12), testutil.NewFakeCache(), fakeTokenService{}, testutil.NewFakeWorkspaceRepo(), testutil.NewFakeFavoriteRepo(), file.NewService(store, zap.NewNop()))
}

type noopStore struct{}

func (noopStore) Put(_ context.Context, _ string, _ io.Reader, _ int64, _ string) error {
	return nil
}

func (noopStore) Get(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, nil
}

func (noopStore) Delete(_ context.Context, _ string) error {
	return nil
}

func (noopStore) Exists(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (noopStore) PresignGet(_ context.Context, _ string, _ time.Duration) (string, error) {
	return "", nil
}

type fakeTokenService struct{}

func (fakeTokenService) Generate(_ string, _ time.Duration) (string, error) {
	return "", nil
}

func (fakeTokenService) Parse(_ string) (string, error) {
	return "", nil
}

func (fakeTokenService) Revoke(_ context.Context, _ string) error {
	return nil
}

func TestCreateUserSuccess(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)
	actor := Actor{ID: "admin-1", Role: models.RoleAdmin}

	resp, err := svc.CreateUser(context.Background(), actor, dto.CreateUserRequest{
		Login:    " Ivanov.II ",
		Password: "password123",
	})
	require.NoError(t, err)
	assert.Equal(t, "ivanov.ii", resp.Login)
	assert.Equal(t, "ivanov.ii", resp.DisplayName)
	assert.Equal(t, models.RoleUser, resp.Role)
	stored, err := repo.FindByLogin(context.Background(), "ivanov.ii")
	require.NoError(t, err)
	assert.NotEqual(t, "password123", stored.PasswordHash)
	assert.True(t, password.NewBCryptHasher(12).Check(stored.PasswordHash, "password123"))
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
	require.NoError(t, err)
	assert.Equal(t, models.RoleModerator, resp.Role)
}

func TestCreateUserModeratorCreatesUser(t *testing.T) {
	repo := newFakeRepo()
	repo.seed("mod.one", models.RoleModerator)
	svc := newTestService(repo)

	resp, err := svc.CreateUser(context.Background(), Actor{ID: "mod-1", Role: models.RoleModerator}, dto.CreateUserRequest{
		Login:    "user.one",
		Password: "password123",
	})
	require.NoError(t, err)
	assert.Equal(t, models.RoleUser, resp.Role)
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
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}

func TestCreateUserInvalidRole(t *testing.T) {
	svc := newTestService(newFakeRepo())

	_, err := svc.CreateUser(context.Background(), Actor{ID: "admin-1", Role: models.RoleAdmin}, dto.CreateUserRequest{
		Login:    "user.one",
		Password: "password123",
		Role:     models.RoleAdmin,
	})
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
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
	require.NoError(t, err)
	assert.Equal(t, "Ivan Ivanov", resp.DisplayName)
}

func TestCreateUserDuplicate(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)
	actor := Actor{ID: "admin-1", Role: models.RoleAdmin}

	req := dto.CreateUserRequest{Login: "ivanov.ii", Password: "password123"}
	_, err := svc.CreateUser(context.Background(), actor, req)
	require.NoError(t, err)
	_, err = svc.CreateUser(context.Background(), actor, req)
	require.Error(t, err)
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
		_, err := svc.CreateUser(context.Background(), actor, req)
		assert.Error(t, err)
	}
}

func TestListUsers(t *testing.T) {
	repo := newFakeRepo()
	repo.seed("ivanov.ii", models.RoleUser)
	repo.seed("petrov.ii", models.RoleUser)
	svc := newTestService(repo)

	page, err := svc.ListUsers(context.Background(), "actor-id", dto.AdminListQuery{})
	require.NoError(t, err)
	assert.Equal(t, 2, page.Total)
	assert.Len(t, page.Items, 2)
	assert.Equal(t, 1, page.Page)
	assert.Equal(t, 20, page.PageSize)
}

func TestListUsersPagination(t *testing.T) {
	repo := newFakeRepo()
	for i := 1; i <= 5; i++ {
		repo.seed(fmt.Sprintf("user.%02d", i), models.RoleUser)
	}
	svc := newTestService(repo)

	page1, err := svc.ListUsers(context.Background(), "actor-id", dto.AdminListQuery{Page: 1, PageSize: 2})
	require.NoError(t, err)
	require.Equal(t, 5, page1.Total)
	require.Len(t, page1.Items, 2)
	assert.Equal(t, "user.01", page1.Items[0].Login)
	assert.Equal(t, "user.02", page1.Items[1].Login)

	page3, err := svc.ListUsers(context.Background(), "actor-id", dto.AdminListQuery{Page: 3, PageSize: 2})
	require.NoError(t, err)
	require.Len(t, page3.Items, 1)
	assert.Equal(t, "user.05", page3.Items[0].Login)

	offPage, err := svc.ListUsers(context.Background(), "actor-id", dto.AdminListQuery{Page: 99, PageSize: 2})
	require.NoError(t, err)
	assert.Empty(t, offPage.Items)
}

func TestListUsersSearch(t *testing.T) {
	repo := newFakeRepo()
	repo.seed("ivanov.ii", models.RoleUser)
	repo.seed("petrov.ii", models.RoleUser)
	repo.seed("ivanova.p", models.RoleUser)
	svc := newTestService(repo)

	page, err := svc.ListUsers(context.Background(), "actor-id", dto.AdminListQuery{Query: "ivan"})
	require.NoError(t, err)
	assert.Equal(t, 2, page.Total)
	got := []string{page.Items[0].Login, page.Items[1].Login}
	assert.Equal(t, []string{"ivanov.ii", "ivanova.p"}, got)
}

func TestListUsersPageSizeCap(t *testing.T) {
	repo := newFakeRepo()
	for i := 1; i <= 150; i++ {
		repo.seed(fmt.Sprintf("user.%03d", i), models.RoleUser)
	}
	svc := newTestService(repo)

	page, err := svc.ListUsers(context.Background(), "actor-id", dto.AdminListQuery{Page: 1, PageSize: 5000})
	require.NoError(t, err)
	assert.Equal(t, 100, page.PageSize)
	assert.Len(t, page.Items, 100)
}

func TestUpdateUserRoleAdminPromotesToModerator(t *testing.T) {
	repo := newFakeRepo()
	targetID := repo.seed("user.one", models.RoleUser)
	svc := newTestService(repo)

	resp, err := svc.UpdateUserRole(context.Background(), Actor{ID: "admin-1", Role: models.RoleAdmin}, targetID, dto.UpdateUserRoleRequest{Role: models.RoleModerator})
	require.NoError(t, err)
	assert.Equal(t, models.RoleModerator, resp.Role)
}

func TestUpdateUserRoleAdminDemotesToUser(t *testing.T) {
	repo := newFakeRepo()
	targetID := repo.seed("mod.one", models.RoleModerator)
	svc := newTestService(repo)

	resp, err := svc.UpdateUserRole(context.Background(), Actor{ID: "admin-1", Role: models.RoleAdmin}, targetID, dto.UpdateUserRoleRequest{Role: models.RoleUser})
	require.NoError(t, err)
	assert.Equal(t, models.RoleUser, resp.Role)
}

func TestUpdateUserRoleModeratorForbidden(t *testing.T) {
	repo := newFakeRepo()
	targetID := repo.seed("user.one", models.RoleUser)
	svc := newTestService(repo)

	_, err := svc.UpdateUserRole(context.Background(), Actor{ID: "mod-1", Role: models.RoleModerator}, targetID, dto.UpdateUserRoleRequest{Role: models.RoleModerator})
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}

func TestUpdateUserRoleAdminProtected(t *testing.T) {
	repo := newFakeRepo()
	otherAdminID := repo.seed("admin.other", models.RoleAdmin)
	svc := newTestService(repo)

	_, err := svc.UpdateUserRole(context.Background(), Actor{ID: "admin-2", Role: models.RoleAdmin}, otherAdminID, dto.UpdateUserRoleRequest{Role: models.RoleUser})
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}

func TestUpdateUserRoleSelf(t *testing.T) {
	repo := newFakeRepo()
	selfID := repo.seed("admin", models.RoleAdmin)
	svc := newTestService(repo)

	_, err := svc.UpdateUserRole(context.Background(), Actor{ID: selfID, Role: models.RoleAdmin}, selfID, dto.UpdateUserRoleRequest{Role: models.RoleUser})
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}

func TestUpdateUserRoleInvalidRole(t *testing.T) {
	repo := newFakeRepo()
	targetID := repo.seed("user.one", models.RoleUser)
	svc := newTestService(repo)

	_, err := svc.UpdateUserRole(context.Background(), Actor{ID: "admin-1", Role: models.RoleAdmin}, targetID, dto.UpdateUserRoleRequest{Role: models.RoleAdmin})
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
}

func TestUpdateUserRoleNotFound(t *testing.T) {
	svc := newTestService(newFakeRepo())

	_, err := svc.UpdateUserRole(context.Background(), Actor{ID: "admin-1", Role: models.RoleAdmin}, "9dfc4d6c-1783-48bd-b8d0-a2dfb59d1f8e", dto.UpdateUserRoleRequest{Role: models.RoleUser})
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
}

func TestDeleteUserAdminDeletesUser(t *testing.T) {
	repo := newFakeRepo()
	targetID := repo.seed("user.one", models.RoleUser)
	svc := newTestService(repo)

	err := svc.DeleteUser(context.Background(), Actor{ID: "admin-1", Role: models.RoleAdmin}, targetID)
	require.NoError(t, err)
	assert.NotContains(t, repo.users, "user.one")
}

func TestDeleteUserAdminDeletesModerator(t *testing.T) {
	repo := newFakeRepo()
	targetID := repo.seed("mod.one", models.RoleModerator)
	svc := newTestService(repo)

	err := svc.DeleteUser(context.Background(), Actor{ID: "admin-1", Role: models.RoleAdmin}, targetID)
	require.NoError(t, err)
	assert.NotContains(t, repo.users, "mod.one")
}

func TestDeleteUserModeratorDeletesUser(t *testing.T) {
	repo := newFakeRepo()
	targetID := repo.seed("user.one", models.RoleUser)
	svc := newTestService(repo)

	err := svc.DeleteUser(context.Background(), Actor{ID: "mod-1", Role: models.RoleModerator}, targetID)
	require.NoError(t, err)
	assert.NotContains(t, repo.users, "user.one")
}

func TestDeleteUserModeratorCannotDeleteModerator(t *testing.T) {
	repo := newFakeRepo()
	modID := repo.seed("mod.two", models.RoleModerator)
	svc := newTestService(repo)

	err := svc.DeleteUser(context.Background(), Actor{ID: "mod-1", Role: models.RoleModerator}, modID)
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}

func TestDeleteUserSelf(t *testing.T) {
	repo := newFakeRepo()
	selfID := repo.seed("admin", models.RoleAdmin)
	svc := newTestService(repo)

	err := svc.DeleteUser(context.Background(), Actor{ID: selfID, Role: models.RoleAdmin}, selfID)
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}

func TestDeleteUserAdminProtected(t *testing.T) {
	repo := newFakeRepo()
	otherAdminID := repo.seed("admin.other", models.RoleAdmin)
	svc := newTestService(repo)

	err := svc.DeleteUser(context.Background(), Actor{ID: "admin-2", Role: models.RoleAdmin}, otherAdminID)
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}

func TestDeleteUserNotFound(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)

	err := svc.DeleteUser(context.Background(), Actor{ID: "admin-1", Role: models.RoleAdmin}, "9dfc4d6c-1783-48bd-b8d0-a2dfb59d1f8e")
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
}

func TestDeleteUserInvalidID(t *testing.T) {
	svc := newTestService(newFakeRepo())

	err := svc.DeleteUser(context.Background(), Actor{ID: "admin-1", Role: models.RoleAdmin}, "not-a-uuid")
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
}
