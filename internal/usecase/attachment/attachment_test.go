package attachment

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mattachment "github.com/tandem/tandem/internal/domain/models/attachment"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	"github.com/tandem/tandem/internal/domain/ports/filestore"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	file "github.com/tandem/tandem/internal/usecase/file"
	"github.com/tandem/tandem/internal/usecase/testutil"
	"go.uber.org/zap"
)

type failStore struct {
	files map[string][]byte
}

func newFailStore() *failStore {
	return &failStore{files: make(map[string][]byte)}
}

func (s *failStore) Put(_ context.Context, key string, reader io.Reader, _ int64, _ string) error {
	data, _ := io.ReadAll(reader)
	s.files[key] = data
	return errors.New("put failed")
}

func (s *failStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	data, ok := s.files[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (s *failStore) Delete(_ context.Context, key string) error {
	delete(s.files, key)
	return nil
}

func (s *failStore) Exists(_ context.Context, key string) (bool, error) {
	_, ok := s.files[key]
	return ok, nil
}

func (s *failStore) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return key, nil
}

type okStore struct {
	files   map[string][]byte
	deleted []string
}

func newOKStore() *okStore {
	return &okStore{files: make(map[string][]byte)}
}

func (s *okStore) Put(_ context.Context, key string, reader io.Reader, _ int64, _ string) error {
	data, _ := io.ReadAll(reader)
	s.files[key] = data
	return nil
}

func (s *okStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	data, ok := s.files[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (s *okStore) Delete(_ context.Context, key string) error {
	delete(s.files, key)
	s.deleted = append(s.deleted, key)
	return nil
}

func (s *okStore) Exists(_ context.Context, key string) (bool, error) {
	_, ok := s.files[key]
	return ok, nil
}

func (s *okStore) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return key, nil
}

type getErrStore struct{}

func (s *getErrStore) Put(context.Context, string, io.Reader, int64, string) error {
	return nil
}

func (s *getErrStore) Get(context.Context, string) (io.ReadCloser, error) {
	return nil, errors.New("get failed")
}

func (s *getErrStore) Delete(context.Context, string) error {
	return nil
}

func (s *getErrStore) Exists(context.Context, string) (bool, error) {
	return true, nil
}

func (s *getErrStore) PresignGet(context.Context, string, time.Duration) (string, error) {
	return "", nil
}

type missingStore struct{}

func (s *missingStore) Put(context.Context, string, io.Reader, int64, string) error {
	return nil
}

func (s *missingStore) Get(context.Context, string) (io.ReadCloser, error) {
	return nil, errors.New("not found")
}

func (s *missingStore) Delete(context.Context, string) error {
	return nil
}

func (s *missingStore) Exists(context.Context, string) (bool, error) {
	return false, nil
}

func (s *missingStore) PresignGet(context.Context, string, time.Duration) (string, error) {
	return "", nil
}

type errAttachmentRepo struct {
	*testutil.FakeAttachmentRepo
	createErr error
	listErr   error
	findErr   error
	deleteErr error
}

func (r *errAttachmentRepo) CreateAttachment(ctx context.Context, a *mattachment.TaskAttachment) error {
	if r.createErr != nil {
		return r.createErr
	}
	return r.FakeAttachmentRepo.CreateAttachment(ctx, a)
}

func (r *errAttachmentRepo) ListAttachmentsByTask(ctx context.Context, taskID string) ([]mattachment.TaskAttachment, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.FakeAttachmentRepo.ListAttachmentsByTask(ctx, taskID)
}

func (r *errAttachmentRepo) FindAttachmentByID(ctx context.Context, id string) (*mattachment.TaskAttachment, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	return r.FakeAttachmentRepo.FindAttachmentByID(ctx, id)
}

func (r *errAttachmentRepo) DeleteAttachment(ctx context.Context, id string) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	return r.FakeAttachmentRepo.DeleteAttachment(ctx, id)
}

type svcFixture struct {
	svc          *Service
	attrs        *errAttachmentRepo
	hub          *testutil.FakeHub
	store        filestore.FileStore
	wsID         string
	taskID       string
	actorID      string
	attachmentID string
	attach       *mattachment.TaskAttachment
}

func newFixture(t *testing.T, store filestore.FileStore) *svcFixture {
	t.Helper()
	f := &svcFixture{
		attrs:   &errAttachmentRepo{FakeAttachmentRepo: testutil.NewFakeAttachmentRepo()},
		hub:     testutil.NewFakeHub(),
		store:   store,
		wsID:    uuid.NewString(),
		taskID:  uuid.NewString(),
		actorID: uuid.NewString(),
	}

	tasks := testutil.NewFakeTaskRepo()
	columns := testutil.NewFakeColumnRepo()
	boards := testutil.NewFakeBoardRepo()
	workspaces := testutil.NewFakeWorkspaceRepo()
	cache := testutil.NewFakeCache()
	boardID := uuid.NewString()
	columnID := uuid.NewString()

	tasks.AddTaskFixture(f.taskID, columnID, "Task", 0)
	tasks.RegisterColumn(columnID, boardID)
	tasks.RegisterColumnWorkspace(columnID, f.wsID)
	columns.AddColumnFixture(columnID, boardID, "To do", 0)
	columns.RegisterBoard(f.wsID, boardID)
	boards.AddBoardFixture(boardID, f.wsID, "Board")
	workspaces.AddWorkspaceFixture(f.wsID, "WS")
	workspaces.AddMemberFixture(f.wsID, f.actorID, mworkspace.RoleMember)

	f.svc = NewService(Deps{
		Attachments: f.attrs,
		Tasks:       tasks,
		Columns:     columns,
		Boards:      boards,
		Workspaces:  workspaces,
		Files:       file.NewService(store, zap.NewNop()),
		Hub:         f.hub,
		Cache:       cache,
		Logger:      zap.NewNop(),
	})

	f.attachmentID = uuid.NewString()
	f.attach = &mattachment.TaskAttachment{
		ID:          f.attachmentID,
		TaskID:      f.taskID,
		Filename:    "doc.pdf",
		ObjectKey:   "attachments/" + f.wsID + "/" + f.actorID + "/" + uuid.NewString() + "_doc.pdf",
		Size:        4,
		ContentType: "application/pdf",
		UploadedBy:  f.actorID,
		CreatedAt:   time.Now(),
	}
	return f
}

func TestCreateSuccess(t *testing.T) {
	f := newFixture(t, newOKStore())
	resp, err := f.svc.Create(context.Background(), f.actorID, f.wsID, f.taskID, "doc.pdf", "application/pdf", bytes.NewReader([]byte("data")), 4)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, f.taskID, resp.TaskID)
	assert.Equal(t, "doc.pdf", resp.Filename)
	assert.Equal(t, "application/pdf", resp.ContentType)
	assert.Equal(t, int64(4), resp.Size)
	assert.Equal(t, f.actorID, resp.UploadedBy)
	assert.Equal(t, "/api/v1/workspaces/"+f.wsID+"/tasks/"+f.taskID+"/attachments/"+resp.ID, resp.URL)
	require.Len(t, f.attrs.Attachments, 1)
	var savedObjectKey string
	for _, a := range f.attrs.Attachments {
		savedObjectKey = a.ObjectKey
	}
	_, ok := f.store.(*okStore).files[savedObjectKey]
	assert.True(t, ok)
	assert.NotEmpty(t, f.hub.Messages)
	assert.Equal(t, "attachment.created", f.hub.Messages[0].Type)
}

