# Implementation Plan

Phased implementation approach for Atari.

For detailed component specifications, see the [components/](components/) directory.

## Technology Stack

- **Language**: Go
- **CLI Framework**: Cobra + Viper
- **TUI Framework**: Bubbletea + Lipgloss
- **Configuration**: YAML (gopkg.in/yaml.v3)

## Phase Overview

| Phase | Focus | Key Deliverables | Status |
|-------|-------|------------------|--------|
| 1 | Core Loop (MVP) | Poll, spawn, log, persist | **Complete** |
| 2 | Control & Monitoring | Daemon mode, pause/resume/stop | **Complete** |
| 3 | BD Activity | Unified event stream | **Complete** |
| 4 | Terminal UI | Bubbletea TUI | **Complete** |
| 5 | Polish & Init | Config, log rotation, cost tracking | **Complete** |
| 6 | Observer Mode | Interactive Q&A pane | **Complete** |
| 7 | Bead Visualization | TUI bead graph/tree | **Complete** |
| 8 | Notifications | Webhooks, IFTTT, Slack | Not started |

---

## Phase 1: Core Loop (MVP) - COMPLETE

**Goal**: Minimal working drain that can run unattended.

**Status**: Complete as of 2026-01-02

### Components Implemented

| Component | Documentation | Implementation |
|-----------|---------------|----------------|
| Controller (basic) | [components/controller.md](components/controller.md) | `internal/controller/` |
| Work Queue | [components/workqueue.md](components/workqueue.md) | `internal/workqueue/` |
| Session Manager | [components/session.md](components/session.md) | `internal/session/` |
| Event Router | [components/events.md](components/events.md) | `internal/events/router.go` |
| Log Sink | [components/sinks.md](components/sinks.md) | `internal/events/logsink.go` |
| State Sink | [components/sinks.md](components/sinks.md) | `internal/events/statesink.go` |
| Test Infrastructure | `internal/testutil/CLAUDE.md` | `internal/testutil/` |
| Integration Tests | `internal/integration/CLAUDE.md` | `internal/integration/` |

### CLI Commands

- `atari start` (foreground only) - **Implemented**
- `atari version` - **Implemented**

### Tasks

1. [x] Project setup (go mod, cobra structure, mise tasks)
2. [x] Core types (Event, State, Config, Bead)
3. [x] Work Queue Manager (bd ready polling, bead selection, backoff)
4. [x] Session Manager (spawn claude, parse stream-json, watchdog timeout)
5. [x] Event Router (channel-based pub/sub with configurable buffer)
6. [x] Log Sink (JSON lines file with append mode)
7. [x] State Sink (state.json persistence with atomic writes)
8. [x] Controller main loop (idle/working/paused/stopping/stopped states)
9. [x] Signal handling (SIGINT, SIGTERM via shutdown package)
10. [x] Exponential backoff for failed beads (moved from Phase 6)
11. [x] Agent state logging via structured logs

### Success Criteria

- [x] `atari start` processes all ready beads
- [x] Logs written to `.atari/atari.log`
- [x] State persisted to `.atari/state.json`
- [x] Graceful shutdown on Ctrl+C
- [x] Recovers state on restart
- [x] Agent state transitions logged via slog

### Notes

- Backoff implementation was pulled forward from Phase 6 into the workqueue
- Integration tests use mock Claude script for reliable testing
- All tests pass: `mise run test` (7 packages, 100+ tests)

---

## Phase 2: Control & Monitoring - COMPLETE

**Goal**: Add daemon mode with external control.

**Status**: Complete as of 2026-01-02

### Components Implemented

| Component | Documentation | Implementation |
|-----------|---------------|----------------|
| Daemon Server | [components/daemon.md](components/daemon.md) | `internal/daemon/server.go` |
| RPC Client | [components/daemon.md](components/daemon.md) | `internal/daemon/client.go` |
| PID Management | [components/daemon.md](components/daemon.md) | `internal/daemon/pid.go` |
| Daemonization | [components/daemon.md](components/daemon.md) | `internal/daemon/daemonize.go` |
| Path Resolution | [components/daemon.md](components/daemon.md) | `internal/daemon/paths.go` |
| JSON-RPC Protocol | [components/daemon.md](components/daemon.md) | `internal/daemon/protocol.go` |
| RPC Handlers | [components/daemon.md](components/daemon.md) | `internal/daemon/handlers.go` |
| Daemon Integration Tests | - | `internal/daemon/integration_test.go` |

