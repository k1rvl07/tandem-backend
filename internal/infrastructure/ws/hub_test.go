package hub

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tandem/tandem/internal/domain/ports/ws"
)

type fakeClient struct {
	id     string
	uid    string
	closed bool
	sent   []*ws.Message
}

func (f *fakeClient) ID() string { return f.id }

func (f *fakeClient) UserID() string { return f.uid }

func (f *fakeClient) Send(msg *ws.Message) { f.sent = append(f.sent, msg) }

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

func TestHubJoinLeaveAndMembers(t *testing.T) {
	h := New()
	defer h.Close()

	c1 := &fakeClient{id: "c1", uid: "user-1"}
	c2 := &fakeClient{id: "c2", uid: "user-2"}
	h.Register(c1)
	h.Register(c2)

	h.JoinRoom("room-1", c1)
	h.JoinRoom("room-1", c2)
	assert.ElementsMatch(t, []string{"user-1", "user-2"}, h.RoomMembers("room-1"))
	require.Len(t, h.RoomMembers("missing"), 0)

	h.LeaveRoom("room-1", c1)
	assert.Equal(t, []string{"user-2"}, h.RoomMembers("room-1"))

	h.LeaveRoom("room-1", c2)
	assert.Empty(t, h.RoomMembers("room-1"))

	h.LeaveRoom("never-existed", c1)
}

func TestHubJoinClosedIsNoop(t *testing.T) {
	h := New()
	h.Close()
	c1 := &fakeClient{id: "c1", uid: "user-1"}
	h.JoinRoom("room-1", c1)
	assert.Empty(t, h.RoomMembers("room-1"))
}

func TestHubUnregister(t *testing.T) {
	h := New()
	defer h.Close()

	c1 := &fakeClient{id: "c1", uid: "user-1"}
	h.Register(c1)
	h.JoinRoom("room-1", c1)

	h.Unregister(c1)
	assert.True(t, c1.closed)
	assert.Empty(t, h.RoomMembers("room-1"))

	h.Unregister(c1)
	assert.True(t, c1.closed)
}

func TestHubBroadcastToRoomSetsRoomAndFilters(t *testing.T) {
	h := New()
	defer h.Close()

	inRoom := &fakeClient{id: "in", uid: "user-1"}
	outRoom := &fakeClient{id: "out", uid: "user-2"}
	h.Register(inRoom)
	h.Register(outRoom)
	h.JoinRoom("room-1", inRoom)

	h.BroadcastToRoom("room-1", &ws.Message{Type: "event", Data: map[string]string{"k": "v"}})
	require.Len(t, inRoom.sent, 1)
	assert.Equal(t, "event", inRoom.sent[0].Type)
	assert.Equal(t, "room-1", inRoom.sent[0].Room)
	assert.Empty(t, outRoom.sent)
}

func TestHubBroadcastAll(t *testing.T) {
	h := New()
	defer h.Close()

	c1 := &fakeClient{id: "c1", uid: "user-1"}
	c2 := &fakeClient{id: "c2", uid: "user-2"}
	h.Register(c1)
	h.Register(c2)

	h.Broadcast(&ws.Message{Type: "all"})
	require.Len(t, c1.sent, 1)
	require.Len(t, c2.sent, 1)
	assert.Equal(t, "all", c1.sent[0].Type)
}

func TestHubSendToUser(t *testing.T) {
	h := New()
	defer h.Close()

	c1 := &fakeClient{id: "c1", uid: "user-1"}
	h.Register(c1)
	h.Register(&fakeClient{id: "c2", uid: "user-2"})

	h.SendToUser("user-1", &ws.Message{Type: "targeted"})
	require.Len(t, c1.sent, 1)
	assert.Equal(t, "targeted", c1.sent[0].Type)
}

func TestHubCloseIsIdempotentAndSnapshotsNil(t *testing.T) {
	h := New()
	c1 := &fakeClient{id: "c1", uid: "user-1"}
	h.Register(c1)
	h.JoinRoom("room-1", c1)

	h.Close()
	assert.True(t, c1.closed)
	h.Close()

	c2 := &fakeClient{id: "c2", uid: "user-2"}
	h.Register(c2)
	assert.True(t, c2.closed)

	h.BroadcastToRoom("room-1", &ws.Message{Type: "x"})
	h.Broadcast(&ws.Message{Type: "x"})
}

func TestClientAddRoomInitializesRooms(t *testing.T) {
	cl := &Client{}
	cl.AddRoom("x")
	assert.ElementsMatch(t, []string{"x"}, cl.Rooms())
}

