# Daemon Package

Background execution with external control via Unix socket RPC.

## Purpose

The daemon package provides:
- Background process management with PID file tracking
- flock-based locking to prevent concurrent daemon instances
- Stale file cleanup for crash recovery
- Unix socket server for JSON-RPC control (future issues)

## Key Types

### Daemon

Main orchestrator for daemon mode:
- Manages controller lifecycle
- Holds references to config, controller, and socket listener
- Tracks running state with mutex protection

### PIDFile

Manages the PID file with flock locking:
- `Write()` - Create and lock PID file with current process ID
- `Read()` - Read PID from file
- `Remove()` - Release lock and remove file
- `IsRunning()` - Check if daemon process is alive
- `CleanupStale()` - Remove stale files after crash

## File Paths

Default locations (in project directory):
- PID file: `.atari/atari.pid`
- Socket: `.atari/atari.sock`

## Usage

```go
// Create daemon
cfg := config.Default()
ctrl := controller.New(...)
d := daemon.New(cfg, ctrl, logger)

// Start (implemented in future issue)
if err := d.Start(ctx); err != nil {
    log.Fatal(err)
}

// Check if running
if d.Running() {
    fmt.Println("Daemon is running")
}
```

## PID File Locking

Uses flock for process synchronization:
- Prevents multiple daemon instances
- Handles crash recovery via stale detection
- Non-blocking lock acquisition

```go
pidFile := daemon.NewPIDFile(".atari/atari.pid")

// Write and lock
if err := pidFile.Write(); err != nil {
    // Another daemon is running
}

// Check if process is alive
if pidFile.IsRunning() {
    // Daemon is active
}

// Clean up stale files
pidFile.CleanupStale(".atari/atari.sock")
```

## Implementation Status

| Feature | Status |
|---------|--------|
| Daemon struct | Done |
| PID file management | Done |
| flock locking | Done |
| Stale detection | Done |
| Unix socket server | Planned (bd-drain-0oe) |
| RPC client | Planned (bd-drain-pjg) |
| Daemonization | Planned (bd-drain-d20) |
