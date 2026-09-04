package profile

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/tandem/tandem/internal/domain/models"
	"github.com/tandem/tandem/internal/domain/ports/filestore"
	"github.com/tandem/tandem/internal/http/dto"
	"github.com/tandem/tandem/internal/infrastructure/password"
	file "github.com/tandem/tandem/internal/usecase/file"
)

type fakeRepo struct {
	users map[string]*models.User
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{users: make(map[string]*models.User)}
}

func (f *fakeRepo) Create(_ context.Context, user *models.User) error {
	f.users[user.Login] = user
	return nil
}

func (f *fakeRepo) FindByID(_ context.Context, id string) (*models.User, error) {
	for _, u := range f.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, errors.New("not found")
}

func (f *fakeRepo) FindByLogin(_ context.Context, login string) (*models.User, error) {
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

func (f *fakeRepo) List(_ context.Context) ([]*models.User, error) {
	users := make([]*models.User, 0, len(f.users))
	for _, u := range f.users {
		users = append(users, u)
	}
	return users, nil
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
	return errors.New("not found")
}

type fakeStore struct {
	files map[string][]byte
}

func newFakeStore() *fakeStore {
	return &fakeStore{files: make(map[string][]byte)}
}

func (s *fakeStore) Put(_ context.Context, key string, reader io.Reader, _ int64, _ string) error {
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

func newTestService(repo *fakeRepo, store filestore.FileStore) *Service {
	hasher := password.NewBCryptHasher()
	return NewService(repo, hasher, file.NewService(store))
}

func TestGetProfile(t *testing.T) {
	repo := newFakeRepo()
	user := &models.User{ID: "u1", Login: "ivanov.ii", PasswordHash: "h", Role: models.RoleUser, DisplayName: "Alice", Bio: "hello"}
	repo.users[user.Login] = user
	svc := newTestService(repo, newFakeStore())

	resp, err := svc.Get(context.Background(), "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.DisplayName != "Alice" || resp.Bio != "hello" || resp.Login != "ivanov.ii" {
		t.Errorf("unexpected profile: %+v", resp)
	}
}

func TestUpdateProfile(t *testing.T) {
	repo := newFakeRepo()
	user := &models.User{ID: "u1", Login: "ivanov.ii", PasswordHash: "h", Role: models.RoleUser}
	repo.users[user.Login] = user
	svc := newTestService(repo, newFakeStore())

	resp, err := svc.UpdateProfile(context.Background(), "u1", dto.UpdateProfileRequest{DisplayName: "Bob", Bio: "dev"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.DisplayName != "Bob" || resp.Bio != "dev" {
		t.Errorf("unexpected profile: %+v", resp)
	}
}

func TestChangePasswordWrongCurrent(t *testing.T) {
	repo := newFakeRepo()
	hasher := password.NewBCryptHasher()
	hash, _ := hasher.Hash("oldpass123")
	user := &models.User{ID: "u1", Login: "ivanov.ii", PasswordHash: hash, Role: models.RoleUser}
	repo.users[user.Login] = user
	svc := newTestService(repo, newFakeStore())

	err := svc.ChangePassword(context.Background(), "u1", dto.ChangePasswordRequest{NewPassword: "newpass123", CurrentPassword: "wrong"})
	if err == nil {
		t.Fatal("expected error for wrong current password")
	}
}

func TestChangePassword(t *testing.T) {
	repo := newFakeRepo()
	hasher := password.NewBCryptHasher()
	hash, _ := hasher.Hash("oldpass123")
	user := &models.User{ID: "u1", Login: "ivanov.ii", PasswordHash: hash, Role: models.RoleUser}
	repo.users[user.Login] = user
	svc := newTestService(repo, newFakeStore())

	err := svc.ChangePassword(context.Background(), "u1", dto.ChangePasswordRequest{NewPassword: "newpass123", CurrentPassword: "oldpass123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasher.Check(repo.users["ivanov.ii"].PasswordHash, "newpass123") {
		t.Error("password was not updated")
	}
}

func TestChangePasswordTooShort(t *testing.T) {
	repo := newFakeRepo()
	hasher := password.NewBCryptHasher()
	hash, _ := hasher.Hash("oldpass123")
	user := &models.User{ID: "u1", Login: "ivanov.ii", PasswordHash: hash, Role: models.RoleUser}
	repo.users[user.Login] = user
	svc := newTestService(repo, newFakeStore())

	err := svc.ChangePassword(context.Background(), "u1", dto.ChangePasswordRequest{NewPassword: "short", CurrentPassword: "oldpass123"})
	if err == nil {
		t.Fatal("expected error for short password")
	}
}