### CLI Commands

- `atari start --daemon` - **Implemented**
- `atari status` (with `--json` option) - **Implemented**
- `atari pause` - **Implemented**
- `atari resume` - **Implemented**
- `atari stop` - **Implemented**
- `atari events` (with `--follow` and `--count` options) - **Implemented**

### Tasks

1. [x] Daemonize process (fork, setsid)
2. [x] PID file management
3. [x] Unix socket listener
4. [x] JSON-RPC protocol
5. [x] Implement pause/resume state transitions
6. [x] Status command with stats
7. [x] Events command (tail log file)

### Success Criteria

- [x] Can start daemon, pause, resume, stop via CLI
- [x] Status command shows current state and stats
- [x] Events command can tail the event stream

### Notes

- Daemon uses Unix socket at `.atari/atari.sock` for RPC communication
- PID tracking via `.atari/atari.pid` (removed, daemon info now in `.atari/daemon.json`)
- Path resolution handles both foreground and daemon modes correctly
- Foreground start now also supports RPC control (socket enabled by default)
- All daemon tests pass: `go test -v ./internal/daemon/...` (8 packages)

---

## Phase 3: BD Activity Integration - COMPLETE

**Goal**: Unified event stream with bd activity.

**Status**: Complete as of 2026-01-02

### Components Implemented

| Component | Documentation | Implementation |
|-----------|---------------|----------------|
| BD Activity Watcher | [components/bdactivity.md](components/bdactivity.md) | `internal/bdactivity/watcher.go` |
| BD Activity Parser | [components/bdactivity.md](components/bdactivity.md) | `internal/bdactivity/parser.go` |
| ProcessRunner Interface | - | `internal/runner/` |
| BD Event Types | [components/events.md](components/events.md) | `internal/events/types.go` |

### Tasks

1. [x] Spawn `bd activity --follow --json`
2. [x] Parse mutation events (create, status, update, comment, closed)
3. [x] Convert to unified event format (BeadCreatedEvent, BeadStatusEvent, etc.)
4. [x] Merge into event stream via shared Router
5. [x] Handle reconnection on failure (exponential backoff with reset)

### Success Criteria

- [x] Bead status changes appear in event stream
- [x] `atari events` shows unified claude + bd events
- [x] Reconnects automatically on bd activity failure

### Notes

- ProcessRunner interface abstracts streaming process execution for testability
- MockProcessRunner enables deterministic testing without real bd process
- Watcher uses exponential backoff: 5s initial, 5min max, resets after 3 successful events
- Parse errors are rate-limited (1 warning per 5 seconds) to avoid log spam
- Controller integration is optional: watcher disabled when processRunner is nil
- All tests pass: `go test -v ./internal/bdactivity/...` (51 tests)
- Silently skips unknown mutation types (bonded, squashed, burned, delete)

---

## Phase 4: Terminal UI - COMPLETE

**Goal**: Rich terminal interface for monitoring.

**Status**: Complete as of 2026-01-02

### Components Implemented

| Component | Documentation | Implementation |
|-----------|---------------|----------------|
| TUI Model | [components/tui.md](components/tui.md) | `internal/tui/model.go` |
| TUI View | [components/tui.md](components/tui.md) | `internal/tui/view.go` |
| TUI Update | [components/tui.md](components/tui.md) | `internal/tui/update.go` |
| Event Formatting | [components/tui.md](components/tui.md) | `internal/tui/format.go` |
| Styles | [components/tui.md](components/tui.md) | `internal/tui/styles.go` |
| Graceful Fallback | [components/tui.md](components/tui.md) | `internal/tui/fallback.go` |
| DrainStateChangedEvent | [components/events.md](components/events.md) | `internal/events/types.go` |