func TestCreateInvalidWorkspaceID(t *testing.T) {
	f := newFixture(t, newOKStore())
	_, err := f.svc.Create(context.Background(), f.actorID, "bad", f.taskID, "doc.pdf", "application/pdf", bytes.NewReader([]byte("data")), 4)
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
	assert.Empty(t, f.attrs.Attachments)
}

func TestCreateInvalidTaskID(t *testing.T) {
	f := newFixture(t, newOKStore())
	_, err := f.svc.Create(context.Background(), f.actorID, f.wsID, "bad", "doc.pdf", "application/pdf", bytes.NewReader([]byte("data")), 4)
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
	assert.Empty(t, f.attrs.Attachments)
}

func TestCreateForbiddenNonMember(t *testing.T) {
	f := newFixture(t, newOKStore())
	_, err := f.svc.Create(context.Background(), uuid.NewString(), f.wsID, f.taskID, "doc.pdf", "application/pdf", bytes.NewReader([]byte("data")), 4)
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
	assert.Empty(t, f.attrs.Attachments)
}

func TestCreateTaskNotInWorkspace(t *testing.T) {
	f := newFixture(t, newOKStore())
	_, err := f.svc.Create(context.Background(), f.actorID, f.wsID, uuid.NewString(), "doc.pdf", "application/pdf", bytes.NewReader([]byte("data")), 4)
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
	assert.Empty(t, f.attrs.Attachments)
}

