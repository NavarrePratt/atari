# CLI Package

CLI entrypoint using Cobra and Viper for command-line parsing and configuration binding.

## Commands

| Command | Status | Description |
|---------|--------|-------------|
| `version` | Implemented | Print version information |
| `start` | Implemented | Start the drain loop (foreground or daemon) |
| `status` | Implemented | Show daemon status via socket |
| `pause` | Implemented | Pause after current bead completes |
| `resume` | Implemented | Resume from paused state |
| `stop` | Implemented | Stop the daemon |
| `events` | Implemented | View recent events |

## Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--verbose` | false | Enable debug logging |
| `--log-file` | .atari/atari.log | Log file path |
| `--state-file` | .atari/state.json | State file path |
| `--socket-path` | .atari/atari.sock | Unix socket path |

## Start Command Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--tui` | false | Enable terminal UI |
| `--max-turns` | 50 | Max turns per Claude session |
| `--label` | "" | Filter bd ready by label |
| `--prompt` | "" | Custom prompt template file |
| `--model` | opus | Claude model to use |
| `--agent-id` | "" | Agent bead ID for state reporting (e.g., bd-xxx) |

## Stop Command Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--graceful` | true | Wait for current bead to complete |

## Events Command Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--follow` | false | Follow event stream (like tail -f) |
| `--count` | 20 | Number of recent events to show |

## Configuration Binding

All flags are bound to Viper with:
- Environment prefix: `ATARI_`
- Hyphens replaced with underscores: `--log-file` -> `ATARI_LOG_FILE`
- Automatic env binding: `viper.AutomaticEnv()`

```go
// Example: override via environment
ATARI_MAX_TURNS=100 atari start

// Example: check value in code
maxTurns := viper.GetInt(FlagMaxTurns)
```

## Files

- `main.go` - Root command, subcommands, flag definitions
- `config.go` - Flag name constants for Viper binding

## Start Command Flow

The `start` command:
1. Creates .atari directory if needed
2. Initializes event router with default buffer size
3. Creates and starts LogSink and StateSink
4. Creates ExecRunner for real command execution
5. Creates workqueue.Manager for work discovery
6. Creates controller.Controller for orchestration
7. Runs controller with graceful shutdown handling (SIGINT/SIGTERM)
8. Cleans up sinks and router on exit

## Integration Points

- `internal/controller` - Start instantiates and runs the controller
- `internal/config` - Flag values map to config.Config struct
- `internal/events` - Router and sinks for event distribution
- `internal/workqueue` - Work discovery via bd ready
- `internal/shutdown` - Graceful shutdown handling
- `internal/testutil` - ExecRunner for command execution
- `internal/daemon` - Daemon mode, RPC server/client for status/pause/resume/stop
