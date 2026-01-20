# Session Manager

Manages Claude Code process lifecycle, including spawning, output streaming, parsing, and termination.

## Purpose

The Session Manager is responsible for:
- Spawning `claude -p` processes with appropriate flags
- Streaming and parsing `stream-json` output in real-time
- Tracking session metadata (ID, turns, cost, duration)
- Handling process termination (normal, error, timeout)
- Emitting parsed events to the event router

## Interface

```go
type Manager struct {
    config *config.Config
    events *events.Router
}

type SessionResult struct {
    SessionID    string
    NumTurns     int
    DurationMs   int
    TotalCostUSD float64
    ExitCode     int
    Error        error
}

// Public API
func New(cfg *config.Config, events *events.Router) *Manager
func (m *Manager) Run(ctx context.Context, bead *workqueue.Bead) (*SessionResult, error)
func (m *Manager) Kill() error
```

## Dependencies

| Component | Usage |
|-----------|-------|
| config.Config | Extra args, timeout, prompt template |
| events.Router | Emit Claude events as they're parsed |

External:
- `claude` CLI binary
- User's global Claude config (`~/.claude/settings.json`, `~/.claude/rules/`, etc.)

## Implementation

### Claude Invocation

By default, atari passes minimal flags to claude, relying on the user's global Claude configuration (model, settings, permissions). This is the natural place for users to configure their Claude behavior.

```go
func (m *Manager) buildCommand(ctx context.Context, bead *workqueue.Bead) *exec.Cmd {
    prompt := m.buildPrompt(bead)

    // Minimal required flags only
    args := []string{
        "--print",
        "--output-format", "stream-json",
    }

    // Add any user-configured extra arguments
    // These override global claude config when specified
    if len(m.config.Claude.ExtraArgs) > 0 {
        args = append(args, m.config.Claude.ExtraArgs...)
    }

    args = append(args, prompt)

    return exec.CommandContext(ctx, "claude", args...)
}
```

### Configuration

Users who want to override their global Claude settings for atari sessions can use `extra_args`:

```yaml
claude:
  # Extra arguments passed to claude CLI
  # These override global ~/.claude/settings.json for atari sessions
  extra_args:
    - "--model"
    - "sonnet"
    - "--max-turns"
    - "100"
```

This approach:
- **Default**: Uses global claude config (model, settings, permissions from `~/.claude/settings.json`)
- **Override**: Users can add extra_args when atari needs different behavior
- **Minimal**: Atari only passes flags essential for operation (`--print`, `--output-format`)

### Default Prompt Template

The default prompt is based on the shell implementation from [EXISTING_IMPLEMENTATION.md](../EXISTING_IMPLEMENTATION.md). It references the user's Claude configuration at `~/.claude/` which includes:
- Rules in `~/.claude/rules/` (issue-tracking.md, session-protocol.md)
- Skills in `~/.claude/skills/` (br-issue-tracking)
- Global instructions in `~/.claude/CLAUDE.md`

```go
const defaultPrompt = `Run "br ready --json" to find available work. Review your skills (br-issue-tracking, git-commit), MCPs (codex for verification), and agents (Explore, Plan). Implement the highest-priority ready issue completely, including all tests and linting. When you discover bugs or issues during implementation, create new br issues with exact context of what you were doing and what you found. Use the Explore and Plan subagents to investigate new issues before creating implementation tasks. Use /commit for atomic commits.`

func (m *Manager) buildPrompt(bead *workqueue.Bead) string {
    if m.config.PromptTemplate != "" {
        return m.expandTemplate(m.config.PromptTemplate, bead)
    }
    return defaultPrompt
}
```

This prompt works with the user's existing Claude Code setup:
- **Skills**: References `bd-issue-tracking` and `git-commit` skills
- **Agents**: Mentions `Explore` and `Plan` subagents for investigation
- **MCPs**: Notes `codex` MCP for verification
- **Workflow**: Aligns with rules in `~/.claude/rules/issue-tracking.md`

### Running a Session

