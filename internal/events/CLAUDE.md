# Events Package

Type definitions and router for the Atari event system. All components communicate through events.

## Key Types

- `Event` interface: Base for all events (Type, Timestamp, Source)
- `BaseEvent`: Embed this in concrete event types
- `EventType`: String constants for event categories
- `Router`: Channel-based pub/sub for broadcasting events

## Event Categories

| Prefix | Source | Examples |
|--------|--------|----------|
| `session.*` | atari | start, end, timeout |
| `claude.*` | claude | text, tool_use, tool_result |
| `drain.*` | atari | start, stop |
| `iteration.*` | atari | start, end |
| `bead.*` | atari/bd | abandoned (internal), created/status/updated/comment/closed (bd activity) |
| `error` | any | generic error |
| `error.parse` | atari | stream-json parse failures |

## BD Activity Events

Events from `bd activity --follow` for real-time bead changes:

| Event Type | Struct | Key Fields |
|------------|--------|------------|
| `bead.created` | BeadCreatedEvent | BeadID, Title, Actor |
| `bead.status` | BeadStatusEvent | BeadID, OldStatus, NewStatus, Actor |
| `bead.updated` | BeadUpdatedEvent | BeadID, Actor |
| `bead.comment` | BeadCommentEvent | BeadID, Actor |
| `bead.closed` | BeadClosedEvent | BeadID, Actor |

All BD activity events use `SourceBD` ("bd") as their source.

## Creating Events

```go
// Use helper constructors
base := events.NewClaudeEvent(events.EventClaudeText)
base := events.NewInternalEvent(events.EventIterationStart)

// Or create directly
evt := &events.ClaudeTextEvent{
    BaseEvent: events.NewClaudeEvent(events.EventClaudeText),
    Text:      "Hello",
}
```

## Bead History

`BeadHistory` tracks processing state for workqueue persistence:
- `HistoryPending`, `HistoryWorking`, `HistoryCompleted`, `HistoryFailed`, `HistoryAbandoned`

## Adding Event Types

1. Add `EventType` constant
2. Create struct embedding `BaseEvent`
3. Add JSON tags for persistence
4. Add tests in types_test.go

## Router

Channel-based pub/sub for broadcasting events to multiple subscribers. Non-blocking emit with configurable buffer sizes.

### Usage

```go
router := events.NewRouter(100) // buffer size per subscriber
defer router.Close()

// Subscribe
ch := router.Subscribe()
// or with custom buffer
ch := router.SubscribeBuffered(500)

// Emit (non-blocking, drops if buffer full)
router.Emit(evt)

// Unsubscribe when done
router.Unsubscribe(ch)
```

### Behavior

- `Emit()` is non-blocking: drops events if subscriber buffer is full (logs warning)
- Multiple subscribers receive all events
- Thread-safe for concurrent access
- `Close()` closes all subscriber channels (idempotent)

## Sinks

Sinks consume events from the router for persistence and analysis. All sinks implement the `Sink` interface.

### Sink Interface

```go
type Sink interface {
    Start(ctx context.Context, events <-chan Event) error
    Stop() error
}
```

### LogSink

Writes events to a JSON lines file for debugging and analysis.

```go
sink := events.NewLogSink(".atari/atari.log")
ch := router.SubscribeBuffered(100)
sink.Start(ctx, ch)
// ... run ...
sink.Stop()
```

- Appends JSON-encoded events, one per line
- Creates parent directories automatically
- Thread-safe writes with mutex protection

### StateSink

Persists runtime state to JSON for crash recovery.

```go
sink := events.NewStateSink(".atari/state.json")
ch := router.SubscribeBuffered(events.StateBufferSize) // 1000
sink.Start(ctx, ch)
// ... run ...
sink.Stop()
```

**State tracking**:
- Drain status (running/stopped)
- Iteration count and current bead
- Per-bead history (status, attempts, errors)
- Aggregate cost and turn counts

**Behavior**:
- Debounced saves (default 5s) to reduce disk writes
- Immediate save on drain stop
- Atomic writes via temp file + rename
- Loads existing state on Start if present