func TestHubPumpsViaRealConn(t *testing.T) {
	serverConnCh := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		up := websocket.Upgrader{}
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		serverConnCh <- conn
	}))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	d := websocket.Dialer{}
	client, _, err := d.Dial(url, nil)
	require.NoError(t, err)
	defer client.Close()

	serverConn := <-serverConnCh
	h := New()
	defer h.Wait()
	defer h.Close()
	cl := NewClient(serverConn, "id1", "user-1")
	assert.Equal(t, "id1", cl.ID())

	disc := make(chan struct{}, 1)
	cl.OnDisconnect = func(_ *Client) { disc <- struct{}{} }
	msgCh := make(chan []byte, 1)
	cl.OnMessage = func(_ *Client, data []byte) { msgCh <- data }
	h.Register(cl)

	cl.AddRoom("room-extra")
	cl.AddRoom("other")
	cl.RemoveRoom("other")
	assert.ElementsMatch(t, []string{"room-extra"}, cl.Rooms())

	require.NoError(t, client.WriteMessage(websocket.PongMessage, nil))
	require.NoError(t, client.WriteMessage(websocket.TextMessage, []byte("hello")))
	select {
	case got := <-msgCh:
		assert.Equal(t, "hello", string(got))
	case <-time.After(2 * time.Second):
		t.Fatal("OnMessage not fired")
	}

	h.JoinRoom("room-1", cl)
	h.BroadcastToRoom("room-1", &ws.Message{Type: "presence"})
	client.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := client.ReadMessage()
	require.NoError(t, err)
	var msg ws.Message
	require.NoError(t, json.Unmarshal(raw, &msg))
	assert.Equal(t, "presence", msg.Type)
	assert.Equal(t, "room-1", msg.Room)

	h.BroadcastToRoom("room-1", &ws.Message{Type: "bad", Data: make(chan int)})
	select {
	case <-disc:
	case <-time.After(2 * time.Second):
		t.Fatal("OnDisconnect not notified for unreadable payload")
	}
}

func TestHubReadPumpThrottleClosesConn(t *testing.T) {
	serverConnCh := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		up := websocket.Upgrader{}
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		serverConnCh <- conn
	}))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	d := websocket.Dialer{}
	client, _, err := d.Dial(url, nil)
	require.NoError(t, err)
	defer client.Close()

	serverConn := <-serverConnCh
	h := New()
	defer h.Wait()
	defer h.Close()
	cl := NewClient(serverConn, "id2", "user-2")
	disc := make(chan struct{}, 1)
	cl.OnDisconnect = func(_ *Client) { disc <- struct{}{} }
	cl.OnMessage = func(_ *Client, _ []byte) {}
	h.Register(cl)

	writes := 0
	for i := 0; i < maxClientMessages+5; i++ {
		if err := client.WriteMessage(websocket.TextMessage, []byte("x")); err != nil {
			if writes < maxClientMessages {
				t.Fatalf("write failed before the throttle limit was reached: %v", err)
			}
			break
		}
		writes++
	}
	select {
	case <-disc:
	case <-time.After(2 * time.Second):
		t.Fatal("client not dropped after message burst")
	}
}

func TestClientSendDoesNotBlockWhenBufferFull(t *testing.T) {
	serverConnCh := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		up := websocket.Upgrader{}
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		serverConnCh <- conn
	}))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	d := websocket.Dialer{}
	client, _, err := d.Dial(url, nil)
	require.NoError(t, err)
	defer client.Close()

	serverConn := <-serverConnCh
	h := New()
	defer h.Wait()
	defer h.Close()
	cl := NewClient(serverConn, "id-slow", "user-slow")
	disc := make(chan struct{}, 1)
	cl.OnDisconnect = func(_ *Client) { disc <- struct{}{} }
	cl.OnMessage = func(_ *Client, _ []byte) {}
	h.Register(cl)

	h.JoinRoom("room-slow", cl)

	flood := &ws.Message{Type: "flood", Data: strings.Repeat("x", 256*1024)}
	start := time.Now()
	for i := 0; i < sendQueueSize+10; i++ {
		h.BroadcastToRoom("room-slow", flood)
	}
	assert.Less(t, time.Since(start), 2*time.Second, "broadcast must not block on a slow client")

	select {
	case <-disc:
	case <-time.After(2 * time.Second):
		t.Fatal("slow client not dropped after send buffer overflow")
	}
}
