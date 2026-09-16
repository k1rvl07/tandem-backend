package token

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tandem/tandem/internal/domain/ports/cache"
	rediscache "github.com/tandem/tandem/internal/infrastructure/redis"
	"github.com/tandem/tandem/internal/pkg/config"
	pkgerrors "github.com/tandem/tandem/internal/pkg/errors"
)

type failingCache struct {
	cache.Cache
	failGet       bool
	failSet       bool
	failGetDel    bool
	failIncr      bool
	failSetPrefix string
}

func (f *failingCache) Get(ctx context.Context, key string) (string, error) {
	if f.failGet {
		return "", errors.New("get failed")
	}
	return f.Cache.Get(ctx, key)
}

func (f *failingCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	if f.failSet || (f.failSetPrefix != "" && strings.HasPrefix(key, f.failSetPrefix)) {
		return errors.New("set failed")
	}
	return f.Cache.Set(ctx, key, value, ttl)
}

func (f *failingCache) GetDel(ctx context.Context, key string) (string, error) {
	if f.failGetDel {
		return "", errors.New("getdel failed")
	}
	return f.Cache.GetDel(ctx, key)
}

func (f *failingCache) Incr(ctx context.Context, key string) (int64, error) {
	if f.failIncr {
		return 0, errors.New("incr failed")
	}
	return f.Cache.Incr(ctx, key)
}

func newFailingManager(t *testing.T) (*JWTManager, *failingCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rc, err := rediscache.New(config.RedisConfig{Addr: mr.Addr()})
	require.NoError(t, err)
	fc := &failingCache{Cache: rc}
	t.Cleanup(func() {
		_ = rc.Close()
		mr.Close()
	})
	return NewJWTManager("test-secret", fc), fc, mr
}

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

	err = manager.Revoke(context.Background(), "user-1")
	assert.Error(t, err)

	_, err = manager.Parse(token)
	require.NoError(t, err)
}

func TestMissingVersionKeyMeansVersionOne(t *testing.T) {
	manager, _ := newManagerWithCache(t)

	token, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)

	require.NoError(t, manager.cache.Delete(context.Background(), versionPrefix+"user-1"))

	subject, err := manager.Parse(token)
	require.NoError(t, err)
	assert.Equal(t, "user-1", subject)
}

func TestCurrentVersionDoesNotWriteVersionKey(t *testing.T) {
	manager, mr := newManagerWithCache(t)

	require.NoError(t, manager.cache.Delete(context.Background(), versionPrefix+"user-1"))

	ver, err := manager.currentVersion(context.Background(), "user-1")
	require.NoError(t, err)
	assert.Equal(t, int64(1), ver)

	require.NoError(t, manager.Revoke(context.Background(), "user-1"))

	stored, err := mr.Get(versionPrefix + "user-1")
	require.NoError(t, err)
	assert.Equal(t, "2", stored)
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

func TestRefreshGenerateAndParse(t *testing.T) {
	manager, _ := newManagerWithCache(t)

	refresh, err := manager.GenerateRefresh("user-1", 7*24*time.Hour)
	require.NoError(t, err)

	subject, err := manager.ParseRefresh(refresh)
	require.NoError(t, err)
	assert.Equal(t, "user-1", subject)
}

func TestRefreshRejectsAccessToken(t *testing.T) {
	manager, _ := newManagerWithCache(t)

	access, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)

	_, err = manager.ParseRefresh(access)
	assert.Error(t, err)
}

func TestAccessRejectsRefreshToken(t *testing.T) {
	manager, _ := newManagerWithCache(t)

	refresh, err := manager.GenerateRefresh("user-1", 7*24*time.Hour)
	require.NoError(t, err)

	_, err = manager.Parse(refresh)
	assert.Error(t, err)
}

func TestRotateRefreshIssuesNewPair(t *testing.T) {
	manager, _ := newManagerWithCache(t)

	refresh, err := manager.GenerateRefresh("user-1", 7*24*time.Hour)
	require.NoError(t, err)

	access, next, err := manager.RotateRefresh(context.Background(), refresh, time.Hour, 7*24*time.Hour)
	require.NoError(t, err)
	require.NotEmpty(t, access)
	require.NotEmpty(t, next)

	subject, err := manager.Parse(access)
	require.NoError(t, err)
	assert.Equal(t, "user-1", subject)
}

