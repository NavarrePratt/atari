# Backend Patterns (cmd/)

Use this as the playbook for building the atari CLI. It follows the Cobra + Viper pattern for configuration and command structure.

## Directory shape

```
cmd/atari/
├── main.go          # Cobra/Viper setup, root command
├── config.go        # Flag/env key constants
├── start.go         # Start command implementation
├── status.go        # Status command implementation
├── pause.go         # Pause command implementation
├── resume.go        # Resume command implementation
└── stop.go          # Stop command implementation
```

## Configuration (Cobra + Viper)

- Set env prefix `ATARI` and replace `-` with `_` for env vars.
- Define flags on Cobra commands; bind every flag to Viper once.
- Example skeleton:

```go
viper.SetEnvPrefix("ATARI")
viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
viper.AutomaticEnv()

cmd := &cobra.Command{Use: "atari", Short: "Applied Training: Automatic Research & Implementation"}
cmd.PersistentFlags().Bool(FlagVerbose, false, "Enable verbose logging")
cmd.PersistentFlags().String(FlagLogFile, ".atari/atari.log", "Log file path")

cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { viper.BindPFlag(f.Name, f) })
```

## Command structure

- `atari start` - Start the daemon (foreground or background)
- `atari status` - Show current state and stats
- `atari pause` - Pause after current bead completes
- `atari resume` - Resume from paused state
- `atari stop` - Stop the daemon
- `atari events` - Tail the event stream
- `atari version` - Show version info

## Logging

- Use `slog` with JSON handler for structured logging.
- Toggle debug level via `--verbose` flag.
- Log to both file and stderr when running in foreground.

## Graceful shutdown

- Use `internal/shutdown.Gracefully()` pattern.
- Handle SIGINT/SIGTERM to complete current bead before exit.
- Persist state before shutdown for recovery.

## Daemon communication

- Unix socket at `.atari/atari.sock` for IPC.
- Simple JSON-RPC protocol for control commands.
- PID file at `.atari/atari.pid` for daemon detection.

## Adding commands

1. Create new file `cmd/atari/<command>.go`.
2. Define command with `&cobra.Command{Use: "<command>", ...}`.
3. Register in `main.go` via `rootCmd.AddCommand(<command>Cmd)`.
4. Add flags specific to the command; bind to Viper.
