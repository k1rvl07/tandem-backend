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
	dadmin "github.com/tandem/tandem/internal/contracts/admin"
	muser "github.com/tandem/tandem/internal/domain/models/user"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	"github.com/tandem/tandem/internal/domain/ports/filestore"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/infrastructure/password"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/usecase/file"
	"github.com/tandem/tandem/internal/usecase/shared/cache"
	"github.com/tandem/tandem/internal/usecase/testutil"
	"go.uber.org/zap"
)

type fakeRepo struct {
	users map[string]*muser.User
	seq   int
	err   error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{users: make(map[string]*muser.User)}
}

func (f *fakeRepo) Create(_ context.Context, user *muser.User) error {
	if _, ok := f.users[user.Login]; ok {
		return errors.New("unique constraint")
	}
	f.users[user.Login] = user
	return nil
}

func (f *fakeRepo) FindByID(_ context.Context, id string) (*muser.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	for _, u := range f.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, pkgerrors.ErrNotFound
}

func (f *fakeRepo) FindByLogin(_ context.Context, login string) (*muser.User, error) {
	u, ok := f.users[login]
	if !ok {
		return nil, pkgerrors.ErrNotFound
	}
	return u, nil
}

func (f *fakeRepo) ExistsByLogin(_ context.Context, login string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	_, ok := f.users[login]
	return ok, nil
}

func (f *fakeRepo) List(_ context.Context) ([]*muser.User, error) {
	users := make([]*muser.User, 0, len(f.users))
	for _, u := range f.users {
		users = append(users, u)
	}
	return users, nil
}

func (f *fakeRepo) ListPage(_ context.Context, query string, limit, offset int) ([]*muser.User, error) {
	users := make([]*muser.User, 0, len(f.users))
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
	if f.err != nil {
		return 0, f.err
	}
	total := 0
	for login := range f.users {
		if query == "" || strings.Contains(login, query) {
			total++
		}
	}
	return total, nil
}

func (f *fakeRepo) Update(_ context.Context, user *muser.User) error {
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
	f.users[login] = &muser.User{ID: id, Login: login, Role: role, DisplayName: login, CreatedAt: time.Unix(0, int64(f.seq))}
	return id
}

func newTestService(repo repository.UserRepository) *Service {
	store := filestore.FileStore(noopStore{})
	return NewService(Deps{Users: repo, Hasher: password.NewBCryptHasher(12), Cache: testutil.NewFakeCache(), Tokens: fakeTokenService{}, Workspaces: testutil.NewFakeWorkspaceRepo(), Favorites: testutil.NewFakeFavoriteRepo(), Files: file.NewService(store, zap.NewNop())})
}

type adminEnv struct {
	svc       *Service
	repo      *fakeRepo
	ws        *testutil.FakeWorkspaceRepo
	favorites *testutil.FakeFavoriteRepo
}

func newAdminEnv() *adminEnv {
	repo := newFakeRepo()
	ws := testutil.NewFakeWorkspaceRepo()
	favorites := testutil.NewFakeFavoriteRepo()
	store := filestore.FileStore(noopStore{})
	svc := NewService(Deps{Users: repo, Hasher: password.NewBCryptHasher(12), Cache: testutil.NewFakeCache(), Tokens: fakeTokenService{}, Workspaces: ws, Favorites: favorites, Files: file.NewService(store, zap.NewNop())})
	return &adminEnv{svc: svc, repo: repo, ws: ws, favorites: favorites}
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

func (fakeTokenService) GenerateRefresh(_ string, _ time.Duration) (string, error) {
	return "", nil
}

func (fakeTokenService) Parse(_ string) (string, error) {
	return "", nil
}

func (fakeTokenService) ParseRefresh(_ string) (string, error) {
	return "", nil
}

func (fakeTokenService) RotateRefresh(_ context.Context, _ string, _, _ time.Duration) (string, string, error) {
	return "", "", nil
}

func (fakeTokenService) Revoke(_ context.Context, _ string) error {
	return nil
}

func TestCreateUserSuccess(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)
	actor := Actor{ID: "admin-1", Role: muser.RoleAdmin}

	resp, err := svc.CreateUser(context.Background(), actor, dadmin.CreateUserRequest{
		Login:    " Ivanov.II ",
		Password: "password123",
	})
	require.NoError(t, err)
	assert.Equal(t, "ivanov.ii", resp.Login)
	assert.Equal(t, "ivanov.ii", resp.DisplayName)
	assert.Equal(t, muser.RoleUser, resp.Role)
	stored, err := repo.FindByLogin(context.Background(), "ivanov.ii")
	require.NoError(t, err)
	assert.NotEqual(t, "password123", stored.PasswordHash)
	assert.True(t, password.NewBCryptHasher(12).Check(stored.PasswordHash, "password123"))
}

