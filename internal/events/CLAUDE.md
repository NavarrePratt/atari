# Events Package

Type definitions for the Atari event system. All components communicate through events.

## Key Types

- `Event` interface: Base for all events (Type, Timestamp, Source)
- `BaseEvent`: Embed this in concrete event types
- `EventType`: String constants for event categories

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
