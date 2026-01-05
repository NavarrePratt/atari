# Observer Package

Provides the TUI observer mode for real-time Q&A about drain activity.

## Components

### SessionBroker (broker.go)

Provides coordination primitives for Claude CLI processes. Currently used by the drain controller. Observer runs independently of drain sessions since they use different models and are separate processes.

**Usage (by drain controller):**
```go
broker := NewSessionBroker()

// Acquire with timeout
err := broker.Acquire(ctx, "drain", 5*time.Second)
if err != nil {
    // Handle timeout or context cancellation
}
defer broker.Release()

// Check current state
holder := broker.Holder() // "drain" or ""
isHeld := broker.IsHeld()
```

**Key behaviors:**
- Thread-safe semaphore-based coordination
- Context cancellation support
- Configurable timeout on acquisition
- Safe to call Release multiple times
- Holder tracking for debugging

**Note:** Observer does not acquire the broker - it runs independently while drain is active.

## Future Components (planned)

- `logreader.go` - Log reader with rotation detection
- `context.go` - Context builder for observer queries
- `query.go` - Observer query execution
