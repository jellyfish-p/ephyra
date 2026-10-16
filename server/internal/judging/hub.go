package judging

import (
	"sync"

	"github.com/google/uuid"
	judgev1 "github.com/jellyfish-p/ephyra/protocol/judge/v1"
)

type Conn struct {
	NodeID   uuid.UUID
	Send     chan *judgev1.ServerMessage
	Profiles []string
	MaxSlots int32
}

type Hub struct {
	mu    sync.Mutex
	nodes map[uuid.UUID]*Conn
}

func NewHub() *Hub {
	return &Hub{nodes: map[uuid.UUID]*Conn{}}
}

func (h *Hub) Register(c *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if old, ok := h.nodes[c.NodeID]; ok {
		close(old.Send)
	}
	h.nodes[c.NodeID] = c
}

func (h *Hub) Unregister(id uuid.UUID, c *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if cur, ok := h.nodes[id]; ok && cur == c {
		delete(h.nodes, id)
		close(c.Send)
	}
}

func (h *Hub) Dispatch(nodeID uuid.UUID, msg *judgev1.ServerMessage) bool {
	h.mu.Lock()
	c, ok := h.nodes[nodeID]
	h.mu.Unlock()
	if !ok {
		return false
	}
	select {
	case c.Send <- msg:
		return true
	default:
		return false
	}
}

func (h *Hub) Connected(nodeID uuid.UUID) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, ok := h.nodes[nodeID]
	return ok
}
