package hub

import (
	"sync"

	"github.com/gorilla/websocket"
	"github.com/tandem/tandem/internal/interfaces/ws"
)

const sendQueueSize = 1024

type Hub struct {
	mu      sync.RWMutex
	clients map[ws.Client]struct{}
	rooms   map[string]map[ws.Client]struct{}
	closed  bool
}

func New() *Hub {
	return &Hub{
		clients: make(map[ws.Client]struct{}),
		rooms:   make(map[string]map[ws.Client]struct{}),
	}
}

func (h *Hub) Register(conn ws.Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		conn.Close()
		return
	}
	h.clients[conn] = struct{}{}
	if c, ok := conn.(*Client); ok {
		c.setHub(h)
		go c.writePump()
		go c.readPump()
	}
}

func (h *Hub) Unregister(conn ws.Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[conn]; !ok {
		return
	}
	delete(h.clients, conn)
	for room, members := range h.rooms {
		delete(members, conn)
		if len(members) == 0 {
			delete(h.rooms, room)
		}
	}
	conn.Close()
}

func (h *Hub) JoinRoom(room string, conn ws.Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	if h.rooms[room] == nil {
		h.rooms[room] = make(map[ws.Client]struct{})
	}
	h.rooms[room][conn] = struct{}{}
}

func (h *Hub) LeaveRoom(room string, conn ws.Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	members := h.rooms[room]
	if members == nil {
		return
	}
	delete(members, conn)
	if len(members) == 0 {
		delete(h.rooms, room)
	}
}

func (h *Hub) BroadcastToRoom(room string, msg *ws.Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	msg.Room = room
	for conn := range h.rooms[room] {
		conn.Send(msg)
	}
}

func (h *Hub) Broadcast(msg *ws.Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for conn := range h.clients {
		conn.Send(msg)
	}
}

func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for conn := range h.clients {
		conn.Close()
	}
	h.clients = make(map[ws.Client]struct{})
	h.rooms = make(map[string]map[ws.Client]struct{})
}

type Client struct {
	hub  *Hub
	conn *websocket.Conn
	id   string
	send chan *ws.Message

	once sync.Once

	OnMessage func(client *Client, data []byte)
}

func NewClient(conn *websocket.Conn, id string) *Client {
	return &Client{
		conn: conn,
		id:   id,
		send: make(chan *ws.Message, sendQueueSize),
	}
}

func (c *Client) setHub(h *Hub) { c.hub = h }

func (c *Client) ID() string { return c.id }

func (c *Client) Send(msg *ws.Message) {
	select {
	case c.send <- msg:
	default:
		c.Close()
	}
}

func (c *Client) Close() {
	c.once.Do(func() {
		close(c.send)
		_ = c.conn.Close()
	})
}

func (c *Client) writePump() {
	defer c.Close()
	for msg := range c.send {
		if err := c.conn.WriteJSON(msg); err != nil {
			return
		}
	}
}

func (c *Client) readPump() {
	defer c.unregisterAndClose()
	c.conn.SetReadLimit(8192)
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		if c.OnMessage != nil {
			c.OnMessage(c, data)
		}
	}
}

func (c *Client) unregisterAndClose() {
	if c.hub != nil {
		c.hub.Unregister(c)
	}
	c.Close()
}

var _ ws.Client = (*Client)(nil)
var _ ws.Hub = (*Hub)(nil)
