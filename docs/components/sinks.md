# Event Sinks

Consumers of the unified event stream that persist, display, or act on events.

## Purpose

Event Sinks are responsible for:
- Consuming events from the event router
- Persisting events to log files (Log Sink)
- Maintaining runtime state and persisting to disk (State Sink)
- Providing building blocks for other consumers (TUI has its own doc)

## Interface

```go
// Sink consumes events from the router
type Sink interface {
    Start(ctx context.Context, events <-chan Event) error
    Stop() error
}

// LogSink writes events to a file
type LogSink struct {
    path   string
    file   *os.File
    mu     sync.Mutex
}

// StateSink maintains and persists runtime state
type StateSink struct {
    path  string
    state *State
    mu    sync.RWMutex
}
```

## Dependencies

| Component | Usage |
|-----------|-------|
| events.Router | Subscribe to event stream |

## Log Sink

### Purpose

Writes all events to a JSON lines file for:
- Post-hoc analysis and debugging
- Cost tracking across sessions
- Audit trail of all actions

### Implementation

```go
type LogSink struct {
    path    string
    file    *os.File
    encoder *json.Encoder
    mu      sync.Mutex
}

func NewLogSink(path string) *LogSink {
    return &LogSink{path: path}
}

func (s *LogSink) Start(ctx context.Context, events <-chan Event) error {
    if err := s.openFile(); err != nil {
        return err
    }

    go s.run(ctx, events)
    return nil
}

func (s *LogSink) openFile() error {
    // Rotate existing log
    if _, err := os.Stat(s.path); err == nil {
        backup := fmt.Sprintf("%s.%s.bak", s.path, time.Now().Format("20060102-150405"))
        os.Rename(s.path, backup)
    }

    // Ensure directory exists
    dir := filepath.Dir(s.path)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return fmt.Errorf("create log directory: %w", err)
    }

    file, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
    if err != nil {
        return fmt.Errorf("open log file: %w", err)
    }

    s.file = file
    s.encoder = json.NewEncoder(file)
    return nil
}

func (s *LogSink) run(ctx context.Context, events <-chan Event) {
    for {
        select {
        case <-ctx.Done():
            return
        case event, ok := <-events:
            if !ok {
                return
            }
            s.write(event)
        }
    }
}

func (s *LogSink) write(event Event) {
    s.mu.Lock()
    defer s.mu.Unlock()

    if s.encoder == nil {
        return
    }

    if err := s.encoder.Encode(event); err != nil {
        // Log to stderr, but don't fail
        fmt.Fprintf(os.Stderr, "failed to write event: %v\n", err)
    }
}

func (s *LogSink) Stop() error {
    s.mu.Lock()
    defer s.mu.Unlock()

    if s.file != nil {
        return s.file.Close()
    }
    return nil
}
```

### Log File Format

Each line is a complete JSON object:

```json
{"type":"drain.start","timestamp":"2024-01-15T14:00:00Z","source":"atari","work_dir":"/path/to/project"}
{"type":"iteration.start","timestamp":"2024-01-15T14:00:01Z","source":"atari","bead_id":"bd-042","title":"Fix auth bug","priority":1,"attempt":1}
{"type":"session.start","timestamp":"2024-01-15T14:00:02Z","source":"claude","model":"opus","tools":["Bash","Read","Edit"]}
{"type":"claude.tool_use","timestamp":"2024-01-15T14:00:05Z","source":"claude","tool_id":"t1","tool_name":"Bash","input":{"command":"git status"}}
{"type":"bead.status","timestamp":"2024-01-15T14:00:10Z","source":"bd","bead_id":"bd-042","old_status":"open","new_status":"in_progress"}
{"type":"session.end","timestamp":"2024-01-15T14:02:00Z","source":"claude","session_id":"abc123","num_turns":8,"duration_ms":118000,"total_cost_usd":0.42}
{"type":"iteration.end","timestamp":"2024-01-15T14:02:01Z","source":"atari","bead_id":"bd-042","success":true,"num_turns":8,"duration_ms":120000,"total_cost_usd":0.42}
```

### Log Rotation

- On startup, existing log is renamed to `{name}.{timestamp}.bak`
- No automatic size-based rotation (logs are typically small)
- Users can implement external rotation if needed

### Log Location

Default: `.atari/atari.log`

Can be overridden via:
- `--log` flag
- `ATARI_LOG` environment variable
- Config file `log_path` setting

---

## State Sink

### Purpose

Maintains runtime state and persists to disk for:
- Resume after crash or restart
- Status reporting via CLI
- Statistics tracking

### State Structure

```go
type State struct {
    Version      int            `json:"version"`
    Status       string         `json:"status"`
    StartedAt    time.Time      `json:"started_at"`
    CurrentBead  string         `json:"current_bead,omitempty"`
    CurrentSession string       `json:"current_session_id,omitempty"`
    Stats        Stats          `json:"stats"`
    History      map[string]*BeadHistory `json:"history"`
}

type Stats struct {
    Iterations     int     `json:"iterations"`
    BeadsCompleted int     `json:"beads_completed"`
    BeadsFailed    int     `json:"beads_failed"`
    TotalCostUSD   float64 `json:"total_cost_usd"`
    TotalTurns     int     `json:"total_turns"`
    TotalDurationMs int64  `json:"total_duration_ms"`
}

type BeadHistory struct {
    Status      string    `json:"status"`
    Attempts    int       `json:"attempts"`
    LastAttempt time.Time `json:"last_attempt,omitempty"`
    LastError   string    `json:"last_error,omitempty"`
}
```

