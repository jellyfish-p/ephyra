package scoring

import (
	"sync"

	"github.com/google/uuid"
)

type Update struct {
	ContestID    uuid.UUID
	SubmissionID uuid.UUID
}

type Hub struct {
	mu   sync.Mutex
	next int
	subs map[int]chan Update
}

func NewHub() *Hub {
	return &Hub{subs: map[int]chan Update{}}
}

func (h *Hub) Subscribe() (<-chan Update, func()) {
	ch := make(chan Update, 16)
	h.mu.Lock()
	id := h.next
	h.next++
	h.subs[id] = ch
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		delete(h.subs, id)
		close(ch)
		h.mu.Unlock()
	}
}

func (h *Hub) Publish(u Update) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ch := range h.subs {
		select {
		case ch <- u:
		default:
		}
	}
}

func (m *Module) events() *Hub {
	m.hubOnce.Do(func() {
		if m.Hub == nil {
			m.Hub = NewHub()
		}
	})
	return m.Hub
}

func (m *Module) SubscribeUpdates() (<-chan Update, func()) {
	return m.events().Subscribe()
}

func (m *Module) NotifySubmission(contestID, submissionID uuid.UUID) {
	m.events().Publish(Update{ContestID: contestID, SubmissionID: submissionID})
}
