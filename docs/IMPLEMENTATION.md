# Implementation Plan

Phased implementation approach for Atari.

For detailed component specifications, see the [components/](components/) directory.

## Technology Stack

- **Language**: Go
- **CLI Framework**: Cobra + Viper
- **TUI Framework**: Bubbletea + Lipgloss
- **Configuration**: YAML (gopkg.in/yaml.v3)

## Phase Overview

| Phase | Focus | Key Deliverables |
|-------|-------|------------------|
| 1 | Core Loop (MVP) | Poll, spawn, log, persist |
| 2 | Control & Monitoring | Daemon mode, pause/resume/stop |
| 3 | BD Activity | Unified event stream |
| 4 | Terminal UI | Bubbletea TUI |
| 5 | Notifications | Webhooks, IFTTT, Slack |
| 6 | Polish | Backoff, config, docs |

---

## Phase 1: Core Loop (MVP)

**Goal**: Minimal working drain that can run unattended.

### Components to Implement

| Component | Documentation |
|-----------|---------------|
| Controller (basic) | [components/controller.md](components/controller.md) |
| Work Queue | [components/workqueue.md](components/workqueue.md) |
| Session Manager | [components/session.md](components/session.md) |
| Event Router | [components/events.md](components/events.md) |
| Log Sink | [components/sinks.md](components/sinks.md) |
| State Sink | [components/sinks.md](components/sinks.md) |

### CLI Commands

- `atari start` (foreground only)
- `atari version`

### Tasks

1. Project setup (go mod, cobra structure, Makefile)
2. Core types (Event, State, Config, Bead)
3. Work Queue Manager (bd ready polling, bead selection)
4. Session Manager (spawn claude, parse stream-json)
5. Event Router (channel-based pub/sub)
6. Log Sink (JSON lines file)
7. State Sink (state.json persistence)
8. Controller main loop
9. Signal handling (SIGINT, SIGTERM)
10. Stuck issue reset after each session
11. Agent state reporting (`bd agent state atari <state>`)

### Success Criteria

- [ ] `atari start` processes all ready beads
- [ ] Logs written to `.atari/atari.log`
- [ ] State persisted to `.atari/state.json`
- [ ] Graceful shutdown on Ctrl+C
- [ ] Recovers state on restart
- [ ] Reports agent state to beads via `bd agent state`

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

### Components to Implement

| Component | Documentation |
|-----------|---------------|
| Init Command | [cli/init-command.md](cli/init-command.md) |
| Configuration | [config/configuration.md](config/configuration.md) |

### Tasks

1. Exponential backoff for failed beads
2. YAML config file parsing
3. Environment variable overrides
4. Custom prompt templates
5. `atari init` command
6. User guide documentation
7. Error messages and suggestions

### Success Criteria

- [ ] Failed beads don't block drain indefinitely
- [ ] Configuration works from file and env
- [ ] `atari init` sets up Claude Code correctly
- [ ] Documentation is complete

---

## File Structure

```
atari/
├── cmd/atari/
│   ├── main.go
│   ├── start.go
│   ├── stop.go
│   ├── pause.go
│   ├── resume.go
│   ├── status.go
│   ├── events.go
│   ├── init.go
│   └── version.go
├── internal/
│   ├── controller/
│   │   ├── controller.go
│   │   └── state.go
│   ├── workqueue/
│   │   ├── queue.go
│   │   └── backoff.go
│   ├── session/
│   │   ├── manager.go
│   │   └── parser.go
│   ├── events/
│   │   ├── router.go
│   │   ├── types.go
│   │   └── sinks.go
│   ├── bdactivity/
│   │   └── watcher.go
│   ├── daemon/
│   │   ├── daemon.go
│   │   └── rpc.go
│   ├── tui/
│   │   ├── model.go
│   │   ├── view.go
│   │   └── styles.go
│   ├── notifications/
│   │   ├── notifier.go
│   │   ├── ifttt.go
│   │   ├── slack.go
│   │   └── webhook.go
│   ├── config/
│   │   ├── config.go
│   │   └── defaults.go
│   └── init/
│       ├── init.go
│       └── templates/
├── docs/
│   ├── CONTEXT.md
│   ├── DESIGN.md
│   ├── IMPLEMENTATION.md
│   ├── USER_GUIDE.md
│   ├── components/
│   ├── cli/
│   └── config/
├── .atari/              # Runtime (gitignored)
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## Testing Strategy

### Unit Tests

Each component should have unit tests for:
- Core logic and state transitions
- Event parsing and formatting
- Configuration loading and validation
- Error handling

### Integration Tests

- Full drain cycle with mock claude/bd
- Daemon start/stop lifecycle
- Pause/resume behavior
- State recovery after simulated crash

### End-to-End Tests

- Real drain on test project with dummy beads
- TUI interaction tests
- Long-running stability test

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

1. `atari start` can process all ready beads autonomously
2. State persists across restarts
3. Pause/resume/stop work correctly
4. TUI provides good visibility into progress
5. Notifications alert on key events
6. Failed beads don't block forever (backoff)
7. `atari init` onboards new users easily
8. Documentation is complete
9. Works on macOS and Linux
