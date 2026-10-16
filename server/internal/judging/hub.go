package judging

import (
	"context"
	"sync"

	"github.com/google/uuid"
	judgev1 "github.com/jellyfish-p/ephyra/protocol/judge/v1"
)

const sendBuf = 32

// Conn is one node's live control stream. The Hub never closes Send;
// Unregister and replacement close Done instead so senders cannot race
// a close.
type Conn struct {
	NodeID   uuid.UUID
	Profiles []string
	MaxSlots int32

	send chan *judgev1.ServerMessage
	done chan struct{}
	once sync.Once
}

func NewConn(nodeID uuid.UUID, profiles []string, maxSlots int32) *Conn {
	return &Conn{
		NodeID:   nodeID,
		Profiles: profiles,
		MaxSlots: maxSlots,
		send:     make(chan *judgev1.ServerMessage, sendBuf),
		done:     make(chan struct{}),
	}
}

func (c *Conn) Messages() <-chan *judgev1.ServerMessage { return c.send }

func (c *Conn) Done() <-chan struct{} { return c.done }

func (c *Conn) close() {
	c.once.Do(func() { close(c.done) })
}

// TrySend enqueues without blocking. False means the connection is gone
// or the buffer is full; the caller must not assume the node received it.
func (c *Conn) TrySend(msg *judgev1.ServerMessage) bool {
	select {
	case <-c.done:
		return false
	default:
	}
	select {
	case c.send <- msg:
		return true
	case <-c.done:
		return false
	default:
		return false
	}
}

// Send waits until the message is buffered, the connection is retired,
// or ctx is done. Use this for heartbeat acks that must not be dropped
// just because the dispatch buffer is momentarily full.
func (c *Conn) Send(ctx context.Context, msg *judgev1.ServerMessage) bool {
	select {
	case c.send <- msg:
		return true
	case <-c.done:
		return false
	case <-ctx.Done():
		return false
	}
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
	if old, ok := h.nodes[c.NodeID]; ok && old != c {
		old.close()
	}
	h.nodes[c.NodeID] = c
}

func (h *Hub) Unregister(id uuid.UUID, c *Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if cur, ok := h.nodes[id]; ok && cur == c {
		delete(h.nodes, id)
		c.close()
	}
}

func (h *Hub) Dispatch(nodeID uuid.UUID, msg *judgev1.ServerMessage) bool {
	h.mu.Lock()
	c, ok := h.nodes[nodeID]
	h.mu.Unlock()
	if !ok {
		return false
	}
	return c.TrySend(msg)
}

func (h *Hub) Connected(nodeID uuid.UUID) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, ok := h.nodes[nodeID]
	return ok
}
