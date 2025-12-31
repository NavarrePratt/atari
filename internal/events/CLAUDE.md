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
| `session.*` | atari | start, end |
| `claude.*` | claude | text, tool_use, tool_result |
| `drain.*` | atari | start, stop |
| `iteration.*` | atari | start, end |
| `bead.*` | atari | abandoned |
| `error` | any | error conditions |

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