### CLI Commands

- `atari start` - **Implemented** (TUI auto-enabled when TTY detected)
- `atari start --tui=false` - **Implemented** (force simple fallback mode)

### Tasks

1. [x] Bubbletea model and update loop
2. [x] Header component (status, stats)
3. [x] Event feed component (scrollable)
4. [x] Footer component (keyboard help)
5. [x] Keyboard handling (p/r/q, arrows)
6. [x] Graceful degradation (no TTY)
7. [x] DrainStateChangedEvent for state tracking

### Success Criteria

- [x] TUI displays current state and events
- [x] Keyboard controls work (pause, resume, quit)
- [x] Scrolling works for event history
- [x] Falls back to simple output when no TTY

### Notes

- TUI uses event-driven status updates via DrainStateChangedEvent
- StatsGetter interface provides backup polling for dropped events
- Startup-only runSimple fallback (no dynamic mode switching)
- Centralized format.go keeps events package focused on types
- SessionEndEvent is authoritative for cost/turns (avoids double-counting)
- All TUI tests pass: `go test -v ./internal/tui/...`

**Future**: Observer Mode - interactive Q&A pane for asking questions about events. See [observer.md](components/observer.md).

---

## Phase 5: Polish & Init - COMPLETE

**Goal**: Production-ready reliability, cost tracking, and onboarding.

**Status**: Complete as of 2026-01-02

### Components Implemented

| Component | Documentation | Implementation |
|-----------|---------------|----------------|
| Init Command | [cli/init-command.md](cli/init-command.md) | `internal/init/` |
| Configuration | [config/configuration.md](config/configuration.md) | `internal/config/loader.go` |
| Cost Tracking | - | `internal/session/parser.go` (atomic.Value result capture) |
| Log Rotation | - | `internal/events/logsink.go`, `cmd/atari/logger.go` |
| Prompt Templates | - | `internal/config/prompt.go` |

### CLI Commands

- `atari init` - **Implemented** (with --dry-run, --force, --minimal, --global flags)

### Tasks

1. [x] Exponential backoff for failed beads - **Done in Phase 1**
2. [x] YAML config file parsing
3. [x] Environment variable overrides - **Done in Phase 1** (via Viper)
4. [x] Custom prompt templates
5. [x] `atari init` command
6. [x] Log rotation for `.atari/atari.log` and `.atari/atari-debug.log`
7. [x] Cost/usage tracking - SessionEndEvent is authoritative source (via atomic.Value)
8. [ ] User guide documentation
9. [ ] Error messages and suggestions

### Success Criteria

- [x] Failed beads don't block drain indefinitely (backoff + max failures)
- [x] Configuration works from file and env
- [x] `atari init` sets up Claude Code correctly
- [x] Log files don't grow unbounded
- [x] Cost tracking shows usage per session (SessionEndEvent with TotalCostUSD)
- [ ] Documentation is complete

### Notes

- **YAML config precedence**: Default() < ~/.config/atari/config.yaml < .atari/config.yaml < --config flag < env vars < CLI flags
- **Log rotation strategies**:
  - Event log (atari.log): Startup rotation with timestamp suffix (preserves tail -f compatibility)
  - Debug log (atari-debug.log): Continuous lumberjack rotation (MaxSizeMB, MaxBackups, MaxAgeDays, Compress)
- **Prompt templates**: Single-pass string replacement via `strings.Replacer` prevents template injection
- **Cost tracking**: Parser.Result() uses atomic.Value for thread-safe access to SessionEndEvent
- **Init command**: Embedded templates via `//go:embed`, backup with --force, append mode for CLAUDE.md

---

## Phase 6: Observer Mode - COMPLETE

**Goal**: Interactive Q&A pane for real-time understanding and intervention guidance.

**Status**: Complete as of 2026-01-02

### Components Implemented

