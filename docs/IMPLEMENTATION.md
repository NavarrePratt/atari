# Implementation Plan

This document outlines the phased implementation approach for atari.

## Technology Choices

**Language: Go**

Rationale:
- Single binary distribution (no runtime dependencies)
- Excellent process management and concurrency
- Good CLI library ecosystem (cobra, bubbletea)
- Consistent with bd (beads) which is also Go
- Fast startup time for daemon

**Key Dependencies:**
- `github.com/spf13/cobra` - CLI framework
- `github.com/charmbracelet/bubbletea` - TUI framework (optional)
- `github.com/charmbracelet/lipgloss` - TUI styling
- `gopkg.in/yaml.v3` - Config parsing

---

## Phase 1: Core Loop (MVP)

**Goal**: Minimal working drain that can run unattended.

### Deliverables

1. **Main loop** that:
   - Polls `bd ready --json`
   - Spawns `claude -p` for each bead
   - Parses stream-json output
   - Logs to file
   - Resets stuck issues after each session

2. **Basic CLI**:
   - `atari start` - Run in foreground
   - `atari version`

3. **State persistence**:
   - Write state.json after each iteration
   - Recover on startup

### Implementation Tasks

```
Phase 1 Tasks:
├── Project setup
│   ├── Initialize Go module
│   ├── Set up cobra CLI structure
│   └── Create Makefile with build/install targets
│
├── Core types
│   ├── Define Event types
│   ├── Define State types
│   ├── Define Config types
│   └── Define Bead/BeadHistory types
│
├── Work Queue Manager
│   ├── Implement bd ready polling
│   ├── Parse JSON output
│   ├── Track bead history
│   └── Select next bead logic
│
├── Session Manager
│   ├── Spawn claude process
│   ├── Stream stdout parsing
│   ├── Handle process exit
│   └── Extract session stats (cost, turns, duration)
│
├── Event Router
│   ├── Create event channel
│   ├── Route claude events
│   └── Add internal events
│
├── Log Sink
│   ├── JSON lines writer
│   ├── Log rotation on startup
│   └── Configurable path
│
├── State Management
│   ├── State file read/write
│   ├── Recovery logic
│   └── Stats tracking
│
├── Stuck Issue Reset
│   ├── Port _bd_reset_stuck_issues logic
│   └── Run after each session
│
└── Integration
    ├── Wire components together
    ├── Main control loop
    └── Signal handling (SIGINT, SIGTERM)
```

### Success Criteria

- [x] Can run `atari start` and it processes all ready beads
- [x] Logs written to file in JSON lines format
- [x] State persisted and recovered on restart
- [x] Graceful shutdown on Ctrl+C

---

## Phase 2: Control & Monitoring

**Goal**: Add daemon mode with external control.

### Deliverables

1. **Daemon mode**:
   - Run in background
   - Unix socket for IPC
   - PID file management

2. **Control commands**:
   - `atari status` - Show current state
   - `atari pause` - Pause after current bead
   - `atari resume` - Resume from pause
   - `atari stop` - Stop daemon

3. **Event streaming**:
   - `atari events --follow` - Tail event log

### Implementation Tasks

```
Phase 2 Tasks:
├── Daemon mode
│   ├── Daemonize process
│   ├── PID file management
│   ├── Unix socket listener
│   └── JSON-RPC protocol
│
├── Control commands
│   ├── Implement status command
│   ├── Implement pause command
│   ├── Implement resume command
│   └── Implement stop command
│
├── State machine
│   ├── Add paused state
│   ├── Add stopping state
│   ├── Transition logic
│   └── State persistence updates
│
├── Event streaming
│   ├── Implement events command
│   ├── Follow mode (tail -f style)
│   └── Count/filter options
│
└── Testing
    ├── Unit tests for state machine
    ├── Integration test for daemon lifecycle
    └── Test pause/resume behavior
```

### Success Criteria

- [x] Can start daemon, pause, resume, stop via CLI
- [x] Status command shows current state and stats
- [x] Events command can tail the event stream

---

## Phase 3: BD Activity Integration

**Goal**: Unified event stream with bd activity.

### Deliverables

1. **BD Activity Stream**:
   - Run `bd activity --follow --json` in background
   - Parse mutation events
   - Merge into event stream

2. **Enhanced Event Display**:
   - Bead status changes visible in real-time
   - Color-coded event types
   - Timestamps and symbols

### Implementation Tasks

```
Phase 3 Tasks:
├── BD Activity Manager
│   ├── Spawn bd activity process
│   ├── Parse JSON output
│   ├── Handle process lifecycle
│   └── Reconnect on failure
│
├── Event merging
│   ├── Unified event format
│   ├── Source tagging (claude vs bd vs internal)
│   └── Chronological ordering
│
├── Enhanced logging
│   ├── Human-readable format option
│   ├── Color support (detect TTY)
│   └── Symbol/emoji indicators
│
└── Testing
    ├── Test bd activity parsing
    ├── Test event merging
    └── Test display formatting
```

### Success Criteria

- [x] Bead status changes appear in event stream
- [x] `atari events` shows unified claude + bd events
- [x] Events are color-coded and easy to read

---

## Phase 4: Terminal UI

