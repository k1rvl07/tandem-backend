package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	"github.com/tandem/tandem/internal/http/middleware/ws_identity"
	"github.com/tandem/tandem/internal/infrastructure/token"
	wshub "github.com/tandem/tandem/internal/infrastructure/ws"
	"github.com/tandem/tandem/internal/pkg/ctxkeys"
	"github.com/tandem/tandem/internal/usecase/testutil"
)

func newWSTestHarness(t *testing.T, workspaces repository.WorkspaceRepository) (*WSHandler, *wshub.Hub, *httptest.Server, *token.JWTManager) {
	t.Helper()
	manager := token.NewJWTManager("test-secret", nil)
	hub := wshub.New()
	h := NewWSHandler(hub, workspaces, nil)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ws", ws_identity.WSIdentity(manager), h.Connect)
	srv := httptest.NewServer(r)
	t.Cleanup(func() {
		srv.Close()
		hub.Close()
	})
	return h, hub, srv, manager
}

func wsDial(t *testing.T, srv *httptest.Server, subprotocols []string, query string) (*websocket.Conn, int, error) {
	t.Helper()
	u := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws" + query
	d := websocket.Dialer{Subprotocols: subprotocols, HandshakeTimeout: 2 * time.Second}
	conn, resp, err := d.Dial(u, nil)
	if resp == nil && err == nil {
		return nil, 0, err
	}
	if resp == nil {
		return nil, 0, err
	}
	return conn, resp.StatusCode, err
}

func TestWSConnectSubprotocol(t *testing.T) {
	_, _, srv, manager := newWSTestHarness(t, nil)
	valid, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)

	conn, status, err := wsDial(t, srv, []string{"tandem", valid}, "")
	require.NoError(t, err)
	require.NotNil(t, conn)
	defer conn.Close()
	assert.Equal(t, http.StatusSwitchingProtocols, status)
	assert.Equal(t, "tandem", conn.Subprotocol())
}

func TestWSConnectQueryTokenRejected(t *testing.T) {
	_, _, srv, manager := newWSTestHarness(t, nil)
	valid, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)

	_, status, err := wsDial(t, srv, nil, "?token="+valid)
	require.Error(t, err)
	assert.Equal(t, http.StatusUnauthorized, status)
}

func TestWSConnectInvalidToken(t *testing.T) {
	_, _, srv, _ := newWSTestHarness(t, nil)

	_, status, err := wsDial(t, srv, []string{"tandem", "bogus-token"}, "")
	require.Error(t, err)
	assert.Equal(t, http.StatusUnauthorized, status)
}

func TestWSConnectMissingToken(t *testing.T) {
	_, _, srv, _ := newWSTestHarness(t, nil)

	_, status, err := wsDial(t, srv, nil, "")
	require.Error(t, err)
	assert.Equal(t, http.StatusUnauthorized, status)
}

func TestWSConnectTandemWithoutToken(t *testing.T) {
	_, _, srv, _ := newWSTestHarness(t, nil)

	_, status, err := wsDial(t, srv, []string{"tandem"}, "")
	require.Error(t, err)
	assert.Equal(t, http.StatusUnauthorized, status)
}

func TestWSJoinDeniedForNonMember(t *testing.T) {
	repo := testutil.NewFakeWorkspaceRepo()
	workspaceID := uuid.NewString()
	repo.AddWorkspaceFixture(workspaceID, "Workspace")
	_, _, srv, manager := newWSTestHarness(t, repo)

	valid, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)
	conn, _, err := wsDial(t, srv, []string{"tandem", valid}, "")
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"join","room":"workspace:`+workspaceID+`"}`)))
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := conn.ReadMessage()
	require.NoError(t, err)
	var msg ws.Message
	require.NoError(t, json.Unmarshal(raw, &msg))
	assert.Equal(t, eventJoinDenied, msg.Type)
	assert.Equal(t, "workspace:"+workspaceID, msg.Room)
}

func TestWSJoinInvalidWorkspaceID(t *testing.T) {
	repo := testutil.NewFakeWorkspaceRepo()
	_, _, srv, manager := newWSTestHarness(t, repo)

	valid, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)
	conn, _, err := wsDial(t, srv, []string{"tandem", valid}, "")
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"join","room":"workspace:not-a-uuid"}`)))
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := conn.ReadMessage()
	require.NoError(t, err)
	var msg ws.Message
	require.NoError(t, json.Unmarshal(raw, &msg))
	assert.Equal(t, eventJoinDenied, msg.Type)
}

