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

1. **Phase 1 (MVP)**: Core loop - poll bd ready, spawn claude, log events, persist state
2. **Phase 2**: Daemon mode with pause/resume/stop control via Unix socket
3. **Phase 3**: BD activity integration for unified event stream
4. **Phase 4**: Terminal UI with bubbletea
5. **Phase 5**: Polish - backoff, config files, custom prompts

## Code Style

- Follow standard Go conventions (Cobra + Viper for CLI)
- Use `internal/` for non-exported packages
- Use `log/slog` for structured logging
- Keep functions small and testable
- Add tests for new functionality
- Use meaningful error messages

## Directory Structure

```
cmd/atari/          # CLI entrypoint (Cobra/Viper)
  main.go           # Root command and subcommands
  config.go         # Flag constants
internal/           # Non-exported packages
  controller/       # Main orchestration loop
  workqueue/        # Work discovery and selection
  session/          # Claude process management
  events/           # Event routing and sinks
  bdactivity/       # BD activity stream watcher
  daemon/           # Daemon mode and RPC
  tui/              # Terminal UI (bubbletea)
  config/           # Configuration loading
  shutdown/         # Graceful shutdown helper
docs/               # Design and implementation docs
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

## References

- Beads repo: `~/.cache/claude/repos/steveyegge/beads/`
- Template patterns: `~/.cache/claude/repos/abatilo/template/`
