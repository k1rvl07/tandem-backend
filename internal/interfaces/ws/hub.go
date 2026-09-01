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
	BroadcastToRoom(room string, msg *Message)
	Broadcast(msg *Message)
	Close()
}

type Client interface {
	ID() string
	Send(msg *Message)
	Close()
}
