# BD Activity Package

Watches `bd activity --follow --json` and emits events to the unified event stream.

## Components

### Watcher

Main component that manages the bd activity process lifecycle.

```go
watcher := bdactivity.New(&cfg.BDActivity, router, runner, logger)

// Start watching (non-blocking)
if err := watcher.Start(ctx); err != nil {
    // Handle error
}

// Check status
if watcher.Running() {
    // Watcher is active
}

// Stop watching
if err := watcher.Stop(); err != nil {
    // Handle error
}
```

**Key behaviors:**
- Non-blocking Start: launches background goroutine
- Automatic reconnection with exponential backoff
- Backoff resets after receiving successful events
- Rate-limited parse warnings (one per 5 seconds)
- Drains stderr to prevent process blocking

### Parser

Converts bd activity JSON lines to typed events.

```go
// Parse a single line
event, err := bdactivity.ParseLine(line)
if err != nil {
    // Invalid JSON
}
if event == nil {
    // Unknown mutation type (silently skipped)
}
```

**Supported mutation types:**
- `create` -> BeadCreatedEvent
- `status` -> BeadStatusEvent or BeadClosedEvent (if closed/completed)
- `update` -> BeadUpdatedEvent
- `comment` -> BeadCommentEvent

**Skipped types** (silently ignored):
- `bonded`, `squashed`, `burned`, `delete`, and any unknown types

## Configuration

From `config.BDActivityConfig`:

| Field | Default | Description |
|-------|---------|-------------|
| Enabled | true | Whether to run the watcher |
| ReconnectDelay | 5s | Initial backoff between reconnects |
| MaxReconnectDelay | 5m | Maximum backoff cap |

## Dependencies

| Component | Usage |
|-----------|-------|
| runner.ProcessRunner | Subprocess execution abstraction |
| events.Router | Event emission target |
| config.BDActivityConfig | Configuration settings |

## Event Flow

```
bd activity --follow --json
    |
    v
[Watcher.watch()]
    |
    v
[ParseLine()] -> event or nil
    |
    v
[Router.Emit()] -> Log sink, State sink, etc.
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Start failure | Emits warning, continues with backoff |
| Process exit | Emits warning, reconnects with backoff |
| Parse error | Emits rate-limited warning, skips line |
| Unknown mutation | Silently skipped (no event, no error) |
| Context cancel | Clean shutdown, no reconnect |

## Testing

Use `testutil.MockProcessRunner` for testing:

```go
mock := testutil.NewMockProcessRunner()
mock.SetOutput(`{"type":"create","issue_id":"bd-001",...}\n`)

watcher := bdactivity.New(cfg, router, mock, logger)
err := watcher.Start(ctx)
```

## Files

| File | Description |
|------|-------------|
| watcher.go | Main Watcher component with lifecycle management |
| parser.go | JSON parsing and event type mapping |
| parser_test.go | Parser unit tests |
| CLAUDE.md | This documentation |