```go
func (m *Manager) Run(ctx context.Context, bead *workqueue.Bead) (*SessionResult, error) {
    cmd := m.buildCommand(ctx, bead)

    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return nil, fmt.Errorf("create stdout pipe: %w", err)
    }

    // Capture stderr for error reporting
    var stderr bytes.Buffer
    cmd.Stderr = &stderr

    if err := cmd.Start(); err != nil {
        return nil, fmt.Errorf("start claude: %w", err)
    }

    m.currentCmd = cmd

    // Parse streaming output
    result := m.parseStream(ctx, stdout)

    // Wait for process to exit
    waitErr := cmd.Wait()

    m.currentCmd = nil

    if waitErr != nil {
        result.Error = fmt.Errorf("claude exited: %w: %s", waitErr, stderr.String())
        if exitErr, ok := waitErr.(*exec.ExitError); ok {
            result.ExitCode = exitErr.ExitCode()
        }
    }

    return result, result.Error
}
```

### Stream Parsing

The `stream-json` output format emits one JSON object per line:

```go
func (m *Manager) parseStream(ctx context.Context, r io.Reader) *SessionResult {
    result := &SessionResult{}
    scanner := bufio.NewScanner(r)

    // Increase buffer size for large outputs
    buf := make([]byte, 0, 64*1024)
    scanner.Buffer(buf, 1024*1024)

    for scanner.Scan() {
        select {
        case <-ctx.Done():
            return result
        default:
        }

        line := scanner.Bytes()
        if len(line) == 0 {
            continue
        }

        event, err := m.parseLine(line)
        if err != nil {
            m.events.Emit(events.Error{Err: fmt.Errorf("parse error: %w", err)})
            continue
        }

        if event != nil {
            m.events.Emit(event)
        }

        // Extract result data from final event
        if r, ok := event.(*events.SessionEnd); ok {
            result.SessionID = r.SessionID
            result.NumTurns = r.NumTurns
            result.DurationMs = r.DurationMs
            result.TotalCostUSD = r.TotalCostUSD
        }
    }

    return result
}
```

### Stream-JSON Event Types

```go
func (m *Manager) parseLine(line []byte) (events.Event, error) {
    var raw struct {
        Type    string `json:"type"`
        Subtype string `json:"subtype,omitempty"`
    }

    if err := json.Unmarshal(line, &raw); err != nil {
        return nil, err
    }

    switch raw.Type {
    case "system":
        return m.parseSystemEvent(line, raw.Subtype)
    case "assistant":
        return m.parseAssistantEvent(line)
    case "user":
        return m.parseUserEvent(line)
    case "result":
        return m.parseResultEvent(line)
    default:
        // Unknown event type, log but continue
        return nil, nil
    }
}
```

### System Events

```json
{"type": "system", "subtype": "init", "model": "claude-3-opus-20240229", "tools": ["Bash", "Read", "Edit", ...]}
{"type": "system", "subtype": "compact_boundary"}
{"type": "system", "subtype": "hook", "hook_type": "SessionStart", ...}
```

```go
func (m *Manager) parseSystemEvent(line []byte, subtype string) (events.Event, error) {
    switch subtype {
    case "init":
        var e struct {
            Model string   `json:"model"`
            Tools []string `json:"tools"`
        }
        if err := json.Unmarshal(line, &e); err != nil {
            return nil, err
        }
        return &events.SessionStart{
            Model: e.Model,
            Tools: e.Tools,
        }, nil

    case "compact_boundary":
        return &events.Compact{}, nil

    default:
        return nil, nil
    }
}
```

### Assistant Events

```json
{
  "type": "assistant",
  "message": {
    "content": [
      {"type": "text", "text": "Let me check the git status..."},
      {"type": "tool_use", "id": "tool_1", "name": "Bash", "input": {"command": "git status"}}
    ]
  }
}
```

```go
func (m *Manager) parseAssistantEvent(line []byte) (events.Event, error) {
    var e struct {
        Message struct {
            Content []json.RawMessage `json:"content"`
        } `json:"message"`
    }

    if err := json.Unmarshal(line, &e); err != nil {
        return nil, err
    }

    var parsed []events.Event

    for _, content := range e.Message.Content {
        var contentType struct {
            Type string `json:"type"`
        }
        json.Unmarshal(content, &contentType)

        switch contentType.Type {
        case "text":
            var t struct {
                Text string `json:"text"`
            }
            json.Unmarshal(content, &t)
            parsed = append(parsed, &events.Text{Text: t.Text})

        case "thinking":
            var t struct {
                Thinking string `json:"thinking"`
            }
            json.Unmarshal(content, &t)
            parsed = append(parsed, &events.Thinking{Text: t.Thinking})

        case "tool_use":
            var t struct {
                ID    string         `json:"id"`
                Name  string         `json:"name"`
                Input map[string]any `json:"input"`
            }
            json.Unmarshal(content, &t)
            parsed = append(parsed, &events.ToolUse{
                ID:    t.ID,
                Name:  t.Name,
                Input: t.Input,
            })
        }
    }

    // Return batch event if multiple, or single event
    if len(parsed) == 1 {
        return parsed[0], nil
    }
    return &events.AssistantBatch{Events: parsed}, nil
}
```

