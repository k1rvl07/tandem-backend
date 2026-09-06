package ws

type Message struct {
	Type string      `json:"type"`
	Data interface{} `json:"data,omitempty"`
	Room string      `json:"room,omitempty"`
}

type Hub interface {
	Register(conn Client)
	Unregister(conn Client)
	JoinRoom(room string, conn Client)
	LeaveRoom(room string, conn Client)
	RoomMembers(room string) []string
	BroadcastToRoom(room string, msg *Message)
	Broadcast(msg *Message)
	SendToUser(userID string, msg *Message)
	Close()
}

type Client interface {
	ID() string
	UserID() string
	Send(msg *Message)
	Close()
}
