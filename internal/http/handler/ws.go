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
	hub        ws.Hub
	tokens     service.TokenService
	workspaces repository.WorkspaceRepository
}

func NewWSHandler(hub ws.Hub, tokens service.TokenService, workspaces repository.WorkspaceRepository) *WSHandler {
	return &WSHandler{hub: hub, tokens: tokens, workspaces: workspaces}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
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
// @Param token query string true "JWT token"
// @Success 101
// @Failure 401 {object} map[string]string
// @Router /ws [get]
func (h *WSHandler) Connect(c *gin.Context) {
	tokenString := c.Query("token")
	if tokenString == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	userID, err := h.tokens.Parse(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	client := wsclient.NewClient(conn, uuid.NewString(), userID)
	client.OnMessage = h.handleMessage
	client.OnDisconnect = func(cl *wsclient.Client) {
		if room := cl.Room(); room != "" {
			h.broadcastPresence(room)
		}
	}
	h.hub.Register(client)
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
		cl.SetRoom(msg.Room)
		h.broadcastPresence(msg.Room)
	case "leave":
		h.hub.LeaveRoom(msg.Room, cl)
		if cl.Room() == msg.Room {
			cl.SetRoom("")
		}
		h.broadcastPresence(msg.Room)
	}
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
