package file

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mtask "github.com/tandem/tandem/internal/domain/models/task"
	mworkspace "github.com/tandem/tandem/internal/domain/models/workspace"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	"github.com/tandem/tandem/internal/usecase/testutil"
	"go.uber.org/zap"
)

func TestPrepareAttachmentValidation(t *testing.T) {
	svc := NewService(newMemStore(), zap.NewNop())
	_, err := svc.PrepareAttachment("", "u1", "doc.pdf", 10)
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
	_, err = svc.PrepareAttachment("ws1", "u1", "doc.pdf", 0)
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
	_, err = svc.PrepareAttachment("ws1", "u1", "doc.pdf", -1)
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
	_, err = svc.PrepareAttachment("ws1", "u1", "doc.pdf", int64(21<<20))
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
	_, err = svc.PrepareAttachment("ws1", "u1", "", 10)
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
}

func TestPrepareAttachmentSuccess(t *testing.T) {
	svc := NewService(newMemStore(), zap.NewNop())
	key, err := svc.PrepareAttachment("ws1", "u1", "doc.pdf", 100)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(key, "attachments/ws1/u1/"))
	assert.True(t, strings.HasSuffix(key, "_doc.pdf"))
}

func TestPutAttachmentDefaultsContentType(t *testing.T) {
	store := newMemStore()
	svc := NewService(store, zap.NewNop())
	err := svc.PutAttachment(context.Background(), "attachments/ws1/u1/k", bytes.NewReader([]byte("x")), 1, "")
	require.NoError(t, err)
	assert.Equal(t, []byte("x"), store.files["attachments/ws1/u1/k"])
}

func TestPutAttachmentStoreError(t *testing.T) {
	store := &putErrStore{}
	svc := NewService(store, zap.NewNop())
	err := svc.PutAttachment(context.Background(), "attachments/ws1/u1/k", bytes.NewReader([]byte("x")), 1, "text/plain")
	require.ErrorIs(t, err, pkgerrors.ErrInternal)
}

func TestOpenFileValidation(t *testing.T) {
	svc := NewService(newMemStore(), zap.NewNop())
	_, err := svc.OpenFile(context.Background(), "")
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
}

func TestOpenFileNotExists(t *testing.T) {
	svc := NewService(newMemStore(), zap.NewNop())
	_, err := svc.OpenFile(context.Background(), "attachments/ws1/u1/k")
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
}

func TestOpenFileExistsError(t *testing.T) {
	svc := NewService(&existsErrStore{}, zap.NewNop())
	_, err := svc.OpenFile(context.Background(), "attachments/ws1/u1/k")
	require.ErrorIs(t, err, pkgerrors.ErrInternal)
}

func TestOpenFileGetError(t *testing.T) {
	svc := NewService(&existsOnlyStore{}, zap.NewNop())
	_, err := svc.OpenFile(context.Background(), "attachments/ws1/u1/k")
	require.ErrorIs(t, err, pkgerrors.ErrInternal)
}

func TestOpenFileSuccess(t *testing.T) {
	store := newMemStore()
	store.files["attachments/ws1/u1/k"] = []byte("data")
	svc := NewService(store, zap.NewNop())
	rc, err := svc.OpenFile(context.Background(), "attachments/ws1/u1/k")
	require.NoError(t, err)
	defer rc.Close()
	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	assert.Equal(t, "data", string(data))
}

func TestUploadImageValidation(t *testing.T) {
	svc := NewService(newMemStore(), zap.NewNop())
	_, err := svc.UploadImage(context.Background(), "u1", "bad", "a.png", "image/png", bytes.NewReader(nil), 1)
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
	_, err = svc.UploadImage(context.Background(), "u1", "avatars", "a.bmp", "image/png", bytes.NewReader(nil), 1)
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
	_, err = svc.UploadImage(context.Background(), "u1", "avatars", "a.png", "image/jpeg", bytes.NewReader(nil), 1)
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
	_, err = svc.UploadImage(context.Background(), "u1", "avatars", "a.png", "", bytes.NewReader(nil), 0)
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
}

