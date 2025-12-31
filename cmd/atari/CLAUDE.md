# CLI Package

CLI entrypoint using Cobra and Viper for command-line parsing and configuration binding.

## Commands

| Command | Status | Description |
|---------|--------|-------------|
| `version` | Implemented | Print version information |
| `start` | Stubbed | Start the drain daemon |
| `status` | Stubbed | Show daemon status via socket |
| `pause` | Stubbed | Pause after current bead completes |
| `resume` | Stubbed | Resume from paused state |
| `stop` | Stubbed | Stop the daemon |
| `events` | Stubbed | View recent events |

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

## Integration Points

- `internal/controller` - Start command will instantiate and run the controller
- `internal/config` - Flag values map to config.Config struct
- `internal/daemon` (Phase 2) - Socket commands for status/pause/resume/stop
