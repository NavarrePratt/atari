# Session Package

Manages Claude Code process lifecycle with timeout watchdog and stream-json parsing.

## Key Types

### Parser

Parses Claude stream-json output and emits typed events.

```go
parser := session.NewParser(reader, router, manager)

// Parse reads until EOF, emitting events to router
err := parser.Parse()
```

Event type mappings:
- `assistant` with `text` content -> `ClaudeTextEvent`
- `assistant` with `thinking` content -> `ClaudeTextEvent`
- `assistant` with `tool_use` content -> `ClaudeToolUseEvent`
- `user` with `tool_result` content -> `ClaudeToolResultEvent`
- `result` -> `SessionEndEvent`
- Parse errors -> `ParseErrorEvent` (parsing continues)

Features:
- 1MB scanner buffer for large tool outputs
- Calls `manager.UpdateActivity()` on each valid event
- Parse errors emit events but don't stop parsing
- Unknown event types are silently ignored

### Manager

Spawns and manages a claude process with stream-json output.

```go
mgr := session.New(cfg, router)

// Start with a prompt (provided via stdin)
err := mgr.Start(ctx, "Implement feature X")

// Read stream-json events from stdout
reader := mgr.Stdout()

// Update activity timestamp (resets timeout watchdog)
mgr.UpdateActivity()

// Check stderr if process fails
stderr := mgr.Stderr()

// Wait for process to complete
err := mgr.Wait()

// Force termination
mgr.Stop()
```

### LimitedWriter

Captures stderr output up to a maximum size (default 64KB).

```go
writer := session.NewLimitedWriter(64 * 1024)
content := writer.String()
```

## Timeout Watchdog

The manager runs a background watchdog that terminates idle sessions:
- Checks activity every 10 seconds
- Kills process if no activity for `cfg.Claude.Timeout` duration
- Emits `SessionTimeoutEvent` before termination
- Call `UpdateActivity()` when processing output to prevent timeout

## Lifecycle

1. `New()` - Create manager with config and event router
2. `Start()` - Spawn claude process with prompt
3. Read from `Stdout()` - Process stream-json events
4. Call `UpdateActivity()` - Keep watchdog happy
5. `Wait()` - Block until process exits
6. Check `Stderr()` - Diagnose failures

## Integration

The session package integrates with:
- `config.Config` - Claude timeout and extra args
- `events.Router` - Emits parsed events and timeout events
- `controller.Controller` - Orchestrates session lifecycle

Typical usage:
```go
mgr := session.New(cfg, router)
err := mgr.Start(ctx, prompt)
if err != nil {
    return err
}

parser := session.NewParser(mgr.Stdout(), router, mgr)
go parser.Parse()

err = mgr.Wait()
```
