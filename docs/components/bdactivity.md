# BD Activity Watcher

> **Historical Note**: This document describes an older implementation approach. The current implementation watches `.beads/issues.jsonl` directly using fsnotify instead of spawning `bd activity --follow`. See [internal/bdactivity/CLAUDE.md](/internal/bdactivity/CLAUDE.md) for the current implementation details and [BEADS_INTEGRATION.md](../BEADS_INTEGRATION.md) for integration patterns.

Integrates with `bd activity --follow` to provide real-time bead mutation events.

## Purpose

The BD Activity Watcher is responsible for:
- Running `bd activity --follow --json` as a background process
- Parsing mutation events from the bd daemon
- Converting bd events to the unified event format
- Handling process lifecycle (start, reconnect, stop)

## Interface

```go
type Watcher struct {
    config *config.Config
    events *events.Router
    cmd    *exec.Cmd
    mu     sync.Mutex
}

// Public API
func New(cfg *config.Config, events *events.Router) *Watcher
func (w *Watcher) Start(ctx context.Context) error
func (w *Watcher) Stop() error
func (w *Watcher) Running() bool
```

## Dependencies

| Component | Usage |
|-----------|-------|
| config.Config | Poll interval settings |
| events.Router | Emit parsed bd events |

External:
- `bd activity --follow --json` command

## Implementation

### Starting the Watcher

```go
func (w *Watcher) Start(ctx context.Context) error {
    w.mu.Lock()
    defer w.mu.Unlock()

    args := []string{"activity", "--follow", "--json"}

    cmd := exec.CommandContext(ctx, "bd", args...)
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return fmt.Errorf("create stdout pipe: %w", err)
    }

    if err := cmd.Start(); err != nil {
        return fmt.Errorf("start bd activity: %w", err)
    }

    w.cmd = cmd

    go w.run(ctx, stdout)
    go w.monitor(ctx)

    return nil
}
```

### Parsing Activity Stream

```go
func (w *Watcher) run(ctx context.Context, r io.Reader) {
    scanner := bufio.NewScanner(r)

    for scanner.Scan() {
        select {
        case <-ctx.Done():
            return
        default:
        }

        line := scanner.Bytes()
        if len(line) == 0 {
            continue
        }

        event, err := w.parseLine(line)
        if err != nil {
            w.events.Emit(&events.Error{
                BaseEvent: events.NewInternalEvent(events.EventError),
                Message:   fmt.Sprintf("bd activity parse error: %v", err),
                Severity:  "warning",
            })
            continue
        }

        if event != nil {
            w.events.Emit(event)
        }
    }

    if err := scanner.Err(); err != nil {
        w.events.Emit(&events.Error{
            BaseEvent: events.NewInternalEvent(events.EventError),
            Message:   fmt.Sprintf("bd activity stream error: %v", err),
            Severity:  "warning",
        })
    }
}
```

### BD Activity JSON Format

Expected output from `bd activity --follow --json`:

```json
{"type":"create","issue_id":"bd-042","title":"Fix auth bug","actor":"user","timestamp":"2024-01-15T14:00:00Z"}
{"type":"status","issue_id":"bd-042","old_status":"open","new_status":"in_progress","actor":"claude","timestamp":"2024-01-15T14:01:00Z"}
{"type":"comment","issue_id":"bd-042","actor":"claude","timestamp":"2024-01-15T14:02:00Z"}
{"type":"status","issue_id":"bd-042","old_status":"in_progress","new_status":"closed","actor":"claude","timestamp":"2024-01-15T14:05:00Z"}
```

### Event Parsing

```go
type bdActivity struct {
    Type      string    `json:"type"`
    IssueID   string    `json:"issue_id"`
    Title     string    `json:"title,omitempty"`
    Actor     string    `json:"actor"`
    Timestamp time.Time `json:"timestamp"`
    OldStatus string    `json:"old_status,omitempty"`
    NewStatus string    `json:"new_status,omitempty"`
    ParentID  string    `json:"parent_id,omitempty"`
}

func (w *Watcher) parseLine(line []byte) (events.Event, error) {
    var activity bdActivity
    if err := json.Unmarshal(line, &activity); err != nil {
        return nil, err
    }

    base := events.BaseEvent{
        EventType: w.mapEventType(activity.Type),
        Time:      activity.Timestamp,
        Src:       events.SourceBD,
    }

    switch activity.Type {
    case "create":
        return &events.BeadCreated{
            BaseEvent: base,
            BeadID:    activity.IssueID,
            Title:     activity.Title,
            Actor:     activity.Actor,
        }, nil

    case "status":
        if activity.NewStatus == "closed" || activity.NewStatus == "completed" {
            return &events.BeadClosed{
                BaseEvent: base,
                BeadID:    activity.IssueID,
                Actor:     activity.Actor,
            }, nil
        }
        return &events.BeadStatus{
            BaseEvent: base,
            BeadID:    activity.IssueID,
            OldStatus: activity.OldStatus,
            NewStatus: activity.NewStatus,
            Actor:     activity.Actor,
        }, nil

    case "update":
        return &events.BeadUpdated{
            BaseEvent: base,
            BeadID:    activity.IssueID,
            Actor:     activity.Actor,
        }, nil

    case "comment":
        return &events.BeadComment{
            BaseEvent: base,
            BeadID:    activity.IssueID,
            Actor:     activity.Actor,
        }, nil

    default:
        // Unknown type, skip
        return nil, nil
    }
}

func (w *Watcher) mapEventType(bdType string) events.EventType {
    switch bdType {
    case "create":
        return events.EventBeadCreated
    case "status":
        return events.EventBeadStatus
    case "update":
        return events.EventBeadUpdated
    case "comment":
        return events.EventBeadComment
    default:
        return events.EventBeadUpdated
    }
}
```