func TestWSJoinNonWorkspaceRoom(t *testing.T) {
	repo := testutil.NewFakeWorkspaceRepo()
	_, _, srv, manager := newWSTestHarness(t, repo)

	valid, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)
	conn, _, err := wsDial(t, srv, []string{"tandem", valid}, "")
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"join","room":"general"}`)))
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := conn.ReadMessage()
	require.NoError(t, err)
	var msg ws.Message
	require.NoError(t, json.Unmarshal(raw, &msg))
	assert.Equal(t, eventJoinDenied, msg.Type)
	assert.Equal(t, "general", msg.Room)
}

func TestWSJoinAllowsMemberAndBroadcastsPresence(t *testing.T) {
	repo := testutil.NewFakeWorkspaceRepo()
	workspaceID := uuid.NewString()
	repo.AddWorkspaceFixture(workspaceID, "Workspace")
	repo.AddMemberFixture(workspaceID, "user-1", "editor")
	_, _, srv, manager := newWSTestHarness(t, repo)

	valid, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)
	conn, _, err := wsDial(t, srv, []string{"tandem", valid}, "")
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"join","room":"workspace:`+workspaceID+`"}`)))
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := conn.ReadMessage()
	require.NoError(t, err)
	var msg ws.Message
	require.NoError(t, json.Unmarshal(raw, &msg))
	assert.Equal(t, "presence", msg.Type)
	assert.Equal(t, "workspace:"+workspaceID, msg.Room)
	rawData, err := json.Marshal(msg.Data)
	require.NoError(t, err)
	var presence presenceMessage
	require.NoError(t, json.Unmarshal(rawData, &presence))
	assert.Equal(t, "workspace:"+workspaceID, presence.Room)
	assert.ElementsMatch(t, []string{"user-1"}, presence.Members)
}

func newWSOriginHarness(t *testing.T, origins []string) (*httptest.Server, *token.JWTManager) {
	t.Helper()
	manager := token.NewJWTManager("test-secret", nil)
	hub := wshub.New()
	h := NewWSHandler(hub, nil, origins)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ws", ws_identity.WSIdentity(manager), h.Connect)
	srv := httptest.NewServer(r)
	t.Cleanup(func() {
		srv.Close()
		hub.Close()
	})
	return srv, manager
}

func wsDialOrigin(t *testing.T, srv *httptest.Server, origin, token string) (*websocket.Conn, error) {
	t.Helper()
	headers := make(http.Header)
	if origin != "" {
		headers.Set("Origin", origin)
	}
	u := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	d := websocket.Dialer{Subprotocols: []string{"tandem", token}, HandshakeTimeout: 2 * time.Second}
	conn, _, err := d.Dial(u, headers)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func TestWSConnectRejectsMissingOriginWithAllowlist(t *testing.T) {
	srv, manager := newWSOriginHarness(t, []string{"http://localhost:5173"})
	valid, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)

	conn, err := wsDialOrigin(t, srv, "", valid)
	require.Error(t, err)
	assert.Nil(t, conn)
}

func TestWSConnectRejectsForeignOrigin(t *testing.T) {
	srv, manager := newWSOriginHarness(t, []string{"http://localhost:5173"})
	valid, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)

	conn, err := wsDialOrigin(t, srv, "http://evil.example", valid)
	require.Error(t, err)
	assert.Nil(t, conn)
}

func TestWSConnectAcceptsMatchingOrigin(t *testing.T) {
	srv, manager := newWSOriginHarness(t, []string{"http://localhost:5173"})
	valid, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)

	conn, err := wsDialOrigin(t, srv, "http://localhost:5173", valid)
	require.NoError(t, err)
	assert.NotNil(t, conn)
	conn.Close()
}

