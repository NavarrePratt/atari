# Shutdown Package

Graceful shutdown helpers for signal handling and cleanup.

## Usage

### Pattern 1: Shutdowner Interface

For components implementing `Shutdowner`:

```go
type Shutdowner interface {
    Shutdown(ctx context.Context) error
}

// Blocks until SIGINT/SIGTERM, then calls Shutdown
shutdown.Gracefully(logger, 30*time.Second, myComponent)
```

### Pattern 2: Function-Based

For ad-hoc shutdown logic:

```go
err := shutdown.RunWithGracefulShutdown(
    ctx,
    logger,
    30*time.Second,
    func(ctx context.Context) error {
        // Main loop - runs until context cancelled
        return runMainLoop(ctx)
    },
    func(ctx context.Context) error {
        // Cleanup - called on signal
        return cleanup(ctx)
    },
)
```

## Signals

Handles SIGINT and SIGTERM. On signal:
1. Logs "shutdown signal received"
2. Cancels runner context
3. Calls shutdown function with timeout
4. Logs completion or timeout warning
