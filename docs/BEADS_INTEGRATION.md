# Beads Integration Guide

How atari integrates with the beads_rust (br) issue tracking system.

## Overview

Atari integrates with beads through two mechanisms:

1. **Work Discovery**: `br ready --json` to find available work
2. **Activity Monitoring**: File watcher on `.beads/issues.jsonl` for real-time events

## Prerequisites

Atari requires the `br` binary to be installed and available in PATH. At startup, atari verifies br is available:

```bash
br --version
```

If br is not found, atari will exit with an error message explaining how to install it.

## Integration Philosophy

Atari uses the **CLI interface** for work discovery rather than importing beads packages directly. This provides:

- **Stability**: CLI is the public, stable API
- **Decoupling**: No dependency on beads internal implementation
- **Simplicity**: No need to track beads version compatibility

For activity monitoring, atari watches the `.beads/issues.jsonl` file directly rather than spawning a process. This approach:

- **Eliminates process management**: No need to spawn/monitor `br activity`
- **Handles file operations gracefully**: Detects file truncation from `br sync`
- **Reduces dependencies**: Only requires fsnotify, not the br binary

## Work Discovery

### How br ready Handles Dependencies

`br ready` returns **only unblocked issues** - issues with no unsatisfied blocking dependencies. This is critical for atari's design:

- Dependencies are set via `br dep add A B --type blocks`
- `br ready` automatically excludes issues blocked by unclosed dependencies
- When a blocking issue closes, blocked issues automatically appear in `br ready`

**Atari does NOT need to understand the dependency graph.** It simply polls `br ready` and works on whatever is returned, trusting beads to handle the sequencing.

### Polling br ready

```go
func (m *Manager) poll(ctx context.Context) ([]Bead, error) {
    args := []string{"ready", "--json"}
    if m.config.Label != "" {
        args = append(args, "--label", m.config.Label)
    }

    cmd := exec.CommandContext(ctx, "br", args...)
    output, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("br ready failed: %w", err)
    }

    var beads []Bead
    if err := json.Unmarshal(output, &beads); err != nil {
        return nil, fmt.Errorf("parse br ready output: %w", err)
    }
    return beads, nil
}
```

### Expected JSON Format

```json
[
  {
    "id": "bd-042",
    "title": "Fix authentication bug",
    "priority": 1,
    "labels": ["bug", "auth"],
    "created_at": "2024-01-15T10:00:00Z",
    "description": "Users are getting logged out unexpectedly..."
  }
]
```

## Activity Monitoring

### File Watcher Approach

Atari monitors bead activity by watching the `.beads/issues.jsonl` file using fsnotify. This is more reliable than spawning a streaming process:

```go
watcher := bdactivity.New(&cfg.BDActivity, router, nil, logger)

// Start watching (non-blocking)
if err := watcher.Start(ctx); err != nil {
    // Handle error
}
```

**Key behaviors:**
- Watches the parent directory to detect file creation
- Debounces rapid file changes (100ms)
- Compares old and new state to emit diff-based events
- Handles file truncation gracefully (br sync rewrites the file)
- No dependency on br binary for monitoring

### Event Types

When bead state changes, the watcher emits `BeadChangedEvent`:

```go
type BeadChangedEvent struct {
    BaseEvent
    BeadID   string     // The bead ID
    OldState *BeadState // nil if bead was created
    NewState *BeadState // nil if bead was deleted
}
```

The watcher detects:
- **Creates**: New bead appears in file
- **Updates**: Status, priority, or other fields change
- **Deletes**: Bead removed from file

### JSONL Format

The `.beads/issues.jsonl` file contains one JSON object per line:

```json
{"id":"bd-042","title":"Fix auth bug","status":"open","priority":1,...}
{"id":"bd-043","title":"Add rate limiting","status":"in_progress",...}
```

## Issue Manipulation

### Updating Issue Status

```go
func updateIssueStatus(id, status string) error {
    cmd := exec.Command("br", "update", id, "--status", status, "--json")
    return cmd.Run()
}
```

### Closing Issues

```go
func closeIssue(id, reason string) error {
    cmd := exec.Command("br", "close", id, "--reason", reason, "--json")
    return cmd.Run()
}
```

### Adding Notes

```go
func addNote(id, note string) error {
    cmd := exec.Command("br", "update", id, "--notes", note, "--json")
    return cmd.Run()
}
```

## Epic Auto-Closure

Atari implements automatic epic closure. When a bead is successfully processed:

1. Check if the bead has a parent epic
2. Check if all sibling beads in the epic are closed
3. If all children are closed, automatically close the epic

This eliminates the need to manually close epics after their work is complete.

## Error Handling

| Scenario | Action |
|----------|--------|
| br command not found | Fatal error at startup |
| br ready returns empty | Not an error - no work available |
| File watcher fails to start | Log warning, continue without activity monitoring |
| JSONL parse error | Log at debug level, skip invalid lines |

## Best Practices

1. **Always use --json flag** for machine-parseable output
2. **Handle empty arrays** gracefully (no work != error)
3. **Use context timeouts** for all br commands
4. **Log br command failures** with full error output

## Testing Without Beads

For unit tests, mock the exec.Command calls:

```go
type CommandRunner interface {
    Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type mockRunner struct {
    responses map[string][]byte
}

func (m *mockRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
    key := name + " " + strings.Join(args, " ")
    if resp, ok := m.responses[key]; ok {
        return resp, nil
    }
    return nil, fmt.Errorf("unexpected command: %s", key)
}
```

For the file watcher, use `NewWithPath` to test with a custom JSONL path:

```go
watcher := bdactivity.NewWithPath(cfg, router, logger, "/tmp/test/issues.jsonl")
```

## Version Compatibility

Atari should work with any beads_rust version that supports:

- `br ready --json`
- `br close --reason`
- `br update --status`
- `.beads/issues.jsonl` file format
