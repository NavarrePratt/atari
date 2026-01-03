# Observer Package

Provides the TUI observer mode for real-time Q&A about drain activity.

## Components

### SessionBroker (broker.go)

Coordinates access to the Claude CLI process. Ensures only one Claude process runs at a time - either a drain session or an observer query.

**Usage:**
```go
broker := NewSessionBroker()

// Acquire with timeout
err := broker.Acquire(ctx, "drain", 5*time.Second)
if err != nil {
    // Handle timeout or context cancellation
}
defer broker.Release()

// Non-blocking attempt
if broker.TryAcquire("observer") {
    defer broker.Release()
    // Run observer query
}

// Check current state
holder := broker.Holder() // "drain", "observer", or ""
isHeld := broker.IsHeld()
```

**Key behaviors:**
- Thread-safe semaphore-based coordination
- Context cancellation support
- Configurable timeout on acquisition
- Safe to call Release multiple times
- Holder tracking for debugging

## Future Components (planned)

- `logreader.go` - Log reader with rotation detection
- `context.go` - Context builder for observer queries
- `query.go` - Observer query execution
