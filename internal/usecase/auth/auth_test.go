package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func (f *fakeUserRepo) ListPage(context.Context, string, int, int) ([]*models.User, error) {
	return f.List(context.Background())
}

func (f *fakeUserRepo) Count(_ context.Context, _ string) (int, error) {
	return len(f.users), nil
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
	tokens := token.NewJWTManager("test-secret", nil)
	hasher := password.NewBCryptHasher(12)
	return NewService(repo, tokens, hasher, 24*time.Hour)
}

func TestLoginSuccess(t *testing.T) {
	repo := newFakeUserRepo()
	hasher := password.NewBCryptHasher(12)
	hash, _ := hasher.Hash("password123")
	err := repo.Create(context.Background(), &models.User{
		ID:           uuid.New().String(),
		Login:        "ivanov.ii",
		PasswordHash: hash,
		Role:         models.RoleUser,
	})
	require.NoError(t, err)
	svc := newTestService(repo)

	resp, err := svc.Login(context.Background(), dto.LoginRequest{
		Login:    " Ivanov.II ",
		Password: "password123",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
	assert.Equal(t, "ivanov.ii", resp.User.Login)
	assert.Equal(t, models.RoleUser, resp.User.Role)
}

func TestLoginWrongPassword(t *testing.T) {
	repo := newFakeUserRepo()
	hasher := password.NewBCryptHasher(12)
	hash, _ := hasher.Hash("password123")
	err := repo.Create(context.Background(), &models.User{
		ID:           uuid.New().String(),
		Login:        "ivanov.ii",
		PasswordHash: hash,
		Role:         models.RoleUser,
	})
	require.NoError(t, err)
	svc := newTestService(repo)

	_, err = svc.Login(context.Background(), dto.LoginRequest{
		Login:    "ivanov.ii",
		Password: "wrong-password",
	})
	require.Error(t, err)
}

func TestLoginUnknownLogin(t *testing.T) {
	svc := newTestService(newFakeUserRepo())

	_, err := svc.Login(context.Background(), dto.LoginRequest{
		Login:    "nobody",
		Password: "password123",
	})
	require.Error(t, err)
}

func TestLoginValidation(t *testing.T) {
	svc := newTestService(newFakeUserRepo())

	cases := []dto.LoginRequest{
		{Login: "", Password: "password123"},
		{Login: "ivanov.ii", Password: ""},
	}
	for _, req := range cases {
		_, err := svc.Login(context.Background(), req)
		assert.Error(t, err)
	}
}

type recordingTokens struct {
	revoked []string
}

func (r *recordingTokens) Generate(string, time.Duration) (string, error) {
	return "token", nil
}

func (r *recordingTokens) Parse(string) (string, error) {
	return "u1", nil
}

func (r *recordingTokens) Revoke(_ context.Context, subject string) error {
	r.revoked = append(r.revoked, subject)
	return nil
}

func TestLogoutRevokesToken(t *testing.T) {
	tokens := &recordingTokens{}
	svc := NewService(newFakeUserRepo(), tokens, password.NewBCryptHasher(12), time.Hour)

	err := svc.Logout(context.Background(), "u1")
	require.NoError(t, err)
	require.Len(t, tokens.revoked, 1)
	require.Equal(t, "u1", tokens.revoked[0])
}
