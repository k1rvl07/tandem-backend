package profile

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	dprofile "github.com/tandem/tandem/internal/contracts/profile"
	muser "github.com/tandem/tandem/internal/domain/models/user"
	"github.com/tandem/tandem/internal/domain/ports/filestore"
	"github.com/tandem/tandem/internal/infrastructure/password"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
	file "github.com/tandem/tandem/internal/usecase/file"
	"github.com/tandem/tandem/internal/usecase/testutil"
	"go.uber.org/zap"
)

type fakeRepo struct {
	users     map[string]*muser.User
	updateErr error
	findErr   error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{users: make(map[string]*muser.User)}
}

func (f *fakeRepo) Create(_ context.Context, user *muser.User) error {
	f.users[user.Login] = user
	return nil
}

func (f *fakeRepo) FindByID(_ context.Context, id string) (*muser.User, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	for _, u := range f.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, errors.New("not found")
}

func (f *fakeRepo) FindByLogin(_ context.Context, login string) (*muser.User, error) {
	u, ok := f.users[login]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}

func (f *fakeRepo) ExistsByLogin(_ context.Context, login string) (bool, error) {
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

func (f *fakeRepo) ListPage(context.Context, string, int, int) ([]*muser.User, error) {
	return f.List(context.Background())
}

func (f *fakeRepo) Count(_ context.Context, _ string) (int, error) {
	return len(f.users), nil
}

func (f *fakeRepo) Update(_ context.Context, user *muser.User) error {
	if f.updateErr != nil {
		return f.updateErr
	}
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
	return errors.New("not found")
}

type fakeStore struct {
	files  map[string][]byte
	putErr error
}

func newFakeStore() *fakeStore {
	return &fakeStore{files: make(map[string][]byte)}
}

func (s *fakeStore) Put(_ context.Context, key string, reader io.Reader, _ int64, _ string) error {
	if s.putErr != nil {
		return s.putErr
	}
	data, _ := io.ReadAll(reader)
	s.files[key] = data
	return nil
}

func (s *fakeStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	data, ok := s.files[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (s *fakeStore) Delete(_ context.Context, key string) error {
	delete(s.files, key)
	return nil
}

func (s *fakeStore) Exists(_ context.Context, key string) (bool, error) {
	_, ok := s.files[key]
	return ok, nil
}

func (s *fakeStore) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return key, nil
}

func newTestService(repo *fakeRepo, store filestore.FileStore) (*Service, *testutil.FakeTokenService) {
	hasher := password.NewBCryptHasher(12)
	tokens := &testutil.FakeTokenService{}
	return NewService(Deps{Users: repo, Hasher: hasher, Files: file.NewService(store, zap.NewNop()), Cache: testutil.NewFakeCache(), Tokens: tokens, TokenTTL: time.Hour, RefreshTTL: 7 * 24 * time.Hour, Logger: zap.NewNop()}), tokens
}

func TestGetProfile(t *testing.T) {
	repo := newFakeRepo()
	user := &muser.User{ID: "u1", Login: "ivanov.ii", PasswordHash: "h", Role: muser.RoleUser, DisplayName: "Alice", Bio: "hello"}
	repo.users[user.Login] = user
	svc, _ := newTestService(repo, newFakeStore())

	resp, err := svc.Get(context.Background(), "u1")
	require.NoError(t, err)
	assert.Equal(t, "Alice", resp.DisplayName)
	assert.Equal(t, "hello", resp.Bio)
	assert.Equal(t, "ivanov.ii", resp.Login)
}

func TestUpdateProfile(t *testing.T) {
	repo := newFakeRepo()
	user := &muser.User{ID: "u1", Login: "ivanov.ii", PasswordHash: "h", Role: muser.RoleUser}
	repo.users[user.Login] = user
	svc, _ := newTestService(repo, newFakeStore())

	resp, err := svc.UpdateProfile(context.Background(), "u1", dprofile.UpdateProfileRequest{DisplayName: "Bob", Bio: "dev"})
	require.NoError(t, err)
	assert.Equal(t, "Bob", resp.DisplayName)
	assert.Equal(t, "dev", resp.Bio)
}

func TestChangePasswordWrongCurrent(t *testing.T) {
	repo := newFakeRepo()
	hasher := password.NewBCryptHasher(12)
	hash, _ := hasher.Hash("oldpass123")
	user := &muser.User{ID: "u1", Login: "ivanov.ii", PasswordHash: hash, Role: muser.RoleUser}
	repo.users[user.Login] = user
	svc, _ := newTestService(repo, newFakeStore())

	_, err := svc.ChangePassword(context.Background(), "u1", dprofile.ChangePasswordRequest{NewPassword: "newpass123", CurrentPassword: "wrong"})
	require.Error(t, err)
}

func TestChangePassword(t *testing.T) {
	repo := newFakeRepo()
	hasher := password.NewBCryptHasher(12)
	hash, _ := hasher.Hash("oldpass123")
	user := &muser.User{ID: "u1", Login: "ivanov.ii", PasswordHash: hash, Role: muser.RoleUser}
	repo.users[user.Login] = user
	svc, tokens := newTestService(repo, newFakeStore())

	resp, err := svc.ChangePassword(context.Background(), "u1", dprofile.ChangePasswordRequest{NewPassword: "newpass123", CurrentPassword: "oldpass123"})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Token)
	require.NotEmpty(t, resp.RefreshToken)
	require.Len(t, tokens.Revoked, 1)
	require.Equal(t, "u1", tokens.Revoked[0])
	assert.True(t, hasher.Check(repo.users["ivanov.ii"].PasswordHash, "newpass123"))
}

func TestChangePasswordTooShort(t *testing.T) {
	repo := newFakeRepo()
	hasher := password.NewBCryptHasher(12)
	hash, _ := hasher.Hash("oldpass123")
	user := &muser.User{ID: "u1", Login: "ivanov.ii", PasswordHash: hash, Role: muser.RoleUser}
	repo.users[user.Login] = user
	svc, _ := newTestService(repo, newFakeStore())

	_, err := svc.ChangePassword(context.Background(), "u1", dprofile.ChangePasswordRequest{NewPassword: "short", CurrentPassword: "oldpass123"})
	require.Error(t, err)
}

func TestRemoveAvatar(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStore()
	key := "avatars/u1/8eb8c3b2-89f4-4b2a-a1bd-3e5d5e7f9b9a.jpg"
	store.files[key] = []byte("avatar")
	user := &muser.User{ID: "u1", Login: "ivanov.ii", PasswordHash: "h", AvatarKey: key}
	repo.users[user.Login] = user
	svc, _ := newTestService(repo, store)

	resp, err := svc.RemoveAvatar(context.Background(), "u1")
	require.NoError(t, err)
	assert.Empty(t, resp.AvatarKey)
	assert.Empty(t, repo.users["ivanov.ii"].AvatarKey)
	_, ok := store.files[key]
	assert.False(t, ok)
}

func TestRemoveAvatarNoAvatar(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStore()
	user := &muser.User{ID: "u1", Login: "ivanov.ii", PasswordHash: "h"}
	repo.users[user.Login] = user
	svc, _ := newTestService(repo, store)

	resp, err := svc.RemoveAvatar(context.Background(), "u1")
	require.NoError(t, err)
	assert.Empty(t, resp.AvatarKey)
	assert.Empty(t, store.files)
}

func TestUploadAvatarRollbackOnUpdateError(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeStore()
	user := &muser.User{ID: "u1", Login: "ivanov.ii", PasswordHash: "h"}
	repo.users[user.Login] = user
	repo.updateErr = errors.New("boom")
	svc, _ := newTestService(repo, store)

	_, err := svc.UploadAvatar(context.Background(), "u1", "photo.jpg", "image/jpeg", bytes.NewReader([]byte("data")), 4)
	require.Error(t, err)
	assert.Empty(t, store.files)
}

func TestGetProfileUserLookupError(t *testing.T) {
	repo := newFakeRepo()
	repo.findErr = errors.New("boom")
	svc, _ := newTestService(repo, newFakeStore())
	_, err := svc.Get(context.Background(), "u1")
	require.Error(t, err)
}

func TestUpdateProfileUserLookupError(t *testing.T) {
	repo := newFakeRepo()
	repo.findErr = errors.New("boom")
	svc, _ := newTestService(repo, newFakeStore())
	_, err := svc.UpdateProfile(context.Background(), "u1", dprofile.UpdateProfileRequest{DisplayName: "Bob"})
	require.Error(t, err)
}

func TestUpdateProfileUpdateError(t *testing.T) {
	repo := newFakeRepo()
	repo.users["ivanov.ii"] = &muser.User{ID: "u1", Login: "ivanov.ii", PasswordHash: "h"}
	repo.updateErr = errors.New("boom")
	svc, _ := newTestService(repo, newFakeStore())
	_, err := svc.UpdateProfile(context.Background(), "u1", dprofile.UpdateProfileRequest{DisplayName: "Bob"})
	require.Error(t, err)
}

func TestUpdateProfileDisplayNameTooLong(t *testing.T) {
	repo := newFakeRepo()
	repo.users["ivanov.ii"] = &muser.User{ID: "u1", Login: "ivanov.ii", PasswordHash: "h"}
	svc, _ := newTestService(repo, newFakeStore())
	long := make([]rune, 51)
	for i := range long {
		long[i] = 'a'
	}
	_, err := svc.UpdateProfile(context.Background(), "u1", dprofile.UpdateProfileRequest{DisplayName: string(long)})
	require.Error(t, err)
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
}

func TestUpdateProfileBioTooLong(t *testing.T) {
	repo := newFakeRepo()
	repo.users["ivanov.ii"] = &muser.User{ID: "u1", Login: "ivanov.ii", PasswordHash: "h"}
	svc, _ := newTestService(repo, newFakeStore())
	long := make([]rune, 401)
	for i := range long {
		long[i] = 'a'
	}
	_, err := svc.UpdateProfile(context.Background(), "u1", dprofile.UpdateProfileRequest{Bio: string(long)})
	require.Error(t, err)
	require.ErrorIs(t, err, pkgerrors.ErrValidation)
}

func TestChangePasswordUserLookupError(t *testing.T) {
	repo := newFakeRepo()
	repo.findErr = errors.New("boom")
	svc, _ := newTestService(repo, newFakeStore())
	_, err := svc.ChangePassword(context.Background(), "u1", dprofile.ChangePasswordRequest{CurrentPassword: "oldpass123", NewPassword: "newpass123"})
	require.Error(t, err)
}

func TestChangePasswordRevokeError(t *testing.T) {
	repo := newFakeRepo()
	hasher := password.NewBCryptHasher(12)
	hash, _ := hasher.Hash("oldpass123")
	repo.users["ivanov.ii"] = &muser.User{ID: "u1", Login: "ivanov.ii", PasswordHash: hash}
	svc, tokens := newTestService(repo, newFakeStore())
	tokens.RevokeErr = errors.New("boom")
	_, err := svc.ChangePassword(context.Background(), "u1", dprofile.ChangePasswordRequest{CurrentPassword: "oldpass123", NewPassword: "newpass123"})
	require.Error(t, err)
}

func TestChangePasswordGenerateAccessError(t *testing.T) {
	repo := newFakeRepo()
	hasher := password.NewBCryptHasher(12)
	hash, _ := hasher.Hash("oldpass123")
	repo.users["ivanov.ii"] = &muser.User{ID: "u1", Login: "ivanov.ii", PasswordHash: hash}
	svc, tokens := newTestService(repo, newFakeStore())
	tokens.GenErr = errors.New("boom")
	_, err := svc.ChangePassword(context.Background(), "u1", dprofile.ChangePasswordRequest{CurrentPassword: "oldpass123", NewPassword: "newpass123"})
	require.Error(t, err)
}

func TestUploadAvatarUserLookupError(t *testing.T) {
	repo := newFakeRepo()
	repo.findErr = errors.New("boom")
	svc, _ := newTestService(repo, newFakeStore())
	_, err := svc.UploadAvatar(context.Background(), "u1", "photo.jpg", "image/jpeg", bytes.NewReader([]byte("data")), 4)
	require.Error(t, err)
}

func TestUploadAvatarStoreError(t *testing.T) {
	repo := newFakeRepo()
	repo.users["ivanov.ii"] = &muser.User{ID: "u1", Login: "ivanov.ii", PasswordHash: "h"}
	store := newFakeStore()
	store.putErr = errors.New("boom")
	svc, _ := newTestService(repo, store)
	_, err := svc.UploadAvatar(context.Background(), "u1", "photo.jpg", "image/jpeg", bytes.NewReader([]byte("data")), 4)
	require.Error(t, err)
}

func TestRemoveAvatarUserLookupError(t *testing.T) {
	repo := newFakeRepo()
	repo.findErr = errors.New("boom")
	svc, _ := newTestService(repo, newFakeStore())
	_, err := svc.RemoveAvatar(context.Background(), "u1")
	require.Error(t, err)
}

func TestRemoveAvatarUpdateError(t *testing.T) {
	repo := newFakeRepo()
	key := "avatars/u1/8eb8c3b2-89f4-4b2a-a1bd-3e5d5e7f9b9a.jpg"
	repo.users["ivanov.ii"] = &muser.User{ID: "u1", Login: "ivanov.ii", PasswordHash: "h", AvatarKey: key}
	repo.updateErr = errors.New("boom")
	store := newFakeStore()
	svc, _ := newTestService(repo, store)
	_, err := svc.RemoveAvatar(context.Background(), "u1")
	require.Error(t, err)
}
