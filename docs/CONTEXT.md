# Context & Research

This document captures all background research and context needed to understand and implement atari.

## Table of Contents

1. [Current Implementation](#current-implementation)
2. [Claude Code Capabilities](#claude-code-capabilities)
3. [Beads (bd) Capabilities](#beads-bd-capabilities)
4. [User's Claude Code Configuration](#users-claude-code-configuration)

---

## Current Implementation

The user has shell functions in `~/.zshrc` that implement a working but limited drain system.

### Original bd-drain Function

Location: `~/.zshrc` (around line 950)

Note: This shell function is the original implementation that atari replaces.

```bash
bd-drain() (
  # Hold "prevent idle system sleep" while this function runs
  local pid="${BASHPID:-$$}"
  caffeinate -i -s -w "$pid" &
  local caf_pid=$!
  trap 'kill "$caf_pid" 2>/dev/null' EXIT INT TERM

  local label=""
  local prompt=""
  local logfile="/tmp/full-bd-drain-logs.json"

  # Parse CLI arguments (--label, --logfile, --help)
  # ...

  # Rotate logs if exists
  if [[ -f "$logfile" ]]; then
    local backup="${logfile}.$(date +%Y%m%d-%H%M%S).bak"
    mv "$logfile" "$backup"
  fi

  local count=0
  local start_time=$SECONDS
  local last_time=$start_time

  # Main loop: while there are ready beads
  while bd ready --json "${bd_args[@]}" | jq -e 'length > 0' >/dev/null; do
    # Time tracking between iterations
    # ...

    ((count++))
    echo "=== Iteration $count ==="
    bd ready "${bd_args[@]}"

    # Run Claude with streaming JSON output
    claude-stream "$prompt" "$logfile"

    # Update CLAUDE.md files (separate session)
    claude-stream "Review git commits..." "$logfile"

    # Reset any stuck in_progress issues
    _bd_reset_stuck_issues
  done

  # Summary with cost/turns from log file
  # ...
)
```

### claude-stream Function

```bash
claude-stream() {
  local prompt="$1"
  local logfile="$2"

  claude --model opus --print --verbose --output-format=stream-json "$prompt" |
    tee -a "$logfile" |
    jq -r "$_CLAUDE_STREAM_JQ"
}
```

### Stream Parsing JQ Filter

The `_CLAUDE_STREAM_JQ` variable contains a comprehensive jq filter that pretty-prints Claude's stream-json output:

```jq
# System messages (init, hooks, compaction boundaries)
if .type == "system" then
  if .subtype == "init" then
    "\n═══ SESSION START ═══\n  Model: \(.model)\n  Tools: \(.tools | length) available"
  elif .subtype == "compact_boundary" then
    "\n─── CONTEXT COMPACTED ───"
  # ...

# Assistant messages (text, thinking, tool_use)
elif .type == "assistant" then
  .message.content[] |
  if .type == "text" then .text
  elif .type == "thinking" then "💭 \(.thinking | .[0:80])..."
  elif .type == "tool_use" then
    # Tool-specific formatting (Bash, Read, Edit, Write, Glob, Grep, Task, Skill, TodoWrite)
    # ...

# Session result summary
elif .type == "result" then
  "\n═══ SESSION END ═══\n  Turns: \(.num_turns)  Duration: ...  Cost: $..."
```

### Stuck Issue Reset

```bash
_bd_reset_stuck_issues() {
  local stuck
  stuck=$(bd list --status=in_progress --json 2>/dev/null | jq -r '.[].id' 2>/dev/null)
  if [[ -n "${stuck// /}" ]]; then
    for id in $stuck; do
      bd update "$id" --status=open --priority=0 \
        --notes "RESET: Previous session ended without closing this issue."
    done
  fi
}
```

### Limitations of Current Approach

| Limitation | Impact |
|------------|--------|
| No persistent state | Cannot resume after Ctrl+C or crash |
| Fresh session per iteration | No session continuity for related beads |
| Polling only | No real-time event reactions |
| No pause/resume | Must kill and restart |
| Sequential CLAUDE.md update | Adds overhead every iteration |
| No unified event stream | bd changes not visible during Claude work |

---

## Claude Code Capabilities

Research from official Claude Code documentation.

### Non-Interactive Execution

```bash
# Basic non-interactive
claude -p "prompt here"

# With structured output
claude -p "prompt" --output-format json
claude -p "prompt" --output-format stream-json

# Tool auto-approval
claude -p "prompt" --allowedTools "Bash,Read,Edit"

# Skip all permissions (use carefully)
claude -p "prompt" --dangerously-skip-permissions

# Limit agent turns (prevent runaway)
claude -p "prompt" --max-turns 10
```

### Output Formats

**JSON output structure:**
```json
{
  "result": "...",
  "session_id": "abc123",
  "usage": {
    "input_tokens": 1000,
    "output_tokens": 500
  },
  "total_cost_usd": 0.05,
  "num_turns": 5,
  "duration_ms": 30000
}
```

**Stream-JSON events:**
```json
// System init
{"type": "system", "subtype": "init", "model": "opus", "tools": [...]}

// Assistant message with tool use
{"type": "assistant", "message": {"content": [{"type": "tool_use", "name": "Bash", "input": {...}}]}}

// Tool result
{"type": "tool_result", "tool_use_id": "...", "content": "..."}

// Session end
{"type": "result", "result": "...", "session_id": "...", "num_turns": 5, "total_cost_usd": 0.05}
```

### Session Management

```bash
# Get session ID from output
session_id=$(claude -p "start" --output-format json | jq -r '.session_id')

# Resume specific session
claude -p "continue" --resume "$session_id"

# Continue most recent session
claude -p "continue" --continue
```

### Hooks

Claude Code supports hooks that trigger on specific events:

**SessionStart** - Triggers on:
- `startup` - True session start
- `resume` - After --resume
- `clear` - After /clear
- `compact` - After context compaction

**SessionEnd** - Triggers when session ends with reason:
- `exit`, `clear`, `logout`, `prompt_input_exit`

**Stop** - Triggers when Claude finishes a response, can decide to continue or stop.

**Hook input (JSON via stdin):**
```json
{
  "session_id": "abc123",
  "transcript_path": "/path/to/session.jsonl",
  "cwd": "/current/dir",
  "hook_event_name": "SessionEnd",
  "reason": "exit"
}
```

### Key Findings for atari

1. Use `--output-format stream-json` for real-time monitoring
2. Use `--max-turns` to prevent runaway sessions
3. Can capture `session_id` from JSON output for potential resume
4. SessionEnd hooks could trigger next atari iteration
5. No built-in queue system - must implement externally

---

## Beads (bd) Capabilities

Research from exploring https://github.com/steveyegge/beads

### Core Commands

```bash
bd ready               # Show unblocked work (entry point)
bd ready --json        # JSON output for parsing
bd create              # Create new issues
bd update <id>         # Modify issues
bd close <id>          # Close completed work
bd list                # Query issues with filters
bd show <id>           # Detailed issue view
```

### Real-Time Activity Stream

```bash
bd activity            # Show last 100 mutation events
bd activity --follow   # Real-time streaming (500ms poll)
bd activity --json     # JSON output
bd activity --mol <id> # Filter by molecule
bd activity --since 5m # Time-based filtering
```

**Event symbols:**
- `+` created/bonded - New issue or molecule
- `→` in_progress - Work started
- `✓` completed - Issue closed
- `✗` failed - Step failed
- `⊘` deleted - Issue removed

### Mutation Events

From `internal/rpc/server_core.go`:

```go
const (
  MutationCreate  = "create"
  MutationUpdate  = "update"
  MutationDelete  = "delete"
  MutationComment = "comment"
  MutationBonded  = "bonded"
  MutationSquashed = "squashed"
  MutationBurned  = "burned"
  MutationStatus  = "status"
)

type MutationEvent struct {
  Type      string
  IssueID   string
  Title     string
  Assignee  string
  Actor     string
  Timestamp time.Time
  OldStatus string
  NewStatus string
  ParentID  string
  StepCount int
}
```

### Agent State Tracking

```bash
bd agent state <name> <state>   # Set agent state
bd agent heartbeat <name>       # Periodic check-in
bd agent show <name>            # Show state details
```

Valid states: `idle`, `spawning`, `running`, `working`, `stuck`, `done`, `stopped`, `dead`

### Context Recovery (bd prime)

```bash
bd prime           # Auto-detect MCP vs CLI mode
bd prime --full    # Force full CLI output (~1-2k tokens)
bd prime --mcp     # Force MCP output (~50 tokens)
bd prime --stealth # No git operations
```

**Important**: `bd prime` should NOT be run automatically after context compaction. The user's Claude config has been updated to clarify this.

### Daemon

```bash
bd daemon              # Start background daemon
bd daemons list        # Show all running daemons
bd daemons health      # Check health
bd daemons logs        # View daemon logs
bd daemons killall     # Stop all daemons
```

Environment variables for automation:
```bash
BEADS_NO_DAEMON=true        # Disable background process
BEADS_ACTOR="atari"         # Audit trail actor name
BEADS_AUTO_START_DAEMON=0   # Prevent daemon creation
```

### Key Findings for atari

1. `bd activity --follow --json` provides real-time event stream
2. `bd agent` commands can track atari controller state
3. Daemon handles multi-process safety
4. No webhooks - must poll or use `--follow`
5. All mutations have JSON output for parsing

---

## User's Claude Code Configuration

Location: `~/.claude/`

### Key Settings

From `~/.claude/settings.json`:
```json
{
  "env": {
    "CLAUDE_BASH_MAINTAIN_PROJECT_WORKING_DIR": "true",
    "CLAUDE_CODE_MAX_OUTPUT_TOKENS": "64000",
    "MAX_THINKING_TOKENS": "31999"
  },
  "permissions": {
    "additionalDirectories": ["/Users/npratt/git"],
    "defaultMode": "bypassPermissions"
  },
  "model": "opus",
  "alwaysThinkingEnabled": true
}
```

### Session Protocol

From `~/.claude/rules/session-protocol.md`:

**Session Startup (User-Initiated Only)**:
- Run ONLY when user explicitly requests
- Do NOT run after context compaction
- Commands: `bd prime`, `bd ready --json`, `git log`, `git status`

**Incremental Progress**:
- Work on ONE issue at a time
- Commit with `/commit` slash command
- Close: `bd close <id> --reason "..." --json`
- Verify feature works end-to-end

### Issue Tracking Rules

From `~/.claude/rules/issue-tracking.md`:

**Do NOT close if:**
- Tests failing
- Implementation partial
- Unresolved errors
- Integration tests not updated for API changes

**Notes format:**
```
COMPLETED: What was done
KEY DECISION: Why this approach
IN PROGRESS: Current state
NEXT: Immediate next step
```

### Available Skills

- `bd-issue-tracking` - Core bd workflow patterns
- `git-commit` - Atomic commits matching project style
- `bd-plan` / `bd-plan-ultra` - Planning workflows

---

## Summary

### What We're Building

A Go daemon that:
1. Polls `bd ready` for available work
2. Spawns Claude Code sessions to work on beads
3. Streams unified events (Claude + bd) for observability
4. Persists state for pause/resume capability
5. Provides optional TUI for monitoring

### Key Integration Points

| Component | How atari Integrates |
|-----------|------------------------|
| Claude Code | `claude -p --output-format stream-json --max-turns N` |
| bd ready | Poll for available work, parse JSON |
| bd activity | `--follow --json` for real-time bead events |
| bd agent | Track atari controller state |
| State file | Persist iteration count, costs, current bead |

### Design Constraints

1. Single Claude session at a time (no parallel execution yet)
2. Must handle Claude session failures gracefully
3. Must reset stuck in_progress issues
4. Should track costs but no hard limits
5. Must survive Ctrl+C and resume later
