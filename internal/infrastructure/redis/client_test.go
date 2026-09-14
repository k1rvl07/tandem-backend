package rediscache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
	"github.com/tandem/tandem/internal/pkg/config"
)

func newTestRedis(t *testing.T) (*Redis, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	r, err := New(config.RedisConfig{Addr: mr.Addr()})
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Close() })
	return r, mr
}

func TestNewPingFailure(t *testing.T) {
	_, err := New(config.RedisConfig{Addr: "127.0.0.1:1"})
	require.Error(t, err)
}

func TestSetGet(t *testing.T) {
	r, mr := newTestRedis(t)
	ctx := context.Background()

	require.NoError(t, r.Set(ctx, "k1", "v1", time.Minute))
	val, err := r.Get(ctx, "k1")
	require.NoError(t, err)
	require.Equal(t, "v1", val)

	ttl := mr.TTL("k1")
	require.Equal(t, time.Minute, ttl)
}

func TestGetMissing(t *testing.T) {
	r, _ := newTestRedis(t)
	val, err := r.Get(context.Background(), "missing")
	require.NoError(t, err)
	require.Empty(t, val)
}

func TestMGet(t *testing.T) {
	r, _ := newTestRedis(t)
	ctx := context.Background()

	require.NoError(t, r.Set(ctx, "a", "1", 0))
	require.NoError(t, r.Set(ctx, "b", "2", 0))

	vals, err := r.MGet(ctx, "a", "b", "missing")
	require.NoError(t, err)
	require.Equal(t, []string{"1", "2", ""}, vals)
}

func TestMGetError(t *testing.T) {
	r, _ := newTestRedis(t)
	require.NoError(t, r.Close())
	_, err := r.MGet(context.Background(), "a", "b")
	require.Error(t, err)
}

func TestGetDel(t *testing.T) {
	r, mr := newTestRedis(t)
	ctx := context.Background()

	require.NoError(t, r.Set(ctx, "token", "secret", time.Hour))
	val, err := r.GetDel(ctx, "token")
	require.NoError(t, err)
	require.Equal(t, "secret", val)

	val, err = r.GetDel(ctx, "token")
	require.NoError(t, err)
	require.Empty(t, val)

	val, err = r.GetDel(ctx, "never-set")
	require.NoError(t, err)
	require.Empty(t, val)

	require.False(t, mr.Exists("token"))
}

func TestDelete(t *testing.T) {
	r, _ := newTestRedis(t)
	ctx := context.Background()

	require.NoError(t, r.Set(ctx, "k", "v", 0))
	require.NoError(t, r.Delete(ctx, "k"))
	val, err := r.Get(ctx, "k")
	require.NoError(t, err)
	require.Empty(t, val)

	require.NoError(t, r.Delete(ctx, "k"))
}

func TestExpire(t *testing.T) {
	r, mr := newTestRedis(t)
	ctx := context.Background()

	require.NoError(t, r.Set(ctx, "k", "v", 0))
	ttl := mr.TTL("k")
	require.Zero(t, ttl)

	require.NoError(t, r.Expire(ctx, "k", 5*time.Second))
	ttl = mr.TTL("k")
	require.Equal(t, 5*time.Second, ttl)
}

func TestIncr(t *testing.T) {
	r, _ := newTestRedis(t)
	ctx := context.Background()

	n, err := r.Incr(ctx, "counter")
	require.NoError(t, err)
	require.Equal(t, int64(1), n)

	n, err = r.Incr(ctx, "counter")
	require.NoError(t, err)
	require.Equal(t, int64(2), n)
}

func TestPingAndRaw(t *testing.T) {
	r, _ := newTestRedis(t)
	require.NoError(t, r.Ping(context.Background()))
	require.NotNil(t, r.Raw())
	require.NoError(t, r.Close())
}

func TestStringKeysStoredAsStrings(t *testing.T) {
	r, mr := newTestRedis(t)
	ctx := context.Background()

	_, err := r.Incr(ctx, "num")
	require.NoError(t, err)
	got, err := mr.Get("num")
	require.NoError(t, err)
	require.Equal(t, "1", got)

	require.NoError(t, r.Set(ctx, "str", "value", 0))
	got, err = mr.Get("str")
	require.NoError(t, err)
	require.Equal(t, "value", got)
}
