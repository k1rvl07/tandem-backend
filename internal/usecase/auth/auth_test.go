package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/tandem/tandem/internal/domain/models"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/http/dto"
	"github.com/tandem/tandem/internal/infrastructure/password"
	"github.com/tandem/tandem/internal/infrastructure/token"
)

type fakeUserRepo struct {
	users map[string]*models.User
	byID  map[string]*models.User
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{
		users: make(map[string]*models.User),
		byID:  make(map[string]*models.User),
	}
}

func (f *fakeUserRepo) Create(_ context.Context, user *models.User) error {
	if _, ok := f.users[user.Login]; ok {
		return errors.New("unique constraint")
	}
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now
	f.users[user.Login] = user
	f.byID[user.ID] = user
	return nil
}

func (f *fakeUserRepo) FindByID(_ context.Context, id string) (*models.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}

func (f *fakeUserRepo) FindByLogin(_ context.Context, login string) (*models.User, error) {
	u, ok := f.users[login]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}

func (f *fakeUserRepo) ExistsByLogin(_ context.Context, login string) (bool, error) {
	_, ok := f.users[login]
	return ok, nil
}

func (f *fakeUserRepo) List(_ context.Context) ([]*models.User, error) {
	users := make([]*models.User, 0, len(f.users))
	for _, u := range f.users {
		users = append(users, u)
	}
	return users, nil
}

func (f *fakeUserRepo) Update(_ context.Context, user *models.User) error {
	if _, ok := f.users[user.Login]; !ok {
		if _, exists := f.byID[user.ID]; !exists {
			return errors.New("not found")
		}
		delete(f.users, f.byID[user.ID].Login)
	}
	old := f.byID[user.ID]
	f.users[user.Login] = user
	f.byID[user.ID] = user
	if old != nil {
		user.CreatedAt = old.CreatedAt
	}
	return nil
}

func (f *fakeUserRepo) Delete(_ context.Context, id string) error {
	user, ok := f.byID[id]
	if !ok {
		return errors.New("not found")
	}
	delete(f.users, user.Login)
	delete(f.byID, id)
	return nil
}

func newTestService(repo repository.UserRepository) *Service {
	tokens := token.NewJWTManager("test-secret")
	hasher := password.NewBCryptHasher()
	return NewService(repo, tokens, hasher, 24*time.Hour)
}

func TestLoginSuccess(t *testing.T) {
	repo := newFakeUserRepo()
	hasher := password.NewBCryptHasher()
	hash, _ := hasher.Hash("password123")
	err := repo.Create(context.Background(), &models.User{
		ID:           uuid.New().String(),
		Login:        "ivanov.ii",
		PasswordHash: hash,
		Role:         models.RoleUser,
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	svc := newTestService(repo)

	resp, err := svc.Login(context.Background(), dto.LoginRequest{
		Login:    " Ivanov.II ",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if resp.Token == "" {
		t.Error("expected non-empty token")
	}
	if resp.User.Login != "ivanov.ii" {
		t.Errorf("expected normalized login, got %q", resp.User.Login)
	}
	if resp.User.Role != models.RoleUser {
		t.Errorf("expected role user, got %q", resp.User.Role)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	repo := newFakeUserRepo()
	hasher := password.NewBCryptHasher()
	hash, _ := hasher.Hash("password123")
	err := repo.Create(context.Background(), &models.User{
		ID:           uuid.New().String(),
		Login:        "ivanov.ii",
		PasswordHash: hash,
		Role:         models.RoleUser,
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	svc := newTestService(repo)

	if _, err := svc.Login(context.Background(), dto.LoginRequest{
		Login:    "ivanov.ii",
		Password: "wrong-password",
	}); err == nil {
		t.Fatal("expected error on wrong password")
	}
}

func TestLoginUnknownLogin(t *testing.T) {
	svc := newTestService(newFakeUserRepo())

	if _, err := svc.Login(context.Background(), dto.LoginRequest{
		Login:    "nobody",
		Password: "password123",
	}); err == nil {
		t.Fatal("expected error on unknown login")
	}
}

func TestLoginValidation(t *testing.T) {
	svc := newTestService(newFakeUserRepo())

	cases := []dto.LoginRequest{
		{Login: "", Password: "password123"},
		{Login: "ivanov.ii", Password: ""},
	}
	for _, req := range cases {
		if _, err := svc.Login(context.Background(), req); err == nil {
			t.Errorf("expected validation error for %+v", req)
		}
	}
}
