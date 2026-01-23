# Atari - Applied Training: Automatic Research & Implementation

A daemon controller that orchestrates Claude Code sessions to automatically work through beads (br) issues.

## Quick Summary

**What we're building**: A Go daemon that:
- Polls `br ready` for available beads
- Spawns Claude Code sessions (`claude -p --output-format stream-json`)
- Streams unified events (Claude + bead activity via file watching)
- Persists state for pause/resume
- Provides optional TUI for monitoring

**Key integration points**:
- Claude Code: `claude -p --output-format stream-json --max-turns N`
- Work discovery: `br ready --json` to find available beads
- Activity monitoring: File watcher on `.beads/issues.jsonl` for real-time changes

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
  daemon/           # Daemon mode and RPC (see internal/daemon/CLAUDE.md)
  runner/           # ProcessRunner interface for streaming processes
  bdactivity/       # JSONL file watcher for bead changes (see internal/bdactivity/CLAUDE.md)
  tui/              # Terminal UI (bubbletea)
docs/               # User documentation
```

## Key Design Decisions

1. **Single worker**: One Claude session at a time (no parallel execution)
2. **Fresh sessions**: Each bead gets a new Claude session (no resume across beads)
3. **State file**: JSON state persisted to `.atari/state.json`
4. **Unix socket**: Daemon control via `.atari/atari.sock`
5. **Event-driven**: All significant actions emit events to unified stream

## Bead Creation Boundary

After creating a bead (via /bd-create skill OR manual `br create`):
- Report the bead ID
- Return to previous task IMMEDIATELY
- Do NOT start working on the newly created bead
- Do NOT investigate, edit files, or implement anything for it

The bead will be picked up later by atari or worked on in a future session.
Exception: Only continue working if user explicitly says "and work on it now".

## Testing

When implementing, always:
1. Write unit tests for new functions
2. Test state transitions manually
3. Verify graceful shutdown behavior
4. Check state recovery after simulated crash

Validate changes with `mise run bake` for end-to-end verification (runs tests + builds container).
