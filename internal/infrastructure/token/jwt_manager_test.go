package token

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rediscache "github.com/tandem/tandem/internal/infrastructure/redis"
	"github.com/tandem/tandem/internal/pkg/config"
)

func newManagerWithCache(t *testing.T) (*JWTManager, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	cache, err := rediscache.New(config.RedisConfig{Addr: mr.Addr()})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = cache.Close()
		mr.Close()
	})
	return NewJWTManager("test-secret", cache), mr
}

func TestGenerateAndParse(t *testing.T) {
	manager, _ := newManagerWithCache(t)

	token, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)

	subject, err := manager.Parse(token)
	require.NoError(t, err)
	assert.Equal(t, "user-1", subject)
}

func TestRevokeInvalidatesToken(t *testing.T) {
	manager, _ := newManagerWithCache(t)

	token, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)

	_, err = manager.Parse(token)
	require.NoError(t, err)

	require.NoError(t, manager.Revoke(context.Background(), "user-1"))

	_, err = manager.Parse(token)
	assert.Error(t, err)
}

func TestNewTokenWorksAfterRevoke(t *testing.T) {
	manager, _ := newManagerWithCache(t)

	old, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)

	require.NoError(t, manager.Revoke(context.Background(), "user-1"))
	_, err = manager.Parse(old)
	assert.Error(t, err)

	fresh, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)

	subject, err := manager.Parse(fresh)
	require.NoError(t, err)
	assert.Equal(t, "user-1", subject)
}

func TestRevokeAffectsOnlySubject(t *testing.T) {
	manager, _ := newManagerWithCache(t)

	userA, err := manager.Generate("user-a", time.Hour)
	require.NoError(t, err)
	userB, err := manager.Generate("user-b", time.Hour)
	require.NoError(t, err)

	require.NoError(t, manager.Revoke(context.Background(), "user-a"))

	_, err = manager.Parse(userA)
	assert.Error(t, err)

	subject, err := manager.Parse(userB)
	require.NoError(t, err)
	assert.Equal(t, "user-b", subject)
}

func TestNilCacheFailOpen(t *testing.T) {
	manager := NewJWTManager("test-secret", nil)

	token, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)

	require.NoError(t, manager.Revoke(context.Background(), "user-1"))

	subject, err := manager.Parse(token)
	require.NoError(t, err)
	assert.Equal(t, "user-1", subject)
}

func TestParseExpiredToken(t *testing.T) {
	manager := NewJWTManager("test-secret", nil)

	token, err := manager.Generate("user-1", -time.Minute)
	require.NoError(t, err)

	_, err = manager.Parse(token)
	assert.Error(t, err)
}

func TestParseInvalidSignature(t *testing.T) {
	manager := NewJWTManager("test-secret", nil)

	other := NewJWTManager("other-secret", nil)
	otherToken, err := other.Generate("user-1", time.Hour)
	require.NoError(t, err)

	_, err = manager.Parse(otherToken)
	assert.Error(t, err)
}

func TestHybridRejectUnknownOnCacheError(t *testing.T) {
	manager, mr := newManagerWithCache(t)

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		Ver:              999,
		RegisteredClaims: jwt.RegisteredClaims{Subject: "user-unknown", Issuer: "tandem", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}).SignedString(manager.secret)
	require.NoError(t, err)

	mr.Close()

	_, err = manager.Parse(signed)
	assert.Error(t, err)
}

func TestHybridParseSucceedsDuringCacheError(t *testing.T) {
	manager, mr := newManagerWithCache(t)

	token, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)

	_, err = manager.Parse(token)
	require.NoError(t, err)

	mr.Close()

	_, err = manager.Parse(token)
	require.NoError(t, err)
}

func TestHybridRevokeDuringCacheError(t *testing.T) {
	manager, mr := newManagerWithCache(t)

	token, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)

	_, err = manager.Parse(token)
	require.NoError(t, err)

	mr.Close()

	require.NoError(t, manager.Revoke(context.Background(), "user-1"))
	_, err = manager.Parse(token)
	assert.Error(t, err)
}

func TestFailClosedWhenVersionKeyMissing(t *testing.T) {
	manager, _ := newManagerWithCache(t)

	token, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)

	require.NoError(t, manager.cache.Delete(context.Background(), versionPrefix+"user-1"))

	_, err = manager.Parse(token)
	assert.Error(t, err)
}

func TestGenerateFailsClosedWhenCacheDown(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	cache, err := rediscache.New(config.RedisConfig{Addr: mr.Addr()})
	require.NoError(t, err)
	defer cache.Close()

	mr.SetError("boom")
	manager := NewJWTManager("test-secret", cache)
	_, err = manager.Generate("user-1", time.Hour)
	assert.Error(t, err)
}
