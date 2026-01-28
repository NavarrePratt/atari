# Shutdown Package

Graceful shutdown helpers for signal handling and cleanup.

## Usage

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
1. Logs "received signal, initiating shutdown"
2. Cancels runner context
3. Calls shutdown function with timeout
4. Logs completion or timeout warning
