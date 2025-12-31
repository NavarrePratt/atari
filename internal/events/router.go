// Package events provides the channel-based pub/sub event router.
package events

import (
	"log/slog"
	"sync"
)

// DefaultBufferSize is the default channel buffer size for subscribers.
const DefaultBufferSize = 100

// subscriberEntry holds a subscriber channel and its metadata.
type subscriberEntry struct {
	ch chan Event
}

// Router manages event flow from producers to consumers.
// It provides a channel-based pub/sub system where producers emit events
// and consumers subscribe to receive them.
type Router struct {
	subscribers []subscriberEntry
	bufferSize  int
	mu          sync.RWMutex
	closed      bool
}

// NewRouter creates a new event router with the specified default buffer size.
// If bufferSize is 0 or negative, DefaultBufferSize is used.
func NewRouter(bufferSize int) *Router {
	if bufferSize <= 0 {
		bufferSize = DefaultBufferSize
	}
	return &Router{
		bufferSize: bufferSize,
	}
}

// Emit publishes an event to all subscribers.
// Events are sent non-blocking: if a subscriber's channel is full, the event
// is dropped and a warning is logged.
// Emit is safe to call concurrently and after Close (becomes a no-op).
func (r *Router) Emit(event Event) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.closed {
		return
	}

	for _, sub := range r.subscribers {
		select {
		case sub.ch <- event:
			// Event delivered
		default:
			// Channel full, drop event
			slog.Warn("event dropped: subscriber channel full",
				"event_type", event.Type(),
				"source", event.Source(),
			)
		}
	}
}

// Subscribe returns a channel that receives all emitted events.
// The channel has the router's default buffer size.
// The returned channel is closed when the router is closed.
func (r *Router) Subscribe() <-chan Event {
	return r.SubscribeBuffered(r.bufferSize)
}

// SubscribeBuffered returns a channel with the specified buffer size.
// Use this for subscribers that need larger buffers to avoid dropped events.
// The returned channel is closed when the router is closed.
func (r *Router) SubscribeBuffered(size int) <-chan Event {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		// Return a closed channel if router is already closed
		ch := make(chan Event)
		close(ch)
		return ch
	}

	ch := make(chan Event, size)
	r.subscribers = append(r.subscribers, subscriberEntry{ch: ch})
	return ch
}

// Unsubscribe removes a subscription and closes its channel.
// It is safe to call with a channel that was never subscribed or already unsubscribed.
func (r *Router) Unsubscribe(ch <-chan Event) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, sub := range r.subscribers {
		if sub.ch == ch {
			// Remove from slice
			r.subscribers = append(r.subscribers[:i], r.subscribers[i+1:]...)
			close(sub.ch)
			return
		}
	}
}

// Close closes all subscriber channels and marks the router as closed.
// Subsequent calls to Emit become no-ops.
// Subsequent calls to Subscribe return closed channels.
// Close is safe to call multiple times.
func (r *Router) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return
	}

	r.closed = true
	for _, sub := range r.subscribers {
		close(sub.ch)
	}
	r.subscribers = nil
}
