# Event System

Defines event types and the router that merges multiple event sources into a unified stream.

## Purpose

The Event System is responsible for:
- Defining a unified event taxonomy across all sources
- Routing events from producers (Claude, bd, internal) to consumers (sinks)
- Providing a channel-based event bus for decoupled communication
- Supporting multiple concurrent consumers

## Interface

```go
// Router manages event flow from producers to consumers
type Router struct {
    subscribers []chan Event
    mu          sync.RWMutex
}

// Event is the base interface for all events
type Event interface {
    Type() EventType
    Timestamp() time.Time
    Source() string
}

// Public API
func NewRouter() *Router
func (r *Router) Emit(event Event)
func (r *Router) Subscribe() <-chan Event
func (r *Router) Unsubscribe(ch <-chan Event)
func (r *Router) Close()
```

## Dependencies

None - the event system is a foundational component with no dependencies.

## Event Types

### Event Type Taxonomy

```go
type EventType string

const (
    // Claude session events
    EventSessionStart EventType = "session.start"
    EventSessionEnd   EventType = "session.end"
    EventCompact      EventType = "session.compact"

    // Claude content events
    EventText     EventType = "claude.text"
    EventThinking EventType = "claude.thinking"
    EventToolUse  EventType = "claude.tool_use"
    EventToolResult EventType = "claude.tool_result"

    // BD bead events
    EventBeadCreated   EventType = "bead.created"
    EventBeadUpdated   EventType = "bead.updated"
    EventBeadStatus    EventType = "bead.status"
    EventBeadClosed    EventType = "bead.closed"
    EventBeadComment   EventType = "bead.comment"
    EventBeadAbandoned EventType = "bead.abandoned"  // Hit max_failures limit

    // Drain control events
    EventDrainStart  EventType = "drain.start"
    EventDrainPause  EventType = "drain.pause"
    EventDrainResume EventType = "drain.resume"
    EventDrainStop   EventType = "drain.stop"

    // Iteration events
    EventIterationStart EventType = "iteration.start"
    EventIterationEnd   EventType = "iteration.end"

    // Error events
    EventError EventType = "error"
)
```

### Source Constants

```go
const (
    SourceClaude   = "claude"
    SourceBD       = "bd"
    SourceInternal = "atari"
)
```

## Event Definitions

### Base Event

```go
type BaseEvent struct {
    EventType EventType `json:"type"`
    Time      time.Time `json:"timestamp"`
    Src       string    `json:"source"`
}

func (e BaseEvent) Type() EventType    { return e.EventType }
func (e BaseEvent) Timestamp() time.Time { return e.Time }
func (e BaseEvent) Source() string     { return e.Src }
```

### Claude Events

```go
// SessionStart emitted when Claude session begins
type SessionStart struct {
    BaseEvent
    Model string   `json:"model"`
    Tools []string `json:"tools"`
}

// SessionEnd emitted when Claude session completes
type SessionEnd struct {
    BaseEvent
    SessionID    string  `json:"session_id"`
    NumTurns     int     `json:"num_turns"`
    DurationMs   int     `json:"duration_ms"`
    TotalCostUSD float64 `json:"total_cost_usd"`
    Result       string  `json:"result,omitempty"`
}

// Compact emitted when context compaction occurs
type Compact struct {
    BaseEvent
}

// Text emitted for assistant text output
type Text struct {
    BaseEvent
    Text string `json:"text"`
}

// Thinking emitted for extended thinking content
type Thinking struct {
    BaseEvent
    Text string `json:"text"`
}

// ToolUse emitted when Claude invokes a tool
type ToolUse struct {
    BaseEvent
    ID    string         `json:"tool_id"`
    Name  string         `json:"tool_name"`
    Input map[string]any `json:"input"`
}

// ToolResult emitted after tool execution
type ToolResult struct {
    BaseEvent
    ID      string `json:"tool_id"`
    Content string `json:"content"`
    IsError bool   `json:"is_error,omitempty"`
}
```

### BD Events