| Component | Documentation | Implementation |
|-----------|---------------|----------------|
| Observer | [components/observer.md](components/observer.md) | `internal/observer/observer.go` |
| SessionBroker | [components/observer.md](components/observer.md) | `internal/observer/broker.go` |
| ContextBuilder | [components/observer.md](components/observer.md) | `internal/observer/context.go` |
| LogReader | [components/observer.md](components/observer.md) | `internal/observer/log_reader.go` |
| TUI Observer Pane | [components/observer.md](components/observer.md) | `internal/tui/observer_pane.go` |
| ObserverConfig | [config/configuration.md](config/configuration.md) | `internal/config/config.go` |
| Mock Claude Script | - | `internal/testutil/mock_claude.go` |
| Observer Fixtures | - | `internal/testutil/observer_fixtures.go` |
| E2E Tests | - | `internal/integration/observer_test.go` |

### Tasks

1. [x] Add ObserverConfig to configuration system
2. [x] Implement Observer struct with Ask/Cancel/Reset
3. [x] Implement ContextBuilder (structured sections from log file)
4. [x] Implement LogReader for reading events from log
5. [x] Implement SessionBroker for Claude CLI coordination
6. [x] Add observer pane to TUI (split view with textarea)
7. [x] Implement text input component for questions
8. [x] Wire up keyboard shortcuts (Enter submit, Ctrl+C cancel, Esc clear)
9. [x] Integration tests with mock claude (12 E2E tests)
10. [x] Unit tests for Observer, SessionBroker, limitedWriter

### Success Criteria

- [x] Can ask questions about current session activity
- [x] Claude responses appear in TUI observer pane
- [x] Follow-up questions can maintain conversation context (via --resume)
- [x] Observer does not interfere with main drain operation (SessionBroker)
- [x] Output truncation prevents runaway responses (100KB limit)
- [x] Cancellation works for long-running queries

### Design Decisions

- **Session model**: Uses `claude --resume <session_id>` for follow-up questions (no stdin REPL)
- **Context source**: Reads from existing `.atari/atari.log` (no separate ring buffer)
- **Structured context**: Drain status, session history with costs, current bead events
- **Event summarization**: Tool name + description/file_path, truncated text (full details via grep)
- **Default model**: Haiku for fast, low-cost Q&A
- **Session coordination**: SessionBroker mutex prevents observer/drain Claude CLI conflicts
- **Output format**: Uses `--output-format text` for clean responses

### Notes

- Observer uses DrainStateProvider interface for current drain state
- SessionBroker ensures only one Claude session active at a time
- Output limited to 100KB with truncation marker
- Retry logic clears sessionID on resume failure
- All tests pass: `go test -v ./internal/observer/...` and `go test -v ./internal/integration/...`

---

## Phase 7: Bead Visualization - COMPLETE

**Goal**: Interactive TUI pane for visualizing bead relationships, status, and hierarchy.

**Status**: Complete as of 2026-01-05

### Components Implemented

| Component | Documentation | Implementation |
|-----------|---------------|----------------|
| Graph Visualizer | [components/graph.md](components/graph.md) | `internal/tui/graph.go` |
| Bead Fetcher | [components/graph.md](components/graph.md) | `internal/tui/fetcher.go` |
| GraphConfig | [config/configuration.md](config/configuration.md) | `internal/config/config.go` |
| Detail Modal | [components/graph.md](components/graph.md) | `internal/tui/modal.go` |
| GraphPane | [components/graph.md](components/graph.md) | `internal/tui/graph_pane.go` |
| Graph Types | [components/graph.md](components/graph.md) | `internal/tui/graph_types.go` |

### Design Decisions

- **Data source**: `bd list --json --status ...` with status filtering
- **Views**: Active (open/in_progress/blocked) vs Backlog (deferred), toggle with `a`
- **Panel system**: Three toggleable panels (e/o/g), uppercase for focus mode
- **Node density**: Compact/Standard/Detailed, cycle with `d`
- **Selection action**: Modal popup (~90% pane size) with full bead info
- **Edge rendering**: Both hierarchy (solid) and dependencies (dashed)
- **Layout direction**: Top-down for vertical splits, left-right for horizontal
- **Refresh**: Manual with `r`
- **Overflow**: Scrolling viewport, collapsible epics with `c`

