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
	if _, ok := f.users[user.Email]; ok {
		return errors.New("unique constraint")
	}
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now
	f.users[user.Email] = user
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

func (f *fakeUserRepo) FindByEmail(_ context.Context, email string) (*models.User, error) {
	u, ok := f.users[email]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}

func (f *fakeUserRepo) ExistsByEmail(_ context.Context, email string) (bool, error) {
	_, ok := f.users[email]
	return ok, nil
}

func newTestService(repo repository.UserRepository) *Service {
	tokens := token.NewJWTManager("test-secret")
	hasher := password.NewBCryptHasher()
	return NewService(repo, tokens, hasher, 24*time.Hour)
}

func TestRegisterSuccess(t *testing.T) {
	repo := newFakeUserRepo()
	svc := newTestService(repo)

	resp, err := svc.Register(context.Background(), dto.RegisterRequest{
		Email:           "  User@Example.com ",
		Password:        "password123",
		ConfirmPassword: "password123",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.User.Email != "user@example.com" {
		t.Errorf("expected lowercase email, got %q", resp.User.Email)
	}
	if resp.User.ID == "" {
		t.Error("expected non-empty user id")
	}
	if _, err := uuid.Parse(resp.User.ID); err != nil {
		t.Errorf("expected uuid id, got %q: %v", resp.User.ID, err)
	}
	if resp.User.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	repo := newFakeUserRepo()
	svc := newTestService(repo)

	req := dto.RegisterRequest{
		Email:           "user@example.com",
		Password:        "password123",
		ConfirmPassword: "password123",
	}
	if _, err := svc.Register(context.Background(), req); err != nil {
		t.Fatalf("first register failed: %v", err)
	}
	if _, err := svc.Register(context.Background(), req); err == nil {
		t.Fatal("expected error on duplicate register")
	}
}

func TestRegisterValidation(t *testing.T) {
	svc := newTestService(newFakeUserRepo())

	cases := []dto.RegisterRequest{
		{Email: "", Password: "password123", ConfirmPassword: "password123"},
		{Email: "not-an-email", Password: "password123", ConfirmPassword: "password123"},
		{Email: "a@b.com", Password: "short", ConfirmPassword: "short"},
		{Email: "a@b.com", Password: "password123", ConfirmPassword: "different"},
	}
	for _, req := range cases {
		if _, err := svc.Register(context.Background(), req); err == nil {
			t.Errorf("expected validation error for %+v", req)
		}
	}
}

func TestRegisterStoresHashedPassword(t *testing.T) {
	repo := newFakeUserRepo()
	svc := newTestService(repo)

	_, err := svc.Register(context.Background(), dto.RegisterRequest{
		Email:           "a@b.com",
		Password:        "password123",
		ConfirmPassword: "password123",
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	stored, err := repo.FindByEmail(context.Background(), "a@b.com")
	if err != nil {
		t.Fatalf("find failed: %v", err)
	}
	if stored.PasswordHash == "password123" {
		t.Error("password must be hashed")
	}
	if !password.NewBCryptHasher().Check(stored.PasswordHash, "password123") {
		t.Error("stored hash should verify against the plain password")
	}
}

func TestLoginSuccess(t *testing.T) {
	repo := newFakeUserRepo()
	svc := newTestService(repo)

	_, err := svc.Register(context.Background(), dto.RegisterRequest{
		Email:           "a@b.com",
		Password:        "password123",
		ConfirmPassword: "password123",
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	resp, err := svc.Login(context.Background(), dto.LoginRequest{
		Email:    "a@b.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if resp.Token == "" {
		t.Error("expected non-empty token")
	}
	if resp.User.Email != "a@b.com" {
		t.Errorf("expected email a@b.com, got %q", resp.User.Email)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	repo := newFakeUserRepo()
	svc := newTestService(repo)

	_, err := svc.Register(context.Background(), dto.RegisterRequest{
		Email:           "a@b.com",
		Password:        "password123",
		ConfirmPassword: "password123",
	})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	if _, err := svc.Login(context.Background(), dto.LoginRequest{
		Email:    "a@b.com",
		Password: "wrong-password",
	}); err == nil {
		t.Fatal("expected error on wrong password")
	}
}
