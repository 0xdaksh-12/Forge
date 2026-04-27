// Package stream provides an in-memory SSE broadcast hub.
// Each job has its own set of subscriber channels; log events are
// fanned-out to all active subscribers with a non-blocking send so a
// slow client never blocks the runner.
package stream

import "sync"

// LogEvent carries one line of output from a running job.
type LogEvent struct {
	JobID  uint
	Seq    int
	Stream string // "stdout" | "stderr"
	Text   string
}

// Hub routes log events to SSE subscribers keyed by job ID.
type Hub struct {
	mu          sync.RWMutex
	subscribers map[uint][]chan LogEvent
	broadcast   chan LogEvent
	quit        chan struct{}
}

// NewHub creates a ready-to-use Hub. Call Run in a goroutine.
func NewHub() *Hub {
	return &Hub{
		subscribers: make(map[uint][]chan LogEvent),
		broadcast:   make(chan LogEvent, 1024),
		quit:        make(chan struct{}),
	}
}

// Run dispatches incoming events to subscribers. Run this in a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case evt := <-h.broadcast:
			h.mu.RLock()
			for _, ch := range h.subscribers[evt.JobID] {
				select {
				case ch <- evt:
				default: // drop if the consumer is too slow
				}
			}
			h.mu.RUnlock()
		case <-h.quit:
			return
		}
	}
}

// Stop shuts down the hub's dispatch loop.
func (h *Hub) Stop() { close(h.quit) }

// Publish enqueues a log event for fan-out. Non-blocking.
func (h *Hub) Publish(evt LogEvent) {
	select {
	case h.broadcast <- evt:
	default:
	}
}

// Subscribe registers a new subscriber channel for the given job ID.
func (h *Hub) Subscribe(jobID uint) chan LogEvent {
	ch := make(chan LogEvent, 256)
	h.mu.Lock()
	h.subscribers[jobID] = append(h.subscribers[jobID], ch)
	h.mu.Unlock()
	return ch
}

// Unsubscribe removes the channel from the hub and closes it.
func (h *Hub) Unsubscribe(jobID uint, ch chan LogEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	subs := h.subscribers[jobID]
	for i, s := range subs {
		if s == ch {
			h.subscribers[jobID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	if len(h.subscribers[jobID]) == 0 {
		delete(h.subscribers, jobID)
	}
	close(ch)
}