### Tasks

1. [x] Add GraphConfig to configuration system
2. [x] Implement BeadFetcher (bd list --json wrapper)
3. [x] Implement Graph struct with node/edge data structures
4. [x] Implement layout algorithm (layer assignment, positioning)
5. [x] Implement node rendering (compact/standard/detailed densities)
6. [x] Implement edge rendering (hierarchy solid, dependency dashed)
7. [x] Add graph pane to TUI panel system
8. [x] Implement keyboard navigation (arrow keys, selection)
9. [x] Implement detail modal (Enter to open, Esc to close)
10. [x] Implement view toggle (Active/Backlog with `a`)
11. [x] Implement epic collapse/expand with `c`
12. [x] Implement current bead highlighting
13. [x] Implement viewport scrolling for large graphs
14. [x] Wire up panel toggle keys (b/B) - changed from g/G
15. [x] Handle all-panels-disabled expanded stats view
16. [x] Unit tests for graph logic
17. [x] Integration tests with mock bd

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `b` | Toggle bead graph panel |
| `B` | Focus bead graph (fullscreen) |
| `Arrow keys` | Navigate between nodes |
| `Enter` | Open detail modal |
| `Esc` | Close modal / exit focus mode |
| `a` | Toggle Active/Backlog view |
| `c` | Collapse/expand selected epic |
| `d` | Cycle density level |
| `r` | Refresh graph data |
| `Tab` | Cycle focus between panels |

### Success Criteria

- [x] Graph renders correctly showing epic/task hierarchy
- [x] Dependency edges (blocks) shown with dashed lines
- [x] Current bead is visually highlighted during processing
- [x] Can navigate and select beads with keyboard
- [x] Detail modal shows full bead information
- [x] Active/Backlog views filter appropriately
- [x] Epics can be collapsed to hide children
- [x] Large graphs scroll correctly
- [x] Panel toggle system works with events/observer/graph

### Notes

- Layout direction adapts to TUI split configuration for optimal use of space
- Detail modal is nearly full-pane size since beads contain detailed descriptions
- Graph uses `bd list --json` which includes embedded dependency objects
- No separate graph computation needed - bd provides all relationship data
- Panel toggle key changed from 'g' to 'b' (g is used for go-to-top)
- Phase 7 epic: bd-drain-dvx (closed)
- All tests pass: `go test -v ./internal/tui/...` (includes graph unit and integration tests)

---

## Phase 8: Notifications

**Goal**: External alerts for key events.

### Components to Implement

| Component | Documentation |
|-----------|---------------|
| Notifications | [components/notifications.md](components/notifications.md) |

### Tasks

1. [ ] Notification sink (event consumer)
2. [ ] IFTTT provider
3. [ ] Slack provider
4. [ ] Discord provider
5. [ ] Generic webhook provider
6. [ ] Rate limiting
7. [ ] Retry logic

### Success Criteria

- [ ] IFTTT notifications work
- [ ] Slack notifications work
- [ ] Configurable triggers per provider
- [ ] Rate limiting prevents spam

---

## File Structure

Current structure (Phase 7 complete):