func TestCreateUserAdminCreatesModerator(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)

	resp, err := svc.CreateUser(context.Background(), Actor{ID: "admin-1", Role: muser.RoleAdmin}, dadmin.CreateUserRequest{
		Login:       "mod.one",
		Password:    "password123",
		DisplayName: "Mod One",
		Role:        muser.RoleModerator,
	})
	require.NoError(t, err)
	assert.Equal(t, muser.RoleModerator, resp.Role)
}

func TestCreateUserModeratorCreatesUser(t *testing.T) {
	repo := newFakeRepo()
	repo.seed("mod.one", muser.RoleModerator)
	svc := newTestService(repo)

	resp, err := svc.CreateUser(context.Background(), Actor{ID: "mod-1", Role: muser.RoleModerator}, dadmin.CreateUserRequest{
		Login:    "user.one",
		Password: "password123",
	})
	require.NoError(t, err)
	assert.Equal(t, muser.RoleUser, resp.Role)
}

func TestCreateUserModeratorCannotCreateModerator(t *testing.T) {
	repo := newFakeRepo()
	repo.seed("mod.one", muser.RoleModerator)
	svc := newTestService(repo)

	_, err := svc.CreateUser(context.Background(), Actor{ID: "mod-1", Role: muser.RoleModerator}, dadmin.CreateUserRequest{
		Login:    "mod.two",
		Password: "password123",
		Role:     muser.RoleModerator,
	})
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}

func TestCreateUserInvalidRole(t *testing.T) {
	svc := newTestService(newFakeRepo())

	_, err := svc.CreateUser(context.Background(), Actor{ID: "admin-1", Role: muser.RoleAdmin}, dadmin.CreateUserRequest{
		Login:    "user.one",
		Password: "password123",
		Role:     muser.RoleAdmin,
	})
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
}

