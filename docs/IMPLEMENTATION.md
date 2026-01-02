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
| 2 | Control & Monitoring | Daemon mode, pause/resume/stop | Not started |
| 3 | BD Activity | Unified event stream | Not started |
| 4 | Terminal UI | Bubbletea TUI | Not started |
| 5 | Notifications | Webhooks, IFTTT, Slack | Not started |
| 6 | Polish | Backoff, config, docs | Partial (backoff done) |

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
11. [x] Agent state reporting (configurable via `--agent-id` flag)

### Success Criteria

- [x] `atari start` processes all ready beads
- [x] Logs written to `.atari/atari.log`
- [x] State persisted to `.atari/state.json`
- [x] Graceful shutdown on Ctrl+C
- [x] Recovers state on restart
- [x] Reports agent state to beads via `bd agent state` (when `--agent-id` configured)

### Notes

- Backoff implementation was pulled forward from Phase 6 into the workqueue
- Agent state reporting requires creating an agent bead and passing `--agent-id bd-xxx`
- Integration tests use mock Claude script for reliable testing
- All tests pass: `mise run test` (7 packages, 100+ tests)

---

## Phase 2: Control & Monitoring

**Goal**: Add daemon mode with external control.

### Components to Implement

| Component | Documentation |
|-----------|---------------|
| Daemon | [components/daemon.md](components/daemon.md) |
| Controller (full state machine) | [components/controller.md](components/controller.md) |

### CLI Commands

- `atari start --daemon`
- `atari status`
- `atari pause`
- `atari resume`
- `atari stop`
- `atari events`

### Tasks

1. Daemonize process (fork, setsid)
2. PID file management
3. Unix socket listener
4. JSON-RPC protocol
5. Implement pause/resume state transitions
6. Status command with stats
7. Events command (tail log file)

### Success Criteria

- [ ] Can start daemon, pause, resume, stop via CLI
- [ ] Status command shows current state and stats
- [ ] Events command can tail the event stream

---

## Phase 3: BD Activity Integration

**Goal**: Unified event stream with bd activity.

### Components to Implement

| Component | Documentation |
|-----------|---------------|
| BD Activity Watcher | [components/bdactivity.md](components/bdactivity.md) |

### Tasks

1. Spawn `bd activity --follow --json`
2. Parse mutation events
3. Convert to unified event format
4. Merge into event stream
5. Handle reconnection on failure

### Success Criteria

- [ ] Bead status changes appear in event stream
- [ ] `atari events` shows unified claude + bd events
- [ ] Reconnects automatically on bd activity failure

---

## Phase 4: Terminal UI

**Goal**: Rich terminal interface for monitoring.

### Components to Implement

| Component | Documentation |
|-----------|---------------|
| TUI | [components/tui.md](components/tui.md) |
| Observer (future) | [components/observer.md](components/observer.md) |

### Tasks

1. Bubbletea model and update loop
2. Header component (status, stats)
3. Event feed component (scrollable)
4. Footer component (keyboard help)
5. Keyboard handling (p/r/q, arrows, o for observer)
6. Graceful degradation (no TTY)

### Success Criteria

- [ ] TUI displays current state and events
- [ ] Keyboard controls work (pause, resume, quit)
- [ ] Scrolling works for event history
- [ ] Falls back to simple output when no TTY

**Future**: Observer Mode - interactive Q&A pane for asking questions about events. See [observer.md](components/observer.md).

---

## Phase 5: Notifications

**Goal**: External alerts for key events.

### Components to Implement

| Component | Documentation |
|-----------|---------------|
| Notifications | [components/notifications.md](components/notifications.md) |

### Tasks

1. Notification sink (event consumer)
2. IFTTT provider
3. Slack provider
4. Discord provider
5. Generic webhook provider
6. Rate limiting
7. Retry logic

### Success Criteria

- [ ] IFTTT notifications work
- [ ] Slack notifications work
- [ ] Configurable triggers per provider
- [ ] Rate limiting prevents spam

---

## Phase 6: Polish & Init

**Goal**: Production-ready reliability and onboarding.

**Status**: Partial (backoff and env vars done in Phase 1)

### Components to Implement

| Component | Documentation |
|-----------|---------------|
| Init Command | [cli/init-command.md](cli/init-command.md) |
| Configuration | [config/configuration.md](config/configuration.md) |

### Tasks

1. [x] Exponential backoff for failed beads - **Done in Phase 1**
2. [ ] YAML config file parsing
3. [x] Environment variable overrides - **Done in Phase 1** (via Viper)
4. [ ] Custom prompt templates
5. [ ] `atari init` command
6. [ ] User guide documentation
7. [ ] Error messages and suggestions

### Success Criteria

- [x] Failed beads don't block drain indefinitely (backoff + max failures)
- [ ] Configuration works from file and env
- [ ] `atari init` sets up Claude Code correctly
- [ ] Documentation is complete

---

## File Structure

Current structure (Phase 1 complete):

```
atari/
├── cmd/atari/
│   ├── main.go              # Root command, all subcommands, flag definitions
│   ├── config.go            # Flag name constants for Viper binding
│   └── CLAUDE.md            # CLI package documentation
├── internal/
│   ├── config/              # Configuration types and defaults
│   │   ├── config.go
│   │   ├── config_test.go
│   │   └── CLAUDE.md
│   ├── controller/          # Main orchestration loop
│   │   ├── controller.go
│   │   ├── controller_test.go
│   │   └── CLAUDE.md
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
│   │   └── CLAUDE.md
│   ├── workqueue/           # Work discovery and selection
│   │   ├── poll.go          # bd ready polling
│   │   ├── poll_test.go
│   │   ├── queue.go         # Selection, backoff, history
│   │   ├── queue_test.go
│   │   └── CLAUDE.md
│   ├── integration/         # End-to-end tests
│   │   ├── drain_test.go
│   │   └── CLAUDE.md
│   ├── bdactivity/          # [Phase 3] BD activity stream watcher
│   ├── daemon/              # [Phase 2] Daemon mode and RPC
│   └── tui/                 # [Phase 4] Terminal UI (bubbletea)
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

Run with: `mise run test`

### Integration Tests - IMPLEMENTED

- [x] Full drain cycle with mock claude/bd (`TestFullDrainCycle`)
- [x] Multiple bead processing (`TestDrainWithMultipleBeads`)
- [x] Failed bead handling (`TestDrainWithFailedBead`)
- [x] Graceful shutdown (`TestGracefulShutdown`)
- [x] Backoff progression (`TestBackoffProgression`)
- [x] Context cancellation (`TestContextCancellation`)
- [x] Pause/resume behavior (`TestPauseResumeDuringDrain`)

Run with: `go test -v ./internal/integration/...`

### End-to-End Tests - PARTIAL

- [x] Real drain on test project with dummy beads (manual testing done)
- [ ] TUI interaction tests (Phase 4)
- [ ] Long-running stability test

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
3. [ ] Pause/resume/stop work correctly - Phase 2 (via daemon socket)
4. [ ] TUI provides good visibility into progress - Phase 4
5. [ ] Notifications alert on key events - Phase 5
6. [x] Failed beads don't block forever (backoff) - **Phase 1**
7. [ ] `atari init` onboards new users easily - Phase 6
8. [ ] Documentation is complete - Ongoing
9. [x] Works on macOS and Linux - **Phase 1** (tested on macOS)
