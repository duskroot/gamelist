package realtime

import (
	"log"
	"sync"

	"den/internal/envelope"

	"github.com/google/uuid"
	"github.com/hertz-contrib/websocket"
)

type Hub struct {
	mu    sync.Mutex
	rooms map[string]map[string][]*websocket.Conn
}

func NewHub() *Hub {
	return &Hub{rooms: map[string]map[string][]*websocket.Conn{}}
}

func (h *Hub) EmitRoom(roomID string, ev Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.write(roomID, "", ev, false)
}

func (h *Hub) EmitUser(roomID, userID string, ev Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.write(roomID, userID, ev, true)
}

func (h *Hub) Connected(roomID string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.rooms[roomID]) > 0
}

func (h *Hub) add(roomID, userID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms == nil {
		h.rooms = map[string]map[string][]*websocket.Conn{}
	}
	users := h.rooms[roomID]
	if users == nil {
		users = map[string][]*websocket.Conn{}
		h.rooms[roomID] = users
	}
	users[userID] = append(users[userID], conn)
}

func (h *Hub) remove(roomID, userID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.drop(roomID, userID, conn)
	_ = conn.Close()
}

func (h *Hub) write(roomID, onlyUser string, ev Event, one bool) {
	users := h.rooms[roomID]
	for userID, conns := range users {
		if one && userID != onlyUser {
			continue
		}
		kept := make([]*websocket.Conn, 0, len(conns))
		for _, conn := range conns {
			if err := writeEvent(conn, ev); err != nil {
				_ = conn.Close()
				continue
			}
			kept = append(kept, conn)
		}
		if len(kept) == 0 {
			delete(users, userID)
		} else {
			users[userID] = kept
		}
	}
	if len(users) == 0 {
		delete(h.rooms, roomID)
	}
}

func (h *Hub) drop(roomID, userID string, conn *websocket.Conn) {
	users := h.rooms[roomID]
	if users == nil {
		return
	}
	kept := make([]*websocket.Conn, 0, len(users[userID]))
	for _, existing := range users[userID] {
		if existing != conn {
			kept = append(kept, existing)
		}
	}
	if len(kept) == 0 {
		delete(users, userID)
	} else {
		users[userID] = kept
	}
	if len(users) == 0 {
		delete(h.rooms, roomID)
	}
}

type success struct {
	ResponseMetadata envelope.Metadata `json:"ResponseMetadata"`
	Result           any               `json:"Result"`
}

func writeEvent(conn *websocket.Conn, ev Event) error {
	meta := envelope.Metadata{
		RequestId: uuid.NewString(),
		Action:    ev.Action,
		Version:   "v1",
		Service:   ev.Service,
	}
	log.Printf("RequestId=%s Action=%s Version=%s Service=%s", meta.RequestId, meta.Action, meta.Version, meta.Service)
	return conn.WriteJSON(success{ResponseMetadata: meta, Result: ev.Result})
}