```
atari/
├── cmd/atari/
│   ├── main.go              # Root command, all subcommands, flag definitions
│   ├── config.go            # Flag name constants for Viper binding
│   ├── logger.go            # TUI logger setup (file-based to avoid TUI corruption)
│   ├── logger_test.go       # CLI wiring tests for TUI logger
│   └── CLAUDE.md            # CLI package documentation
├── internal/
│   ├── config/              # Configuration types and defaults
│   │   ├── config.go
│   │   ├── config_test.go
│   │   ├── loader.go         # YAML config file loading (Phase 5)
│   │   ├── loader_test.go
│   │   ├── prompt.go         # Prompt template expansion (Phase 5)
│   │   ├── prompt_test.go
│   │   └── CLAUDE.md
│   ├── controller/          # Main orchestration loop
│   │   ├── controller.go
│   │   ├── controller_test.go
│   │   └── CLAUDE.md
│   ├── daemon/              # Daemon mode and RPC (Phase 2)
│   │   ├── CLAUDE.md
│   │   ├── client.go        # RPC client for CLI commands
│   │   ├── client_test.go
│   │   ├── daemon.go        # Daemon struct and lifecycle
│   │   ├── daemon_test.go
│   │   ├── daemonize.go     # Fork/setsid for background mode
│   │   ├── daemonize_test.go
│   │   ├── handlers.go      # RPC command handlers
│   │   ├── integration_test.go  # Full daemon integration tests
│   │   ├── paths.go         # Path resolution for daemon files
│   │   ├── paths_test.go
│   │   ├── pid.go           # PID file management
│   │   ├── pid_test.go
│   │   ├── protocol.go      # JSON-RPC types
│   │   ├── server.go        # Unix socket listener
│   │   └── server_test.go
│   ├── events/              # Event types, router, sinks
│   │   ├── types.go         # Event interface, concrete types
│   │   ├── types_test.go
│   │   ├── router.go        # Channel-based pub/sub
│   │   ├── router_test.go
│   │   ├── logsink.go       # JSON lines log file
│   │   ├── logsink_test.go
│   │   ├── statesink.go     # State persistence
│   │   ├── statesink_test.go
│   │   └── CLAUDE.md
│   ├── session/             # Claude process lifecycle
│   │   ├── manager.go       # Process spawning, stdin/stdout/stderr
│   │   ├── manager_test.go
│   │   ├── parser.go        # stream-json parsing
│   │   ├── parser_test.go
│   │   └── CLAUDE.md
│   ├── shutdown/            # Graceful shutdown helper
│   │   └── shutdown.go
│   ├── testutil/            # Test infrastructure
│   │   ├── cmdrunner.go     # CommandRunner interface, MockRunner
│   │   ├── cmdrunner_test.go
│   │   ├── fixtures.go      # Sample JSON responses
│   │   ├── fixtures_test.go
│   │   ├── helpers.go       # Test helpers (TempDir, WriteFile, etc.)
│   │   ├── helpers_test.go
│   │   ├── mockclaude.go    # Mock Claude session generators
│   │   ├── mockclaude_test.go
│   │   ├── mock_claude.go   # Mock Claude script for observer tests (Phase 6)
│   │   ├── observer_fixtures.go  # Observer log event fixtures (Phase 6)
│   │   └── CLAUDE.md
│   ├── workqueue/           # Work discovery and selection
│   │   ├── poll.go          # bd ready polling
│   │   ├── poll_test.go
│   │   ├── queue.go         # Selection, backoff, history
│   │   ├── queue_test.go
│   │   └── CLAUDE.md
│   ├── init/                # Init command (Phase 5)
│   │   ├── init.go           # File installation logic
│   │   ├── init_test.go
│   │   ├── templates.go      # Embedded template loading
│   │   ├── templates/        # Embedded template files
│   │   └── CLAUDE.md
│   ├── integration/         # End-to-end tests
│   │   ├── drain_test.go
│   │   ├── observer_test.go  # Observer E2E tests (Phase 6)
│   │   └── CLAUDE.md
│   ├── runner/              # ProcessRunner interface for streaming processes
│   │   ├── runner.go        # Interface and ExecProcessRunner
│   │   ├── mock.go          # MockProcessRunner for testing
│   │   └── CLAUDE.md
│   ├── bdactivity/          # BD activity stream watcher (Phase 3)
│   │   ├── watcher.go       # Spawns bd activity, reconnects on failure
│   │   ├── watcher_test.go
│   │   ├── parser.go        # JSON to typed event conversion
│   │   ├── parser_test.go
│   │   └── CLAUDE.md
│   ├── observer/            # Observer mode Q&A (Phase 6)
│   │   ├── observer.go      # Observer struct, Ask, Cancel, Reset
│   │   ├── observer_test.go
│   │   ├── broker.go        # SessionBroker for Claude CLI coordination
│   │   ├── broker_test.go
│   │   ├── context.go       # ContextBuilder from log file and drain state
│   │   ├── context_test.go
│   │   ├── log_reader.go    # LogReader for reading events from log
│   │   ├── log_reader_test.go
│   │   ├── types.go         # DrainState, DrainStateProvider, SessionHistory
│   │   └── CLAUDE.md
│   └── tui/                 # Terminal UI (bubbletea) - Phase 4, 6, 7
│       ├── model.go         # Bubbletea model definition
│       ├── view.go          # View rendering (header, events, footer)
│       ├── view_test.go
│       ├── update.go        # Update loop and keyboard handling
│       ├── update_test.go
│       ├── format.go        # Event formatting for display
│       ├── format_test.go
│       ├── styles.go        # Lipgloss styles
│       ├── fallback.go      # Non-TTY fallback mode
│       ├── fallback_test.go
│       ├── observer_pane.go # Observer Q&A pane (Phase 6)
│       ├── observer_pane_test.go
│       ├── graph.go         # Graph visualizer (Phase 7)
│       ├── graph_test.go
│       ├── graph_pane.go    # Graph panel integration (Phase 7)
│       ├── graph_pane_test.go
│       ├── graph_types.go   # Graph data structures (Phase 7)
│       ├── fetcher.go       # BeadFetcher for bd list (Phase 7)
│       ├── fetcher_test.go
│       ├── modal.go         # Detail modal component (Phase 7)
│       ├── modal_test.go
│       ├── graph_integration_test.go  # Graph integration tests (Phase 7)
│       ├── tui.go           # Public API (Run, RunSimple)
│       └── CLAUDE.md
├── docs/
│   ├── CONTEXT.md           # Background research
│   ├── DESIGN.md            # Architecture decisions
│   ├── IMPLEMENTATION.md    # This file
│   ├── components/          # Component specifications
│   ├── cli/                 # CLI command documentation
│   └── config/              # Configuration file formats
├── .atari/                  # Runtime directory (gitignored)
├── .beads/                  # Issue tracking (git-tracked)
├── go.mod
├── go.sum
├── .mise.toml               # Task runner configuration
└── README.md
```

