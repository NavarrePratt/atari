# atari

**Applied Training: Automatic Research & Implementation**

A daemon that orchestrates Claude Code sessions to automatically work through [beads](https://github.com/Dicklesworthstone/beads_rust) issues. Create beads for your planned work, start atari, and let it autonomously process ready issues until completion.

## Prerequisites

- [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code) - `claude` command must be installed and authenticated
- [beads_rust](https://github.com/Dicklesworthstone/beads_rust) - `br` command for issue tracking (`cargo install beads_rust`)
- A project with `.beads/` initialized (`br init`)

## Quick Start

```bash
# Initialize atari configuration (sets up Claude Code integration)
atari init

# Start processing beads
atari start

# Or run as background daemon with TUI
atari start --daemon --tui
```

## Basic Commands

```bash
atari start           # Start processing beads
atari status          # Show current state
atari pause           # Pause after current bead completes
atari resume          # Resume processing
atari stop            # Stop the daemon
atari events --follow # Watch events in real-time
```

## How It Works

1. Atari polls `br ready` for available beads
2. Spawns a Claude Code session for each bead
3. Tracks progress, costs, and failures
4. Retries failed beads with exponential backoff
5. Continues until no ready beads remain

**Note**: Atari processes one bead at a time (single worker). This ensures focused attention on each task and prevents resource contention.

## Configuration

Atari uses `.atari/` directory for state and configuration:

```
.atari/
├── config.yaml   # Configuration file
├── state.json    # Persistent state
├── atari.log     # Event log
└── atari.sock    # Daemon control socket
```

Configuration can be set via CLI flags, environment variables (`ATARI_*` prefix), or `.atari/config.yaml`.

See [docs/config/configuration.md](docs/config/configuration.md) for detailed configuration options.

## Documentation

- [Getting Started](docs/getting-started.md) - Installation and first run
- [Workflow Guide](docs/workflow.md) - Two-terminal planning workflow
- [TUI Guide](docs/tui.md) - Terminal UI features and keybinds
- [Configuration Reference](docs/config/configuration.md) - All configuration options

## Development

Requires [mise](https://mise.jdx.dev/) for tool version management.

```bash
mise run build    # Build binary
mise run test     # Run tests
mise run lint     # Lint code
mise run fmt      # Format code
mise run install  # Install locally
mise run bake     # Build container
mise run dev      # Hot-reload development
```

Or use raw Go commands:

```bash
go build -o atari ./cmd/atari
go test ./...
```

### Requirements

- Go 1.25+
- Claude Code CLI
- beads_rust CLI

---

[GitHub](https://github.com/navarrepratt/atari)
