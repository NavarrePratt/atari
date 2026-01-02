# Atari - Applied Training: Automatic Research & Implementation

A daemon controller that orchestrates Claude Code sessions to automatically work through beads (bd) issues.

## Project Context

Read these documents in order for full context:
1. `docs/CONTEXT.md` - Background research on Claude Code and beads capabilities
2. `docs/DESIGN.md` - Architecture and design decisions
3. `docs/IMPLEMENTATION.md` - Phased implementation plan

## Quick Summary

**What we're building**: A Go daemon that:
- Polls `bd ready` for available beads
- Spawns Claude Code sessions (`claude -p --output-format stream-json`)
- Streams unified events (Claude + bd activity)
- Persists state for pause/resume
- Provides optional TUI for monitoring

**Key integration points**:
- Claude Code: `claude -p --output-format stream-json --max-turns N`
- BD ready: `bd ready --json` for work discovery
- BD activity: `bd activity --follow --json` for real-time bead events
- BD agent: `bd agent state atari <state>` for tracking

## Development Commands

All commands use mise (tool version manager). See `.mise.toml` for tool versions and task definitions.

```bash
# Build
mise run build

# Run
./atari start

# Test
mise run test

# Install locally
mise run install

# Lint
mise run lint

# Format
mise run fmt

# Build container
mise run bake

# Development with hot-reload (requires reflex)
mise run dev

# Or use raw Go commands
go build -o atari ./cmd/atari
go test ./...
```

## Implementation Phases

See `docs/IMPLEMENTATION.md` for detailed specs.

| Phase | Focus | Status |
|-------|-------|--------|
| 1 | Core Loop (MVP) | Complete - workqueue, session, events, sinks, controller done |
| 2 | Control & Monitoring | Not started |
| 3 | BD Activity | Not started |
| 4 | Terminal UI | Not started |
| 5 | Notifications | Not started |
| 6 | Polish & Init | Partially done (backoff in workqueue) |

## Code Style

- Follow standard Go conventions (Cobra + Viper for CLI)
- Use `internal/` for non-exported packages
- Use `log/slog` for structured logging
- Keep functions small and testable
- Add tests for new functionality
- Use meaningful error messages

## ANSI Escape Code Prevention

The `.beads/issues.jsonl` file contains ANSI escape codes in some issue descriptions from earlier sessions. When creating issues or commits:

- **Never copy text directly** from colored terminal output or existing issue descriptions
- **Write from scratch** - paraphrase rather than copy verbatim
- ANSI codes (`\x1b[`, `^[[`) appear as garbage characters in git logs

## Directory Structure

```
cmd/atari/          # CLI entrypoint (see cmd/atari/CLAUDE.md)
  main.go           # Root command and subcommands
  config.go         # Flag constants
internal/           # Non-exported packages
  config/           # Configuration types and defaults (see internal/config/CLAUDE.md)
  events/           # Event types, router, sinks (see internal/events/CLAUDE.md)
  shutdown/         # Graceful shutdown helper (see internal/shutdown/CLAUDE.md)
  testutil/         # Test infrastructure: mocks, fixtures (see internal/testutil/CLAUDE.md)
  session/          # Claude process lifecycle (see internal/session/CLAUDE.md)
  workqueue/        # Work discovery and selection (see internal/workqueue/CLAUDE.md)
  controller/       # Main orchestration loop (see internal/controller/CLAUDE.md)
  integration/      # End-to-end tests (see internal/integration/CLAUDE.md)
  bdactivity/       # [planned] BD activity stream watcher
  daemon/           # [planned] Daemon mode and RPC
  tui/              # [planned] Terminal UI (bubbletea)
docs/               # Design and implementation docs
  components/       # Detailed component specifications
  cli/              # CLI command documentation
  config/           # Configuration file formats
```

## Key Design Decisions

1. **Single worker**: One Claude session at a time (no parallel execution)
2. **Fresh sessions**: Each bead gets a new Claude session (no resume across beads)
3. **State file**: JSON state persisted to `.atari/state.json`
4. **Unix socket**: Daemon control via `.atari/atari.sock`
5. **Event-driven**: All significant actions emit events to unified stream

## Testing

When implementing, always:
1. Write unit tests for new functions
2. Test state transitions manually
3. Verify graceful shutdown behavior
4. Check state recovery after simulated crash

Validate changes with `mise run bake` for end-to-end verification (runs tests + builds container).