Planned additions for future phases marked with `[Phase N]`.

---

## Testing Strategy

### Unit Tests - IMPLEMENTED

Each component has unit tests covering:
- [x] Core logic and state transitions (controller, workqueue)
- [x] Event parsing and formatting (session parser, event types)
- [x] Configuration loading and validation (config package)
- [x] Error handling (mock runner errors, session failures)
- [x] BD Activity parsing and watcher lifecycle (bdactivity) - Phase 3
- [x] ProcessRunner interface and mock (runner) - Phase 3
- [x] TUI view rendering, update logic, event formatting (tui) - Phase 4
- [x] YAML config loading and precedence (config/loader) - Phase 5
- [x] Prompt template expansion and loading (config/prompt) - Phase 5
- [x] Init command file operations (init) - Phase 5
- [x] Observer Ask, Cancel, Reset (observer) - Phase 6
- [x] SessionBroker coordination (observer) - Phase 6
- [x] ContextBuilder and LogReader (observer) - Phase 6
- [x] limitedWriter output truncation (observer) - Phase 6
- [x] Graph visualizer and layout (tui/graph) - Phase 7
- [x] Graph pane panel integration (tui/graph_pane) - Phase 7
- [x] BeadFetcher bd list wrapper (tui/fetcher) - Phase 7
- [x] Detail modal component (tui/modal) - Phase 7

Run with: `mise run test`

### Integration Tests - IMPLEMENTED

