# atari

**Applied Training: Automatic Research & Implementation**

A daemon controller that orchestrates Claude Code sessions to automatically work through beads (bd) issues until all ready work is complete.

## Problem Statement

When using Claude Code with the beads issue tracker, the ideal workflow is:
1. Create beads for planned work (via `bd create` or planning sessions)
2. Have Claude automatically work through all ready beads without human intervention
3. Monitor progress in real-time with good observability
4. Survive interruptions and resume later

The shell-script approach has limitations:
- No persistent state between iterations
- No real-time bead status visualization
- Cannot pause/resume gracefully
- Polling-only, no event-driven architecture

## Solution

A daemon controller written in Go that:
- Maintains persistent state across restarts
- Provides unified event stream (Claude activity + bd activity)
- Manages Claude Code sessions programmatically
- Offers terminal UI for monitoring with observer mode
- Can be paused, resumed, and controlled externally
- Supports exponential backoff for failed beads

## Implementation Status

| Phase | Focus | Status |
|-------|-------|--------|
| 1 | Core Loop (MVP) | Complete |
| 2 | Control & Monitoring | Complete |
| 3 | BD Activity | Complete |
| 4 | Terminal UI | Complete |
| 5 | Polish & Init | Complete |
| 6 | Observer Mode | Complete |
| 7 | Bead Visualization | Complete |
| 8 | Notifications | Not started |

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         atari daemon                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │              Controller (State Machine)                   │  │
│  │       idle → working → paused → stopping → stopped        │  │
│  └────────────────────────┬──────────────────────────────────┘  │
│              ┌────────────┼────────────┬────────────┐           │
│              ▼            ▼            ▼            ▼           │
│       ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│       │WorkQueue │  │ Session  │  │BDActivity│  │  Events  │   │
│       │(bd ready)│  │ Manager  │  │ (watch)  │  │  Router  │   │
│       └──────────┘  └──────────┘  └──────────┘  └──────────┘   │
│              └────────────┬────────────┬────────────┘           │
│                           ▼            ▼                        │
│            ┌────────────────────────────────────────┐           │
│            │        Unified Event Stream            │           │
│            └───┬────────┬────────┬────────┬────────┘           │
│                ▼        ▼        ▼        ▼                     │
│          ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐           │
│          │LogSink │ │  State │ │  TUI   │ │Observer│           │
│          │        │ │  Sink  │ │        │ │        │           │
│          └────────┘ └────────┘ └────────┘ └────────┘           │
│               │          │          │          │                │
└───────────────┼──────────┼──────────┼──────────┼────────────────┘
                ▼          ▼          ▼          ▼
          atari.log   state.json   Terminal   Claude API
```

## Quick Start

```bash
# Initialize atari configuration (optional, sets up Claude Code config)
atari init

# Start in foreground
atari start

# Start as background daemon with TUI
atari start --daemon --tui

# Check status
atari status
atari status --json

# Watch events in real-time
atari events --follow

# Pause (finish current bead, then stop)
atari pause

# Resume
atari resume

# Stop daemon
atari stop
atari stop --force  # Stop immediately
```

## CLI Reference

### Commands

| Command | Description |
|---------|-------------|
| `atari start` | Start the drain loop |
| `atari status` | Show daemon state and statistics |
| `atari pause` | Pause after current bead completes |
| `atari resume` | Resume from paused state |
| `atari stop` | Stop the daemon |
| `atari events` | View recent events |
| `atari init` | Initialize Claude Code configuration |
| `atari version` | Print version information |

### Global Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--verbose` | Enable debug logging | false |
| `--log-file` | Log file path | `.atari/atari.log` |
| `--state-file` | State file path | `.atari/state.json` |
| `--socket-path` | Unix socket path | `.atari/atari.sock` |
| `--config` | Config file path | `.atari/config.yaml` |

### Start Command Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--daemon` | Run as background daemon | false |
| `--tui` | Enable terminal UI | auto-detect TTY |
| `--max-turns N` | Limit Claude session turns | 0 (unlimited) |
| `--label LABEL` | Filter `bd ready` by label | none |
| `--prompt FILE` | Custom prompt template file | built-in |
| `--bd-activity-enabled` | Enable BD activity watcher | true |
| `--observer-enabled` | Enable observer mode in TUI | true |
| `--observer-model` | Claude model for observer | haiku |
| `--graph-enabled` | Enable bead graph pane | true |
| `--graph-density` | Graph density (compact/standard/detailed) | standard |

### Events Command Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--follow` | Tail event stream | false |
| `--count N` | Show last N events | 20 |

### Status Command Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--json` | Output as JSON | false |

## Configuration

Atari supports configuration via (in order of precedence):
1. CLI flags
2. Environment variables (`ATARI_*` prefix)
3. Config file (`.atari/config.yaml`)
4. Built-in defaults

### Example Configuration

```yaml
# .atari/config.yaml

claude:
  timeout: 5m           # Per-session timeout
  max_turns: 0          # 0 = unlimited
  extra_args: []        # Additional CLI args

workqueue:
  poll_interval: 5s     # Polling frequency
  label: ""             # Label filter

backoff:
  initial: 1m           # First retry delay
  max: 1h               # Maximum delay
  multiplier: 2.0       # Exponential factor
  max_failures: 5       # Abandon threshold

bdactivity:
  enabled: true         # Enable activity watcher
  reconnect_delay: 5s   # Initial reconnect backoff
  max_reconnect_delay: 5m

observer:
  enabled: true         # Enable TUI observer
  model: haiku          # Model for queries
  recent_events: 20     # Context size
  layout: horizontal    # Pane layout

graph:
  enabled: true         # Enable graph pane
  density: standard     # Node density
  auto_refresh_interval: 5s

log_rotation:
  max_size_mb: 100
  max_backups: 3
  max_age_days: 7
  compress: true

# Custom session prompt (optional)
prompt: |
  Run "bd ready --json" to find available work...
```

### Environment Variables

All config options can be set via environment variables with `ATARI_` prefix:

```bash
ATARI_CLAUDE_TIMEOUT=10m
ATARI_WORKQUEUE_POLL_INTERVAL=10s
ATARI_BACKOFF_MAX_FAILURES=3
ATARI_OBSERVER_MODEL=sonnet
```

## File Layout

```
.atari/
├── config.yaml      # Configuration file
├── state.json       # Persistent state
├── atari.log        # Event log (JSON lines)
├── atari.sock       # Unix socket (daemon RPC)
└── daemon.json      # Daemon info
```

## Development

Requires [mise](https://mise.jdx.dev/) for tool version management.

```bash
# Build
mise run build

# Run tests
mise run test

# Lint
mise run lint

# Format
mise run fmt

# Install locally
mise run install

# Build container
mise run bake

# Development with hot-reload
mise run dev
```

Or use raw Go commands:

```bash
go build -o atari ./cmd/atari
go test ./...
```

## Requirements

- Go 1.25+
- Claude Code CLI (`claude` command)
- beads CLI (`bd` command)
- A project with `.beads/` initialized

## Documentation

- [Design Document](docs/DESIGN.md) - Architecture and design decisions
- [Context & Research](docs/CONTEXT.md) - Background research on Claude Code and beads
- [Implementation Plan](docs/IMPLEMENTATION.md) - Phased implementation approach

## License

MIT
