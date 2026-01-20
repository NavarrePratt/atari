# Controller

The central orchestration component that coordinates all other components and manages the main execution loop.

## Purpose

The Controller is responsible for:
- Managing the overall daemon state (idle, working, paused, stopping)
- Coordinating the Work Queue and Session Manager
- Handling control signals (pause, resume, stop)
- Orchestrating graceful shutdown

## Interface

```go
type Controller struct {
    state       State
    workQueue   *workqueue.Manager
    session     *session.Manager
    events      *events.Router
    config      *config.Config
}

// Public API
func New(cfg *config.Config) *Controller
func (c *Controller) Run(ctx context.Context) error
func (c *Controller) Pause() error
func (c *Controller) Resume() error
func (c *Controller) Stop() error
func (c *Controller) State() State
func (c *Controller) Stats() Stats
```

## Dependencies

| Component | Usage |
|-----------|-------|
| workqueue.Manager | Get next bead to work on |
| session.Manager | Spawn and manage Claude sessions |
| events.Router | Emit internal events |
| config.Config | Read configuration values |

## State Machine

```
                    +----------+
                    |   init   |
                    +----+-----+
                         | start
                         v
              +---------------------+
              |                     |
    +-------->|        idle         |<--------+
    |         |   (no ready beads)  |         |
    |         +----------+----------+         |
    |                    | bead available     |
    |                    v                    |
    |         +---------------------+         |
    |         |                     |         |
    |         |       working       |---------+ bead completed
    |         |  (claude session)   |         | (no more beads)
    |         +----------+----------+         |
    |                    |                    |
    |      +-------------+-------------+      |
    |      | pause       | stop        |      |
    |      v             v             |      |
    | +---------+  +-----------+       |      |
    | | paused  |  | stopping  |       |      |
    | +----+----+  +-----+-----+       |      |
    |      | resume      | session    |      |
    |      |             | ends       |      |
    +------+             v            |      |
                  +-----------+       |      |
                  |  stopped  |<------+      |
                  +-----------+              |
                         ^                   |
                         +-------------------+
                           stop (when idle)
```

### States

| State | Description |
|-------|-------------|
| `idle` | No work available, polling for beads |
| `working` | Claude session active on a bead |
| `paused` | User requested pause, waiting for current bead to complete |
| `stopping` | Graceful shutdown in progress, waiting for session |
| `stopped` | Daemon has stopped |

### Transitions

| From | To | Trigger |
|------|-----|---------|
| idle | working | Bead available from work queue |
| idle | stopped | Stop command received |
| working | idle | Session completed, more beads may exist |
| working | paused | Pause command received |
| working | stopping | Stop command received |
| paused | working | Resume command received (if bead available) |
| paused | idle | Resume command received (no beads) |
| paused | stopped | Stop command received |
| stopping | stopped | Current session ends |

## Implementation

### Main Loop

```go
func (c *Controller) Run(ctx context.Context) error {
    c.events.Emit(events.DrainStart{})
    slog.Info("controller started", "state", "idle")

    for {
        select {
        case <-ctx.Done():
            return c.shutdown()
        default:
        }

        switch c.state {
        case StateIdle:
            c.runIdle(ctx)
        case StateWorking:
            c.runWorking(ctx)
        case StatePaused:
            c.runPaused(ctx)
        case StateStopping:
            c.runStopping(ctx)
        case StateStopped:
            slog.Info("controller stopped", "state", "stopped")
            return nil
        }
    }
}
```

### Idle State Handler

```go
func (c *Controller) runIdle(ctx context.Context) {
    bead, err := c.workQueue.Next(ctx)
    if err != nil {
        c.events.Emit(events.Error{Err: err})
        time.Sleep(c.config.PollInterval)
        return
    }

    if bead == nil {
        // No work available, wait and retry
        time.Sleep(c.config.PollInterval)
        return
    }

    c.currentBead = bead
    c.state = StateWorking
    slog.Info("starting session", "bead", bead.ID)
    c.events.Emit(events.IterationStart{Bead: bead})
}
```

### Working State Handler

```go
func (c *Controller) runWorking(ctx context.Context) {
    slog.Info("session active", "bead", c.currentBead.ID)

    result, err := c.session.Run(ctx, c.currentBead)

    if err != nil {
        c.workQueue.RecordFailure(c.currentBead.ID, err)
        c.events.Emit(events.Error{Err: err, BeadID: c.currentBead.ID})
    } else {
        c.workQueue.RecordSuccess(c.currentBead.ID)
    }

    c.events.Emit(events.IterationEnd{
        Bead:   c.currentBead,
        Result: result,
    })

    // Reset any stuck in_progress issues
    c.resetStuckIssues()

    c.currentBead = nil

    // Transition based on pending commands
    switch {
    case c.pendingStop:
        c.state = StateStopped
        slog.Info("controller stopped")
    case c.pendingPause:
        c.state = StatePaused
        slog.Info("controller paused")
    default:
        c.state = StateIdle
        slog.Info("session complete, returning to idle")
    }
}
```

### Signal Handling

```go
func (c *Controller) setupSignals(ctx context.Context) context.Context {
    ctx, cancel := context.WithCancel(ctx)

    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

    go func() {
        <-sigCh
        c.events.Emit(events.DrainStop{Reason: "signal"})
        c.Stop()
        cancel()
    }()

    return ctx
}
```

### Stuck Issue Reset

After each session ends, reset any beads left in `in_progress` status:

```go
func (c *Controller) resetStuckIssues() error {
    // List all in_progress issues
    cmd := exec.Command("br", "list", "--status=in_progress", "--json")
    output, err := cmd.Output()
    if err != nil {
        return err
    }

    var issues []struct{ ID string }
    if err := json.Unmarshal(output, &issues); err != nil {
        return err
    }

    for _, issue := range issues {
        resetCmd := exec.Command("br", "update", issue.ID,
            "--status=open",
            "--priority=0",
            "--notes", "RESET: Previous session ended without closing this issue.")
        if err := resetCmd.Run(); err != nil {
            c.events.Emit(events.Error{
                Err:    fmt.Errorf("failed to reset %s: %w", issue.ID, err),
                BeadID: issue.ID,
            })
        }
    }

    return nil
}
```

## Testing

### Unit Tests

- State transitions: verify all valid transitions work
- Invalid transitions: verify invalid transitions return errors
- Signal handling: verify SIGINT/SIGTERM trigger shutdown

### Integration Tests

- Full cycle: idle -> working -> idle with mock session manager
- Pause during work: verify session completes before pause takes effect
- Stop during work: verify graceful vs immediate stop behavior

### Test Fixtures

```go
// Mock work queue that returns controlled sequence of beads
type mockWorkQueue struct {
    beads []Bead
    index int
}

// Mock session manager that simulates Claude sessions
type mockSessionManager struct {
    results []SessionResult
    delay   time.Duration
}
```

## Error Handling

| Error | Action |
|-------|--------|
| Work queue poll fails | Log error, retry with backoff |
| Session spawn fails | Record failure, continue to next bead |
| Reset stuck issues fails | Log warning, continue |
| State file write fails | Log error, continue (best effort) |

## Future Considerations

- **Parallel workers**: Multiple controllers coordinating via shared state
- **Priority preemption**: Interrupt low-priority work for high-priority beads
- **Health checks**: Periodic self-checks with automatic recovery