### Process Monitoring

The watcher monitors the bd activity process and reconnects on failure:

```go
func (w *Watcher) monitor(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        default:
        }

        w.mu.Lock()
        cmd := w.cmd
        w.mu.Unlock()

        if cmd == nil {
            return
        }

        // Wait for process to exit
        err := cmd.Wait()

        select {
        case <-ctx.Done():
            return
        default:
        }

        // Process died unexpectedly, try to reconnect
        w.events.Emit(&events.Error{
            BaseEvent: events.NewInternalEvent(events.EventError),
            Message:   fmt.Sprintf("bd activity exited: %v, reconnecting...", err),
            Severity:  "warning",
        })

        // Backoff before reconnecting
        time.Sleep(5 * time.Second)

        if err := w.Start(ctx); err != nil {
            w.events.Emit(&events.Error{
                BaseEvent: events.NewInternalEvent(events.EventError),
                Message:   fmt.Sprintf("bd activity reconnect failed: %v", err),
                Severity:  "error",
            })
            // Continue trying
            time.Sleep(30 * time.Second)
        }
    }
}
```

### Stopping the Watcher

```go
func (w *Watcher) Stop() error {
    w.mu.Lock()
    defer w.mu.Unlock()

    if w.cmd == nil || w.cmd.Process == nil {
        return nil
    }

    // Send SIGTERM
    if err := w.cmd.Process.Signal(syscall.SIGTERM); err != nil {
        // Process may have already exited
        return nil
    }

    // Wait briefly
    done := make(chan error, 1)
    go func() {
        done <- w.cmd.Wait()
    }()

    select {
    case <-done:
    case <-time.After(2 * time.Second):
        w.cmd.Process.Kill()
    }

    w.cmd = nil
    return nil
}

func (w *Watcher) Running() bool {
    w.mu.Lock()
    defer w.mu.Unlock()
    return w.cmd != nil && w.cmd.Process != nil
}
```

## BD Mutation Types

From beads source (`internal/rpc/server_core.go`):

| Type | Description |
|------|-------------|
| `create` | New issue created |
| `update` | Issue metadata changed |
| `status` | Status transition (open -> in_progress -> closed) |
| `comment` | Comment added |
| `bonded` | Issue bonded to molecule |
| `squashed` | Issues merged |
| `burned` | Issue deleted |
| `delete` | Issue removed |

## Event Display

BD events are formatted for human readability:

```
[14:01:00] → bd-042 in_progress (claude)
[14:02:00] + bd-043 created "Add rate limiting"
[14:05:00] ✓ bd-042 closed (claude)
```

Symbols:
- `+` created
- `→` in_progress
- `✓` closed/completed
- `✗` failed
- `⊘` deleted

## Integration with Controller

The BD Activity Watcher is optional and runs in parallel with the main controller:

```go
func (c *Controller) Run(ctx context.Context) error {
    // Start BD activity watcher (non-blocking, errors logged)
    if c.config.BDActivity.Enabled {
        if err := c.bdWatcher.Start(ctx); err != nil {
            c.events.Emit(&events.Error{
                Message:  fmt.Sprintf("bd activity unavailable: %v", err),
                Severity: "warning",
            })
            // Continue without bd activity - not fatal
        }
    }

    // ... main loop
}
```

## Testing

### Unit Tests

- Event parsing: test all mutation types
- Reconnection: verify reconnect on process exit
- Stop: verify clean shutdown

### Test Fixtures

```go
var sampleActivity = `{"type":"create","issue_id":"bd-001","title":"Test issue","actor":"test","timestamp":"2024-01-15T14:00:00Z"}
{"type":"status","issue_id":"bd-001","old_status":"open","new_status":"in_progress","actor":"test","timestamp":"2024-01-15T14:01:00Z"}
{"type":"status","issue_id":"bd-001","old_status":"in_progress","new_status":"closed","actor":"test","timestamp":"2024-01-15T14:02:00Z"}
`

func TestParseActivity(t *testing.T) {
    w := New(&config.Config{}, events.NewRouter())

    lines := strings.Split(sampleActivity, "\n")
    for _, line := range lines {
        if line == "" {
            continue
        }
        event, err := w.parseLine([]byte(line))
        if err != nil {
            t.Errorf("parse error: %v", err)
        }
        if event == nil {
            t.Error("expected event, got nil")
        }
    }
}
```

### Integration Tests

- Full lifecycle with mock bd activity
- Reconnection behavior after crash
- Concurrent operation with controller

## Error Handling

| Error | Action |
|-------|--------|
| `bd activity` not available | Log warning, continue without it |
| Process exits | Reconnect with backoff |
| Parse error | Log warning, skip line |
| Reconnect fails | Keep retrying with increasing backoff |

## Configuration

```yaml
bd_activity:
  enabled: true
  reconnect_delay: 5s
  max_reconnect_delay: 5m
```

## Future Considerations

- **Filtering**: Only watch specific issues or labels
- **Molecule tracking**: Track molecule-level events
- **Historical replay**: Catch up on events since last run
- **Direct RPC**: Connect to bd daemon via gRPC instead of CLI