```go
// BeadCreated emitted when a new bead is created
type BeadCreated struct {
    BaseEvent
    BeadID string `json:"bead_id"`
    Title  string `json:"title"`
    Actor  string `json:"actor"`
}

// BeadUpdated emitted when bead metadata changes
type BeadUpdated struct {
    BaseEvent
    BeadID string         `json:"bead_id"`
    Fields map[string]any `json:"fields"`
    Actor  string         `json:"actor"`
}

// BeadStatus emitted when bead status changes
type BeadStatus struct {
    BaseEvent
    BeadID    string `json:"bead_id"`
    OldStatus string `json:"old_status"`
    NewStatus string `json:"new_status"`
    Actor     string `json:"actor"`
}

// BeadClosed emitted when a bead is closed
type BeadClosed struct {
    BaseEvent
    BeadID string `json:"bead_id"`
    Reason string `json:"reason"`
    Actor  string `json:"actor"`
}

// BeadComment emitted when a comment is added
type BeadComment struct {
    BaseEvent
    BeadID  string `json:"bead_id"`
    Comment string `json:"comment"`
    Actor   string `json:"actor"`
}

// BeadAbandoned emitted when a bead hits max_failures limit
type BeadAbandoned struct {
    BaseEvent
    BeadID      string `json:"bead_id"`
    Attempts    int    `json:"attempts"`
    MaxFailures int    `json:"max_failures"`
    LastError   string `json:"last_error"`
}
```

### Internal Events

```go
// DrainStart emitted when atari starts
type DrainStart struct {
    BaseEvent
    WorkDir string `json:"work_dir"`
    Config  string `json:"config,omitempty"`
}

// DrainPause emitted when pause requested
type DrainPause struct {
    BaseEvent
    Reason string `json:"reason,omitempty"`
}

// DrainResume emitted when resuming from pause
type DrainResume struct {
    BaseEvent
}

// DrainStop emitted when stopping
type DrainStop struct {
    BaseEvent
    Reason string `json:"reason,omitempty"`
}

// IterationStart emitted when beginning work on a bead
type IterationStart struct {
    BaseEvent
    BeadID   string `json:"bead_id"`
    Title    string `json:"title"`
    Priority int    `json:"priority"`
    Attempt  int    `json:"attempt"`
}

// IterationEnd emitted when bead work completes
type IterationEnd struct {
    BaseEvent
    BeadID       string  `json:"bead_id"`
    Success      bool    `json:"success"`
    NumTurns     int     `json:"num_turns"`
    DurationMs   int     `json:"duration_ms"`
    TotalCostUSD float64 `json:"total_cost_usd"`
    Error        string  `json:"error,omitempty"`
}

// Error emitted for any error condition
type Error struct {
    BaseEvent
    Err      error  `json:"-"`
    Message  string `json:"message"`
    BeadID   string `json:"bead_id,omitempty"`
    Severity string `json:"severity"` // "warning", "error", "fatal"
}
```

## Implementation

### Router

```go
type Router struct {
    subscribers []chan Event
    bufferSize  int
    mu          sync.RWMutex
    closed      bool
}

func NewRouter() *Router {
    return &Router{
        bufferSize: 100,
    }
}

func (r *Router) Emit(event Event) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    if r.closed {
        return
    }

    for _, ch := range r.subscribers {
        select {
        case ch <- event:
        default:
            // Channel full, drop event (or could log warning)
        }
    }
}

func (r *Router) Subscribe() <-chan Event {
    r.mu.Lock()
    defer r.mu.Unlock()

    ch := make(chan Event, r.bufferSize)
    r.subscribers = append(r.subscribers, ch)
    return ch
}

func (r *Router) Unsubscribe(ch <-chan Event) {
    r.mu.Lock()
    defer r.mu.Unlock()

    for i, sub := range r.subscribers {
        if sub == ch {
            r.subscribers = append(r.subscribers[:i], r.subscribers[i+1:]...)
            close(sub)
            break
        }
    }
}

func (r *Router) Close() {
    r.mu.Lock()
    defer r.mu.Unlock()

    r.closed = true
    for _, ch := range r.subscribers {
        close(ch)
    }
    r.subscribers = nil
}
```

### Event Construction Helpers