### Implementation

```go
type StateSink struct {
    path  string
    state *State
    mu    sync.RWMutex
}

func NewStateSink(path string) *StateSink {
    return &StateSink{
        path: path,
        state: &State{
            Version: 1,
            Status:  "idle",
            History: make(map[string]*BeadHistory),
        },
    }
}

func (s *StateSink) Load() error {
    s.mu.Lock()
    defer s.mu.Unlock()

    data, err := os.ReadFile(s.path)
    if os.IsNotExist(err) {
        return nil // Fresh start
    }
    if err != nil {
        return fmt.Errorf("read state file: %w", err)
    }

    var state State
    if err := json.Unmarshal(data, &state); err != nil {
        return fmt.Errorf("parse state file: %w", err)
    }

    s.state = &state
    return nil
}

func (s *StateSink) Start(ctx context.Context, events <-chan Event) error {
    if err := s.Load(); err != nil {
        // Log warning but continue - we can start fresh
        fmt.Fprintf(os.Stderr, "warning: failed to load state: %v\n", err)
    }

    s.state.StartedAt = time.Now()
    s.state.Status = "idle"

    go s.run(ctx, events)
    return nil
}

func (s *StateSink) run(ctx context.Context, events <-chan Event) {
    for {
        select {
        case <-ctx.Done():
            s.save()
            return
        case event, ok := <-events:
            if !ok {
                s.save()
                return
            }
            s.handleEvent(event)
        }
    }
}

func (s *StateSink) handleEvent(event Event) {
    s.mu.Lock()
    defer s.mu.Unlock()

    switch e := event.(type) {
    case *IterationStart:
        s.state.Status = "working"
        s.state.CurrentBead = e.BeadID
        s.state.Stats.Iterations++

        if _, exists := s.state.History[e.BeadID]; !exists {
            s.state.History[e.BeadID] = &BeadHistory{}
        }
        s.state.History[e.BeadID].Status = "working"
        s.state.History[e.BeadID].Attempts = e.Attempt

    case *SessionStart:
        // Could track session ID here

    case *IterationEnd:
        s.state.CurrentBead = ""
        s.state.CurrentSession = ""
        s.state.Stats.TotalCostUSD += e.TotalCostUSD
        s.state.Stats.TotalTurns += e.NumTurns
        s.state.Stats.TotalDurationMs += int64(e.DurationMs)

        if e.Success {
            s.state.Stats.BeadsCompleted++
            s.state.History[e.BeadID].Status = "completed"
        } else {
            s.state.Stats.BeadsFailed++
            s.state.History[e.BeadID].Status = "failed"
            s.state.History[e.BeadID].LastError = e.Error
        }
        s.state.History[e.BeadID].LastAttempt = event.Timestamp()

        s.state.Status = "idle"

    case *DrainPause:
        s.state.Status = "paused"

    case *DrainResume:
        s.state.Status = "idle"

    case *DrainStop:
        s.state.Status = "stopped"
    }

    // Save after significant events
    s.saveUnlocked()
}

func (s *StateSink) save() {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.saveUnlocked()
}

func (s *StateSink) saveUnlocked() {
    dir := filepath.Dir(s.path)
    if err := os.MkdirAll(dir, 0755); err != nil {
        fmt.Fprintf(os.Stderr, "failed to create state directory: %v\n", err)
        return
    }

    data, err := json.MarshalIndent(s.state, "", "  ")
    if err != nil {
        fmt.Fprintf(os.Stderr, "failed to marshal state: %v\n", err)
        return
    }

    // Write to temp file then rename for atomicity
    tmp := s.path + ".tmp"
    if err := os.WriteFile(tmp, data, 0644); err != nil {
        fmt.Fprintf(os.Stderr, "failed to write state: %v\n", err)
        return
    }

    if err := os.Rename(tmp, s.path); err != nil {
        fmt.Fprintf(os.Stderr, "failed to rename state file: %v\n", err)
    }
}

func (s *StateSink) Stop() error {
    s.save()
    return nil
}
```

### State File Example

`.atari/state.json`:

```json
{
  "version": 1,
  "status": "idle",
  "started_at": "2024-01-15T14:00:00Z",
  "current_bead": "",
  "stats": {
    "iterations": 5,
    "beads_completed": 4,
    "beads_failed": 1,
    "total_cost_usd": 2.35,
    "total_turns": 42,
    "total_duration_ms": 480000
  },
  "history": {
    "bd-040": {"status": "completed", "attempts": 1},
    "bd-041": {"status": "completed", "attempts": 1},
    "bd-042": {"status": "completed", "attempts": 1},
    "bd-043": {"status": "completed", "attempts": 1},
    "bd-039": {"status": "failed", "attempts": 3, "last_error": "tests failing", "last_attempt": "2024-01-15T14:30:00Z"}
  }
}
```

