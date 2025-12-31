# CLI Commands

Command-line interface for atari using Cobra.

## Overview

```
atari - Automated bead drain controller for Claude Code

Usage:
  atari [command]

Commands:
  start       Start the drain controller
  stop        Stop the running daemon
  pause       Pause after current bead completes
  resume      Resume from paused state
  status      Show current status and statistics
  events      View event stream
  stats       Show detailed statistics
  init        Initialize Claude Code configuration
  version     Print version information
  help        Help about any command

Flags:
  -h, --help      Help for atari
  -v, --verbose   Enable verbose output
      --config    Config file (default: .atari/config.yaml)
```

## Commands

### atari start

Start the drain controller.

```
Usage:
  atari start [flags]

Flags:
      --daemon         Run as background daemon
      --tui            Enable terminal UI (default if TTY)
      --no-tui         Disable terminal UI
      --log FILE       Log file path (default: .atari/atari.log)
      --max-turns N    Max turns per Claude session (default: 50)
      --label LABEL    Filter beads by label
      --prompt FILE    Custom prompt template file
      --model MODEL    Claude model (default: opus)
  -h, --help           Help for start

Examples:
  # Start in foreground with TUI
  atari start

  # Start as background daemon
  atari start --daemon

  # Start with custom settings
  atari start --max-turns 100 --label urgent

  # Start with verbose logging
  atari start -v --log /tmp/atari.log
```

**Implementation:**

```go
var startCmd = &cobra.Command{
    Use:   "start",
    Short: "Start the drain controller",
    Long:  `Start the atari drain controller to process beads automatically.`,
    RunE:  runStart,
}

func init() {
    startCmd.Flags().Bool("daemon", false, "Run as background daemon")
    startCmd.Flags().Bool("tui", true, "Enable terminal UI")
    startCmd.Flags().Bool("no-tui", false, "Disable terminal UI")
    startCmd.Flags().String("log", "", "Log file path")
    startCmd.Flags().Int("max-turns", 50, "Max turns per session")
    startCmd.Flags().String("label", "", "Filter beads by label")
    startCmd.Flags().String("prompt", "", "Custom prompt template")
    startCmd.Flags().String("model", "opus", "Claude model")

    rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
    cfg := loadConfig(cmd)

    // Check for existing daemon
    client := daemon.NewClient(cfg.SocketPath)
    if client.IsRunning() {
        return fmt.Errorf("daemon already running; use 'atari stop' first")
    }

    // Daemonize if requested
    if cfg.Daemon {
        return daemonize()
    }

    // Build and run controller
    ctrl := controller.New(cfg)
    events := events.NewRouter()

    // Set up sinks
    logSink := sinks.NewLogSink(cfg.LogPath)
    stateSink := sinks.NewStateSink(cfg.StatePath)

    eventCh := events.Subscribe()
    logSink.Start(ctx, eventCh)
    stateSink.Start(ctx, events.Subscribe())

    // Optional TUI
    if cfg.TUI && isTerminal() {
        tui := tui.New(events.Subscribe(),
            tui.WithOnPause(ctrl.Pause),
            tui.WithOnResume(ctrl.Resume),
            tui.WithOnQuit(ctrl.Stop),
        )
        go ctrl.Run(ctx)
        return tui.Run()
    }

    return ctrl.Run(ctx)
}
```

### atari stop

Stop the running daemon.

```
Usage:
  atari stop [flags]

Flags:
      --graceful   Wait for current bead to complete (default: true)
      --force      Stop immediately (SIGKILL)
  -h, --help       Help for stop

Examples:
  # Graceful stop (wait for current bead)
  atari stop

  # Immediate stop
  atari stop --force
```

### atari pause

Pause the drain after the current bead completes.

```
Usage:
  atari pause [flags]

Flags:
  -h, --help   Help for pause

Examples:
  atari pause
  # Output: Pausing... will stop after bd-042 completes
```

### atari resume

Resume from paused state.

```
Usage:
  atari resume [flags]

Flags:
  -h, --help   Help for resume

Examples:
  atari resume
  # Output: Resuming...
```

### atari status

Show current status and statistics.

```
Usage:
  atari status [flags]

Flags:
      --json   Output as JSON
  -h, --help   Help for status

Examples:
  atari status
  # Output:
  # Status: WORKING
  # Current: bd-042 "Fix auth bug"
  # Uptime: 2h15m
  #
  # Statistics:
  #   Beads completed: 12
  #   Beads failed: 1
  #   Total cost: $5.42
  #   Total turns: 156

  atari status --json
  # {"status":"working","current_bead":"bd-042",...}
```

**Implementation:**