func TestCreateRejectsEmptyFile(t *testing.T) {
	f := newFixture(t, newOKStore())
	_, err := f.svc.Create(context.Background(), f.actorID, f.wsID, f.taskID, "", "application/pdf", bytes.NewReader(nil), 0)
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
	assert.Empty(t, f.attrs.Attachments)
}

func TestCreateAttachmentRepoError(t *testing.T) {
	f := newFixture(t, newOKStore())
	f.attrs.createErr = errors.New("db down")
	_, err := f.svc.Create(context.Background(), f.actorID, f.wsID, f.taskID, "doc.pdf", "application/pdf", bytes.NewReader([]byte("data")), 4)
	require.ErrorIs(t, err, f.attrs.createErr)
	assert.Empty(t, f.attrs.Attachments)
}

func TestCreateRollsBackObjectOnUploadFailure(t *testing.T) {
	store := newFailStore()
	f := newFixture(t, store)
	_, err := f.svc.Create(context.Background(), f.actorID, f.wsID, f.taskID, "doc.pdf", "application/pdf", bytes.NewReader([]byte("data")), 4)
	require.Error(t, err)
	assert.Empty(t, store.files)
	assert.Empty(t, f.attrs.Attachments)
	assert.Empty(t, f.hub.Messages)
}

func TestCreateRollbackWarnsWhenDeleteFails(t *testing.T) {
	f := newFixture(t, newFailStore())
	f.attrs.deleteErr = errors.New("delete failed")
	_, err := f.svc.Create(context.Background(), f.actorID, f.wsID, f.taskID, "doc.pdf", "application/pdf", bytes.NewReader([]byte("data")), 4)
	require.Error(t, err)
	assert.Len(t, f.attrs.Attachments, 1)
	assert.Empty(t, f.hub.Messages)
}

func TestListSuccessAndCached(t *testing.T) {
	f := newFixture(t, newOKStore())
	require.NoError(t, f.attrs.CreateAttachment(context.Background(), f.attach))

	resp, err := f.svc.List(context.Background(), f.actorID, f.wsID, f.taskID)
	require.NoError(t, err)
	require.Len(t, resp, 1)
	assert.Equal(t, f.attachmentID, resp[0].ID)
	assert.Equal(t, "/api/v1/workspaces/"+f.wsID+"/tasks/"+f.taskID+"/attachments/"+f.attachmentID, resp[0].URL)

	cached, err := f.svc.List(context.Background(), f.actorID, f.wsID, f.taskID)
	require.NoError(t, err)
	require.Len(t, cached, 1)
	assert.Equal(t, resp[0].ID, cached[0].ID)
	assert.Equal(t, resp[0].URL, cached[0].URL)
}