func TestUploadImageSuccess(t *testing.T) {
	store := newMemStore()
	svc := NewService(store, zap.NewNop())
	key, err := svc.UploadImage(context.Background(), "u1", "avatars", "a.PNG", "image/png", bytes.NewReader([]byte("img")), 3)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(key, "avatars/u1/"))
	assert.True(t, strings.HasSuffix(key, ".png"))
	assert.Equal(t, []byte("img"), store.files[key])
}

func TestUploadImageStoreError(t *testing.T) {
	svc := NewService(&putErrStore{}, zap.NewNop())
	_, err := svc.UploadImage(context.Background(), "u1", "avatars", "a.png", "image/png", bytes.NewReader([]byte("img")), 3)
	require.ErrorIs(t, err, pkgerrors.ErrInternal)
}

func TestValidateImageKeyEmpty(t *testing.T) {
	svc := NewService(newMemStore(), zap.NewNop())
	require.NoError(t, svc.ValidateImageKey("", "u1"))
}

func TestValidateImageKeyInvalid(t *testing.T) {
	svc := NewService(newMemStore(), zap.NewNop())
	err := svc.ValidateImageKey("covers", "u1")
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
	err = svc.ValidateImageKey("covers/u1/bad.ext", "u1")
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
	err = svc.ValidateImageKey("covers/u2/x.png", "u1")
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
	err = svc.ValidateImageKey("covers/u1/abc.png", "u1")
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
}

func TestValidateImageKeySuccess(t *testing.T) {
	svc := NewService(newMemStore(), zap.NewNop())
	require.NoError(t, svc.ValidateImageKey(imageKey("avatars", "u1"), "u1"))
}

func TestRemoveEmptyKey(t *testing.T) {
	svc := NewService(newMemStore(), zap.NewNop())
	require.NoError(t, svc.Remove(context.Background(), ""))
}

func TestRemoveStoreError(t *testing.T) {
	store := &errDeleteStore{}
	svc := NewService(store, zap.NewNop())
	err := svc.Remove(context.Background(), "avatars/u1/x.png")
	require.ErrorIs(t, err, pkgerrors.ErrInternal)
}

func TestRemoveManySkipsEmptyAndKeepsGoingOnError(t *testing.T) {
	store := newMemStore()
	store.files["avatars/u1/a.png"] = []byte("a")
	store.files["avatars/u1/b.png"] = []byte("b")
	svc := NewService(store, zap.NewNop())
	svc.RemoveMany(context.Background(), []string{"", "avatars/u1/a.png", "avatars/u1/zz.png"})
	assert.NotContains(t, store.files, "avatars/u1/a.png")
	assert.Contains(t, store.files, "avatars/u1/b.png")
}

func TestRemoveManyLogsOnDeleteError(t *testing.T) {
	svc := NewService(&errDeleteStore{}, zap.NewNop())
	svc.RemoveMany(context.Background(), []string{"avatars/u1/a.png"})
}

func TestSignImageMissingKey(t *testing.T) {
	svc := NewService(newMemStore(), zap.NewNop())
	_, err := svc.SignImage(context.Background(), "u1", "")
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
}

func TestSignImageMalformedKey(t *testing.T) {
	svc := NewService(newMemStore(), zap.NewNop())
	_, err := svc.SignImage(context.Background(), "u1", "covers")
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
	_, err = svc.SignImage(context.Background(), "u1", "covers/u1/bad.ext")
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
	_, err = svc.SignImage(context.Background(), "u1", "covers/notauuid/x.png")
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
	_, err = svc.SignImage(context.Background(), "u1", "covers/u1/abc.png")
	require.ErrorIs(t, err, pkgerrors.ErrNotFound)
}

func TestSignImagePresignError(t *testing.T) {
	svc := NewService(&presignErrStore{}, zap.NewNop())
	_, err := svc.SignImage(context.Background(), "u1", imageKey("avatars", testutil.NewUUID()))
	require.ErrorIs(t, err, pkgerrors.ErrInternal)
}

func TestSignImageNonAvatarNoAccess(t *testing.T) {
	svc := NewService(newMemStore(), zap.NewNop())
	_, err := svc.SignImage(context.Background(), "u1", imageKey("covers", testutil.NewUUID()))
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}