### State Queries

```go
func (s *StateSink) State() *State {
    s.mu.RLock()
    defer s.mu.RUnlock()

    // Return a copy
    copy := *s.state
    return &copy
}

func (s *StateSink) Stats() Stats {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.state.Stats
}

func (s *StateSink) History() map[string]*BeadHistory {
    s.mu.RLock()
    defer s.mu.RUnlock()

    // Return a copy
    copy := make(map[string]*BeadHistory)
    for k, v := range s.state.History {
        h := *v
        copy[k] = &h
    }
    return copy
}
```

### Recovery Logic

On startup, if state file exists:

```go
func (s *StateSink) NeedsRecovery() bool {
    s.mu.RLock()
    defer s.mu.RUnlock()

    // Recovery needed if we were in the middle of something
    return s.state.Status == "working" || s.state.CurrentBead != ""
}

func (s *StateSink) RecoveryInfo() (beadID string, status string) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.state.CurrentBead, s.state.Status
}
```

---

## Console Sink (Optional)

For non-TUI mode, a simple console sink can write formatted events to stdout:

```go
type ConsoleSink struct {
    out    io.Writer
    colors bool
}

func NewConsoleSink(out io.Writer, colors bool) *ConsoleSink {
    return &ConsoleSink{out: out, colors: colors}
}

func (s *ConsoleSink) Start(ctx context.Context, events <-chan Event) error {
    go s.run(ctx, events)
    return nil
}

func (s *ConsoleSink) run(ctx context.Context, events <-chan Event) {
    for {
        select {
        case <-ctx.Done():
            return
        case event, ok := <-events:
            if !ok {
                return
            }
            s.write(event)
        }
    }
}

func (s *ConsoleSink) write(event Event) {
    ts := event.Timestamp().Format("15:04:05")
    formatted := s.format(event)
    if formatted != "" {
        fmt.Fprintf(s.out, "[%s] %s\n", ts, formatted)
    }
}

func (s *ConsoleSink) format(event Event) string {
    switch e := event.(type) {
    case *IterationStart:
        return fmt.Sprintf("BEAD %s %q (priority %d)", e.BeadID, e.Title, e.Priority)
    case *SessionStart:
        return fmt.Sprintf("SESSION %s | max-turns: %d", e.Model, 50) // TODO: from config
    case *ToolUse:
        return e.Format()
    case *BeadStatus:
        return e.Format()
    case *SessionEnd:
        return fmt.Sprintf("SESSION END | turns: %d | cost: $%.2f | duration: %ds",
            e.NumTurns, e.TotalCostUSD, e.DurationMs/1000)
    case *Error:
        return fmt.Sprintf("ERROR: %s", e.Message)
    default:
        return ""
    }
}

func (s *ConsoleSink) Stop() error {
    return nil
}
```

## Testing

### Unit Tests

- LogSink: verify JSON lines written correctly
- StateSink: verify state transitions and persistence
- Recovery: verify state loaded on restart

### Test Cases

```go
func TestLogSinkWrites(t *testing.T) {
    tmp := t.TempDir()
    path := filepath.Join(tmp, "test.log")

    sink := NewLogSink(path)
    events := make(chan Event, 10)

    ctx, cancel := context.WithCancel(context.Background())
    sink.Start(ctx, events)

    events <- &DrainStart{BaseEvent: NewInternalEvent(EventDrainStart)}
    time.Sleep(10 * time.Millisecond)

    cancel()
    sink.Stop()

    data, _ := os.ReadFile(path)
    if !strings.Contains(string(data), "drain.start") {
        t.Error("expected drain.start in log")
    }
}

func TestStateSinkPersistence(t *testing.T) {
    tmp := t.TempDir()
    path := filepath.Join(tmp, "state.json")

    // First run
    sink1 := NewStateSink(path)
    events1 := make(chan Event, 10)
    ctx1, cancel1 := context.WithCancel(context.Background())
    sink1.Start(ctx1, events1)

    events1 <- &IterationEnd{
        BaseEvent:    NewInternalEvent(EventIterationEnd),
        BeadID:       "bd-001",
        Success:      true,
        TotalCostUSD: 0.50,
    }
    time.Sleep(10 * time.Millisecond)
    cancel1()
    sink1.Stop()

    // Second run - should load state
    sink2 := NewStateSink(path)
    sink2.Load()

    stats := sink2.Stats()
    if stats.BeadsCompleted != 1 {
        t.Errorf("expected 1 completed, got %d", stats.BeadsCompleted)
    }
}
```

## Error Handling

| Error | Action |
|-------|--------|
| Log file write fails | Log to stderr, continue |
| State file corrupt | Start fresh, log warning |
| State directory missing | Create it |
| Atomic rename fails | Log error, state may be stale |

## Future Considerations

- **Log compression**: Gzip old logs automatically
- **State backup**: Keep last N state files
- **Remote sink**: Send events to external service
- **Metrics sink**: Export Prometheus metrics