func TestListInvalidWorkspaceID(t *testing.T) {
	f := newFixture(t, newOKStore())
	_, err := f.svc.List(context.Background(), f.actorID, "bad", f.taskID)
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
}

func TestListInvalidTaskID(t *testing.T) {
	f := newFixture(t, newOKStore())
	_, err := f.svc.List(context.Background(), f.actorID, f.wsID, "bad")
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
}

func TestListForbiddenNonMember(t *testing.T) {
	f := newFixture(t, newOKStore())
	_, err := f.svc.List(context.Background(), uuid.NewString(), f.wsID, f.taskID)
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}

func TestListRepoError(t *testing.T) {
	f := newFixture(t, newOKStore())
	f.attrs.listErr = errors.New("db down")
	_, err := f.svc.List(context.Background(), f.actorID, f.wsID, f.taskID)
	require.ErrorIs(t, err, f.attrs.listErr)
}

func TestListTaskNotInWorkspace(t *testing.T) {
	f := newFixture(t, newOKStore())
	_, err := f.svc.List(context.Background(), f.actorID, f.wsID, uuid.NewString())
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
}

func TestDownloadSuccess(t *testing.T) {
	store := newOKStore()
	f := newFixture(t, store)
	require.NoError(t, f.attrs.CreateAttachment(context.Background(), f.attach))
	store.files[f.attach.ObjectKey] = []byte("data")

	resp, rc, err := f.svc.Download(context.Background(), f.actorID, f.wsID, f.taskID, f.attachmentID)
	require.NoError(t, err)
	require.NotNil(t, rc)
	defer rc.Close()
	require.NotNil(t, resp)
	assert.Equal(t, f.attachmentID, resp.ID)
	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, "data", string(data))
}

func TestDownloadInvalidIDs(t *testing.T) {
	f := newFixture(t, newOKStore())
	_, _, err := f.svc.Download(context.Background(), f.actorID, "bad", f.taskID, f.attachmentID)
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
	_, _, err = f.svc.Download(context.Background(), f.actorID, f.wsID, "bad", f.attachmentID)
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
	_, _, err = f.svc.Download(context.Background(), f.actorID, f.wsID, f.taskID, "bad")
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
	_, _, err = f.svc.Download(context.Background(), uuid.NewString(), f.wsID, f.taskID, f.attachmentID)
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}

func TestDownloadAttachmentNotFound(t *testing.T) {
	f := newFixture(t, newOKStore())
	_, _, err := f.svc.Download(context.Background(), f.actorID, f.wsID, f.taskID, uuid.NewString())
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
}

func TestDownloadAttachmentFromAnotherTask(t *testing.T) {
	f := newFixture(t, newOKStore())
	other := *f.attach
	other.ID = uuid.NewString()
	other.TaskID = uuid.NewString()
	require.NoError(t, f.attrs.CreateAttachment(context.Background(), &other))

	_, _, err := f.svc.Download(context.Background(), f.actorID, f.wsID, f.taskID, other.ID)
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
}

func TestDownloadObjectMissing(t *testing.T) {
	f := newFixture(t, new(missingStore{}))
	require.NoError(t, f.attrs.CreateAttachment(context.Background(), f.attach))
	_, _, err := f.svc.Download(context.Background(), f.actorID, f.wsID, f.taskID, f.attachmentID)
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
}

func TestDownloadObjectGetError(t *testing.T) {
	f := newFixture(t, new(getErrStore{}))
	require.NoError(t, f.attrs.CreateAttachment(context.Background(), f.attach))
	_, _, err := f.svc.Download(context.Background(), f.actorID, f.wsID, f.taskID, f.attachmentID)
	require.ErrorIs(t, err, pkgerrors.ErrInternal)
}

func TestDownloadTaskNotInWorkspace(t *testing.T) {
	f := newFixture(t, newOKStore())
	_, _, err := f.svc.Download(context.Background(), f.actorID, f.wsID, uuid.NewString(), f.attachmentID)
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
}

