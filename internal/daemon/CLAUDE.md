# Daemon Package

Background execution with external control via Unix socket RPC.

## Purpose

The daemon package provides:
- Background process management with PID file tracking
- flock-based locking to prevent concurrent daemon instances
- Stale file cleanup for crash recovery
- Unix socket server for JSON-RPC control
- RPC client for CLI commands
- Daemonization (fork/setsid) for background mode

## Key Types

### Daemon

Main orchestrator for daemon mode:
- Manages controller lifecycle
- Holds references to config, controller, and socket listener
- Tracks running state with mutex protection

### Server

Unix socket server for RPC:
- Listens on `.atari/atari.sock`
- Handles JSON-RPC requests (status, pause, resume, stop)
- Thread-safe connection handling

### Client

RPC client for CLI commands:
- Connects to daemon socket
- Provides Status(), Pause(), Resume(), Stop() methods
- Handles connection errors and timeouts

### PIDFile

Manages the PID file with flock locking:
- `Write()` - Create and lock PID file with current process ID
- `Read()` - Read PID from file
- `Remove()` - Release lock and remove file
- `IsRunning()` - Check if daemon process is alive
- `CleanupStale()` - Remove stale files after crash

## File Paths

Default locations (in project directory):
- Daemon info: `.atari/daemon.json`
- Socket: `.atari/atari.sock`
- PID file: `.atari/atari.pid` (legacy, now using daemon.json)

## Usage

```go
// Create and start daemon
cfg := config.Default()
ctrl := controller.New(...)
d := daemon.New(cfg, ctrl, logger)

if err := d.Start(ctx); err != nil {
    log.Fatal(err)
}

// Check if running
if d.Running() {
    fmt.Println("Daemon is running")
}

// Use client from CLI commands
client := daemon.NewClient(socketPath)
status, err := client.Status()
if err != nil {
    log.Fatal(err)
}
fmt.Printf("State: %s\n", status.Status)
```

## Daemonization

Background mode uses fork/setsid:

```go
// In start command with --daemon flag
if daemonize {
    daemon.Daemonize() // Forks and exits parent
}
// Child continues as daemon
```

## Files

| File | Purpose |
|------|---------|
| daemon.go | Daemon struct and lifecycle |
| server.go | Unix socket listener |
| client.go | RPC client for CLI |
| handlers.go | RPC command handlers |
| protocol.go | JSON-RPC types |
| pid.go | PID file management |
| daemonize.go | Fork/setsid for background mode |
| paths.go | Path resolution for daemon files |
| integration_test.go | Full daemon integration tests |

## Implementation Status

| Feature | Status |
|---------|--------|
| Daemon struct | Done |
| PID file management | Done |
| flock locking | Done |
| Stale detection | Done |
| Unix socket server | Done |
| RPC client | Done |
| Daemonization | Done |
| Path resolution | Done |
| Integration tests | Done |
