package hub

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tandem/tandem/internal/domain/ports/ws"
)

type fakeClient struct {
	id     string
	uid    string
	closed bool
}

func (f *fakeClient) ID() string { return f.id }

func (f *fakeClient) UserID() string { return f.uid }

func (f *fakeClient) Send(*ws.Message) {}

func (f *fakeClient) Close() { f.closed = true }

var _ ws.Client = (*fakeClient)(nil)

func TestHubLimitsConnectionsPerUser(t *testing.T) {
	h := New()
	defer h.Close()

	for i := 0; i < maxConnsPerUser; i++ {
		cl := &fakeClient{id: fmt.Sprintf("c%d", i), uid: "user-1"}
		h.Register(cl)
		assert.False(t, cl.closed, "connection %d must be accepted", i)
	}

	excess := &fakeClient{id: "excess", uid: "user-1"}
	h.Register(excess)
	assert.True(t, excess.closed, "excess connection must be rejected")

	other := &fakeClient{id: "other", uid: "user-2"}
	h.Register(other)
	assert.False(t, other.closed, "different user must be unaffected")
}

func TestHubCloseThenWaitReturns(t *testing.T) {
	h := New()
	done := make(chan struct{})
	go func() {
		h.Wait()
		close(done)
	}()
	h.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Wait did not return after Close")
	}
}

func TestHubRejectsRegisterAfterClose(t *testing.T) {
	h := New()
	h.Close()
	cl := &fakeClient{id: "late", uid: "user-1"}
	h.Register(cl)
	assert.True(t, cl.closed)
}

func TestMessageThrottleBlocksAtLimit(t *testing.T) {
	th := newMessageThrottle(2, time.Minute)
	now := time.Now()
	th.now = func() time.Time { return now }

	require.True(t, th.Allow())
	require.True(t, th.Allow())
	assert.False(t, th.Allow(), "third message in window must be blocked")

	now = now.Add(2 * time.Minute)
	assert.True(t, th.Allow(), "window must slide after expiry")
	assert.True(t, th.Allow())
	assert.False(t, th.Allow())
}
