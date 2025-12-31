# Observer Mode

Interactive Q&A pane for asking questions about the event stream during operation.

**Note**: This component is planned for future implementation, not the initial POC.

## Purpose

The Observer Mode allows users to:
- Ask questions about events without pausing the main drain
- Get context about what's happening in real-time
- Investigate specific events or patterns
- Understand Claude's decision-making process

## Interface

```go
type Observer struct {
    config    *config.ObserverConfig
    events    *events.RingBuffer
    session   *claudeSession
}

type ObserverConfig struct {
    Model         string  // Default: "haiku"
    EventCount    int     // Number of events for context (default: 50)
    ShowCost      bool    // Display cost tracking (default: true)
}

// Public API
func New(cfg *ObserverConfig, events *events.RingBuffer) *Observer
func (o *Observer) Start(ctx context.Context) error
func (o *Observer) Stop() error
func (o *Observer) Ask(question string) (<-chan string, error)
func (o *Observer) IsActive() bool
func (o *Observer) SessionCost() float64
```

## Dependencies

| Component | Usage |
|-----------|-------|
| events.RingBuffer | Access to recent event history |
| config.ObserverConfig | Model, event count, display settings |

External:
- `claude` CLI binary (haiku model by default)

## Design Decisions

### Model Selection

- **Default**: Haiku - fast responses, low cost for Q&A
- **Configurable**: Users can switch to sonnet/opus for deeper analysis
- Haiku is appropriate because observer questions are typically:
  - Event summarization
  - Pattern identification
  - Simple clarifications

### Context Window

- **Event count**: Configurable number of recent events (default: 50)
- Events are serialized to JSON for Claude context
- Older events are trimmed to fit context window
- User can adjust based on memory/cost tradeoff

### Session Lifecycle

- Session starts when observer pane is opened in TUI
- Session persists while pane is open (maintains conversation history)
- Session terminates when pane is closed
- No persistence across pane open/close cycles

### Cost Tracking

- Observer sessions track cost separately from main drain
- Display is configurable (on by default)
- Helps users understand the overhead of observer queries

## Implementation

### Observer Session

```go
type claudeSession struct {
    cmd     *exec.Cmd
    stdin   io.WriteCloser
    stdout  io.ReadCloser
    cost    float64
    mu      sync.Mutex
}

func (o *Observer) Start(ctx context.Context) error {
    o.mu.Lock()
    defer o.mu.Unlock()

    if o.session != nil {
        return nil // Already running
    }

    args := []string{
        "--print",
        "--model", o.config.Model,
        "--output-format", "stream-json",
    }

    cmd := exec.CommandContext(ctx, "claude", args...)

    stdin, err := cmd.StdinPipe()
    if err != nil {
        return fmt.Errorf("create stdin pipe: %w", err)
    }

    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return fmt.Errorf("create stdout pipe: %w", err)
    }

    if err := cmd.Start(); err != nil {
        return fmt.Errorf("start claude: %w", err)
    }

    o.session = &claudeSession{
        cmd:    cmd,
        stdin:  stdin,
        stdout: stdout,
    }

    return nil
}
```

### Building Context

```go
func (o *Observer) buildContext() string {
    events := o.events.Recent(o.config.EventCount)

    var b strings.Builder
    b.WriteString("You are an observer assistant helping the user understand ")
    b.WriteString("what's happening in an automated bead processing session.\n\n")
    b.WriteString("Recent events (newest last):\n\n")

    for _, e := range events {
        b.WriteString(formatEventForContext(e))
        b.WriteString("\n")
    }

    return b.String()
}

func formatEventForContext(e events.Event) string {
    ts := e.Timestamp().Format("15:04:05")

    switch ev := e.(type) {
    case *events.ToolUse:
        return fmt.Sprintf("[%s] Tool: %s - %s", ts, ev.Name, summarizeInput(ev.Input))
    case *events.Text:
        return fmt.Sprintf("[%s] Claude: %s", ts, truncate(ev.Text, 200))
    case *events.BeadStatus:
        return fmt.Sprintf("[%s] Bead %s: %s -> %s", ts, ev.ID, ev.OldStatus, ev.NewStatus)
    case *events.SessionStart:
        return fmt.Sprintf("[%s] Session started (model: %s)", ts, ev.Model)
    case *events.SessionEnd:
        return fmt.Sprintf("[%s] Session ended (turns: %d, cost: $%.2f)", ts, ev.NumTurns, ev.TotalCostUSD)
    case *events.Error:
        return fmt.Sprintf("[%s] ERROR: %s", ts, ev.Message)
    default:
        return fmt.Sprintf("[%s] %T", ts, e)
    }
}
```

### Asking Questions