func TestRotateRefreshRejectsReuse(t *testing.T) {
	manager, _ := newManagerWithCache(t)

	refresh, err := manager.GenerateRefresh("user-1", 7*24*time.Hour)
	require.NoError(t, err)

	_, _, err = manager.RotateRefresh(context.Background(), refresh, time.Hour, 7*24*time.Hour)
	require.NoError(t, err)

	_, _, err = manager.RotateRefresh(context.Background(), refresh, time.Hour, 7*24*time.Hour)
	assert.Error(t, err)
}

func TestRotateRefreshRejectsAccessToken(t *testing.T) {
	manager, _ := newManagerWithCache(t)

	access, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)

	_, _, err = manager.RotateRefresh(context.Background(), access, time.Hour, 7*24*time.Hour)
	assert.Error(t, err)
}

func TestRefreshRevokedByVersion(t *testing.T) {
	manager, _ := newManagerWithCache(t)

	refresh, err := manager.GenerateRefresh("user-1", 7*24*time.Hour)
	require.NoError(t, err)

	require.NoError(t, manager.Revoke(context.Background(), "user-1"))

	_, err = manager.ParseRefresh(refresh)
	assert.Error(t, err)
}

func TestGenerateRefreshNilCache(t *testing.T) {
	manager := NewJWTManager("test-secret", nil)
	refresh, err := manager.GenerateRefresh("user-1", time.Hour)
	require.NoError(t, err)
	subject, err := manager.ParseRefresh(refresh)
	require.NoError(t, err)
	assert.Equal(t, "user-1", subject)
}

func TestGenerateRefreshSetError(t *testing.T) {
	manager, fc, _ := newFailingManager(t)
	fc.failSet = true
	_, err := manager.GenerateRefresh("user-1", time.Hour)
	assert.Error(t, err)
}

func TestRotateRefreshNilCacheRejected(t *testing.T) {
	manager := NewJWTManager("test-secret", nil)
	refresh, err := manager.GenerateRefresh("user-1", time.Hour)
	require.NoError(t, err)

	_, _, err = manager.RotateRefresh(context.Background(), refresh, time.Hour, time.Hour)
	assert.ErrorIs(t, err, pkgerrors.ErrUnauthorized)
}

func TestRotateRefreshGetDelError(t *testing.T) {
	manager, fc, _ := newFailingManager(t)
	refresh, err := manager.GenerateRefresh("user-1", time.Hour)
	require.NoError(t, err)

	fc.failGetDel = true
	_, _, err = manager.RotateRefresh(context.Background(), refresh, time.Hour, time.Hour)
	assert.Error(t, err)
}

func TestRotateRefreshMintPairRefreshWriteError(t *testing.T) {
	manager, fc, _ := newFailingManager(t)
	refresh, err := manager.GenerateRefresh("user-1", time.Hour)
	require.NoError(t, err)
	_, err = manager.ParseRefresh(refresh)
	require.NoError(t, err)

	fc.failSetPrefix = refreshPrefix
	_, _, err = manager.RotateRefresh(context.Background(), refresh, time.Hour, time.Hour)
	assert.Error(t, err)
}

func TestRevokeInitializesVersionKey(t *testing.T) {
	manager, _ := newManagerWithCache(t)
	require.NoError(t, manager.cache.Delete(context.Background(), versionPrefix+"user-1"))
	require.NoError(t, manager.Revoke(context.Background(), "user-1"))
	require.NoError(t, manager.Revoke(context.Background(), "user-1"))
}

func TestRevokeIncrError(t *testing.T) {
	manager, fc, _ := newFailingManager(t)
	_, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)
	require.NoError(t, manager.Revoke(context.Background(), "user-1"))

	fc.failIncr = true
	err = manager.Revoke(context.Background(), "user-1")
	assert.Error(t, err)
}

func TestRevokeInitializeSetError(t *testing.T) {
	manager, fc, _ := newFailingManager(t)
	require.NoError(t, manager.cache.Delete(context.Background(), versionPrefix+"user-1"))

	fc.failSet = true
	err := manager.Revoke(context.Background(), "user-1")
	assert.Error(t, err)
}

func TestRotateRefreshMintPairAccessWriteError(t *testing.T) {
	manager, fc, _ := newFailingManager(t)
	refresh, err := manager.GenerateRefresh("user-1", time.Hour)
	require.NoError(t, err)
	_, err = manager.ParseRefresh(refresh)
	require.NoError(t, err)

	fc.failSetPrefix = versionPrefix

	access, next, err := manager.RotateRefresh(context.Background(), refresh, time.Hour, time.Hour)
	require.NoError(t, err)
	require.NotEmpty(t, access)
	require.NotEmpty(t, next)

	subject, err := manager.Parse(access)
	require.NoError(t, err)
	assert.Equal(t, "user-1", subject)
}

