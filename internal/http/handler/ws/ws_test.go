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