func TestWSConnectAcceptsAnyOriginWithoutAllowlist(t *testing.T) {
	srv, manager := newWSOriginHarness(t, nil)
	valid, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)

	conn, err := wsDialOrigin(t, srv, "http://anything.example", valid)
	require.NoError(t, err)
	assert.NotNil(t, conn)
	conn.Close()
}

func TestOffersTandem(t *testing.T) {
	assert.True(t, offersTandem("tandem"))
	assert.True(t, offersTandem("tandem, <jwt>"))
	assert.True(t, offersTandem(" chat, tandem ,chat"))
	assert.False(t, offersTandem(""))
	assert.False(t, offersTandem("chat"))
	assert.False(t, offersTandem("chat, x"))
}

func TestWSConnectRejectsUpgradeWithoutHeaders(t *testing.T) {
	manager := token.NewJWTManager("test-secret", nil)
	hub := wshub.New()
	h := NewWSHandler(hub, nil, nil)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ws", ws_identity.WSIdentity(manager), h.Connect)
	srv := httptest.NewServer(r)
	defer srv.Close()
	defer hub.Close()

	valid, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/ws", nil)
	require.NoError(t, err)
	req.Header.Set("Sec-WebSocket-Protocol", "tandem, "+valid)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.NotEqual(t, http.StatusSwitchingProtocols, resp.StatusCode)
}

func TestWSJoinMalformedMessagesAreIgnored(t *testing.T) {
	repo := testutil.NewFakeWorkspaceRepo()
	workspaceID := uuid.NewString()
	repo.AddWorkspaceFixture(workspaceID, "Workspace")
	repo.AddMemberFixture(workspaceID, "user-1", "editor")
	_, _, srv, manager := newWSTestHarness(t, repo)

	valid, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)
	conn, _, err := wsDial(t, srv, []string{"tandem", valid}, "")
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte("not-json")))
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"join"}`)))
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"join","room":"workspace:`+workspaceID+`"}`)))
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := conn.ReadMessage()
	require.NoError(t, err)
	var msg ws.Message
	require.NoError(t, json.Unmarshal(raw, &msg))
	assert.Equal(t, "presence", msg.Type)
}

func TestWSLeaveAndInvalidMessagesDoNotBreakSession(t *testing.T) {
	repo := testutil.NewFakeWorkspaceRepo()
	workspaceID := uuid.NewString()
	repo.AddWorkspaceFixture(workspaceID, "Workspace")
	repo.AddMemberFixture(workspaceID, "user-1", "editor")
	_, _, srv, manager := newWSTestHarness(t, repo)

	valid, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)
	conn, _, err := wsDial(t, srv, []string{"tandem", valid}, "")
	require.NoError(t, err)
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"join","room":"workspace:`+workspaceID+`"}`)))
	_, raw, err := conn.ReadMessage()
	require.NoError(t, err)
	var msg ws.Message
	require.NoError(t, json.Unmarshal(raw, &msg))
	assert.Equal(t, "presence", msg.Type)

	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"leave","room":"workspace:`+workspaceID+`"}`)))
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"leave","room":"workspace:not-a-uuid"}`)))
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"leave","room":"banana"}`)))
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"join","room":"workspace:`+uuid.NewString()+`"}`)))
	_, raw, err = conn.ReadMessage()
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &msg))
	assert.Equal(t, eventJoinDenied, msg.Type)
}

func TestConnectWithoutUserContext(t *testing.T) {
	hub := wshub.New()
	h := NewWSHandler(hub, nil, nil)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ws", h.Connect)
	srv := httptest.NewServer(r)
	defer srv.Close()
	defer hub.Close()

	req := httptest.NewRequest(http.MethodGet, "http://example.com/ws", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestConnectWithNonStringUserContext(t *testing.T) {
	hub := wshub.New()
	h := NewWSHandler(hub, nil, nil)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ws", func(c *gin.Context) {
		c.Set(ctxkeys.CtxUserID, 42)
		h.Connect(c)
	})
	srv := httptest.NewServer(r)
	defer srv.Close()
	defer hub.Close()

	req := httptest.NewRequest(http.MethodGet, "http://example.com/ws", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