- [x] Full drain cycle with mock claude/bd (`TestFullDrainCycle`)
- [x] Multiple bead processing (`TestDrainWithMultipleBeads`)
- [x] Failed bead handling (`TestDrainWithFailedBead`)
- [x] Graceful shutdown (`TestGracefulShutdown`)
- [x] Backoff progression (`TestBackoffProgression`)
- [x] Context cancellation (`TestContextCancellation`)
- [x] Pause/resume behavior (`TestPauseResumeDuringDrain`)
- [x] Daemon lifecycle (`TestDaemonLifecycle_WithController`) - Phase 2
- [x] Daemon pause/resume (`TestDaemonPauseResume_WithController`) - Phase 2
- [x] Daemon graceful stop (`TestDaemonGracefulStop`) - Phase 2
- [x] Daemon force stop (`TestDaemonForceStop`) - Phase 2
- [x] Daemon status with stats (`TestDaemonStatus_Stats`) - Phase 2
- [x] BD Activity watcher lifecycle (start, stop, reconnect) - Phase 3
- [x] BD Activity parser (all mutation types) - Phase 3
- [x] BD Activity event flow through router - Phase 3
- [x] Observer basic query with mock claude (`TestObserverBasicQuery`) - Phase 6
- [x] Observer broker coordination (`TestObserverBrokerCoordination`) - Phase 6
- [x] Observer cancel and timeout (`TestObserverCancel`, `TestObserverTimeout`) - Phase 6
- [x] Observer context building (`TestObserverContextIncludesLogEvents`) - Phase 6
- [x] Observer session history (`TestObserverSessionHistory`) - Phase 6
- [x] Observer error handling (`TestObserverErrorFromClaude`) - Phase 6
- [x] Observer model configuration (`TestObserverModelConfiguration`) - Phase 6
- [x] Observer edge cases (`TestObserverEmptyLog`, `TestObserverNoLogFile`) - Phase 6
- [x] Graph pane integration with TUI model - Phase 7
- [x] Graph data fetching and rendering - Phase 7
- [x] Detail modal keyboard navigation - Phase 7
- [x] Panel toggle system (events/observer/graph) - Phase 7

Run with: `go test -v ./internal/integration/...` or `go test -v ./internal/daemon/...` or `go test -v ./internal/bdactivity/...` or `go test -v ./internal/tui/...` or `go test -v ./internal/observer/...`

### End-to-End Tests - PARTIAL

- [x] Real drain on test project with dummy beads (manual testing done)
- [x] TUI unit tests (view, update, format, fallback) - Phase 4
- [x] CLI wiring tests (TUI logger configuration) - Phase 4
- [x] TUI teatest integration tests (lifecycle, events, callbacks, observer, graph) - Phase 7
- [ ] Long-running stability test

### TUI Integration Testing - IMPLEMENTED

Thorough TUI integration testing was added in Phase 7 using multiple approaches:

1. **Bubbletea's `teatest` package**: Provides headless TUI testing with programmatic key sending and output assertions. Used for testing keyboard handling and view updates in isolation. Implemented in `internal/tui/*_integration_test.go`.

2. **CLI integration tests**: CLI wiring tests added in Phase 4 catch integration issues between main.go and the TUI package.

3. **Simulated event streams**: Integration tests feed controlled sequences of events through the TUI and verify display updates correctly. Tests cover rapid events, panel toggling, and scroll behavior.

Future considerations:
- Screenshot/golden file testing for visual regression testing
- Long-running stability tests

---

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Claude output format changes | Version check, graceful degradation |
| bd activity format changes | Version check, warn on unknown fields |
| State file corruption | JSON validation, backup before write |
| Runaway Claude sessions | --max-turns limit, timeout watchdog |
| Socket permission issues | Clear error message, suggest fix |

---

## Definition of Done

The project is complete when:

1. [x] `atari start` can process all ready beads autonomously - **Phase 1**
2. [x] State persists across restarts - **Phase 1**
3. [x] Pause/resume/stop work correctly - **Phase 2** (via daemon socket)
4. [x] TUI provides good visibility into progress - **Phase 4**
5. [x] Observer mode allows interactive Q&A - **Phase 6**
6. [x] Failed beads don't block forever (backoff) - **Phase 1**
7. [x] `atari init` onboards new users easily - **Phase 5**
8. [ ] Documentation is complete - Blocking (core docs match features before milestone)
9. [x] Works on macOS and Linux - **Phase 1** (tested on macOS)
10. [ ] Notifications alert on key events - Phase 8 (optional)