func TestSignImageAccessCheckRepoError(t *testing.T) {
	svc := NewService(newMemStore(), zap.NewNop())
	tasks := testutil.NewFakeTaskRepo()
	svc.WithAccessCheck(tasks, testutil.NewFakeWorkspaceRepo())
	_, err := svc.SignImage(context.Background(), "u1", imageKey("covers", testutil.NewUUID()))
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}

func TestSignImageDeniedForUnreferencedTaskAsMember(t *testing.T) {
	svc := NewService(newMemStore(), zap.NewNop())
	tasks := testutil.NewFakeTaskRepo()
	workspaces := testutil.NewFakeWorkspaceRepo()
	svc.WithAccessCheck(tasks, workspaces)

	key := imageKey("covers", testutil.NewUUID())
	task := &mtask.Task{ColumnID: "c1", ImageKey: key}
	tasks.Tasks[testutil.NewUUID()] = task
	workspaces.AddMemberFixture("ws-no", "u1", mworkspace.RoleMember)
	tasks.ColumnWorkspaces["c1"] = "ws1"

	_, err := svc.SignImage(context.Background(), "u1", key)
	require.ErrorIs(t, err, pkgerrors.ErrForbidden)
}

type errDeleteStore struct{ files map[string][]byte }

func newErrDeleteStore() *errDeleteStore {
	return &errDeleteStore{files: make(map[string][]byte)}
}

func (s *errDeleteStore) Put(context.Context, string, io.Reader, int64, string) error {
	return nil
}

func (s *errDeleteStore) Get(context.Context, string) (io.ReadCloser, error) {
	return nil, nil
}

func (s *errDeleteStore) Delete(context.Context, string) error {
	return errors.New("delete failed")
}

func (s *errDeleteStore) Exists(context.Context, string) (bool, error) {
	return true, nil
}

func (s *errDeleteStore) PresignGet(context.Context, string, time.Duration) (string, error) {
	return "", nil
}

type existsErrStore struct{}

func (s *existsErrStore) Put(context.Context, string, io.Reader, int64, string) error {
	return nil
}

func (s *existsErrStore) Get(context.Context, string) (io.ReadCloser, error) {
	return nil, nil
}

func (s *existsErrStore) Delete(context.Context, string) error {
	return nil
}

func (s *existsErrStore) Exists(context.Context, string) (bool, error) {
	return false, errors.New("exists failed")
}

func (s *existsErrStore) PresignGet(context.Context, string, time.Duration) (string, error) {
	return "", nil
}

type existsOnlyStore struct{}

func (s *existsOnlyStore) Put(context.Context, string, io.Reader, int64, string) error {
	return nil
}

func (s *existsOnlyStore) Get(context.Context, string) (io.ReadCloser, error) {
	return nil, errors.New("get failed")
}

func (s *existsOnlyStore) Delete(context.Context, string) error {
	return nil
}

func (s *existsOnlyStore) Exists(context.Context, string) (bool, error) {
	return true, nil
}

func (s *existsOnlyStore) PresignGet(context.Context, string, time.Duration) (string, error) {
	return "", nil
}

type putErrStore struct{}

func (s *putErrStore) Put(context.Context, string, io.Reader, int64, string) error {
	return errors.New("put failed")
}

func (s *putErrStore) Get(context.Context, string) (io.ReadCloser, error) {
	return nil, nil
}

func (s *putErrStore) Delete(context.Context, string) error {
	return nil
}

func (s *putErrStore) Exists(context.Context, string) (bool, error) {
	return false, nil
}

func (s *putErrStore) PresignGet(context.Context, string, time.Duration) (string, error) {
	return "", nil
}

type presignErrStore struct{}

func (s *presignErrStore) Put(context.Context, string, io.Reader, int64, string) error {
	return nil
}

func (s *presignErrStore) Get(context.Context, string) (io.ReadCloser, error) {
	return nil, nil
}

func (s *presignErrStore) Delete(context.Context, string) error {
	return nil
}

func (s *presignErrStore) Exists(context.Context, string) (bool, error) {
	return true, nil
}

func (s *presignErrStore) PresignGet(context.Context, string, time.Duration) (string, error) {
	return "", errors.New("presign failed")
}