**Goal**: Rich terminal interface for monitoring.

### Deliverables

1. **TUI mode** (`atari start --tui`):
   - Current bead display
   - Live event feed
   - Stats panel
   - Keyboard controls

2. **Layout**:
   ```
   ┌─ ATARI ─────────────────────────────────────────────────┐
   │ Status: WORKING                      Cost: $2.35        │
   │ Current: bd-042 "Fix auth bug"       Turns: 42          │
   │ Progress: 4 completed, 1 failed, 3 remaining            │
   ├─ Events ────────────────────────────────────────────────┤
   │ 14:23:45 $ go test ./...                                │
   │ 14:23:50 ✓ BEAD bd-042 closed                          │
   │ 14:23:51 📋 BEAD bd-043 "Add rate limiting"            │
   │ 14:23:52 🚀 SESSION started                             │
   │ 14:23:54 📄 Read: src/ratelimit.go                     │
   │ ...                                                     │
   ├─────────────────────────────────────────────────────────┤
   │ [p] pause  [r] resume  [q] quit                         │
   └─────────────────────────────────────────────────────────┘
   ```

### Implementation Tasks

```
Phase 4 Tasks:
├── TUI framework setup
│   ├── Bubbletea model
│   ├── View components
│   └── Update handlers
│
├── Layout components
│   ├── Header (status, stats)
│   ├── Event feed (scrollable)
│   ├── Footer (keyboard help)
│   └── Responsive sizing
│
├── Keyboard handling
│   ├── p = pause
│   ├── r = resume
│   ├── q = quit
│   └── Arrow keys for scrolling
│
├── Event feed
│   ├── Ring buffer for events
│   ├── Auto-scroll to bottom
│   ├── Manual scroll mode
│   └── Color formatting
│
└── Integration
    ├── TUI flag handling
    ├── Graceful degradation (no TTY)
    └── Testing on different terminal sizes
```

### Success Criteria

- [x] TUI displays current state and events
- [x] Keyboard controls work
- [x] Scrolling works for event history
- [x] Graceful exit on q

---

## Phase 5: Polish & Edge Cases

**Goal**: Production-ready reliability.

### Deliverables

1. **Backoff logic** for failed beads
2. **Configuration file** support
3. **Custom prompt templates**
4. **Better error messages**
5. **Documentation**

### Implementation Tasks

```
Phase 5 Tasks:
├── Backoff implementation
│   ├── Exponential backoff for failures
│   ├── Max attempts before skip
│   ├── Backoff reset on success
│   └── Configurable parameters
│
├── Configuration
│   ├── YAML config file parsing
│   ├── Config file discovery
│   ├── Environment variable overrides
│   └── Config validation
│
├── Prompt templates
│   ├── Default embedded template
│   ├── Custom template file support
│   ├── Template variables
│   └── Template validation
│
├── Error handling
│   ├── Better error messages
│   ├── Suggestions for common issues
│   ├── Debug logging flag
│   └── Error codes for scripting
│
├── Documentation
│   ├── README with examples
│   ├── Man page generation
│   ├── --help improvements
│   └── Troubleshooting guide
│
└── Testing
    ├── End-to-end tests
    ├── Edge case tests
    ├── Performance testing
    └── CI/CD setup
```

### Success Criteria

- [x] Failed beads don't block drain indefinitely
- [x] Configuration works from file and env
- [x] Custom prompts can be used
- [x] Errors are clear and actionable

---

## File Structure

```
atari/
├── cmd/
│   └── atari/
│       └── main.go           # Entry point
├── internal/
│   ├── controller/
│   │   ├── controller.go     # Main orchestration
│   │   └── state.go          # State machine
│   ├── workqueue/
│   │   ├── queue.go          # Work queue manager
│   │   └── backoff.go        # Backoff logic
│   ├── session/
│   │   ├── manager.go        # Session manager
│   │   └── parser.go         # Stream-json parser
│   ├── events/
│   │   ├── router.go         # Event router
│   │   ├── types.go          # Event types
│   │   └── sinks.go          # Log, TUI, State sinks
│   ├── bdactivity/
│   │   └── watcher.go        # BD activity stream
│   ├── daemon/
│   │   ├── daemon.go         # Daemon mode
│   │   └── rpc.go            # Unix socket RPC
│   ├── tui/
│   │   ├── model.go          # Bubbletea model
│   │   ├── view.go           # View rendering
│   │   └── styles.go         # Lipgloss styles
│   └── config/
│       ├── config.go         # Config loading
│       └── defaults.go       # Default values
├── docs/
│   ├── CONTEXT.md            # Background research
│   ├── DESIGN.md             # Architecture
│   └── IMPLEMENTATION.md     # This file
├── .atari/                   # Runtime directory (gitignored)
│   ├── state.json
│   ├── atari.log
│   └── atari.sock
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## Testing Strategy

### Unit Tests

- State machine transitions
- Event parsing (claude, bd)
- Backoff calculations
- Config loading

### Integration Tests

- Full drain cycle with mock claude/bd
- Daemon start/stop lifecycle
- Pause/resume behavior
- Recovery from state file

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
5. Failed beads don't block forever (backoff)
6. Documentation is complete
7. Works on macOS and Linux
