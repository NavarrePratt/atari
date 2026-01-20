# BD Activity Package

Watches `.beads/issues.jsonl` for changes and emits events to the unified event stream.

## Components

### Watcher

Main component that monitors the JSONL file for bead state changes using fsnotify.

```go
watcher := bdactivity.New(&cfg.BDActivity, router, nil, logger)

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
- Watches parent directory to detect file creation
- Debounces rapid file changes (100ms)
- Compares old and new state to emit diff-based events
- Handles file truncation gracefully (br sync rewrites)
- No dependency on bd binary

### Parser

Converts JSONL lines to BeadState for diff comparison.

```go
// Parse a single line from JSONL file
state, err := bdactivity.ParseJSONLLine(line)
if err != nil {
    // Invalid JSON
}
if state == nil {
    // Empty line or missing ID
}
```

**Legacy ParseLine** is kept for backward compatibility with bd activity stream format.

## Event Types

### BeadChangedEvent (new)

Emitted when a bead's state changes in the JSONL file:

```go
type BeadChangedEvent struct {
    BaseEvent
    BeadID   string     // The bead ID
    OldState *BeadState // nil if bead was created
    NewState *BeadState // nil if bead was deleted
}
```

### Legacy Events (kept for backward compatibility)

- BeadCreatedEvent
- BeadStatusEvent
- BeadClosedEvent
- BeadUpdatedEvent
- BeadCommentEvent

## Configuration

From `config.BDActivityConfig`:

| Field | Default | Description |
|-------|---------|-------------|
| Enabled | true | Whether to run the watcher |
| ReconnectDelay | 5s | (Legacy, unused in file watcher) |
| MaxReconnectDelay | 5m | (Legacy, unused in file watcher) |

## Dependencies

| Component | Usage |
|-----------|-------|
| fsnotify | File system change notifications |
| events.Router | Event emission target |
| config.BDActivityConfig | Configuration settings |

## Event Flow

```
.beads/issues.jsonl
    |
    v (fsnotify)
[Watcher.runLoop()]
    |
    v (debounce)
[loadAndDiff()]
    |
    v
[parseJSONLFile()] -> map[id]*BeadState
    |
    v (compare old vs new)
[Router.Emit(BeadChangedEvent)]
```

## Edge Cases

| Scenario | Behavior |
|----------|----------|
| File doesn't exist | Watches directory, detects file creation |
| File truncated | Re-reads entire file, emits delete events for removed beads |
| Rapid changes | Debounced to 100ms before processing |
| Parse errors | Logs at debug level, skips invalid lines |
| Context cancel | Clean shutdown |

## Testing

Use `NewWithPath` to test with a custom JSONL path:

```go
watcher := bdactivity.NewWithPath(cfg, router, logger, "/tmp/test/issues.jsonl")
err := watcher.Start(ctx)
```

## Files

| File | Description |
|------|-------------|
| watcher.go | File watcher with fsnotify, diff-based event emission |
| parser.go | JSONL parsing and legacy bd activity format support |
| watcher_test.go | Watcher unit tests with file operations |
| parser_test.go | Parser unit tests |
| CLAUDE.md | This documentation |
