package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/tandem/tandem/internal/domain/ports/repository"
	"github.com/tandem/tandem/internal/domain/ports/service"
	"github.com/tandem/tandem/internal/domain/ports/ws"
	wsclient "github.com/tandem/tandem/internal/infrastructure/ws"
	"github.com/tandem/tandem/internal/pkg/validate"
)

type WSHandler struct {
	hub         ws.Hub
	tokens      service.TokenService
	workspaces  repository.WorkspaceRepository
	checkOrigin func(*http.Request) bool
}

func NewWSHandler(hub ws.Hub, tokens service.TokenService, workspaces repository.WorkspaceRepository, allowedOrigins []string) *WSHandler {
	allowed := make(map[string]bool, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[origin] = true
	}
	checkOrigin := func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" || len(allowed) == 0 {
			return true
		}
		return allowed[origin]
	}
	return &WSHandler{hub: hub, tokens: tokens, workspaces: workspaces, checkOrigin: checkOrigin}
}

type clientMessage struct {
	Type string          `json:"type"`
	Room string          `json:"room,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
}

type presenceMessage struct {
	Room    string   `json:"room"`
	Members []string `json:"members"`
}

// @Summary WebSocket endpoint
// @Tags ws
// @Param Sec-WebSocket-Protocol header string false "Subprotocol: 'tandem, <JWT>'"
// @Success 101
// @Failure 401 {object} map[string]string
// @Router /ws [get]
func (h *WSHandler) Connect(c *gin.Context) {
	tokenString := tokenFromSubprotocol(c.GetHeader("Sec-WebSocket-Protocol"))
	if tokenString == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	userID, err := h.tokens.Parse(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     h.checkOrigin,
	}
	responseHeader := http.Header{}
	if offersTandem(c.GetHeader("Sec-WebSocket-Protocol")) {
		responseHeader.Set("Sec-WebSocket-Protocol", "tandem")
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, responseHeader)
	if err != nil {
		return
	}

	client := wsclient.NewClient(conn, uuid.NewString(), userID)
	client.OnMessage = h.handleMessage
	client.OnDisconnect = func(cl *wsclient.Client) {
		for _, room := range cl.Rooms() {
			h.broadcastPresence(room)
		}
	}
	h.hub.Register(client)
}

func tokenFromSubprotocol(header string) string {
	if !offersTandem(header) {
		return ""
	}
	entries := strings.Split(header, ",")
	for _, entry := range entries {
		if token := strings.TrimSpace(entry); token != "" && token != "tandem" {
			return token
		}
	}
	return ""
}

func offersTandem(header string) bool {
	entries := strings.Split(header, ",")
	for _, entry := range entries {
		if strings.TrimSpace(entry) == "tandem" {
			return true
		}
	}
	return false
}

func (h *WSHandler) handleMessage(cl *wsclient.Client, data []byte) {
	var msg clientMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}
	if msg.Room == "" {
		return
	}
	switch msg.Type {
	case "join":
		if !h.canJoin(cl.UserID(), msg.Room) {
			return
		}
		h.hub.JoinRoom(msg.Room, cl)
		cl.AddRoom(msg.Room)
		h.broadcastPresence(msg.Room)
	case "leave":
		if !validWorkspaceRoom(msg.Room) {
			return
		}
		h.hub.LeaveRoom(msg.Room, cl)
		cl.RemoveRoom(msg.Room)
		h.broadcastPresence(msg.Room)
	}
}

func validWorkspaceRoom(room string) bool {
	if !strings.HasPrefix(room, "workspace:") {
		return false
	}
	workspaceID := strings.TrimPrefix(room, "workspace:")
	return validate.UUID(workspaceID) == nil
}

func (h *WSHandler) canJoin(userID, room string) bool {
	if !strings.HasPrefix(room, "workspace:") {
		return true
	}
	workspaceID := strings.TrimPrefix(room, "workspace:")
	if err := validate.UUID(workspaceID); err != nil {
		return false
	}
	_, err := h.workspaces.FindMember(context.Background(), workspaceID, userID)
	return err == nil
}

func (h *WSHandler) broadcastPresence(room string) {
	members := h.hub.RoomMembers(room)
	h.hub.BroadcastToRoom(room, &ws.Message{
		Type: "presence",
		Room: room,
		Data: presenceMessage{Room: room, Members: members},
	})
}
