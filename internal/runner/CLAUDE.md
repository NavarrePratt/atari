# Runner Package

Abstractions for streaming process execution with testability support.

## Key Types

### ProcessRunner Interface

Abstracts subprocess execution for streaming output. Unlike `CommandRunner` (testutil) which returns output after completion, `ProcessRunner` provides streaming access to stdout and stderr.

```go
type ProcessRunner interface {
    Start(ctx context.Context, name string, args ...string) (stdout, stderr io.ReadCloser, err error)
    Wait() error
    Kill() error
}
```

Methods:
- `Start`: Spawns process, returns readers for stdout/stderr
- `Wait`: Blocks until process exits, returns exit error
- `Kill`: Sends SIGKILL immediately (safe to call multiple times)

### ExecProcessRunner

Production implementation using os/exec.

```go
runner := runner.NewExecProcessRunner()

stdout, stderr, err := runner.Start(ctx, "claude", "-p", "--output-format", "stream-json")
if err != nil {
    return err
}

// Read streaming output
go processStderr(stderr)
for scanner.Scan() {
    processLine(scanner.Bytes())
}

err = runner.Wait()
```

## Thread Safety

ExecProcessRunner is thread-safe:
- Multiple goroutines can safely call Kill
- Start can only be called once per instance
- Wait blocks until process completes

## Context Cancellation

When the context passed to Start is cancelled, the process receives SIGKILL automatically via exec.CommandContext.

## Usage with Session Manager

This package will replace direct exec.Cmd usage in session.Manager:

```go
// Before: Direct exec.Cmd
m.cmd = exec.CommandContext(ctx, "claude", args...)
m.stdout, _ = m.cmd.StdoutPipe()

// After: ProcessRunner interface
stdout, stderr, _ := m.runner.Start(ctx, "claude", args...)
```

Benefits:
- Testable: Mock implementations can return fake output
- Consistent: Same interface for all process spawning
- Safe: Mutex protection for concurrent access
