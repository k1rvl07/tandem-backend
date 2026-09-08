package workspace

import (
	"context"
	"errors"
	"io"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mcolumn "github.com/tandem/tandem/internal/domain/models/column"
	muser "github.com/tandem/tandem/internal/domain/models/user"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	dworkspace "github.com/tandem/tandem/internal/http/dto/workspace"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/usecase/file"
	"github.com/tandem/tandem/internal/usecase/testutil"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type fakeWorkspaceRepo struct {
	workspaces map[string]*mworkspace.Workspace
	members    map[string]map[string]*mworkspace.WorkspaceMember
}

func newFakeWorkspaceRepo() *fakeWorkspaceRepo {
	return &fakeWorkspaceRepo{
		workspaces: make(map[string]*mworkspace.Workspace),
		members:    make(map[string]map[string]*mworkspace.WorkspaceMember),
	}
}

func (f *fakeWorkspaceRepo) CreateWorkspace(_ context.Context, ws *mworkspace.Workspace) error {
	f.workspaces[ws.ID] = &mworkspace.Workspace{
		ID: ws.ID, Name: ws.Name, Description: ws.Description,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	return nil
}

func (f *fakeWorkspaceRepo) FindWorkspaceByID(_ context.Context, id string) (*mworkspace.Workspace, error) {
	ws, ok := f.workspaces[id]
	if !ok {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	return ws, nil
}

func (f *fakeWorkspaceRepo) FindWorkspaceByInvite(_ context.Context, token string) (*mworkspace.Workspace, error) {
	for _, ws := range f.workspaces {
		if ws.InviteToken != "" && ws.InviteToken == token {
			return ws, nil
		}
	}
	return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
}

func (f *fakeWorkspaceRepo) UpdateWorkspace(_ context.Context, ws *mworkspace.Workspace) error {
	existing, ok := f.workspaces[ws.ID]
	if !ok {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	existing.Name = ws.Name
	existing.Description = ws.Description
	existing.InviteToken = ws.InviteToken
	return nil
}

func (f *fakeWorkspaceRepo) UpdateInvite(_ context.Context, wsID, token string, expiresAt *time.Time) error {
	ws, ok := f.workspaces[wsID]
	if !ok {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	ws.InviteToken = token
	ws.InviteExpiresAt = expiresAt
	return nil
}

func (f *fakeWorkspaceRepo) DeleteWorkspace(_ context.Context, id string) error {
	if _, ok := f.workspaces[id]; !ok {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	delete(f.workspaces, id)
	delete(f.members, id)
	return nil
}

func (f *fakeWorkspaceRepo) ListWorkspacesForUser(_ context.Context, userID string) ([]mworkspace.WorkspaceMembership, error) {
	var out []mworkspace.WorkspaceMembership
	for wsID, mm := range f.members {
		if m, ok := mm[userID]; ok {
			out = append(out, mworkspace.WorkspaceMembership{
				Workspace: *f.workspaces[wsID],
				Role:      m.Role,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (f *fakeWorkspaceRepo) AddMember(_ context.Context, wsID, userID string, role mworkspace.WorkspaceRole) error {
	if _, ok := f.members[wsID][userID]; ok {
		return pkgerrors.Wrap(pkgerrors.ErrConflict, errors.New("unique constraint"))
	}
	if f.members[wsID] == nil {
		f.members[wsID] = make(map[string]*mworkspace.WorkspaceMember)
	}
	f.members[wsID][userID] = &mworkspace.WorkspaceMember{
		WorkspaceID: wsID, UserID: userID, Role: role, CreatedAt: time.Now(),
	}
	return nil
}

func (f *fakeWorkspaceRepo) FindMember(_ context.Context, wsID, userID string) (*mworkspace.WorkspaceMember, error) {
	m, ok := f.members[wsID][userID]
	if !ok {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	return m, nil
}

func (f *fakeWorkspaceRepo) RemoveMember(_ context.Context, wsID, userID string) error {
	if _, ok := f.members[wsID][userID]; !ok {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	delete(f.members[wsID], userID)
	return nil
}

func (f *fakeWorkspaceRepo) UpdateMemberRole(_ context.Context, wsID, userID string, role mworkspace.WorkspaceRole) error {
	m, ok := f.members[wsID][userID]
	if !ok {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	m.Role = role
	return nil
}

func (f *fakeWorkspaceRepo) ListMembers(_ context.Context, wsID string) ([]mworkspace.WorkspaceMember, error) {
	var out []mworkspace.WorkspaceMember
	for _, m := range f.members[wsID] {
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (f *fakeWorkspaceRepo) DeleteMembersByWorkspace(_ context.Context, wsID string) error {
	delete(f.members, wsID)
	return nil
}

func (f *fakeWorkspaceRepo) DeleteMembersByUser(_ context.Context, userID string) error {
	for wsID, members := range f.members {
		delete(members, userID)
		if len(members) == 0 {
			delete(f.members, wsID)
		}
	}
	return nil
}

func (f *fakeWorkspaceRepo) TransferOwnership(_ context.Context, wsID, oldOwnerID, newOwnerID string) error {
	oldMember, okOld := f.members[wsID][oldOwnerID]
	newMember, okNew := f.members[wsID][newOwnerID]
	if !okOld || !okNew {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	oldMember.Role = mworkspace.RoleEditor
	newMember.Role = mworkspace.RoleOwner
	return nil
}

type fakeUserRepo struct {
	users map[string]*muser.User
}

func (f *fakeUserRepo) seedUser(login string) *muser.User {
	u := &muser.User{ID: uuid.New().String(), Login: login, Role: muser.RoleUser, DisplayName: login}
	f.users[u.ID] = u
	return u
}

func (f *fakeUserRepo) FindByID(_ context.Context, id string) (*muser.User, error) {
	u, ok := f.users[id]
	if !ok {
		return nil, pkgerrors.ErrNotFound
	}
	return u, nil
}

func (f *fakeUserRepo) FindByLogin(_ context.Context, login string) (*muser.User, error) {
	for _, u := range f.users {
		if u.Login == login {
			return u, nil
		}
	}
	return nil, pkgerrors.ErrNotFound
}

func (f *fakeUserRepo) Create(_ context.Context, _ *muser.User) error       { return nil }
func (f *fakeUserRepo) ExistsByLogin(context.Context, string) (bool, error) { return false, nil }
func (f *fakeUserRepo) List(context.Context) ([]*muser.User, error)         { return nil, nil }
func (f *fakeUserRepo) ListPage(context.Context, string, int, int) ([]*muser.User, error) {
	return nil, nil
}
func (f *fakeUserRepo) Count(context.Context, string) (int, error) { return 0, nil }
func (f *fakeUserRepo) Update(context.Context, *muser.User) error  { return nil }
func (f *fakeUserRepo) Delete(context.Context, string) error       { return nil }

var _ repository.UserRepository = (*fakeUserRepo)(nil)
var _ repository.WorkspaceRepository = (*fakeWorkspaceRepo)(nil)

type testEnv struct {
	svc      *Service
	wsRepo   *fakeWorkspaceRepo
	userRepo *fakeUserRepo
	boards   *testutil.FakeBoardRepo
	columns  *testutil.FakeColumnRepo
}

func newTestEnv() *testEnv {
	wsRepo := newFakeWorkspaceRepo()
	userRepo := &fakeUserRepo{users: make(map[string]*muser.User)}
	favorites := testutil.NewFakeFavoriteRepo()
	boards := testutil.NewFakeBoardRepo()
	columns := testutil.NewFakeColumnRepo()
	tasks := testutil.NewFakeTaskRepo()
	files := file.NewService(noopWorkspaceStore{}, zap.NewNop())
	return &testEnv{
		svc:      NewService(wsRepo, userRepo, favorites, boards, columns, tasks, files, testutil.NewFakeHub(), testutil.NewFakeCache(), zap.NewNop()),
		wsRepo:   wsRepo,
		userRepo: userRepo,
		boards:   boards,
		columns:  columns,
	}
}

type noopWorkspaceStore struct{}

func (noopWorkspaceStore) Put(context.Context, string, io.Reader, int64, string) error {
	return nil
}

func (noopWorkspaceStore) Get(context.Context, string) (io.ReadCloser, error) {
	return nil, nil
}

func (noopWorkspaceStore) Delete(context.Context, string) error {
	return nil
}

func (noopWorkspaceStore) Exists(context.Context, string) (bool, error) {
	return false, nil
}

func (noopWorkspaceStore) PresignGet(context.Context, string, time.Duration) (string, error) {
	return "", nil
}

func (e *testEnv) seedWorkspace(owner, editor, member string) string {
	env := e
	wsID := uuid.New().String()
	env.wsRepo.workspaces[wsID] = &mworkspace.Workspace{
		ID: wsID, Name: "Team Space", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	env.wsRepo.members[wsID] = map[string]*mworkspace.WorkspaceMember{
		owner:  {WorkspaceID: wsID, UserID: owner, Role: mworkspace.RoleOwner, CreatedAt: time.Now()},
		editor: {WorkspaceID: wsID, UserID: editor, Role: mworkspace.RoleEditor, CreatedAt: time.Now()},
		member: {WorkspaceID: wsID, UserID: member, Role: mworkspace.RoleMember, CreatedAt: time.Now()},
	}
	return wsID
}

func TestCreateWorkspaceOwnerAdded(t *testing.T) {
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.one")

	resp, err := e.svc.Create(context.Background(), owner.ID, dworkspace.CreateWorkspaceRequest{Name: "  My Team  "})
	require.NoError(t, err)
	assert.Equal(t, "My Team", resp.Name)
	assert.Equal(t, string(mworkspace.RoleOwner), resp.Role)
	member, err := e.wsRepo.FindMember(context.Background(), resp.ID, owner.ID)
	require.NoError(t, err)
	assert.Equal(t, mworkspace.RoleOwner, member.Role)

	boards, err := e.boards.ListBoards(context.Background(), resp.ID)
	require.NoError(t, err)
	require.Len(t, boards, 1)
	assert.Equal(t, "Main", boards[0].Name)
	assert.True(t, boards[0].IsMain)
	cols, err := e.columns.ListColumns(context.Background(), boards[0].ID)
	require.NoError(t, err)
	assert.Equal(t, len(mcolumn.DefaultColumnNames), len(cols))
}

func TestCreateWorkspaceValidation(t *testing.T) {
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.one")

	_, err := e.svc.Create(context.Background(), owner.ID, dworkspace.CreateWorkspaceRequest{Name: ""})
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
}

func TestListWorkspacesForUser(t *testing.T) {
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.one")
	editor := e.userRepo.seedUser("editor.one")
	wsID := e.seedWorkspace(owner.ID, editor.ID, uuid.NewString())

	resp, err := e.svc.List(context.Background(), owner.ID)
	require.NoError(t, err)
	require.Len(t, resp, 1)
	assert.Equal(t, wsID, resp[0].ID)
	assert.Equal(t, "owner", resp[0].Role)

	resp, err = e.svc.List(context.Background(), editor.ID)
	require.NoError(t, err)
	assert.Equal(t, "editor", resp[0].Role)
}

func TestGetWorkspaceIncludesMembers(t *testing.T) {
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.one")
	editor := e.userRepo.seedUser("editor.one")
	member := e.userRepo.seedUser("member.one")
	wsID := e.seedWorkspace(owner.ID, editor.ID, member.ID)

	resp, err := e.svc.Get(context.Background(), member.ID, wsID)
	require.NoError(t, err)
	assert.Equal(t, "member", resp.Role)
	require.Len(t, resp.Members, 3)
}

func TestGetWorkspaceForbiddenForNonMember(t *testing.T) {
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.one")
	editor := e.userRepo.seedUser("editor.one")
	stranger := e.userRepo.seedUser("stranger.one")
	wsID := e.seedWorkspace(owner.ID, editor.ID, uuid.NewString())

	_, err := e.svc.Get(context.Background(), stranger.ID, wsID)
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}

func TestUpdateWorkspacePermissions(t *testing.T) {
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.one")
	editor := e.userRepo.seedUser("editor.one")
	member := e.userRepo.seedUser("member.one")
	wsID := e.seedWorkspace(owner.ID, editor.ID, member.ID)
	ctx := context.Background()
	req := dworkspace.UpdateWorkspaceRequest{Name: "Renamed", Description: "desc"}

	_, err := e.svc.Update(ctx, editor.ID, wsID, req)
	require.NoError(t, err)
	_, err = e.svc.Update(ctx, member.ID, wsID, req)
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
	_, err = e.svc.Update(ctx, owner.ID, wsID, req)
	require.NoError(t, err)
}

func TestDeleteWorkspaceOwnerOnly(t *testing.T) {
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.one")
	editor := e.userRepo.seedUser("editor.one")
	member := e.userRepo.seedUser("member.one")
	wsID := e.seedWorkspace(owner.ID, editor.ID, member.ID)
	ctx := context.Background()

	err := e.svc.Delete(ctx, member.ID, wsID)
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
	err = e.svc.Delete(ctx, editor.ID, wsID)
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
	err = e.svc.Delete(ctx, owner.ID, wsID)
	require.NoError(t, err)
	_, err = e.wsRepo.FindWorkspaceByID(ctx, wsID)
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
	_, err = e.wsRepo.FindMember(ctx, wsID, owner.ID)
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
}

func TestAddMember(t *testing.T) {
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.one")
	editor := e.userRepo.seedUser("editor.one")
	member := e.userRepo.seedUser("member.one")
	newbie := e.userRepo.seedUser("newbie.one")
	wsID := e.seedWorkspace(owner.ID, editor.ID, member.ID)
	ctx := context.Background()

	created, err := e.svc.AddMember(ctx, owner.ID, wsID, dworkspace.AddMemberRequest{Login: newbie.Login})
	require.NoError(t, err)
	assert.Equal(t, string(mworkspace.RoleMember), created.Role)
	_, err = e.svc.AddMember(ctx, owner.ID, wsID, dworkspace.AddMemberRequest{Login: "newbie.one", Role: "editor"})
	require.ErrorIs(t, err, pkgerrors.ErrConflict)
}

func TestAddMemberPermissions(t *testing.T) {
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.one")
	editor := e.userRepo.seedUser("editor.one")
	member := e.userRepo.seedUser("member.one")
	newbie := e.userRepo.seedUser("newbie.one")
	wsID := e.seedWorkspace(owner.ID, editor.ID, member.ID)
	ctx := context.Background()

	_, err := e.svc.AddMember(ctx, editor.ID, wsID, dworkspace.AddMemberRequest{Login: newbie.Login})
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
	_, err = e.svc.AddMember(ctx, member.ID, wsID, dworkspace.AddMemberRequest{Login: newbie.Login})
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}

func TestAddMemberRoleOwnerRejected(t *testing.T) {
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.one")
	editor := e.userRepo.seedUser("editor.one")
	member := e.userRepo.seedUser("member.one")
	wsID := e.seedWorkspace(owner.ID, editor.ID, member.ID)

	_, err := e.svc.AddMember(context.Background(), owner.ID, wsID, dworkspace.AddMemberRequest{Login: "newbie.one", Role: "owner"})
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
	_, err = e.svc.AddMember(context.Background(), owner.ID, wsID, dworkspace.AddMemberRequest{Login: "newbie.one", Role: "boss"})
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
}

func TestAddMemberUnknownUser(t *testing.T) {
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.one")
	editor := e.userRepo.seedUser("editor.one")
	member := e.userRepo.seedUser("member.one")
	wsID := e.seedWorkspace(owner.ID, editor.ID, member.ID)

	_, err := e.svc.AddMember(context.Background(), owner.ID, wsID, dworkspace.AddMemberRequest{Login: "ghost.user"})
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
}

func TestRemoveMember(t *testing.T) {
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.one")
	editor := e.userRepo.seedUser("editor.one")
	member := e.userRepo.seedUser("member.one")
	wsID := e.seedWorkspace(owner.ID, editor.ID, member.ID)
	ctx := context.Background()

	err := e.svc.RemoveMember(ctx, editor.ID, wsID, member.ID)
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
	err = e.svc.RemoveMember(ctx, owner.ID, wsID, owner.ID)
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
	err = e.svc.RemoveMember(ctx, owner.ID, wsID, member.ID)
	require.NoError(t, err)
	_, err = e.wsRepo.FindMember(ctx, wsID, member.ID)
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
	err = e.svc.RemoveMember(ctx, owner.ID, wsID, member.ID)
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
}

func TestTransferOwner(t *testing.T) {
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.one")
	editor := e.userRepo.seedUser("editor.one")
	member := e.userRepo.seedUser("member.one")
	wsID := e.seedWorkspace(owner.ID, editor.ID, member.ID)
	ctx := context.Background()

	err := e.svc.TransferOwner(ctx, member.ID, wsID, dworkspace.TransferOwnerRequest{UserID: owner.ID})
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
	err = e.svc.TransferOwner(ctx, owner.ID, wsID, dworkspace.TransferOwnerRequest{UserID: owner.ID})
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
	err = e.svc.TransferOwner(ctx, owner.ID, wsID, dworkspace.TransferOwnerRequest{UserID: uuid.NewString()})
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)

	err = e.svc.TransferOwner(ctx, owner.ID, wsID, dworkspace.TransferOwnerRequest{UserID: editor.ID})
	require.NoError(t, err)
	newOwner, err := e.wsRepo.FindMember(ctx, wsID, editor.ID)
	require.NoError(t, err)
	assert.Equal(t, mworkspace.RoleOwner, newOwner.Role)
	oldOwner, _ := e.wsRepo.FindMember(ctx, wsID, owner.ID)
	assert.Equal(t, mworkspace.RoleEditor, oldOwner.Role)
}

func TestUpdateMemberRole(t *testing.T) {
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.one")
	editor := e.userRepo.seedUser("editor.one")
	member := e.userRepo.seedUser("member.one")
	other := e.userRepo.seedUser("other.one")
	wsID := e.seedWorkspace(owner.ID, editor.ID, member.ID)
	ctx := context.Background()

	_, err := e.svc.UpdateRole(ctx, member.ID, wsID, editor.ID, dworkspace.UpdateMemberRoleRequest{Role: string(mworkspace.RoleMember)})
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
	_, err = e.svc.UpdateRole(ctx, owner.ID, wsID, other.ID, dworkspace.UpdateMemberRoleRequest{Role: string(mworkspace.RoleMember)})
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
	_, err = e.svc.UpdateRole(ctx, owner.ID, wsID, owner.ID, dworkspace.UpdateMemberRoleRequest{Role: string(mworkspace.RoleMember)})
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
	_, err = e.svc.UpdateRole(ctx, owner.ID, wsID, editor.ID, dworkspace.UpdateMemberRoleRequest{Role: "superadmin"})
	require.ErrorIs(t, err, pkgerrors.ErrValidation)

	resp, err := e.svc.UpdateRole(ctx, owner.ID, wsID, editor.ID, dworkspace.UpdateMemberRoleRequest{Role: string(mworkspace.RoleMember)})
	require.NoError(t, err)
	assert.Equal(t, string(mworkspace.RoleMember), resp.Role)
	assert.Equal(t, editor.ID, resp.ID)
	stored, err := e.wsRepo.FindMember(ctx, wsID, editor.ID)
	require.NoError(t, err)
	assert.Equal(t, mworkspace.RoleMember, stored.Role)
}

func TestSetThemePermissions(t *testing.T) {
	ctx := context.Background()
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.theme")
	editor := e.userRepo.seedUser("editor.theme")
	member := e.userRepo.seedUser("member.theme")
	wsID := e.seedWorkspace(owner.ID, editor.ID, member.ID)

	_, err := e.svc.SetTheme(ctx, owner.ID, wsID, "be123c")
	assert.NoError(t, err)
	resp, err := e.svc.SetTheme(ctx, editor.ID, wsID, "16a34a")
	require.NoError(t, err)
	assert.Equal(t, "16a34a", resp.Theme)
	_, err = e.svc.SetTheme(ctx, member.ID, wsID, "db2777")
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}

func TestGetInviteGeneratesToken(t *testing.T) {
	ctx := context.Background()
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.invite")
	editor := e.userRepo.seedUser("editor.invite")
	member := e.userRepo.seedUser("member.invite")
	wsID := e.seedWorkspace(owner.ID, editor.ID, member.ID)

	resp, err := e.svc.GetInvite(ctx, owner.ID, wsID)
	require.NoError(t, err)
	require.NotNil(t, resp.InviteToken)
	assert.NotEmpty(t, *resp.InviteToken)
	second, err := e.svc.GetInvite(ctx, editor.ID, wsID)
	require.NoError(t, err)
	require.NotNil(t, second.InviteToken)
	assert.Equal(t, *resp.InviteToken, *second.InviteToken)
	_, err = e.svc.GetInvite(ctx, member.ID, wsID)
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
	_, err = e.svc.GetInvite(ctx, uuid.NewString(), wsID)
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}

func TestDisableInvite(t *testing.T) {
	ctx := context.Background()
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.invite")
	member := e.userRepo.seedUser("member.invite")
	wsID := e.seedWorkspace(owner.ID, uuid.NewString(), member.ID)

	_, err := e.svc.GetInvite(ctx, owner.ID, wsID)
	require.NoError(t, err)
	err = e.svc.DisableInvite(ctx, owner.ID, wsID)
	require.NoError(t, err)
	ws := e.wsRepo.workspaces[wsID]
	assert.Empty(t, ws.InviteToken)
	regenerated, err := e.svc.GetInvite(ctx, owner.ID, wsID)
	require.NoError(t, err)
	require.NotNil(t, regenerated.InviteToken)
	assert.NotEmpty(t, *regenerated.InviteToken)
	err = e.svc.DisableInvite(ctx, member.ID, wsID)
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}

func TestJoinByInvite(t *testing.T) {
	ctx := context.Background()
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.invite")
	member := e.userRepo.seedUser("member.invite")
	newbie := e.userRepo.seedUser("newbie.invite")
	wsID := e.seedWorkspace(owner.ID, owner.ID, member.ID)

	invite, err := e.svc.GetInvite(ctx, owner.ID, wsID)
	require.NoError(t, err)
	token := *invite.InviteToken
	require.NotEmpty(t, token)

	resp, err := e.svc.JoinByInvite(ctx, newbie.ID, token)
	require.NoError(t, err)
	assert.Equal(t, wsID, resp.ID)
	stored, err := e.wsRepo.FindMember(ctx, wsID, newbie.ID)
	require.NoError(t, err)
	assert.Equal(t, mworkspace.RoleMember, stored.Role)

	_, err = e.svc.JoinByInvite(ctx, newbie.ID, token)
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
	_, err = e.svc.JoinByInvite(ctx, newbie.ID, "bogus-token")
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
}

func TestJoinByInviteDisabled(t *testing.T) {
	ctx := context.Background()
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.invite")
	newbie := e.userRepo.seedUser("newbie.invite")
	wsID := e.seedWorkspace(owner.ID, uuid.NewString(), uuid.NewString())

	invite, err := e.svc.GetInvite(ctx, owner.ID, wsID)
	require.NoError(t, err)
	token := *invite.InviteToken
	require.NotEmpty(t, token)
	err = e.svc.DisableInvite(ctx, owner.ID, wsID)
	require.NoError(t, err)
	_, err = e.svc.JoinByInvite(ctx, newbie.ID, token)
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
}

func TestGetInviteSetsExpiry(t *testing.T) {
	ctx := context.Background()
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.invite")
	wsID := e.seedWorkspace(owner.ID, uuid.NewString(), uuid.NewString())

	resp, err := e.svc.GetInvite(ctx, owner.ID, wsID)
	require.NoError(t, err)
	require.NotNil(t, resp.ExpiresAt)
	require.False(t, resp.ExpiresAt.Before(time.Now()))
	require.False(t, resp.ExpiresAt.After(time.Now().Add(inviteTTL+time.Second)))
}

func TestJoinByInviteExpired(t *testing.T) {
	ctx := context.Background()
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.invite")
	newbie := e.userRepo.seedUser("newbie.invite")
	wsID := e.seedWorkspace(owner.ID, uuid.NewString(), uuid.NewString())

	invite, err := e.svc.GetInvite(ctx, owner.ID, wsID)
	require.NoError(t, err)
	token := *invite.InviteToken
	past := time.Now().Add(-time.Hour)
	e.wsRepo.workspaces[wsID].InviteExpiresAt = &past

	_, err = e.svc.JoinByInvite(ctx, newbie.ID, token)
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
	assert.Empty(t, e.wsRepo.workspaces[wsID].InviteToken)
}
