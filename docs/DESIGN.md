# Design Document

This document describes the architecture and design decisions for bd-drain.

## Table of Contents

1. [Goals](#goals)
2. [Non-Goals](#non-goals)
3. [Architecture](#architecture)
4. [Components](#components)
5. [Event System](#event-system)
6. [State Management](#state-management)
7. [CLI Interface](#cli-interface)
8. [Configuration](#configuration)
9. [Error Handling](#error-handling)

---

## Goals

1. **Autonomous execution**: Run Claude Code sessions to completion without human intervention
2. **Observability**: Unified real-time stream of Claude and bd events
3. **Resilience**: Survive crashes/interrupts and resume from last known state
4. **Controllability**: Pause, resume, stop via CLI commands
5. **Simplicity**: Single daemon, single Claude worker, clear state model

## Non-Goals

1. **Parallel execution**: Multiple Claude sessions working simultaneously (future work)
2. **Distributed operation**: Running across multiple machines
3. **Web UI**: Browser-based dashboard (terminal TUI only)
4. **Cost limits**: Hard spending caps (monitoring only)
5. **Session chaining**: Resuming Claude sessions across beads (each bead gets fresh session)

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              bd-drain daemon                                 │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │                           Controller                                     ││
│  │                                                                          ││
│  │  State: idle | working | paused | stopping                              ││
│  │  Current bead: bd-xxx or nil                                            ││
│  │                                                                          ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│         │                    │                    │                         │
│         ▼                    ▼                    ▼                         │
│  ┌─────────────┐     ┌─────────────┐     ┌─────────────┐                   │
│  │ Work Queue  │     │  Session    │     │   Event     │                   │
│  │   Manager   │     │  Manager    │     │   Router    │                   │
│  │             │     │             │     │             │                   │
│  │ bd ready    │     │ claude -p   │     │ Merge:      │                   │
│  │ polling     │     │ lifecycle   │     │ - claude    │                   │
│  │ backoff     │     │ output      │     │ - bd        │                   │
│  │ history     │     │ parsing     │     │ - internal  │                   │
│  └─────────────┘     └─────────────┘     └─────────────┘                   │
│         │                    │                    │                         │
│         └────────────────────┴────────────────────┘                         │
│                              │                                               │
│                              ▼                                               │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │                        Event Bus (channels)                             ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│         │                    │                    │                         │
│         ▼                    ▼                    ▼                         │
│  ┌─────────────┐     ┌─────────────┐     ┌─────────────┐                   │
│  │  Log Sink   │     │  TUI Sink   │     │ State Sink  │                   │
│  │             │     │ (optional)  │     │             │                   │
│  │ JSON lines  │     │ Rich term   │     │ Persist to  │                   │
│  │ to file     │     │ display     │     │ state.json  │                   │
│  └─────────────┘     └─────────────┘     └─────────────┘                   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           External Processes                                 │
│                                                                              │
│  ┌─────────────┐     ┌─────────────┐     ┌─────────────┐                   │
│  │   claude    │     │ bd activity │     │ bd daemon   │                   │
│  │   -p ...    │     │  --follow   │     │ (if active) │                   │
│  └─────────────┘     └─────────────┘     └─────────────┘                   │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Components

### Controller

The main orchestration loop.

**States:**
- `idle` - No work available, waiting for beads
- `working` - Claude session active on a bead
- `paused` - User requested pause, waiting for current bead to complete
- `stopping` - Graceful shutdown in progress

**Responsibilities:**
- State machine transitions
- Coordinating work queue and session manager
- Handling control signals (pause, resume, stop)
- Graceful shutdown

### Work Queue Manager

Discovers and tracks available work.

**Responsibilities:**
- Poll `bd ready --json` at configurable interval
- Track bead history (attempted, completed, failed)
- Implement exponential backoff for repeatedly-failing beads
- Select next bead to work on (highest priority, not recently failed)

**Data structures:**
```go
type BeadHistory struct {
    ID           string
    Attempts     int
    LastAttempt  time.Time
    LastError    string
    Status       string  // "pending", "working", "completed", "failed"
}

type WorkQueue struct {
    Ready    []Bead        // From bd ready
    History  map[string]BeadHistory
    Backoff  time.Duration // Current backoff for failed beads
}
```

### Session Manager

Manages Claude Code process lifecycle.

**Responsibilities:**
- Spawn `claude -p` with appropriate flags
- Stream and parse output (stream-json format)
- Track session metadata (ID, start time, turns, cost)
- Handle process termination (normal, error, timeout)
- Reset stuck in_progress issues after session ends

**Claude invocation:**
```bash
claude \
  --print \
  --model opus \
  --output-format stream-json \
  --max-turns 50 \
  --allowedTools "Bash,Read,Edit,Write,Glob,Grep,Task,Skill,TodoWrite" \
  "$PROMPT"
```

**Prompt template:**
```
Run "bd ready --json" to find available work. Work on the highest-priority
ready issue. Use the bd-issue-tracking skill for workflow guidance.

Requirements:
- Implement the issue completely, including tests
- Use /commit for atomic commits
- Close the issue with bd close when done
- If you discover bugs during implementation, create new bd issues

Do NOT:
- Work on multiple issues in one session
- Leave issues in_progress without closing
- Skip verification steps
```

### Event Router

Merges multiple event sources into unified stream.

**Event sources:**
1. Claude stream-json output
2. bd activity --follow output
3. Internal daemon events (state changes, errors)

**Event types:**
```go
type EventType string

const (
    // Claude events
    EventSessionStart    EventType = "session.start"
    EventSessionEnd      EventType = "session.end"
    EventToolUse         EventType = "tool.use"
    EventToolResult      EventType = "tool.result"
    EventThinking        EventType = "thinking"
    EventText            EventType = "text"
    EventCompact         EventType = "compact"

    // BD events
    EventBeadCreated     EventType = "bead.created"
    EventBeadUpdated     EventType = "bead.updated"
    EventBeadClosed      EventType = "bead.closed"
    EventBeadStatus      EventType = "bead.status"

    // Internal events
    EventDrainStart      EventType = "drain.start"
    EventDrainPause      EventType = "drain.pause"
    EventDrainResume     EventType = "drain.resume"
    EventDrainStop       EventType = "drain.stop"
    EventIterationStart  EventType = "iteration.start"
    EventIterationEnd    EventType = "iteration.end"
    EventError           EventType = "error"
)

type Event struct {
    Type      EventType
    Timestamp time.Time
    Source    string     // "claude", "bd", "drain"
    Data      any        // Type-specific payload
}
```

### Event Sinks

Consumers of the unified event stream.

**Log Sink:**
- Writes JSON lines to file
- Rotates on startup (keep previous as .bak)
- Used for post-hoc analysis and debugging

**TUI Sink (optional):**
- Rich terminal display using bubbletea or similar
- Shows: current bead, recent events, stats
- Keyboard controls: p=pause, r=resume, q=quit

**State Sink:**
- Persists critical state on every significant event
- Enables resume after crash

---

## Event System

### Claude Stream Events

Parsed from `--output-format stream-json`:

```go
// System init
type ClaudeInit struct {
    Model      string
    Tools      []string
    MCPServers []string
}

// Tool use
type ClaudeToolUse struct {
    ID    string
    Name  string
    Input map[string]any
}

// Session result
type ClaudeResult struct {
    SessionID    string
    NumTurns     int
    DurationMs   int
    TotalCostUSD float64
    Result       string
}
```

### BD Activity Events

Parsed from `bd activity --follow --json`:

```go
type BDMutation struct {
    Type      string    // create, update, status, etc.
    IssueID   string
    Title     string
    Actor     string
    Timestamp time.Time
    OldStatus string
    NewStatus string
}
```

### Event Display Format

For terminal output, events are formatted with color and symbols:

```
[14:23:01] ▶ DRAIN Started in /path/to/project
[14:23:02] 📋 BEAD bd-042 "Fix auth bug" (priority 1)
[14:23:03] 🚀 SESSION opus | max-turns: 50
[14:23:05] $ git status
[14:23:06] 📄 Read: src/auth.go
[14:23:08] ✏️  Edit: src/auth.go
[14:23:10] → BEAD bd-042 in_progress
[14:23:45] $ go test ./...
[14:23:50] ✓ BEAD bd-042 closed "Fixed nil pointer in auth handler"
[14:23:51] 💰 SESSION END | turns: 8 | cost: $0.42 | duration: 48s
[14:23:52] 📋 BEAD bd-043 "Add rate limiting" (priority 2)
...
```

---

## State Management

### State File

Location: `.bd-drain/state.json` in project directory

```json
{
  "version": 1,
  "status": "working",
  "started_at": "2024-01-15T10:00:00Z",
  "current_bead": "bd-042",
  "current_session_id": "abc123",
  "stats": {
    "iterations": 5,
    "beads_completed": 4,
    "beads_failed": 1,
    "total_cost_usd": 2.35,
    "total_turns": 42,
    "total_duration_ms": 180000
  },
  "history": {
    "bd-040": {"status": "completed", "attempts": 1},
    "bd-041": {"status": "completed", "attempts": 1},
    "bd-042": {"status": "working", "attempts": 1},
    "bd-039": {"status": "failed", "attempts": 3, "last_error": "tests failing"}
  }
}
```

### State Transitions

```
                    ┌─────────┐
                    │  init   │
                    └────┬────┘
                         │ start
                         ▼
              ┌──────────────────────┐
              │                      │
    ┌────────▶│        idle          │◀────────┐
    │         │   (no ready beads)   │         │
    │         └──────────┬───────────┘         │
    │                    │ bead available      │
    │                    ▼                     │
    │         ┌──────────────────────┐         │
    │         │                      │         │
    │         │       working        │─────────┤ bead completed
    │         │  (claude session)    │         │ (no more beads)
    │         └──────────┬───────────┘         │
    │                    │                     │
    │      ┌─────────────┼─────────────┐       │
    │      │ pause       │ stop        │       │
    │      ▼             ▼             │       │
    │ ┌─────────┐  ┌───────────┐       │       │
    │ │ paused  │  │ stopping  │       │       │
    │ └────┬────┘  └─────┬─────┘       │       │
    │      │ resume      │ session    │       │
    │      │             │ ends       │       │
    └──────┘             ▼            │       │
                  ┌───────────┐       │       │
                  │  stopped  │◀──────┘       │
                  └───────────┘               │
                         ▲                    │
                         └────────────────────┘
                           stop (when idle)
```

### Recovery on Startup

When bd-drain starts:

1. Check for existing state file
2. If found and status != "stopped":
   - Log recovery message
   - Reset any in_progress beads (via `_bd_reset_stuck_issues` logic)
   - Resume from idle state
3. If not found or status == "stopped":
   - Start fresh

---

## CLI Interface

### Commands

```bash
# Start the drain daemon
bd-drain start [flags]
  --tui              Enable terminal UI
  --log FILE         Log file path (default: .bd-drain/drain.log)
  --max-turns N      Max turns per Claude session (default: 50)
  --label LABEL      Filter bd ready by label
  --prompt FILE      Custom prompt template

# Check daemon status
bd-drain status
  Output: JSON with current state, stats, recent events

# Pause (finish current bead, then wait)
bd-drain pause

# Resume after pause
bd-drain resume

# Stop immediately (SIGTERM to claude if running)
bd-drain stop

# Stop gracefully (wait for current bead)
bd-drain stop --graceful

# View recent events
bd-drain events [--follow] [--count N]

# View statistics
bd-drain stats
```

### Daemon Communication

The daemon listens on a Unix socket: `.bd-drain/drain.sock`

Protocol: Simple JSON-RPC over Unix socket

```go
type Request struct {
    Method string `json:"method"` // "status", "pause", "resume", "stop"
    Params any    `json:"params,omitempty"`
}

type Response struct {
    Result any    `json:"result,omitempty"`
    Error  string `json:"error,omitempty"`
}
```

---

## Configuration

### Config File

Location: `.bd-drain/config.yaml` or `~/.config/bd-drain/config.yaml`

```yaml
# Claude settings
claude:
  model: opus
  max_turns: 50
  allowed_tools:
    - Bash
    - Read
    - Edit
    - Write
    - Glob
    - Grep
    - Task
    - Skill
    - TodoWrite

# Polling intervals
intervals:
  bd_ready_poll: 5s      # How often to check bd ready when idle
  bd_activity_poll: 1s   # bd activity --follow poll rate

# Backoff for failed beads
backoff:
  initial: 1m
  max: 1h
  multiplier: 2

# Prompt template (can also be file path)
prompt: |
  Run "bd ready --json" to find available work...
```

### Environment Variables

```bash
BD_DRAIN_LOG=/path/to/log        # Override log location
BD_DRAIN_CONFIG=/path/to/config  # Override config location
BD_DRAIN_NO_TUI=1                # Disable TUI even if --tui passed
BEADS_ACTOR=bd-drain             # Actor name for bd audit trail
```

---

## Error Handling

### Claude Session Failures

| Scenario | Action |
|----------|--------|
| Non-zero exit code | Log error, increment bead attempts, apply backoff |
| Timeout (no output for 5min) | Kill process, reset bead, log timeout |
| Parse error on output | Log warning, continue (best effort) |
| Session ends without closing bead | Reset bead to open with P0 priority |

### BD Command Failures

| Scenario | Action |
|----------|--------|
| `bd ready` fails | Log error, retry with backoff |
| `bd activity` fails | Log warning, continue without activity stream |
| `bd update/close` fails | Log error, retry once, then continue |

### Daemon Failures

| Scenario | Action |
|----------|--------|
| State file corrupt | Start fresh, log warning |
| Socket already exists | Check if daemon running, error if yes |
| SIGTERM received | Graceful shutdown (wait for current bead) |
| SIGKILL received | State file remains, recovery on next start |

### Bead Failure Backoff

```go
func (wq *WorkQueue) GetBackoff(beadID string) time.Duration {
    history := wq.History[beadID]
    if history.Attempts == 0 {
        return 0
    }

    backoff := wq.Config.InitialBackoff
    for i := 1; i < history.Attempts; i++ {
        backoff *= time.Duration(wq.Config.BackoffMultiplier)
        if backoff > wq.Config.MaxBackoff {
            backoff = wq.Config.MaxBackoff
            break
        }
    }

    return backoff
}
```

---

## Future Considerations

These are explicitly out of scope but worth noting for future work:

1. **Parallel execution**: Multiple Claude workers on independent beads
2. **Session resume**: Continue Claude session across bead boundaries
3. **Web dashboard**: Browser-based monitoring
4. **Webhooks**: Push notifications for events
5. **Cost limits**: Hard caps on spending
6. **Priority boosting**: Auto-elevate stuck beads
7. **Integration with GitHub Actions**: Run drain in CI
