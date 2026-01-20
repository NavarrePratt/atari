# Testutil Package

Test infrastructure for unit and integration testing. Provides mocks, fixtures, and helpers for testing atari components.

## Components

### CommandRunner Interface

Abstraction for command execution, allowing tests to mock external commands (br, claude).

```go
type CommandRunner interface {
    Run(ctx context.Context, name string, args ...string) ([]byte, error)
}
```

### MockRunner

Mock implementation with canned responses and call recording.

```go
mock := testutil.NewMockRunner()

// Configure responses
mock.SetResponse("br", []string{"ready", "--json"}, []byte(`[...]`))
mock.SetError("br", []string{"close", "bd-001"}, errors.New("not found"))

// Execute (records calls)
output, err := mock.Run(ctx, "br", "ready", "--json")

// Assert
testutil.AssertCalled(t, mock, "br", "ready", "--json")
testutil.AssertCallCount(t, mock, "br", 2)
```

**Prefix matching**: Commands with variable args can use prefix matching - if no exact match is found, the runner checks if any registered key is a prefix of the actual command.

**Dynamic responses**: For tests needing call-count-based response variations:

```go
mock.DynamicResponse = func(ctx context.Context, name string, args []string) ([]byte, error, bool) {
    if name == "br" && len(args) > 0 && args[0] == "ready" {
        // Return different beads on each call
        callCount++
        if callCount == 1 {
            return []byte(`[{"id": "bd-001"}]`), nil, true
        }
        return []byte(`[]`), nil, true
    }
    return nil, nil, false  // Fall through to normal lookup
}
```

### ExecRunner

Production implementation using os/exec. Used by the CLI start command for real command execution.

```go
runner := testutil.NewExecRunner()
output, err := runner.Run(ctx, "br", "ready", "--json")
```

### MockProcessRunner

Mock implementation of runner.ProcessRunner for testing streaming process scenarios (e.g., br activity --follow).

```go
mock := testutil.NewMockProcessRunner()

// Configure output
mock.SetOutput("line1\nline2\n")
mock.SetStderr("warning message\n")

// Configure errors
mock.SetStartError(errors.New("command not found"))
mock.SetWaitError(errors.New("exit code 1"))

// Start the mock process
stdout, stderr, err := mock.Start(ctx, "br", "activity", "--follow")
data, _ := io.ReadAll(stdout)
err = mock.Wait()

// State inspection
mock.StartCount()    // Number of Start calls
mock.Processes()     // All recorded process starts
mock.Started()       // Whether currently started
mock.Killed()        // Whether Kill was called
mock.WaitCalled()    // Whether Wait was called
mock.Reset()         // Clear state for reuse
```

**Dynamic behavior**: For tests needing different responses per attempt:

```go
mock.OnStart(func(attempt int, name string, args []string) (stdout, stderr string, err error) {
    if attempt == 1 {
        return "", "", errors.New("first attempt fails")
    }
    return "success on retry", "", nil
})
```

## Fixtures

Pre-defined JSON responses for common scenarios:

| Fixture | Description |
|---------|-------------|
| `SampleBeadReadyJSON` | br ready response with multiple beads |
| `SingleBeadReadyJSON` | br ready response with one bead |
| `EmptyBeadReadyJSON` | br ready response when no beads available |
| `SampleClaudeInit` | Claude system init event |
| `SampleClaudeAssistant` | Claude text response event |
| `SampleClaudeToolUse` | Claude tool use event |
| `SampleClaudeToolResult` | Tool result event |
| `SampleClaudeResultSuccess` | Successful session result |
| `SampleStateJSON` | Sample state.json file |

## Mock Claude Sessions

Generate mock Claude stream-json output for testing session parsing.

```go
// Successful session
output := testutil.NewSuccessfulSession("session-001")

// Session that closes a bead
output := testutil.NewSuccessfulSessionWithBRClose("session-001", "bd-001")

// Failed session
output := testutil.NewFailedSession("session-001", "command failed")

// Max turns exceeded
output := testutil.NewMaxTurnsSession("session-001", 10)

// Timeout (truncated output)
output := testutil.NewTimeoutSession("session-001")

// Custom tool usage
output := testutil.NewSessionWithToolUse("session-001", []testutil.ToolCall{
    {Name: "Bash", Input: map[string]any{"command": "ls"}, Result: "file.txt\n"},
})

// Use as io.Reader or string
reader := output.Reader()
str := output.String()
```

## Test Helpers

```go
// Temporary directories
dir, cleanup := testutil.TempDir(t)
defer cleanup()

// File operations
path := testutil.WriteFile(t, dir, "config.json", `{"key": "value"}`)
content := testutil.ReadFile(t, path)
exists := testutil.FileExists(t, path)

// Setup with atari structure
dir, cleanup := testutil.SetupTestDir(t)              // Creates .atari/
dir, cleanup := testutil.SetupTestDirWithState(t, json) // Creates .atari/state.json

// Mock setup helpers
testutil.SetupMockBRReady(mock, testutil.SampleBeadReadyJSON)
testutil.SetupMockBRAgentState(mock, "agent-id")
testutil.SetupMockBRClose(mock, "bd-001")
```

## Usage Pattern

```go
func TestController(t *testing.T) {
    dir, cleanup := testutil.SetupTestDir(t)
    defer cleanup()

    mock := testutil.NewMockRunner()
    testutil.SetupMockBRReady(mock, testutil.SingleBeadReadyJSON)

    // Create component with mock runner
    ctrl := controller.New(mock, dir)

    // Run test...

    testutil.AssertCalled(t, mock, "br", "ready", "--json")
}
```