func TestDeleteSuccess(t *testing.T) {
	store := newOKStore()
	f := newFixture(t, store)
	require.NoError(t, f.attrs.CreateAttachment(context.Background(), f.attach))
	store.files[f.attach.ObjectKey] = []byte("data")

	err := f.svc.Delete(context.Background(), f.actorID, f.wsID, f.taskID, f.attachmentID)
	require.NoError(t, err)
	assert.Empty(t, f.attrs.Attachments)
	assert.Equal(t, []string{f.attach.ObjectKey}, store.deleted)
	require.Len(t, f.hub.Messages, 1)
	assert.Equal(t, "attachment.deleted", f.hub.Messages[0].Type)
	msg := f.hub.Messages[0].Data.(map[string]string)
	assert.Equal(t, f.attachmentID, msg["id"])
}

func TestDeleteInvalidIDs(t *testing.T) {
	f := newFixture(t, newOKStore())
	err := f.svc.Delete(context.Background(), f.actorID, "bad", f.taskID, f.attachmentID)
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
	err = f.svc.Delete(context.Background(), f.actorID, f.wsID, "bad", f.attachmentID)
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
	err = f.svc.Delete(context.Background(), f.actorID, f.wsID, f.taskID, "bad")
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
	err = f.svc.Delete(context.Background(), uuid.NewString(), f.wsID, f.taskID, f.attachmentID)
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}

func TestDeleteNotFound(t *testing.T) {
	f := newFixture(t, newOKStore())
	err := f.svc.Delete(context.Background(), f.actorID, f.wsID, f.taskID, uuid.NewString())
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
}

func TestDeleteHiddenTaskNotFound(t *testing.T) {
	f := newFixture(t, newOKStore())
	other := *f.attach
	other.ID = uuid.NewString()
	other.TaskID = uuid.NewString()
	require.NoError(t, f.attrs.CreateAttachment(context.Background(), &other))

	err := f.svc.Delete(context.Background(), f.actorID, f.wsID, f.taskID, other.ID)
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
}

func TestDeleteRepoError(t *testing.T) {
	f := newFixture(t, newOKStore())
	require.NoError(t, f.attrs.CreateAttachment(context.Background(), f.attach))
	f.attrs.deleteErr = errors.New("db down")
	err := f.svc.Delete(context.Background(), f.actorID, f.wsID, f.taskID, f.attachmentID)
	require.Error(t, err)
	assert.Len(t, f.attrs.Attachments, 1)
}

func TestDeleteTaskNotInWorkspace(t *testing.T) {
	f := newFixture(t, newOKStore())
	require.NoError(t, f.attrs.CreateAttachment(context.Background(), f.attach))
	err := f.svc.Delete(context.Background(), f.actorID, f.wsID, uuid.NewString(), f.attachmentID)
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
}

func TestAttachmentResponseMapping(t *testing.T) {
	f := newFixture(t, newOKStore())
	resp := f.svc.AttachmentResponse(f.attach, f.wsID, f.taskID)
	require.NotNil(t, resp)
	assert.Equal(t, f.attachmentID, resp.ID)
	assert.Equal(t, f.taskID, resp.TaskID)
	assert.Equal(t, "doc.pdf", resp.Filename)
	assert.Equal(t, "application/pdf", resp.ContentType)
	assert.Equal(t, int64(4), resp.Size)
	assert.Equal(t, f.actorID, resp.UploadedBy)
	assert.Equal(t, "/api/v1/workspaces/"+f.wsID+"/tasks/"+f.taskID+"/attachments/"+f.attachmentID, resp.URL)
}

var _ filestore.FileStore = (*failStore)(nil)
var _ filestore.FileStore = (*okStore)(nil)
var _ filestore.FileStore = (*getErrStore)(nil)
var _ filestore.FileStore = (*missingStore)(nil)
