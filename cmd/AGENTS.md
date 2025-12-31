# Backend Patterns (cmd/)

Use this as the playbook for building the bd-drain CLI. It follows the Cobra + Viper pattern for configuration and command structure.

## Directory shape

```
cmd/bd-drain/
├── main.go          # Cobra/Viper setup, root command
├── config.go        # Flag/env key constants
├── start.go         # Start command implementation
├── status.go        # Status command implementation
├── pause.go         # Pause command implementation
├── resume.go        # Resume command implementation
└── stop.go          # Stop command implementation
```

## Configuration (Cobra + Viper)

- Set env prefix `BD_DRAIN` and replace `-` with `_` for env vars.
- Define flags on Cobra commands; bind every flag to Viper once.
- Example skeleton:

```go
viper.SetEnvPrefix("BD_DRAIN")
viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
viper.AutomaticEnv()

cmd := &cobra.Command{Use: "bd-drain", Short: "Claude Code bead worker daemon"}
cmd.PersistentFlags().Bool(FlagVerbose, false, "Enable verbose logging")
cmd.PersistentFlags().String(FlagLogFile, ".bd-drain/drain.log", "Log file path")

cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { viper.BindPFlag(f.Name, f) })
```

## Command structure

- `bd-drain start` - Start the drain daemon (foreground or background)
- `bd-drain status` - Show current state and stats
- `bd-drain pause` - Pause after current bead completes
- `bd-drain resume` - Resume from paused state
- `bd-drain stop` - Stop the daemon
- `bd-drain events` - Tail the event stream
- `bd-drain version` - Show version info

## Logging

- Use `slog` with JSON handler for structured logging.
- Toggle debug level via `--verbose` flag.
- Log to both file and stderr when running in foreground.

## Graceful shutdown

- Use `internal/shutdown.Gracefully()` pattern.
- Handle SIGINT/SIGTERM to complete current bead before exit.
- Persist state before shutdown for recovery.

## Daemon communication

- Unix socket at `.bd-drain/drain.sock` for IPC.
- Simple JSON-RPC protocol for control commands.
- PID file at `.bd-drain/drain.pid` for daemon detection.

## Adding commands

1. Create new file `cmd/bd-drain/<command>.go`.
2. Define command with `&cobra.Command{Use: "<command>", ...}`.
3. Register in `main.go` via `rootCmd.AddCommand(<command>Cmd)`.
4. Add flags specific to the command; bind to Viper.
