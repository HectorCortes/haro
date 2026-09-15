package broker

import "sync"

// Hub fans out persisted broker notifications to active JSON-RPC connections.
// Subscribers are removed when their writer reports a closed connection.
type Hub struct {
	mu          sync.RWMutex
	nextID      uint64
	subscribers map[uint64]func(string, any) error
}

func NewHub() *Hub {
	return &Hub{subscribers: make(map[uint64]func(string, any) error)}
}

func (h *Hub) Subscribe(send func(string, any) error) func() {
	if h == nil || send == nil {
		return func() {}
	}
	h.mu.Lock()
	h.nextID++
	id := h.nextID
	h.subscribers[id] = send
	h.mu.Unlock()
	return func() {
		h.mu.Lock()
		delete(h.subscribers, id)
		h.mu.Unlock()
	}
}

func (h *Hub) Broadcast(method string, params any) {
	if h == nil || method == "" {
		return
	}
	h.mu.RLock()
	snapshot := make(map[uint64]func(string, any) error, len(h.subscribers))
	for id, send := range h.subscribers {
		snapshot[id] = send
	}
	h.mu.RUnlock()
	for id, send := range snapshot {
		if err := send(method, params); err != nil {
			h.mu.Lock()
			delete(h.subscribers, id)
			h.mu.Unlock()
		}
	}
}