### Result Event

```json
{
  "type": "result",
  "result": "I've completed the task...",
  "session_id": "abc123",
  "num_turns": 8,
  "duration_ms": 45000,
  "total_cost_usd": 0.42
}
```

```go
func (m *Manager) parseResultEvent(line []byte) (events.Event, error) {
    var e struct {
        Result       string  `json:"result"`
        SessionID    string  `json:"session_id"`
        NumTurns     int     `json:"num_turns"`
        DurationMs   int     `json:"duration_ms"`
        TotalCostUSD float64 `json:"total_cost_usd"`
    }

    if err := json.Unmarshal(line, &e); err != nil {
        return nil, err
    }

    return &events.SessionEnd{
        SessionID:    e.SessionID,
        NumTurns:     e.NumTurns,
        DurationMs:   e.DurationMs,
        TotalCostUSD: e.TotalCostUSD,
        Result:       e.Result,
    }, nil
}
```

### Process Termination

```go
func (m *Manager) Kill() error {
    if m.currentCmd == nil || m.currentCmd.Process == nil {
        return nil
    }

    // Send SIGTERM for graceful shutdown
    if err := m.currentCmd.Process.Signal(syscall.SIGTERM); err != nil {
        // Process may have already exited
        return nil
    }

    // Wait briefly for graceful exit
    done := make(chan error, 1)
    go func() {
        done <- m.currentCmd.Wait()
    }()

    select {
    case <-done:
        return nil
    case <-time.After(5 * time.Second):
        // Force kill if still running
        return m.currentCmd.Process.Kill()
    }
}
```

### Timeout Handling

Sessions that produce no output for extended periods are killed:

```go
func (m *Manager) Run(ctx context.Context, bead *workqueue.Bead) (*SessionResult, error) {
    // Create timeout context
    timeout := m.config.Claude.Timeout
    if timeout == 0 {
        timeout = 5 * time.Minute
    }

    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    // Watchdog goroutine
    lastActivity := time.Now()
    var activityMu sync.Mutex

    go func() {
        ticker := time.NewTicker(30 * time.Second)
        defer ticker.Stop()

        for {
            select {
            case <-ctx.Done():
                return
            case <-ticker.C:
                activityMu.Lock()
                idle := time.Since(lastActivity)
                activityMu.Unlock()

                if idle > timeout {
                    m.events.Emit(events.Error{
                        Err: fmt.Errorf("session timeout: no activity for %v", idle),
                    })
                    m.Kill()
                    cancel()
                    return
                }
            }
        }
    }()

    // Update lastActivity on each parsed event
    // ... (in parseStream)
}
```

## Testing

### Unit Tests

- Command building: verify flags are set correctly
- Stream parsing: test each event type
- Timeout handling: verify kill after inactivity

### Test Fixtures

```go
// Sample stream-json output for testing
var sampleStream = `{"type":"system","subtype":"init","model":"opus","tools":["Bash","Read"]}
{"type":"assistant","message":{"content":[{"type":"text","text":"Checking status..."}]}}
{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"git status"}}]}}
{"type":"result","session_id":"abc","num_turns":2,"duration_ms":5000,"total_cost_usd":0.05}
`

func TestParseStream(t *testing.T) {
    m := New(&config.Config{}, events.NewRouter())
    r := strings.NewReader(sampleStream)
    result := m.parseStream(context.Background(), r)

    if result.NumTurns != 2 {
        t.Errorf("expected 2 turns, got %d", result.NumTurns)
    }
}
```

### Integration Tests

- Full session with mock claude binary
- Timeout behavior with stalled process
- Kill behavior during active session

## Error Handling

| Error | Action |
|-------|--------|
| `claude` not found | Return error with suggestion to install |
| Non-zero exit code | Include exit code and stderr in result |
| Parse error on line | Log warning, continue parsing |
| Timeout | Kill process, return timeout error |
| Context cancelled | Kill process, return context error |

## Future Considerations

- **Session resume**: Use `--resume` flag across related beads
- **Output buffering**: Configurable buffer sizes for large outputs
- **Custom parsers**: Plugin architecture for different output formats
- **Resource limits**: Memory/CPU limits for claude process
