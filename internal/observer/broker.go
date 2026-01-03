// Package observer provides the TUI observer mode for real-time Q&A.
package observer

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	// ErrTimeout is returned when lock acquisition times out.
	ErrTimeout = errors.New("session broker: acquisition timeout")
	// ErrNotHolder is returned when Release is called by a non-holder.
	ErrNotHolder = errors.New("session broker: caller is not the current holder")
)

// SessionBroker coordinates access to the Claude CLI process.
// Only one Claude process can run at a time - either a drain session or an observer query.
type SessionBroker struct {
	mu     sync.RWMutex
	holder string        // "drain", "observer", or "" if unlocked
	sem    chan struct{} // semaphore channel: token present = unlocked
}

// NewSessionBroker creates a new SessionBroker ready for use.
func NewSessionBroker() *SessionBroker {
	b := &SessionBroker{
		sem: make(chan struct{}, 1),
	}
	// Initialize as unlocked by adding a token
	b.sem <- struct{}{}
	return b
}

// Acquire attempts to acquire the session lock for the given holder.
// It blocks until the lock is acquired, the timeout expires, or the context is cancelled.
// The holder string identifies who holds the lock (e.g., "drain" or "observer").
func (b *SessionBroker) Acquire(ctx context.Context, holder string, timeout time.Duration) error {
	if holder == "" {
		return errors.New("session broker: holder cannot be empty")
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-b.sem:
		// Successfully acquired the semaphore
		b.mu.Lock()
		b.holder = holder
		b.mu.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return ErrTimeout
	}
}

// TryAcquire attempts to acquire the session lock without blocking.
// Returns true if the lock was acquired, false otherwise.
func (b *SessionBroker) TryAcquire(holder string) bool {
	if holder == "" {
		return false
	}

	select {
	case <-b.sem:
		b.mu.Lock()
		b.holder = holder
		b.mu.Unlock()
		return true
	default:
		return false
	}
}

// Release releases the session lock.
// It is safe to call Release multiple times, but only the first call has an effect.
func (b *SessionBroker) Release() {
	b.mu.Lock()
	if b.holder == "" {
		b.mu.Unlock()
		return // Already released
	}
	b.holder = ""
	b.mu.Unlock()

	// Return the token to the semaphore
	select {
	case b.sem <- struct{}{}:
		// Released successfully
	default:
		// Semaphore already has a token (shouldn't happen with correct usage)
	}
}

// Holder returns the current holder of the lock, or empty string if unlocked.
func (b *SessionBroker) Holder() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.holder
}

// IsHeld returns true if the lock is currently held by any holder.
func (b *SessionBroker) IsHeld() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.holder != ""
}
