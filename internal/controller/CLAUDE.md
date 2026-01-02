# Controller Package

The central orchestration component that coordinates the drain loop.

## Purpose

The controller manages:
- State machine: idle, working, paused, stopping, stopped
- Work queue polling and bead selection
- Claude session execution
- Graceful shutdown with WaitGroup

## Key Types

```go
type Controller struct { ... }
type State string  // idle, working, paused, stopping, stopped
type Stats struct { ... }
type SessionResult struct { ... }
```

## Public API

```go
func New(cfg *config.Config, wq *workqueue.Manager, router *events.Router, cmdRunner testutil.CommandRunner, processRunner runner.ProcessRunner, logger *slog.Logger) *Controller
func (c *Controller) Run(ctx context.Context) error  // Main drain loop
func (c *Controller) Stop()     // Request graceful shutdown
func (c *Controller) Pause()    // Pause after current iteration
func (c *Controller) Resume()   // Resume from paused state
func (c *Controller) State() State
func (c *Controller) Stats() Stats
func (c *Controller) CurrentBead() string  // Currently active bead ID
```

## State Machine

```
idle -> working (bead available)
idle -> stopped (stop requested)
working -> idle (session completed)
working -> paused (pause requested)
working -> stopping (stop requested)
paused -> idle (resume, bead available)
paused -> stopped (stop requested)
stopping -> stopped (session ends)
```

## Events Emitted

- `DrainStartEvent` - when Run() begins
- `DrainStopEvent` - when shutdown completes
- `IterationStartEvent` - before processing a bead
- `IterationEndEvent` - after bead processing (success or failure)
- `SessionStartEvent` - when Claude session starts
- `BeadAbandonedEvent` - when bead exceeds max failures
- `ErrorEvent` - on work queue or session errors

## Agent State Reporting

The controller reports its state to beads via `bd agent state atari <state>` on each state transition:

| Controller State | Agent State |
|-----------------|-------------|
| idle | idle |
| working | running |
| paused | idle |
| stopping | stopped |
| stopped | dead |

Agent state reporting is best-effort: errors are logged but do not affect controller operation.

## BD Activity Watcher Integration

When `config.BDActivity.Enabled` is true and a `processRunner` is provided, the controller creates and manages a BD Activity Watcher:

- **Startup**: Watcher starts in `Run()` before the main drain loop (best-effort, non-fatal)
- **Events**: BD activity events are emitted to the shared event router
- **Shutdown**: Watcher stops during `shutdown()` before emitting DrainStopEvent

The watcher is optional: if disabled or if start fails, the controller continues operating normally. This allows the drain loop to function even without real-time BD activity monitoring.

## Dependencies

- `config.Config` - configuration values
- `workqueue.Manager` - work discovery and selection
- `events.Router` - event publication
- `session.Manager` - Claude process lifecycle
- `session.Parser` - stream-json parsing
- `testutil.CommandRunner` - command execution (for agent state reporting)
- `runner.ProcessRunner` - streaming process execution (optional, for BD activity)
- `bdactivity.Watcher` - BD activity stream monitoring (optional)

## Testing

Tests use mock command runner and short intervals:

```go
cfg := testConfig()  // 10ms poll interval
runner := testutil.NewMockRunner()
runner.SetResponse("bd", []string{"ready", "--json"}, []byte("[]"))
wq := workqueue.New(cfg, runner)
c := New(cfg, wq, router, runner, nil, nil)  // nil processRunner, nil logger
```

Pass `nil` for processRunner to disable BD activity watching in tests.
