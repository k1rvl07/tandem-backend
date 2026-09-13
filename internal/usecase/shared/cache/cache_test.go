package cache_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	cacheutil "github.com/tandem/tandem/internal/usecase/shared/cache"
	"github.com/tandem/tandem/internal/usecase/testutil"
)

func TestVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("nil cache returns zero", func(t *testing.T) {
		t.Parallel()
		if got := cacheutil.Version(ctx, nil, cacheutil.WSVerKey+"ws"); got != "0" {
			t.Fatalf("Version(nil) = %q, want %q", got, "0")
		}
	})

	t.Run("miss returns zero", func(t *testing.T) {
		t.Parallel()
		c := testutil.NewFakeCache()
		if got := cacheutil.Version(ctx, c, cacheutil.WSVerKey+"ws"); got != "0" {
			t.Fatalf("Version(miss) = %q, want %q", got, "0")
		}
	})

	t.Run("hit returns stored value", func(t *testing.T) {
		t.Parallel()
		c := testutil.NewFakeCache()
		key := cacheutil.WSVerKey + "ws"
		if err := c.Set(ctx, key, "7", 0); err != nil {
			t.Fatal(err)
		}
		if got := cacheutil.Version(ctx, c, key); got != "7" {
			t.Fatalf("Version(hit) = %q, want %q", got, "7")
		}
	})
}

func TestBump(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("nil cache is a no-op", func(t *testing.T) {
		t.Parallel()
		cacheutil.Bump(ctx, nil, cacheutil.UVerKey+"u")
	})

	t.Run("increments and extends ttl", func(t *testing.T) {
		t.Parallel()
		c := testutil.NewFakeCache()
		key := cacheutil.WSVerKey + "ws"
		cacheutil.Bump(ctx, c, key)
		cacheutil.Bump(ctx, c, key)
		if got := cacheutil.Version(ctx, c, key); got != "2" {
			t.Fatalf("Version after 2 bumps = %q, want %q", got, "2")
		}
	})
}

func TestBumpWorkspace(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("bumps workspace version key", func(t *testing.T) {
		t.Parallel()
		c := testutil.NewFakeCache()
		cacheutil.BumpWorkspace(ctx, c, "ws1")
		if got := cacheutil.Version(ctx, c, cacheutil.WSVerKey+"ws1"); got != "1" {
			t.Fatalf("Version after BumpWorkspace = %q, want %q", got, "1")
		}
	})
}

func TestLoad(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("nil cache returns false", func(t *testing.T) {
		t.Parallel()
		var dst map[string]string
		if cacheutil.Load(ctx, nil, "k", &dst) {
			t.Fatal("Load(nil) = true, want false")
		}
	})

	t.Run("miss returns false", func(t *testing.T) {
		t.Parallel()
		c := testutil.NewFakeCache()
		var dst map[string]string
		if cacheutil.Load(ctx, c, "k", &dst) {
			t.Fatal("Load(miss) = true, want false")
		}
	})

	t.Run("invalid json returns false", func(t *testing.T) {
		t.Parallel()
		c := testutil.NewFakeCache()
		if err := c.Set(ctx, "k", "{broken", 0); err != nil {
			t.Fatal(err)
		}
		var dst map[string]string
		if cacheutil.Load(ctx, c, "k", &dst) {
			t.Fatal("Load(bad json) = true, want false")
		}
	})

	t.Run("valid json fills dst", func(t *testing.T) {
		t.Parallel()
		c := testutil.NewFakeCache()
		if err := c.Set(ctx, "k", `{"a":"b"}`, 0); err != nil {
			t.Fatal(err)
		}
		var dst map[string]string
		if !cacheutil.Load(ctx, c, "k", &dst) {
			t.Fatal("Load(ok) = false, want true")
		}
		if dst["a"] != "b" {
			t.Fatalf("Load dst = %#v, want a=b", dst)
		}
	})
}

func TestUnmarshal(t *testing.T) {
	t.Parallel()

	t.Run("empty input returns false", func(t *testing.T) {
		t.Parallel()
		if cacheutil.Unmarshal("", &map[string]string{}) {
			t.Fatal("Unmarshal(empty) = true, want false")
		}
	})

	t.Run("invalid json returns false", func(t *testing.T) {
		t.Parallel()
		var dst map[string]string
		if cacheutil.Unmarshal("{", &dst) {
			t.Fatal("Unmarshal(bad) = true, want false")
		}
	})

	t.Run("valid json fills dst", func(t *testing.T) {
		t.Parallel()
		var dst map[string]string
		if !cacheutil.Unmarshal(`{"x":"y"}`, &dst) {
			t.Fatal("Unmarshal(ok) = false, want true")
		}
		if dst["x"] != "y" {
			t.Fatalf("Unmarshal dst = %#v, want x=y", dst)
		}
	})
}

func TestStore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("nil cache is a no-op", func(t *testing.T) {
		t.Parallel()
		cacheutil.Store(ctx, nil, "k", map[string]string{"a": "b"}, time.Minute)
	})

	t.Run("round trip through load", func(t *testing.T) {
		t.Parallel()
		c := testutil.NewFakeCache()
		src := map[string]string{"a": "b", "c": "d"}
		cacheutil.Store(ctx, c, "k", src, time.Minute)
		var dst map[string]string
		if !cacheutil.Load(ctx, c, "k", &dst) {
			t.Fatal("round trip Load = false, want true")
		}
		if len(dst) != len(src) || dst["a"] != "b" || dst["c"] != "d" {
			t.Fatalf("round trip dst = %#v, want %#v", dst, src)
		}
	})

	t.Run("stores raw json", func(t *testing.T) {
		t.Parallel()
		c := testutil.NewFakeCache()
		cacheutil.Store(ctx, c, "k", map[string]int{"n": 42}, 0)
		raw, err := c.Get(ctx, "k")
		if err != nil {
			t.Fatal(err)
		}
		var parsed map[string]int
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			t.Fatalf("stored value is not valid json: %v", err)
		}
		if parsed["n"] != 42 {
			t.Fatalf("stored json = %#v, want n=42", parsed)
		}
	})
}

func TestQueryHash(t *testing.T) {
	t.Parallel()

	t.Run("stable for equal inputs", func(t *testing.T) {
		t.Parallel()
		if cacheutil.QueryHash("ws", "filter") != cacheutil.QueryHash("ws", "filter") {
			t.Fatal("QueryHash not stable for equal inputs")
		}
	})

	t.Run("differs for different inputs", func(t *testing.T) {
		t.Parallel()
		if cacheutil.QueryHash("ws", "filter") == cacheutil.QueryHash("ws", "other") {
			t.Fatal("QueryHash equal for different inputs")
		}
	})

	t.Run("respects part boundaries", func(t *testing.T) {
		t.Parallel()
		if cacheutil.QueryHash("ab", "c") == cacheutil.QueryHash("a", "bc") {
			t.Fatal("QueryHash ignored part boundary")
		}
	})
}