```go
func NewEvent(eventType EventType, source string) BaseEvent {
    return BaseEvent{
        EventType: eventType,
        Time:      time.Now(),
        Src:       source,
    }
}

func NewClaudeEvent(eventType EventType) BaseEvent {
    return NewEvent(eventType, SourceClaude)
}

func NewBDEvent(eventType EventType) BaseEvent {
    return NewEvent(eventType, SourceBD)
}

func NewInternalEvent(eventType EventType) BaseEvent {
    return NewEvent(eventType, SourceInternal)
}
```

## JSON Serialization

All events serialize to JSON for logging and RPC:

```json
{
  "type": "claude.tool_use",
  "timestamp": "2024-01-15T14:23:05Z",
  "source": "claude",
  "tool_id": "tool_abc123",
  "tool_name": "Bash",
  "input": {"command": "git status"}
}
```

```json
{
  "type": "bead.status",
  "timestamp": "2024-01-15T14:23:10Z",
  "source": "bd",
  "bead_id": "bd-042",
  "old_status": "open",
  "new_status": "in_progress",
  "actor": "claude"
}
```

```json
{
  "type": "iteration.end",
  "timestamp": "2024-01-15T14:25:00Z",
  "source": "atari",
  "bead_id": "bd-042",
  "success": true,
  "num_turns": 8,
  "duration_ms": 115000,
  "total_cost_usd": 0.42
}
```

## Display Formatting

For human-readable output, events are formatted with symbols and colors:

```go
func (e *ToolUse) Format() string {
    switch e.Name {
    case "Bash":
        cmd := e.Input["command"].(string)
        return fmt.Sprintf("$ %s", truncate(cmd, 60))
    case "Read":
        return fmt.Sprintf("Read: %s", e.Input["file_path"])
    case "Edit":
        return fmt.Sprintf("Edit: %s", e.Input["file_path"])
    case "Write":
        return fmt.Sprintf("Write: %s", e.Input["file_path"])
    case "Glob":
        return fmt.Sprintf("Glob: %s", e.Input["pattern"])
    case "Grep":
        return fmt.Sprintf("Grep: %s", e.Input["pattern"])
    default:
        return fmt.Sprintf("Tool: %s", e.Name)
    }
}

func (e *BeadStatus) Format() string {
    symbol := statusSymbol(e.NewStatus)
    return fmt.Sprintf("%s %s %s", symbol, e.BeadID, e.NewStatus)
}

func statusSymbol(status string) string {
    switch status {
    case "in_progress":
        return "→"
    case "closed", "completed":
        return "✓"
    case "failed":
        return "✗"
    default:
        return "•"
    }
}
```

## Testing

### Unit Tests

- Event construction: verify all fields populated
- Router: test emit/subscribe/unsubscribe lifecycle
- JSON serialization: round-trip all event types
- Format methods: verify human-readable output

### Test Cases

```go
func TestRouterEmitSubscribe(t *testing.T) {
    r := NewRouter()
    ch := r.Subscribe()

    event := &Text{
        BaseEvent: NewClaudeEvent(EventText),
        Text:      "Hello",
    }

    r.Emit(event)

    select {
    case received := <-ch:
        if received.Type() != EventText {
            t.Errorf("expected %s, got %s", EventText, received.Type())
        }
    case <-time.After(time.Second):
        t.Error("timeout waiting for event")
    }
}

func TestEventJSON(t *testing.T) {
    event := &ToolUse{
        BaseEvent: NewClaudeEvent(EventToolUse),
        ID:        "tool_1",
        Name:      "Bash",
        Input:     map[string]any{"command": "ls"},
    }

    data, err := json.Marshal(event)
    if err != nil {
        t.Fatal(err)
    }

    var decoded ToolUse
    if err := json.Unmarshal(data, &decoded); err != nil {
        t.Fatal(err)
    }

    if decoded.Name != "Bash" {
        t.Errorf("expected Bash, got %s", decoded.Name)
    }
}
```

## Error Handling

| Error | Action |
|-------|--------|
| Channel full on emit | Drop event (non-blocking) |
| Emit after close | Silently ignore |
| Invalid event type | Log warning, continue |

## Future Considerations

- **Event persistence**: Replay events from log
- **Event filtering**: Subscribe to specific event types only
- **Backpressure**: Slow down producers when consumers lag
- **Remote subscribers**: WebSocket or gRPC event streaming