```go
func (o *Observer) Ask(question string) (<-chan string, error) {
    o.mu.Lock()
    defer o.mu.Unlock()

    if o.session == nil {
        return nil, fmt.Errorf("observer session not started")
    }

    // Build prompt with context
    context := o.buildContext()
    prompt := fmt.Sprintf("%s\n\nUser question: %s", context, question)

    // Send to claude
    if _, err := o.session.stdin.Write([]byte(prompt + "\n")); err != nil {
        return nil, fmt.Errorf("write prompt: %w", err)
    }

    // Stream response
    responses := make(chan string, 100)
    go o.streamResponse(responses)

    return responses, nil
}

func (o *Observer) streamResponse(out chan<- string) {
    defer close(out)

    scanner := bufio.NewScanner(o.session.stdout)
    for scanner.Scan() {
        line := scanner.Bytes()

        var msg struct {
            Type    string `json:"type"`
            Message struct {
                Content []struct {
                    Type string `json:"type"`
                    Text string `json:"text"`
                } `json:"content"`
            } `json:"message"`
            TotalCostUSD float64 `json:"total_cost_usd"`
        }

        if err := json.Unmarshal(line, &msg); err != nil {
            continue
        }

        if msg.Type == "assistant" {
            for _, content := range msg.Message.Content {
                if content.Type == "text" {
                    out <- content.Text
                }
            }
        }

        if msg.Type == "result" {
            o.session.cost += msg.TotalCostUSD
            return // Response complete
        }
    }
}
```

### Cleanup

```go
func (o *Observer) Stop() error {
    o.mu.Lock()
    defer o.mu.Unlock()

    if o.session == nil {
        return nil
    }

    // Close stdin to signal end of input
    o.session.stdin.Close()

    // Wait for process to exit
    done := make(chan error, 1)
    go func() {
        done <- o.session.cmd.Wait()
    }()

    select {
    case <-done:
    case <-time.After(5 * time.Second):
        o.session.cmd.Process.Kill()
    }

    o.session = nil
    return nil
}
```

## Event Ring Buffer

The observer needs access to recent events without affecting the main event stream:

```go
type RingBuffer struct {
    events []events.Event
    size   int
    head   int
    count  int
    mu     sync.RWMutex
}

func NewRingBuffer(size int) *RingBuffer {
    return &RingBuffer{
        events: make([]events.Event, size),
        size:   size,
    }
}

func (rb *RingBuffer) Add(e events.Event) {
    rb.mu.Lock()
    defer rb.mu.Unlock()

    rb.events[rb.head] = e
    rb.head = (rb.head + 1) % rb.size
    if rb.count < rb.size {
        rb.count++
    }
}

func (rb *RingBuffer) Recent(n int) []events.Event {
    rb.mu.RLock()
    defer rb.mu.RUnlock()

    if n > rb.count {
        n = rb.count
    }

    result := make([]events.Event, n)
    start := (rb.head - n + rb.size) % rb.size

    for i := 0; i < n; i++ {
        result[i] = rb.events[(start+i)%rb.size]
    }

    return result
}
```

## Configuration

```yaml
observer:
  enabled: true
  model: haiku              # Model for observer queries
  event_count: 50           # Number of events for context
  show_cost: true           # Display observer session cost
```

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `enabled` | bool | true | Enable observer mode in TUI |
| `model` | string | "haiku" | Claude model for observer |
| `event_count` | int | 50 | Events to include in context |
| `show_cost` | bool | true | Show observer cost in TUI |

## TUI Integration

The observer appears as a pane in the TUI. See [tui.md](tui.md) for integration details.

### Observer Pane Layout

```
┌─ ATARI ─────────────────────────────────────────────────────────┐
│ Status: WORKING                            Cost: $2.35          │
│ Current: bd-042 "Fix auth bug"             Turns: 42            │
├─ Events ────────────────────────────────────────────────────────┤
│ [event feed...]                                                 │
│                                                                 │
├─ Observer (haiku) ──────────────────────── Cost: $0.03 ─────────┤
│ > Why did Claude run the tests twice?                           │
│                                                                 │
│ The tests were run twice because the first run had a flaky      │
│ failure in test_auth.go. Claude retried to confirm whether      │
│ it was a genuine failure or transient issue.                    │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│ [o] observer  [p] pause  [r] resume  [q] quit  [↑↓] scroll      │
└─────────────────────────────────────────────────────────────────┘
```

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `o` | Toggle observer pane |
| `Enter` | Send question (when in observer) |
| `Esc` | Close observer pane |
| `Tab` | Switch focus between events and observer |

## Example Queries

Users might ask:
- "Why did Claude choose to modify that file?"
- "What error caused the last retry?"
- "Summarize what happened in the last 5 minutes"
- "Is the current approach likely to succeed?"
- "What's taking so long?"

## Testing

### Unit Tests

- Context building: verify event serialization
- Ring buffer: test circular behavior
- Session lifecycle: start, query, stop

### Integration Tests

- Full Q&A flow with mock claude
- TUI pane toggle behavior
- Cost tracking accuracy

## Error Handling

| Error | Action |
|-------|--------|
| Claude not available | Show error in observer pane |
| Session crashes | Auto-restart on next query |
| Response timeout | Show timeout message, allow retry |
| Context too large | Reduce event count, warn user |

## Future Considerations

- **Voice queries**: Whisper integration for hands-free questions
- **Auto-insights**: Proactive observations without user prompting
- **Query history**: Persist Q&A across sessions
- **Event filtering**: Focus context on specific event types
- **Model switching**: Hot-swap models mid-session
