package ws

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tandem/tandem/internal/infrastructure/token"
	wshub "github.com/tandem/tandem/internal/infrastructure/ws"
)

func newWSTestHarness(t *testing.T) (*WSHandler, *wshub.Hub, *httptest.Server, *token.JWTManager) {
	t.Helper()
	manager := token.NewJWTManager("test-secret", nil)
	hub := wshub.New()
	h := NewWSHandler(hub, manager, nil, nil)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/ws", h.Connect)
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
	_, _, srv, manager := newWSTestHarness(t)
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
	_, _, srv, manager := newWSTestHarness(t)
	valid, err := manager.Generate("user-1", time.Hour)
	require.NoError(t, err)

	_, status, err := wsDial(t, srv, nil, "?token="+valid)
	require.Error(t, err)
	assert.Equal(t, http.StatusUnauthorized, status)
}

func TestWSConnectInvalidToken(t *testing.T) {
	_, _, srv, _ := newWSTestHarness(t)

	_, status, err := wsDial(t, srv, []string{"tandem", "bogus-token"}, "")
	require.Error(t, err)
	assert.Equal(t, http.StatusUnauthorized, status)
}

func TestWSConnectMissingToken(t *testing.T) {
	_, _, srv, _ := newWSTestHarness(t)

	_, status, err := wsDial(t, srv, nil, "")
	require.Error(t, err)
	assert.Equal(t, http.StatusUnauthorized, status)
}

func TestWSConnectTandemWithoutToken(t *testing.T) {
	_, _, srv, _ := newWSTestHarness(t)

	_, status, err := wsDial(t, srv, []string{"tandem"}, "")
	require.Error(t, err)
	assert.Equal(t, http.StatusUnauthorized, status)
}
