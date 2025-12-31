# Integration Package

End-to-end tests for the atari drain loop. These tests exercise the full controller with mocked external commands.

## Test Environment

The `testEnv` struct provides isolated test environments:

```go
env := newTestEnv(t)
defer env.cleanup()

// Components available:
env.cfg       // Fast test config (10ms poll, 5s timeout)
env.runner    // MockRunner with agent state mocks pre-configured
env.router    // Event router with subscriber
env.tempDir   // Temporary directory with mock claude script
```

## Mock Claude Script

Integration tests use a mock `claude` bash script that:
- Reads prompt from stdin
- Outputs stream-json events (system init, assistant, tool_use, tool_result, result)
- Exits with configurable success/failure

Two versions:
- `createMockClaude()` - Successful session with bd close
- `createFailingMockClaude()` - Session that exits with error

The mock is prepended to PATH so the session manager finds it.

## Test Configuration

Fast intervals for quick tests:

```go
cfg.WorkQueue.PollInterval = 10 * time.Millisecond  // vs 5s production
cfg.Claude.Timeout = 5 * time.Second                // vs 5m production
cfg.Backoff.Initial = 10 * time.Millisecond         // vs 1m production
cfg.Backoff.Max = 50 * time.Millisecond             // vs 1h production
cfg.Backoff.MaxFailures = 3                         // vs 5 production
```

## Event Collection

Tests collect and assert on emitted events:

```go
// Wait for events after controller stops
env.collectEvents(100 * time.Millisecond)

// Find specific event types
if evt := env.findEvent(events.EventDrainStart); evt == nil {
    t.Error("expected DrainStartEvent")
}

// Count events
iterCount := env.countEvents(events.EventIterationStart)
```

## Dynamic Responses

For tests needing variable mock responses:

```go
callCount := 0
env.runner.DynamicResponse = func(ctx context.Context, name string, args []string) ([]byte, error, bool) {
    if name == "bd" && args[0] == "ready" {
        callCount++
        if callCount > 1 {
            return []byte("[]"), nil, true  // Empty after first call
        }
        return beadJSON, nil, true
    }
    return nil, nil, false  // Fall through to static responses
}
```

## Test Cases

| Test | Validates |
|------|-----------|
| `TestFullDrainCycle` | Complete cycle: start, process bead, stop |
| `TestDrainWithMultipleBeads` | Processing multiple beads in priority order |
| `TestDrainWithFailedBead` | Failure recording in history |
| `TestGracefulShutdown` | Clean stop within timeout |
| `TestBackoffProgression` | Exponential backoff and bead abandonment |
| `TestContextCancellation` | Context-based shutdown |
| `TestPauseResumeDuringDrain` | Pause/resume from idle state |

## Patterns

1. **Controller lifecycle**: Start in goroutine, sleep, stop, wait on done channel
2. **Timeout protection**: Use `context.WithTimeout` and select with `time.After`
3. **State assertions**: Check `ctrl.State()` after operations
4. **History verification**: Use `wq.History()` to verify bead tracking

## Running Tests

```bash
go test ./internal/integration/...

# With verbose output
go test -v ./internal/integration/...

# Run specific test
go test -v ./internal/integration/... -run TestFullDrainCycle
```
