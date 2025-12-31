# Beads Integration Guide

How atari integrates with the beads (bd) issue tracking system.

## Overview

Atari integrates with beads through three mechanisms:

1. **Work Discovery**: `bd ready --json` to find available work
2. **Activity Streaming**: `bd activity --follow --json` for real-time events
3. **Agent State Tracking**: `bd agent state atari <state>` to report status

## Integration Philosophy

Atari uses the **CLI interface** rather than importing beads packages directly. This provides:

- **Stability**: CLI is the public, stable API
- **Decoupling**: No dependency on beads internal implementation
- **Simplicity**: No need to track beads version compatibility

Do NOT import from `github.com/steveyegge/beads/internal/*` - these are private implementation details.

## Work Discovery

### How bd ready Handles Dependencies

`bd ready` returns **only unblocked issues** - issues with no unsatisfied blocking dependencies. This is critical for atari's design:

- Dependencies are set via `bd dep add A B --type blocks` (or the `bd-sequence` skill)
- `bd ready` automatically excludes issues blocked by unclosed dependencies
- When a blocking issue closes, blocked issues automatically appear in `bd ready`

**Atari does NOT need to understand the dependency graph.** It simply polls `bd ready` and works on whatever is returned, trusting beads to handle the sequencing.

### Polling bd ready

```go
func (m *Manager) poll(ctx context.Context) ([]Bead, error) {
    args := []string{"ready", "--json"}
    if m.config.Label != "" {
        args = append(args, "--label", m.config.Label)
    }

    cmd := exec.CommandContext(ctx, "bd", args...)
    output, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("bd ready failed: %w", err)
    }

    var beads []Bead
    if err := json.Unmarshal(output, &beads); err != nil {
        return nil, fmt.Errorf("parse bd ready output: %w", err)
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

## Activity Streaming

### Spawning bd activity

```go
func (w *Watcher) Start(ctx context.Context) error {
    cmd := exec.CommandContext(ctx, "bd", "activity", "--follow", "--json")
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return err
    }

    if err := cmd.Start(); err != nil {
        return err
    }

    go w.parseEvents(stdout)
    return nil
}
```

### MutationEvent Format

Events from `bd activity --follow --json`:

```json
{
  "type": "update",
  "issue_id": "bd-042",
  "title": "Fix authentication bug",
  "assignee": "",
  "actor": "claude",
  "timestamp": "2024-01-15T14:23:10Z",
  "old_status": "open",
  "new_status": "in_progress"
}
```

Event types: `create`, `update`, `delete`, `comment`, `step_add`, `step_remove`

## Agent State Tracking

Beads tracks agents (automated workers) and their states. Atari registers itself as an agent named "atari".

### Agent States

Beads defines these standard agent states:

| State | Meaning |
|-------|---------|
| `idle` | Agent is running but not actively working |
| `spawning` | Agent is starting a work session |
| `running` | Agent has an active work session |
| `working` | Agent is actively processing (alias for running) |
| `stuck` | Agent encountered a problem |
| `done` | Agent completed its work |
| `stopped` | Agent has been stopped |
| `dead` | Agent process has terminated |

### Reporting State

```go
func (c *Controller) reportAgentState(state string) error {
    cmd := exec.Command("bd", "agent", "state", "atari", state)
    return cmd.Run()
}
```

### State Transitions

Report state on every significant transition:

```
startup        -> bd agent state atari idle
found work     -> bd agent state atari spawning
session active -> bd agent state atari running
session ends   -> bd agent state atari idle
pause          -> bd agent state atari idle
stop           -> bd agent state atari stopped
```

### Why This Matters

1. **Visibility**: `bd agent list` shows all active agents
2. **Debugging**: If atari gets stuck, beads knows its last reported state
3. **Coordination**: Future multi-agent scenarios can use this for coordination
4. **Monitoring**: External tools can query agent status

## Issue Manipulation

### Updating Issue Status

```go
func updateIssueStatus(id, status string) error {
    cmd := exec.Command("bd", "update", id, "--status", status, "--json")
    return cmd.Run()
}
```

### Closing Issues

```go
func closeIssue(id, reason string) error {
    cmd := exec.Command("bd", "close", id, "--reason", reason, "--json")
    return cmd.Run()
}
```

### Adding Notes

```go
func addNote(id, note string) error {
    cmd := exec.Command("bd", "update", id, "--notes", note, "--json")
    return cmd.Run()
}
```

## Error Handling

| Scenario | Action |
|----------|--------|
| bd command not found | Fatal error - bd required |
| bd daemon not running | Attempt `bd daemon` or fatal |
| bd ready returns empty | Not an error - no work available |
| bd activity disconnects | Reconnect with backoff |
| Agent state update fails | Log warning, continue (best effort) |

## Best Practices

1. **Always use --json flag** for machine-parseable output
2. **Handle empty arrays** gracefully (no work != error)
3. **Report agent state** on every transition
4. **Reconnect activity stream** automatically on disconnect
5. **Use context timeouts** for all bd commands
6. **Log bd command failures** with full error output

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

## Version Compatibility

Atari should work with any beads version that supports:

- `bd ready --json`
- `bd activity --follow --json`
- `bd agent state <name> <state>`

These commands are part of beads' stable CLI interface.