func TestCurrentVersionGetErrorFallsBackToMem(t *testing.T) {
	manager, fc, _ := newFailingManager(t)
	first, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)

	fc.failGet = true
	second, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)
	require.NotEmpty(t, second)

	subj, err := manager.Parse(first)
	require.NoError(t, err)
	assert.Equal(t, "user-1", subj)
}

func TestVersionValidGetErrorWithoutMemRejects(t *testing.T) {
	manager, fc, _ := newFailingManager(t)
	fc.failGet = true

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		Ver:              1,
		RegisteredClaims: jwt.RegisteredClaims{Subject: "user-unknown", Issuer: "tandem", Audience: jwt.ClaimStrings{"tandem"}},
	}).SignedString(manager.secret)
	require.NoError(t, err)

	_, err = manager.Parse(signed)
	assert.Error(t, err)
}

func TestParseRejectsNonHMACSignature(t *testing.T) {
	manager, _ := newManagerWithCache(t)
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	signed, err := jwt.NewWithClaims(jwt.SigningMethodRS256, Claims{
		Ver:              1,
		RegisteredClaims: jwt.RegisteredClaims{Subject: "user-1", Issuer: "tandem", Audience: jwt.ClaimStrings{"tandem"}},
	}).SignedString(rsaKey)
	require.NoError(t, err)

	_, err = manager.Parse(signed)
	assert.Error(t, err)
}

func TestCurrentVersionSucceedsWhenSetFails(t *testing.T) {
	manager, fc, _ := newFailingManager(t)
	fc.failSet = true
	_, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)
}

func TestVersionValidRejectsGarbageValue(t *testing.T) {
	manager, mr := newManagerWithCache(t)
	token, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)
	require.NoError(t, mr.Set(versionPrefix+"user-1", "abc"))

	_, err = manager.Parse(token)
	assert.Error(t, err)
}

func TestParseRejectsHS384Signature(t *testing.T) {
	manager, _ := newManagerWithCache(t)
	bad, err := jwt.NewWithClaims(jwt.SigningMethodHS384, Claims{
		Ver:              1,
		RegisteredClaims: jwt.RegisteredClaims{Subject: "user-1", Issuer: "tandem", Audience: jwt.ClaimStrings{"tandem"}},
	}).SignedString(manager.secret)
	require.NoError(t, err)

	_, err = manager.Parse(bad)
	assert.Error(t, err)
}

func TestParseFailsOnWrongIssuer(t *testing.T) {
	manager, _ := newManagerWithCache(t)
	bad, err := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		Ver:              1,
		RegisteredClaims: jwt.RegisteredClaims{Subject: "user-1", Issuer: "other", Audience: jwt.ClaimStrings{"tandem"}},
	}).SignedString(manager.secret)
	require.NoError(t, err)

	_, err = manager.Parse(bad)
	assert.Error(t, err)
}

func TestParseFailsOnMissingSubject(t *testing.T) {
	manager, _ := newManagerWithCache(t)
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		Ver:              1,
		RegisteredClaims: jwt.RegisteredClaims{Issuer: "tandem", Audience: jwt.ClaimStrings{"tandem"}},
	}).SignedString(manager.secret)
	require.NoError(t, err)

	_, err = manager.Parse(signed)
	assert.Error(t, err)
}

func TestRandomJTI(t *testing.T) {
	a, err := randomJTI()
	require.NoError(t, err)
	b, err := randomJTI()
	require.NoError(t, err)
	require.Len(t, a, 32)
	require.NotEqual(t, a, b)
}

func TestManagerMemExpiry(t *testing.T) {
	manager := NewJWTManager("test-secret", nil)
	manager.memPut("u", 3)
	got, ok := manager.memGet("u")
	require.True(t, ok)
	assert.Equal(t, int64(3), got)

	manager.mem["u"] = memVersion{ver: 9, at: time.Now().Add(-11 * time.Minute)}
	_, ok = manager.memGet("u")
	assert.False(t, ok)

	manager.mem["u"] = memVersion{ver: 0, at: time.Now()}
	got, ok = manager.memGet("u")
	assert.True(t, ok)
	assert.Equal(t, int64(0), got)
}