func TestCreateUserCustomDisplayName(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)
	actor := Actor{ID: "admin-1", Role: muser.RoleAdmin}

	resp, err := svc.CreateUser(context.Background(), actor, dadmin.CreateUserRequest{
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
	actor := Actor{ID: "admin-1", Role: muser.RoleAdmin}

	req := dadmin.CreateUserRequest{Login: "ivanov.ii", Password: "password123"}
	_, err := svc.CreateUser(context.Background(), actor, req)
	require.NoError(t, err)
	_, err = svc.CreateUser(context.Background(), actor, req)
	require.Error(t, err)
}

func TestCreateUserValidation(t *testing.T) {
	svc := newTestService(newFakeRepo())
	actor := Actor{ID: "admin-1", Role: muser.RoleAdmin}

	cases := []dadmin.CreateUserRequest{
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
	repo.seed("ivanov.ii", muser.RoleUser)
	repo.seed("petrov.ii", muser.RoleUser)
	svc := newTestService(repo)

	page, err := svc.ListUsers(context.Background(), dadmin.AdminListQuery{})
	require.NoError(t, err)
	assert.Equal(t, 2, page.Total)
	assert.Len(t, page.Items, 2)
	assert.Equal(t, 1, page.Page)
	assert.Equal(t, 20, page.PageSize)
}

func TestListUsersPagination(t *testing.T) {
	repo := newFakeRepo()
	for i := 1; i <= 5; i++ {
		repo.seed(fmt.Sprintf("user.%02d", i), muser.RoleUser)
	}
	svc := newTestService(repo)

	page1, err := svc.ListUsers(context.Background(), dadmin.AdminListQuery{Page: 1, PageSize: 2})
	require.NoError(t, err)
	require.Equal(t, 5, page1.Total)
	require.Len(t, page1.Items, 2)
	assert.Equal(t, "user.01", page1.Items[0].Login)
	assert.Equal(t, "user.02", page1.Items[1].Login)

	page3, err := svc.ListUsers(context.Background(), dadmin.AdminListQuery{Page: 3, PageSize: 2})
	require.NoError(t, err)
	require.Len(t, page3.Items, 1)
	assert.Equal(t, "user.05", page3.Items[0].Login)

	offPage, err := svc.ListUsers(context.Background(), dadmin.AdminListQuery{Page: 99, PageSize: 2})
	require.NoError(t, err)
	assert.Empty(t, offPage.Items)
}

func TestListUsersSearch(t *testing.T) {
	repo := newFakeRepo()
	repo.seed("ivanov.ii", muser.RoleUser)
	repo.seed("petrov.ii", muser.RoleUser)
	repo.seed("ivanova.p", muser.RoleUser)
	svc := newTestService(repo)

	page, err := svc.ListUsers(context.Background(), dadmin.AdminListQuery{Query: "ivan"})
	require.NoError(t, err)
	assert.Equal(t, 2, page.Total)
	got := []string{page.Items[0].Login, page.Items[1].Login}
	assert.Equal(t, []string{"ivanov.ii", "ivanova.p"}, got)
}

func TestListUsersPageSizeCap(t *testing.T) {
	repo := newFakeRepo()
	for i := 1; i <= 150; i++ {
		repo.seed(fmt.Sprintf("user.%03d", i), muser.RoleUser)
	}
	svc := newTestService(repo)

	page, err := svc.ListUsers(context.Background(), dadmin.AdminListQuery{Page: 1, PageSize: 5000})
	require.NoError(t, err)
	assert.Equal(t, 100, page.PageSize)
	assert.Len(t, page.Items, 100)
}

func TestUpdateUserRoleAdminPromotesToModerator(t *testing.T) {
	repo := newFakeRepo()
	targetID := repo.seed("user.one", muser.RoleUser)
	svc := newTestService(repo)

	resp, err := svc.UpdateUserRole(context.Background(), Actor{ID: "admin-1", Role: muser.RoleAdmin}, targetID, dadmin.UpdateUserRoleRequest{Role: muser.RoleModerator})
	require.NoError(t, err)
	assert.Equal(t, muser.RoleModerator, resp.Role)
}

func TestUpdateUserRoleAdminDemotesToUser(t *testing.T) {
	repo := newFakeRepo()
	targetID := repo.seed("mod.one", muser.RoleModerator)
	svc := newTestService(repo)

	resp, err := svc.UpdateUserRole(context.Background(), Actor{ID: "admin-1", Role: muser.RoleAdmin}, targetID, dadmin.UpdateUserRoleRequest{Role: muser.RoleUser})
	require.NoError(t, err)
	assert.Equal(t, muser.RoleUser, resp.Role)
}

func TestUpdateUserRoleModeratorForbidden(t *testing.T) {
	repo := newFakeRepo()
	targetID := repo.seed("user.one", muser.RoleUser)
	svc := newTestService(repo)

	_, err := svc.UpdateUserRole(context.Background(), Actor{ID: "mod-1", Role: muser.RoleModerator}, targetID, dadmin.UpdateUserRoleRequest{Role: muser.RoleModerator})
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}

func TestUpdateUserRoleAdminProtected(t *testing.T) {
	repo := newFakeRepo()
	otherAdminID := repo.seed("admin.other", muser.RoleAdmin)
	svc := newTestService(repo)

	_, err := svc.UpdateUserRole(context.Background(), Actor{ID: "admin-2", Role: muser.RoleAdmin}, otherAdminID, dadmin.UpdateUserRoleRequest{Role: muser.RoleUser})
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}

func TestUpdateUserRoleSelf(t *testing.T) {
	repo := newFakeRepo()
	selfID := repo.seed("admin", muser.RoleAdmin)
	svc := newTestService(repo)

	_, err := svc.UpdateUserRole(context.Background(), Actor{ID: selfID, Role: muser.RoleAdmin}, selfID, dadmin.UpdateUserRoleRequest{Role: muser.RoleUser})
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}

func TestUpdateUserRoleInvalidRole(t *testing.T) {
	repo := newFakeRepo()
	targetID := repo.seed("user.one", muser.RoleUser)
	svc := newTestService(repo)

	_, err := svc.UpdateUserRole(context.Background(), Actor{ID: "admin-1", Role: muser.RoleAdmin}, targetID, dadmin.UpdateUserRoleRequest{Role: muser.RoleAdmin})
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
}

func TestUpdateUserRoleNotFound(t *testing.T) {
	svc := newTestService(newFakeRepo())

	_, err := svc.UpdateUserRole(context.Background(), Actor{ID: "admin-1", Role: muser.RoleAdmin}, "9dfc4d6c-1783-48bd-b8d0-a2dfb59d1f8e", dadmin.UpdateUserRoleRequest{Role: muser.RoleUser})
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
}

func TestDeleteUserAdminDeletesUser(t *testing.T) {
	repo := newFakeRepo()
	targetID := repo.seed("user.one", muser.RoleUser)
	svc := newTestService(repo)

	err := svc.DeleteUser(context.Background(), Actor{ID: "admin-1", Role: muser.RoleAdmin}, targetID)
	require.NoError(t, err)
	assert.NotContains(t, repo.users, "user.one")
}

func TestDeleteUserAdminDeletesModerator(t *testing.T) {
	repo := newFakeRepo()
	targetID := repo.seed("mod.one", muser.RoleModerator)
	svc := newTestService(repo)

	err := svc.DeleteUser(context.Background(), Actor{ID: "admin-1", Role: muser.RoleAdmin}, targetID)
	require.NoError(t, err)
	assert.NotContains(t, repo.users, "mod.one")
}

func TestDeleteUserModeratorDeletesUser(t *testing.T) {
	repo := newFakeRepo()
	targetID := repo.seed("user.one", muser.RoleUser)
	svc := newTestService(repo)

	err := svc.DeleteUser(context.Background(), Actor{ID: "mod-1", Role: muser.RoleModerator}, targetID)
	require.NoError(t, err)
	assert.NotContains(t, repo.users, "user.one")
}

func TestDeleteUserModeratorCannotDeleteModerator(t *testing.T) {
	repo := newFakeRepo()
	modID := repo.seed("mod.two", muser.RoleModerator)
	svc := newTestService(repo)

	err := svc.DeleteUser(context.Background(), Actor{ID: "mod-1", Role: muser.RoleModerator}, modID)
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}

func TestDeleteUserSelf(t *testing.T) {
	repo := newFakeRepo()
	selfID := repo.seed("admin", muser.RoleAdmin)
	svc := newTestService(repo)

	err := svc.DeleteUser(context.Background(), Actor{ID: selfID, Role: muser.RoleAdmin}, selfID)
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}

func TestDeleteUserAdminProtected(t *testing.T) {
	repo := newFakeRepo()
	otherAdminID := repo.seed("admin.other", muser.RoleAdmin)
	svc := newTestService(repo)

	err := svc.DeleteUser(context.Background(), Actor{ID: "admin-2", Role: muser.RoleAdmin}, otherAdminID)
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}

func TestDeleteUserNotFound(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo)

	err := svc.DeleteUser(context.Background(), Actor{ID: "admin-1", Role: muser.RoleAdmin}, "9dfc4d6c-1783-48bd-b8d0-a2dfb59d1f8e")
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
}

func TestDeleteUserInvalidID(t *testing.T) {
	svc := newTestService(newFakeRepo())

	err := svc.DeleteUser(context.Background(), Actor{ID: "admin-1", Role: muser.RoleAdmin}, "not-a-uuid")
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
}

func TestDeleteUserBumpsWorkspaceVersions(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	adminID := repo.seed("admin.bump", muser.RoleAdmin)
	victimID := repo.seed("victim.bump", muser.RoleUser)

	cacheImpl := testutil.NewFakeCache()
	wsRepo := testutil.NewFakeWorkspaceRepo()
	wsRepo.AddWorkspaceFixture("ws-shared", "shared")
	wsRepo.AddWorkspaceFixture("ws-other", "other")
	wsRepo.AddMemberFixture("ws-shared", victimID, mworkspace.RoleMember)
	wsRepo.AddMemberFixture("ws-shared", adminID, mworkspace.RoleOwner)
	wsRepo.AddMemberFixture("ws-other", adminID, mworkspace.RoleOwner)

	svc := NewService(Deps{Users: repo, Hasher: password.NewBCryptHasher(12), Cache: cacheImpl, Tokens: fakeTokenService{}, Workspaces: wsRepo, Favorites: testutil.NewFakeFavoriteRepo(), Files: file.NewService(filestore.FileStore(noopStore{}), zap.NewNop())})

	require.Equal(t, "0", cache.VersionWorkspace(ctx, cacheImpl, "ws-shared"))

	err := svc.DeleteUser(ctx, Actor{ID: adminID, Role: muser.RoleAdmin}, victimID)
	require.NoError(t, err)

	assert.Equal(t, "1", cache.VersionWorkspace(ctx, cacheImpl, "ws-shared"))
	assert.Equal(t, "0", cache.VersionWorkspace(ctx, cacheImpl, "ws-other"))
	assert.Equal(t, "1", cache.VersionUsers(ctx, cacheImpl))
}

func TestCreateUserExistsError(t *testing.T) {
	e := newAdminEnv()
	e.repo.err = errors.New("db")
	ctx := context.Background()
	_, err := e.svc.CreateUser(ctx, Actor{ID: "admin", Role: muser.RoleAdmin}, dadmin.CreateUserRequest{Login: "new.user", Password: "password123"})
	require.Error(t, err)
	e.repo.err = nil
}

func TestListUsersCountError(t *testing.T) {
	e := newAdminEnv()
	e.repo.seed("ivanov.ii", muser.RoleUser)
	e.repo.err = errors.New("db")
	_, err := e.svc.ListUsers(context.Background(), dadmin.AdminListQuery{})
	require.Error(t, err)
	e.repo.err = nil
}

func TestUpdateUserRoleFindError(t *testing.T) {
	e := newAdminEnv()
	targetID := e.repo.seed("petrov.ii", muser.RoleUser)
	e.repo.err = errors.New("db")
	_, err := e.svc.UpdateUserRole(context.Background(), Actor{ID: "admin", Role: muser.RoleAdmin}, targetID, dadmin.UpdateUserRoleRequest{Role: muser.RoleUser})
	require.Error(t, err)
	e.repo.err = nil
}

func TestDeleteUserFindError(t *testing.T) {
	e := newAdminEnv()
	targetID := e.repo.seed("petrov.ii", muser.RoleUser)
	e.repo.err = errors.New("db")
	err := e.svc.DeleteUser(context.Background(), Actor{ID: "admin", Role: muser.RoleAdmin}, targetID)
	require.Error(t, err)
	e.repo.err = nil
}

func TestDeleteUserWorkspaceError(t *testing.T) {
	e := newAdminEnv()
	targetID := e.repo.seed("petrov.ii", muser.RoleUser)
	e.ws.Err = errors.New("db")
	err := e.svc.DeleteUser(context.Background(), Actor{ID: "admin", Role: muser.RoleAdmin}, targetID)
	require.Error(t, err)
	e.ws.Err = nil
}

func TestDeleteUserFavoritesError(t *testing.T) {
	e := newAdminEnv()
	targetID := e.repo.seed("petrov.ii", muser.RoleUser)
	e.favorites.Err = errors.New("db")
	err := e.svc.DeleteUser(context.Background(), Actor{ID: "admin", Role: muser.RoleAdmin}, targetID)
	require.Error(t, err)
	e.favorites.Err = nil
}
