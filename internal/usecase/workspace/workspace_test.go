package workspace

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tandem/tandem/internal/domain/models"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/http/dto"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"gorm.io/gorm"
)

type fakeWorkspaceRepo struct {
	workspaces map[string]*models.Workspace
	members    map[string]map[string]*models.WorkspaceMember
}

func newFakeWorkspaceRepo() *fakeWorkspaceRepo {
	return &fakeWorkspaceRepo{
		workspaces: make(map[string]*models.Workspace),
		members:    make(map[string]map[string]*models.WorkspaceMember),
	}
}

func (f *fakeWorkspaceRepo) CreateWorkspace(_ context.Context, ws *models.Workspace) error {
	f.workspaces[ws.ID] = &models.Workspace{
		ID: ws.ID, Name: ws.Name, Description: ws.Description,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	return nil
}

func (f *fakeWorkspaceRepo) FindWorkspaceByID(_ context.Context, id string) (*models.Workspace, error) {
	ws, ok := f.workspaces[id]
	if !ok {
		return nil, pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	return ws, nil
}

func (f *fakeWorkspaceRepo) UpdateWorkspace(_ context.Context, ws *models.Workspace) error {
	existing, ok := f.workspaces[ws.ID]
	if !ok {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	existing.Name = ws.Name
	existing.Description = ws.Description
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

func (f *fakeWorkspaceRepo) ListWorkspacesForUser(_ context.Context, userID string) ([]models.WorkspaceMembership, error) {
	var out []models.WorkspaceMembership
	for wsID, mm := range f.members {
		if m, ok := mm[userID]; ok {
			out = append(out, models.WorkspaceMembership{
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

func (f *fakeWorkspaceRepo) AddMember(_ context.Context, wsID, userID string, role models.WorkspaceRole) error {
	if _, ok := f.members[wsID][userID]; ok {
		return pkgerrors.Wrap(pkgerrors.ErrConflict, errors.New("unique constraint"))
	}
	if f.members[wsID] == nil {
		f.members[wsID] = make(map[string]*models.WorkspaceMember)
	}
	f.members[wsID][userID] = &models.WorkspaceMember{
		WorkspaceID: wsID, UserID: userID, Role: role, CreatedAt: time.Now(),
	}
	return nil
}

func (f *fakeWorkspaceRepo) FindMember(_ context.Context, wsID, userID string) (*models.WorkspaceMember, error) {
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

func (f *fakeWorkspaceRepo) UpdateMemberRole(_ context.Context, wsID, userID string, role models.WorkspaceRole) error {
	m, ok := f.members[wsID][userID]
	if !ok {
		return pkgerrors.Wrap(pkgerrors.ErrNotFound, gorm.ErrRecordNotFound)
	}
	m.Role = role
	return nil
}

func (f *fakeWorkspaceRepo) ListMembers(_ context.Context, wsID string) ([]models.WorkspaceMember, error) {
	var out []models.WorkspaceMember
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

type fakeUserRepo struct {
	users map[string]*models.User
}

func (f *fakeUserRepo) seedUser(login string) *models.User {
	u := &models.User{ID: uuid.New().String(), Login: login, Role: models.RoleUser, DisplayName: login}
	f.users[u.ID] = u
	return u
}

func (f *fakeUserRepo) FindByID(_ context.Context, id string) (*models.User, error) {
	u, ok := f.users[id]
	if !ok {
		return nil, pkgerrors.ErrNotFound
	}
	return u, nil
}

func (f *fakeUserRepo) FindByLogin(_ context.Context, login string) (*models.User, error) {
	for _, u := range f.users {
		if u.Login == login {
			return u, nil
		}
	}
	return nil, pkgerrors.ErrNotFound
}

func (f *fakeUserRepo) Create(_ context.Context, _ *models.User) error      { return nil }
func (f *fakeUserRepo) ExistsByLogin(context.Context, string) (bool, error) { return false, nil }
func (f *fakeUserRepo) List(context.Context) ([]*models.User, error)        { return nil, nil }
func (f *fakeUserRepo) Update(context.Context, *models.User) error          { return nil }
func (f *fakeUserRepo) Delete(context.Context, string) error                { return nil }

var _ repository.UserRepository = (*fakeUserRepo)(nil)
var _ repository.WorkspaceRepository = (*fakeWorkspaceRepo)(nil)

type testEnv struct {
	svc      *Service
	wsRepo   *fakeWorkspaceRepo
	userRepo *fakeUserRepo
}

func newTestEnv() *testEnv {
	wsRepo := newFakeWorkspaceRepo()
	userRepo := &fakeUserRepo{users: make(map[string]*models.User)}
	return &testEnv{
		svc:      NewService(wsRepo, userRepo),
		wsRepo:   wsRepo,
		userRepo: userRepo,
	}
}

func (e *testEnv) seedWorkspace(owner, editor, viewer string) string {
	env := e
	wsID := uuid.New().String()
	env.wsRepo.workspaces[wsID] = &models.Workspace{
		ID: wsID, Name: "Team Space", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	env.wsRepo.members[wsID] = map[string]*models.WorkspaceMember{
		owner:  {WorkspaceID: wsID, UserID: owner, Role: models.RoleOwner, CreatedAt: time.Now()},
		editor: {WorkspaceID: wsID, UserID: editor, Role: models.RoleEditor, CreatedAt: time.Now()},
		viewer: {WorkspaceID: wsID, UserID: viewer, Role: models.RoleViewer, CreatedAt: time.Now()},
	}
	return wsID
}

func TestCreateWorkspaceOwnerAdded(t *testing.T) {
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.one")

	resp, err := e.svc.Create(context.Background(), owner.ID, dto.CreateWorkspaceRequest{Name: "  My Team  "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Name != "My Team" {
		t.Errorf("expected trimmed name, got %q", resp.Name)
	}
	if resp.Role != string(models.RoleOwner) {
		t.Errorf("expected owner role, got %q", resp.Role)
	}
	member, err := e.wsRepo.FindMember(context.Background(), resp.ID, owner.ID)
	if err != nil {
		t.Fatalf("owner membership missing: %v", err)
	}
	if member.Role != models.RoleOwner {
		t.Errorf("expected owner membership, got %q", member.Role)
	}
}

func TestCreateWorkspaceValidation(t *testing.T) {
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.one")

	_, err := e.svc.Create(context.Background(), owner.ID, dto.CreateWorkspaceRequest{Name: ""})
	if !errors.Is(err, pkgerrors.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestListWorkspacesForUser(t *testing.T) {
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.one")
	editor := e.userRepo.seedUser("editor.one")
	wsID := e.seedWorkspace(owner.ID, editor.ID, uuid.NewString())

	resp, err := e.svc.List(context.Background(), owner.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(resp))
	}
	if resp[0].ID != wsID || resp[0].Role != "owner" {
		t.Errorf("unexpected workspace response: %+v", resp[0])
	}

	resp, err = e.svc.List(context.Background(), editor.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp[0].Role != "editor" {
		t.Errorf("expected editor role, got %q", resp[0].Role)
	}
}

func TestGetWorkspaceIncludesMembers(t *testing.T) {
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.one")
	editor := e.userRepo.seedUser("editor.one")
	viewer := e.userRepo.seedUser("viewer.one")
	wsID := e.seedWorkspace(owner.ID, editor.ID, viewer.ID)

	resp, err := e.svc.Get(context.Background(), viewer.ID, wsID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Role != "viewer" {
		t.Errorf("expected viewer role, got %q", resp.Role)
	}
	if len(resp.Members) != 3 {
		t.Fatalf("expected 3 members, got %d", len(resp.Members))
	}
}

func TestGetWorkspaceForbiddenForNonMember(t *testing.T) {
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.one")
	editor := e.userRepo.seedUser("editor.one")
	stranger := e.userRepo.seedUser("stranger.one")
	wsID := e.seedWorkspace(owner.ID, editor.ID, uuid.NewString())

	_, err := e.svc.Get(context.Background(), stranger.ID, wsID)
	if !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestUpdateWorkspacePermissions(t *testing.T) {
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.one")
	editor := e.userRepo.seedUser("editor.one")
	viewer := e.userRepo.seedUser("viewer.one")
	wsID := e.seedWorkspace(owner.ID, editor.ID, viewer.ID)
	ctx := context.Background()
	req := dto.UpdateWorkspaceRequest{Name: "Renamed", Description: "desc"}

	if _, err := e.svc.Update(ctx, editor.ID, wsID, req); err != nil {
		t.Fatalf("editor update failed: %v", err)
	}
	if _, err := e.svc.Update(ctx, viewer.ID, wsID, req); !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("expected forbidden for viewer, got %v", err)
	}
	if _, err := e.svc.Update(ctx, owner.ID, wsID, req); err != nil {
		t.Fatalf("owner update failed: %v", err)
	}
}

func TestDeleteWorkspaceOwnerOnly(t *testing.T) {
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.one")
	editor := e.userRepo.seedUser("editor.one")
	viewer := e.userRepo.seedUser("viewer.one")
	wsID := e.seedWorkspace(owner.ID, editor.ID, viewer.ID)
	ctx := context.Background()

	if err := e.svc.Delete(ctx, viewer.ID, wsID); !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("expected forbidden for viewer, got %v", err)
	}
	if err := e.svc.Delete(ctx, editor.ID, wsID); !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("expected forbidden for editor, got %v", err)
	}
	if err := e.svc.Delete(ctx, owner.ID, wsID); err != nil {
		t.Fatalf("owner delete failed: %v", err)
	}
	if _, err := e.wsRepo.FindWorkspaceByID(ctx, wsID); !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Fatalf("expected workspace gone, got %v", err)
	}
	if _, err := e.wsRepo.FindMember(ctx, wsID, owner.ID); !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Fatalf("expected members gone, got %v", err)
	}
}

func TestAddMember(t *testing.T) {
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.one")
	editor := e.userRepo.seedUser("editor.one")
	viewer := e.userRepo.seedUser("viewer.one")
	newbie := e.userRepo.seedUser("newbie.one")
	wsID := e.seedWorkspace(owner.ID, editor.ID, viewer.ID)
	ctx := context.Background()

	created, err := e.svc.AddMember(ctx, owner.ID, wsID, dto.AddMemberRequest{Login: newbie.Login})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.Role != string(models.RoleViewer) {
		t.Errorf("expected default viewer role, got %q", created.Role)
	}
	if _, err := e.svc.AddMember(ctx, owner.ID, wsID, dto.AddMemberRequest{Login: "newbie.one", Role: "editor"}); !errors.Is(err, pkgerrors.ErrConflict) {
		t.Fatalf("expected conflict on duplicate, got %v", err)
	}
}

func TestAddMemberPermissions(t *testing.T) {
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.one")
	editor := e.userRepo.seedUser("editor.one")
	viewer := e.userRepo.seedUser("viewer.one")
	newbie := e.userRepo.seedUser("newbie.one")
	wsID := e.seedWorkspace(owner.ID, editor.ID, viewer.ID)
	ctx := context.Background()

	if _, err := e.svc.AddMember(ctx, editor.ID, wsID, dto.AddMemberRequest{Login: newbie.Login}); !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("expected forbidden for editor, got %v", err)
	}
	if _, err := e.svc.AddMember(ctx, viewer.ID, wsID, dto.AddMemberRequest{Login: newbie.Login}); !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("expected forbidden for viewer, got %v", err)
	}
}

func TestAddMemberRoleOwnerRejected(t *testing.T) {
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.one")
	editor := e.userRepo.seedUser("editor.one")
	viewer := e.userRepo.seedUser("viewer.one")
	wsID := e.seedWorkspace(owner.ID, editor.ID, viewer.ID)

	_, err := e.svc.AddMember(context.Background(), owner.ID, wsID, dto.AddMemberRequest{Login: "newbie.one", Role: "owner"})
	if !errors.Is(err, pkgerrors.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
	_, err = e.svc.AddMember(context.Background(), owner.ID, wsID, dto.AddMemberRequest{Login: "newbie.one", Role: "boss"})
	if !errors.Is(err, pkgerrors.ErrValidation) {
		t.Fatalf("expected validation error for invalid role, got %v", err)
	}
}

func TestAddMemberUnknownUser(t *testing.T) {
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.one")
	editor := e.userRepo.seedUser("editor.one")
	viewer := e.userRepo.seedUser("viewer.one")
	wsID := e.seedWorkspace(owner.ID, editor.ID, viewer.ID)

	_, err := e.svc.AddMember(context.Background(), owner.ID, wsID, dto.AddMemberRequest{Login: "ghost.user"})
	if !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestRemoveMember(t *testing.T) {
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.one")
	editor := e.userRepo.seedUser("editor.one")
	viewer := e.userRepo.seedUser("viewer.one")
	wsID := e.seedWorkspace(owner.ID, editor.ID, viewer.ID)
	ctx := context.Background()

	if err := e.svc.RemoveMember(ctx, editor.ID, wsID, viewer.ID); !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("expected forbidden for editor, got %v", err)
	}
	if err := e.svc.RemoveMember(ctx, owner.ID, wsID, owner.ID); !errors.Is(err, pkgerrors.ErrValidation) {
		t.Fatalf("expected validation error removing owner, got %v", err)
	}
	if err := e.svc.RemoveMember(ctx, owner.ID, wsID, viewer.ID); err != nil {
		t.Fatalf("owner remove failed: %v", err)
	}
	if _, err := e.wsRepo.FindMember(ctx, wsID, viewer.ID); !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Fatalf("expected member removed, got %v", err)
	}
	if err := e.svc.RemoveMember(ctx, owner.ID, wsID, viewer.ID); !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Fatalf("expected not found for removed member, got %v", err)
	}
}

func TestTransferOwner(t *testing.T) {
	e := newTestEnv()
	owner := e.userRepo.seedUser("owner.one")
	editor := e.userRepo.seedUser("editor.one")
	viewer := e.userRepo.seedUser("viewer.one")
	wsID := e.seedWorkspace(owner.ID, editor.ID, viewer.ID)
	ctx := context.Background()

	if err := e.svc.TransferOwner(ctx, viewer.ID, wsID, dto.TransferOwnerRequest{UserID: owner.ID}); !errors.Is(err, pkgerrors.ErrForbidden) {
		t.Fatalf("expected forbidden for viewer, got %v", err)
	}
	if err := e.svc.TransferOwner(ctx, owner.ID, wsID, dto.TransferOwnerRequest{UserID: owner.ID}); !errors.Is(err, pkgerrors.ErrValidation) {
		t.Fatalf("expected validation error transferring to self, got %v", err)
	}
	if err := e.svc.TransferOwner(ctx, owner.ID, wsID, dto.TransferOwnerRequest{UserID: uuid.NewString()}); !errors.Is(err, pkgerrors.ErrNotFound) {
		t.Fatalf("expected not found for non-member, got %v", err)
	}

	if err := e.svc.TransferOwner(ctx, owner.ID, wsID, dto.TransferOwnerRequest{UserID: editor.ID}); err != nil {
		t.Fatalf("transfer failed: %v", err)
	}
	newOwner, err := e.wsRepo.FindMember(ctx, wsID, editor.ID)
	if err != nil {
		t.Fatalf("new owner missing: %v", err)
	}
	if newOwner.Role != models.RoleOwner {
		t.Errorf("expected new owner role, got %q", newOwner.Role)
	}
	oldOwner, _ := e.wsRepo.FindMember(ctx, wsID, owner.ID)
	if oldOwner.Role != models.RoleEditor {
		t.Errorf("expected old owner demoted to editor, got %q", oldOwner.Role)
	}
}