```go
func runStatus(cmd *cobra.Command, args []string) error {
    cfg := loadConfig(cmd)
    client := daemon.NewClient(cfg.SocketPath)

    if !client.IsRunning() {
        fmt.Println("Daemon not running")
        return nil
    }

    status, err := client.Status()
    if err != nil {
        return fmt.Errorf("get status: %w", err)
    }

    if jsonOutput, _ := cmd.Flags().GetBool("json"); jsonOutput {
        data, _ := json.MarshalIndent(status, "", "  ")
        fmt.Println(string(data))
        return nil
    }

    // Human-readable output
    fmt.Printf("Status: %s\n", strings.ToUpper(status.Status))
    if status.CurrentBead != "" {
        fmt.Printf("Current: %s\n", status.CurrentBead)
    }
    fmt.Printf("Uptime: %s\n", status.Uptime)
    fmt.Println()
    fmt.Println("Statistics:")
    fmt.Printf("  Beads completed: %d\n", status.Stats.BeadsCompleted)
    fmt.Printf("  Beads failed: %d\n", status.Stats.BeadsFailed)
    fmt.Printf("  Total cost: $%.2f\n", status.Stats.TotalCostUSD)
    fmt.Printf("  Total turns: %d\n", status.Stats.TotalTurns)

    return nil
}
```

### atari events

View the event stream.

```
Usage:
  atari events [flags]

Flags:
      --follow       Follow events in real-time (like tail -f)
      --count N      Show last N events (default: 50)
      --type TYPE    Filter by event type
      --json         Output as JSON
  -h, --help         Help for events

Examples:
  # Show recent events
  atari events

  # Follow live events
  atari events --follow

  # Filter by type
  atari events --type tool.use

  # JSON output for scripting
  atari events --json --count 100
```

### atari stats

Show detailed statistics.

```
Usage:
  atari stats [flags]

Flags:
      --json   Output as JSON
  -h, --help   Help for stats

Examples:
  atari stats
  # Output:
  # Session Statistics
  # ==================
  # Started: 2024-01-15 10:00:00
  # Uptime: 4h30m
  #
  # Beads:
  #   Completed: 15
  #   Failed: 2
  #   In backoff: 1
  #
  # Costs:
  #   Total: $8.42
  #   Average per bead: $0.49
  #
  # Performance:
  #   Total turns: 203
  #   Average turns per bead: 11.9
  #   Total duration: 2h15m
  #   Average duration per bead: 8m30s
  #
  # Failed Beads:
  #   bd-039: tests failing (3 attempts)
  #   bd-044: timeout (2 attempts)
```

### atari init

Initialize Claude Code configuration for use with atari.

See [init-command.md](init-command.md) for detailed documentation.

```
Usage:
  atari init [flags]

Flags:
      --dry-run      Show what would be changed without making changes
      --force        Overwrite existing configuration
      --minimal      Only add essential configuration
  -h, --help         Help for init

Examples:
  # Preview changes
  atari init --dry-run

  # Apply configuration
  atari init

  # Force overwrite
  atari init --force
```

### atari version

Print version information.

```
Usage:
  atari version

Examples:
  atari version
  # Output: atari v0.1.0 (abc1234) built 2024-01-15
```

## Global Flags

```
Flags available to all commands:

  -v, --verbose   Enable verbose output
      --config    Config file path (default: .atari/config.yaml)
      --debug     Enable debug logging
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Invalid arguments |
| 3 | Daemon not running |
| 4 | Daemon already running |
| 5 | Configuration error |

## Shell Completion

```bash
# Bash
atari completion bash > /etc/bash_completion.d/atari

# Zsh
atari completion zsh > "${fpath[1]}/_atari"

# Fish
atari completion fish > ~/.config/fish/completions/atari.fish
```

## Implementation Structure

```go
// cmd/atari/main.go
package main

import (
    "os"
    "github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
    Use:   "atari",
    Short: "Automated bead drain controller for Claude Code",
    Long: `Atari orchestrates Claude Code sessions to automatically
work through beads (bd) issues.`,
}

func main() {
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}

// cmd/atari/start.go
// cmd/atari/stop.go
// cmd/atari/pause.go
// cmd/atari/resume.go
// cmd/atari/status.go
// cmd/atari/events.go
// cmd/atari/stats.go
// cmd/atari/init.go
// cmd/atari/version.go
```

## Configuration Loading

```go
func loadConfig(cmd *cobra.Command) *config.Config {
    cfg := config.Default()

    // Load from file
    configPath, _ := cmd.Flags().GetString("config")
    if configPath == "" {
        configPath = ".atari/config.yaml"
    }
    cfg.LoadFile(configPath)

    // Override with flags
    if v, _ := cmd.Flags().GetInt("max-turns"); v != 0 {
        cfg.Claude.MaxTurns = v
    }
    if v, _ := cmd.Flags().GetString("label"); v != "" {
        cfg.Label = v
    }
    // ... etc

    // Override with environment
    cfg.LoadEnv()

    return cfg
}
```
