# Observer Mode

Interactive Q&A pane for asking questions about drain activity during operation.

## Purpose

Observer Mode enables real-time understanding and intervention guidance:

- Ask questions about what Claude is doing without pausing the drain
- Get context about current progress and decision-making
- Identify when manual intervention might be needed
- Understand errors or unexpected behavior as they happen

**Primary use cases**:
- Real-time understanding: "What's happening right now?"
- Intervention guidance: "Should I pause and fix something?"

Post-hoc investigation is better done in a normal Claude Code session since relevant info is persisted on beads.

## Interface

```go
// Observer handles interactive Q&A queries using Claude CLI.
type Observer struct {
    config        *config.ObserverConfig
    broker        *SessionBroker
    builder       *ContextBuilder
    stateProvider DrainStateProvider
    runnerFactory func() runner.ProcessRunner

    mu        sync.Mutex
    sessionID string // Claude session ID for --resume
    runner    runner.ProcessRunner
    cancel    context.CancelFunc
}

type ObserverConfig struct {
    Enabled      bool    // Default: false
    Model        string  // Default: "haiku"
    RecentEvents int     // Events for current bead context (default: 20)
    ShowCost     bool    // Display cost tracking (default: true)
    Layout       string  // "horizontal" (default) or "vertical"
}

// DrainStateProvider provides drain state for context building
type DrainStateProvider interface {
    GetDrainState() DrainState
}

type DrainState struct {
    Status      string
    Uptime      time.Duration
    TotalCost   float64
    CurrentBead *CurrentBeadInfo
}

type CurrentBeadInfo struct {
    ID        string
    Title     string
    StartedAt time.Time
}

// Public API
func NewObserver(cfg *ObserverConfig, broker *SessionBroker, builder *ContextBuilder, stateProvider DrainStateProvider) *Observer
func (o *Observer) Ask(ctx context.Context, question string) (string, error)
func (o *Observer) Cancel()   // Cancel in-progress query
func (o *Observer) Reset()    // Clear session for fresh start
```

## Dependencies

| Component | Usage |
|-----------|-------|
| config.ObserverConfig | Model, event count, display settings |
| SessionBroker | Coordinates access to Claude CLI (one session at a time) |
| ContextBuilder | Builds structured prompts from log events |
| LogReader | Reads events from `.atari/atari.log` |
| DrainStateProvider | Current drain state and statistics |

External:
- `claude` CLI binary (haiku model by default)

## Design Decisions

### Session Model

Claude CLI does not support a true stdin REPL mode. Instead, we use **chainable sessions with `--resume`**:

1. First question establishes a session with full context
2. Subsequent questions resume the same session, maintaining conversation history
3. Each question spawns a new process but preserves context via session ID

```go
// First question - establishes session with context
sessionID, response := askWithContext(ctx, model, contextPrompt, question)

// Follow-up questions resume the session
response := askResume(ctx, model, sessionID, question)
```

This approach:
- Maintains conversation history without re-sending full context
- Allows natural follow-up questions
- Session terminates when observer pane closes

### Context Source

Context is built from the existing event log file (`.atari/atari.log`) rather than a separate ring buffer:

- Reuses existing infrastructure (no duplicate state)
- Survives observer restarts
- Works even if observer starts after events occurred
- Log rotation is handled by existing LogSink

### Structured Context

Context is organized into sections for clarity:

1. **Drain Status**: Current state, uptime, total cost
2. **Session History**: Completed beads with outcomes and costs
3. **Current Bead**: Active work with recent events
4. **Tips**: How to retrieve full event details

### Event Summarization

Events are summarized to fit context limits:

| Event Type | Included Fields |
|------------|-----------------|
| claude.text | First 200 chars of text |
| claude.tool_use | tool_name + description (Bash) or file_path |
| claude.tool_result | First 100 chars + tool_id for lookup |
| session.start/end | Bead ID, turns, cost |
| iteration.start/end | Bead ID, outcome |
| bead.* | Bead ID, status change |

Full event details can be retrieved via grep using the tool_id.

### Model Selection

- **Default**: Haiku - fast responses, low cost for Q&A
- **Configurable**: Users can switch to sonnet for deeper analysis
- Haiku is appropriate because observer questions are typically:
  - Event summarization
  - Pattern identification
  - Simple clarifications
  - "What's happening?" queries

## Implementation

Context building is handled by `ContextBuilder`, which reads events from `LogReader` and combines them with drain state from `DrainStateProvider`.

### Building Context

```go
// ContextBuilder assembles structured prompts from log events and drain state.
type ContextBuilder struct {
    logReader *LogReader
    config    *config.ObserverConfig
}

func (b *ContextBuilder) Build(state DrainState) (string, error) {
    var out strings.Builder

    // System prompt
    out.WriteString(systemPrompt)

    // Drain status section
    out.WriteString("## Drain Status\n")
    out.WriteString(fmt.Sprintf("State: %s | Uptime: %s | Total cost: $%.2f\n\n",
        state.Status,
        formatDuration(state.Uptime),
        state.TotalCost))

    // Session history from log events
    history := b.logReader.ReadSessionHistory()
    if len(history) > 0 {
        out.WriteString("## Session History\n")
        out.WriteString("| Bead | Title | Outcome | Cost | Turns |\n")
        out.WriteString("|------|-------|---------|------|-------|\n")
        for _, h := range history {
            out.WriteString(fmt.Sprintf("| %s | %s | %s | $%.2f | %d |\n",
                h.BeadID, truncate(h.Title, 30), h.Outcome, h.Cost, h.Turns))
        }
        out.WriteString("\n")
    }

    // Current bead section
    if state.CurrentBead != nil {
        out.WriteString(fmt.Sprintf("## Current Bead: %s\n", state.CurrentBead.ID))
        out.WriteString(fmt.Sprintf("Title: %s\n", state.CurrentBead.Title))
        out.WriteString(fmt.Sprintf("Started: %s ago\n\n", formatDuration(time.Since(state.CurrentBead.StartedAt))))

        // Recent events for current bead
        events := b.logReader.ReadRecentEvents(state.CurrentBead.ID, b.config.RecentEvents)
        if len(events) > 0 {
            out.WriteString("### Recent Activity\n")
            for _, e := range events {
                out.WriteString(formatEvent(e))
                out.WriteString("\n")
            }
            out.WriteString("\n")
        }
    }

    // Tips section
    out.WriteString("## Retrieving Full Event Details\n")
    out.WriteString("Events are stored in `.atari/atari.log` as JSON lines.\n")
    out.WriteString("To see full event details:\n")
    out.WriteString("  grep '<tool_id>' .atari/atari.log | jq .\n")
    out.WriteString("To get bead details:\n")
    out.WriteString("  bd show <bead-id>\n")

    return out.String(), nil
}
```

### Event Formatting

```go
func (o *Observer) formatEvent(e events.Event) string {
    ts := e.Timestamp().Format("15:04:05")

    switch ev := e.(type) {
    case *events.ClaudeTextEvent:
        return fmt.Sprintf("[%s] Claude: %s", ts, truncate(ev.Text, 200))

    case *events.ClaudeToolUseEvent:
        summary := formatToolSummary(ev.ToolName, ev.Input)
        return fmt.Sprintf("[%s] Tool: %s %s (%s)", ts, ev.ToolName, summary, shortID(ev.ToolID))

    case *events.ClaudeToolResultEvent:
        content := truncate(ev.Content, 100)
        if ev.IsError {
            return fmt.Sprintf("[%s] Result ERROR: %s (%s)", ts, content, shortID(ev.ToolID))
        }
        return fmt.Sprintf("[%s] Result: %s (%s)", ts, content, shortID(ev.ToolID))

    case *events.SessionStartEvent:
        return fmt.Sprintf("[%s] Session started for %s", ts, ev.BeadID)

    case *events.SessionEndEvent:
        return fmt.Sprintf("[%s] Session ended (turns: %d, cost: $%.2f)", ts, ev.NumTurns, ev.TotalCostUSD)

    case *events.IterationStartEvent:
        return fmt.Sprintf("[%s] Started bead %s: %s", ts, ev.BeadID, truncate(ev.Title, 40))

    case *events.IterationEndEvent:
        outcome := "completed"
        if !ev.Success {
            outcome = "failed"
        }
        return fmt.Sprintf("[%s] Bead %s %s ($%.2f)", ts, ev.BeadID, outcome, ev.TotalCostUSD)

    case *events.ErrorEvent:
        return fmt.Sprintf("[%s] ERROR: %s", ts, ev.Message)

    default:
        return fmt.Sprintf("[%s] %s", ts, e.Type())
    }
}

func formatToolSummary(toolName string, input map[string]any) string {
    switch toolName {
    case "Bash":
        if desc, ok := input["description"].(string); ok {
            return fmt.Sprintf("%q", truncate(desc, 40))
        }
        if cmd, ok := input["command"].(string); ok {
            return fmt.Sprintf("%q", truncate(cmd, 40))
        }
    case "Read", "Write", "Edit":
        if path, ok := input["file_path"].(string); ok {
            return filepath.Base(path)
        }
    case "Glob", "Grep":
        if pattern, ok := input["pattern"].(string); ok {
            return fmt.Sprintf("%q", pattern)
        }
    }
    return ""
}

func shortID(toolID string) string {
    // "toolu_01YSTWRyWLogpciN1XgcpZbK" -> "toolu_01YST..."
    if len(toolID) > 15 {
        return toolID[:15] + "..."
    }
    return toolID
}
```

### Asking Questions

```go
// Ask executes a query and returns the response synchronously.
// It acquires the session broker, builds context, and runs claude CLI.
func (o *Observer) Ask(ctx context.Context, question string) (string, error) {
    // Acquire session broker with timeout (coordinates with drain session)
    err := o.broker.Acquire(ctx, holderName, defaultAcquireTimeout)
    if err != nil {
        return "", fmt.Errorf("failed to acquire session: %w", err)
    }
    defer o.broker.Release()

    // Build context from drain state and log events
    state := DrainState{}
    if o.stateProvider != nil {
        state = o.stateProvider.GetDrainState()
    }

    contextStr, err := o.builder.Build(state)
    if err != nil {
        return "", fmt.Errorf("%w: %v", ErrNoContext, err)
    }

    // Build prompt with context and question
    prompt := fmt.Sprintf("%s\n\nQuestion: %s", contextStr, question)

    // Execute query with retry on resume failure
    response, err := o.executeQuery(ctx, prompt)
    if err != nil && o.sessionID != "" {
        // Retry once with fresh session
        o.sessionID = ""
        response, err = o.executeQuery(ctx, prompt)
    }

    return response, err
}

// executeQuery runs the claude CLI and captures output.
func (o *Observer) executeQuery(ctx context.Context, prompt string) (string, error) {
    queryCtx, cancel := context.WithTimeout(ctx, defaultQueryTimeout)
    defer cancel()

    o.mu.Lock()
    o.cancel = cancel
    o.runner = o.runnerFactory()
    o.mu.Unlock()

    // Build command arguments
    args := o.buildArgs(prompt)

    // Start the process using ProcessRunner
    stdout, stderr, err := o.runner.Start(queryCtx, "claude", args...)
    if err != nil {
        return "", fmt.Errorf("failed to start claude: %w", err)
    }

    // Read output with size limit (100KB max)
    output, _ := o.readOutput(stdout, stderr)

    // Wait for process to complete
    _ = o.runner.Wait()

    // Handle context cancellation
    if queryCtx.Err() == context.Canceled {
        return "", ErrCancelled
    }
    if queryCtx.Err() == context.DeadlineExceeded {
        _ = o.runner.Kill()
        return output, ErrQueryTimeout
    }

    return output, nil
}

// buildArgs constructs the claude CLI arguments.
func (o *Observer) buildArgs(prompt string) []string {
    args := []string{"-p", prompt, "--output-format", "text"}

    // Add --resume if we have a session ID
    if o.sessionID != "" {
        args = append([]string{"--resume", o.sessionID}, args...)
    }

    // Add model if specified and not default
    if o.config != nil && o.config.Model != "" && o.config.Model != "haiku" {
        args = append(args, "--model", o.config.Model)
    }

    return args
}

// Cancel terminates the current query if one is running.
func (o *Observer) Cancel() {
    o.mu.Lock()
    defer o.mu.Unlock()

    if o.cancel != nil {
        o.cancel()
    }
    if o.runner != nil {
        _ = o.runner.Kill()
    }
}

// Reset clears the session state for a fresh start.
func (o *Observer) Reset() {
    o.mu.Lock()
    defer o.mu.Unlock()
    o.sessionID = ""
}
```

### Loading Events from Log

LogReader handles reading events from `.atari/atari.log` with log rotation detection.

```go
// LogReader reads events from the atari log file.
type LogReader struct {
    logPath string
}

func NewLogReader(logPath string) *LogReader {
    return &LogReader{logPath: logPath}
}

// ReadRecentEvents returns the last N events for the specified bead.
func (r *LogReader) ReadRecentEvents(beadID string, limit int) []events.Event {
    file, err := os.Open(r.logPath)
    if err != nil {
        return nil
    }
    defer file.Close()

    var currentBeadEvents []events.Event
    inCurrentBead := false

    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := scanner.Bytes()

        var base struct {
            Type   string `json:"type"`
            BeadID string `json:"bead_id,omitempty"`
        }
        if err := json.Unmarshal(line, &base); err != nil {
            continue
        }

        // Track when we enter the current bead's iteration
        if base.Type == "iteration.start" && base.BeadID == beadID {
            inCurrentBead = true
            currentBeadEvents = nil // Reset - start fresh
        }

        if inCurrentBead {
            evt := parseEvent(line)
            if evt != nil {
                currentBeadEvents = append(currentBeadEvents, evt)
            }
        }
    }

    // Return last N events
    if len(currentBeadEvents) > limit {
        return currentBeadEvents[len(currentBeadEvents)-limit:]
    }
    return currentBeadEvents
}

// ReadSessionHistory returns completed bead sessions from the log.
func (r *LogReader) ReadSessionHistory() []SessionHistory {
    file, err := os.Open(r.logPath)
    if err != nil {
        return nil
    }
    defer file.Close()

    var history []SessionHistory
    beadMap := make(map[string]*SessionHistory)

    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := scanner.Bytes()

        var base struct {
            Type         string  `json:"type"`
            BeadID       string  `json:"bead_id,omitempty"`
            Title        string  `json:"title,omitempty"`
            Success      bool    `json:"success,omitempty"`
            NumTurns     int     `json:"num_turns,omitempty"`
            TotalCostUSD float64 `json:"total_cost_usd,omitempty"`
        }
        if err := json.Unmarshal(line, &base); err != nil {
            continue
        }

        switch base.Type {
        case "iteration.start":
            beadMap[base.BeadID] = &SessionHistory{
                BeadID: base.BeadID,
                Title:  base.Title,
            }
        case "iteration.end":
            if h, ok := beadMap[base.BeadID]; ok {
                h.Turns = base.NumTurns
                h.Cost = base.TotalCostUSD
                if base.Success {
                    h.Outcome = "completed"
                } else {
                    h.Outcome = "failed"
                }
                history = append(history, *h)
                delete(beadMap, base.BeadID)
            }
        }
    }

    return history
}

type SessionHistory struct {
    BeadID  string
    Title   string
    Outcome string
    Cost    float64
    Turns   int
}
```

## Configuration

```yaml
observer:
  enabled: true
  model: haiku              # Model for observer queries
  recent_events: 20         # Events for current bead context
  show_cost: true           # Display observer session cost
  layout: horizontal        # Layout: "horizontal" (side-by-side) or "vertical" (stacked)
```

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `enabled` | bool | true | Enable observer mode in TUI |
| `model` | string | "haiku" | Claude model for observer |
| `recent_events` | int | 20 | Recent events to include for current bead |
| `show_cost` | bool | true | Show observer cost in TUI |
| `layout` | string | "horizontal" | Pane layout: "horizontal" (side-by-side) or "vertical" (stacked) |

## TUI Integration

The observer appears as a pane in the TUI when activated.

### Layout

The layout is configurable via `observer.layout` setting. Default is `horizontal` (side-by-side).

**Horizontal layout (default)** - events left, observer right:

```
+-- ATARI ------------------------------------------------------------------+
| Status: WORKING                                        Cost: $2.35        |
| Current: bd-042 "Fix auth bug"                         Turns: 42          |
+-- Events -------------------------------+-- Observer (haiku) -- $0.03 ----+
| [14:02:13] Tool: Bash "Run tests"       | > Why did Claude run tests     |
| [14:02:14] Result: "PASS ok github..."  |   twice?                        |
| [14:02:15] Claude: "Tests pass. Now..." |                                 |
| [14:02:16] Tool: Edit types.go          | The tests were run twice        |
| [14:02:17] Result: "ok" (toolu_01DEF..) | because the first run had a     |
| [14:02:18] Claude: "Updated the type.." | flaky failure in test_auth.go.  |
|                                         | Claude retried to confirm       |
|                                         | whether it was genuine.         |
|                                         |                                 |
+-----------------------------------------+---------------------------------+
| [o] observer  [p] pause  [r] resume  [q] quit  [Tab] focus  [Ctrl+R] refresh |
+---------------------------------------------------------------------------+
```

**Vertical layout** - events top, observer bottom:

```
+-- ATARI ------------------------------------------------------+
| Status: WORKING                          Cost: $2.35          |
| Current: bd-042 "Fix auth bug"           Turns: 42            |
+-- Events -----------------------------------------------------+
| [14:02:13] Tool: Bash "Run tests" (toolu_01YST...)           |
| [14:02:14] Result: "PASS ok github.com/..." (toolu_01YST...) |
| [14:02:15] Claude: "Tests pass. Now I'll update the docs..." |
|                                                               |
+-- Observer (haiku) ------------------------- Cost: $0.03 -----+
| > Why did Claude run the tests twice?                         |
|                                                               |
| The tests were run twice because the first run had a flaky    |
| failure in test_auth.go. Claude retried to confirm whether    |
| it was a genuine failure or transient issue.                  |
|                                                               |
+---------------------------------------------------------------+
| [o] observer  [p] pause  [r] resume  [q] quit  [Ctrl+R] refresh|
+---------------------------------------------------------------+
```

### Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `o` | Toggle observer pane |
| `Enter` | Send question (when in observer input) |
| `Esc` | Close observer pane |
| `Tab` | Switch focus between events and observer |
| `Ctrl+R` | Refresh observer context (rebuild from current log state) |

### Focus States

- **Events focused**: Arrow keys scroll event feed
- **Observer focused**: Text input active, arrow keys navigate conversation

## Example Context

```
You are an observer assistant helping the user understand what's happening
in an automated bead processing session (Atari drain).

Your role:
- Answer questions about current activity
- Help identify issues or unexpected behavior
- Suggest when manual intervention might be needed
- Explain Claude's decision-making based on visible events

You have access to tools. If you need more details about an event, you can
use grep to look it up in the log file.

## Drain Status
State: working | Uptime: 2h 15m | Total cost: $4.23

## Session History
| Bead | Title | Outcome | Cost | Turns |
|------|-------|---------|------|-------|
| bd-drain-abc | Fix auth bug | completed | $0.36 | 8 |
| bd-drain-def | Add validation | completed | $1.27 | 24 |

## Current Bead: bd-drain-ghi
Title: Refactor parser
Started: 3m ago

### Recent Activity
[14:02:13] Started bead bd-drain-ghi: Refactor parser
[14:02:13] Session started for bd-drain-ghi
[14:02:15] Claude: "Let me check the existing parser implementation..."
[14:02:16] Tool: Read parser.go (toolu_01ABC...)
[14:02:17] Result: "package parser\n\nimport..." (toolu_01ABC...)
[14:02:18] Claude: "I see the parser uses a state machine. Let me..."
[14:02:19] Tool: Bash "Run test suite" (toolu_01DEF...)
[14:02:20] Result: "PASS ok github.com/npratt/atari..." (toolu_01DEF...)
[14:02:21] Claude: "Tests pass. Now I'll refactor the parse loop..."

## Retrieving Full Event Details
Events are stored in `.atari/atari.log` as JSON lines.
To see full event details:
  grep '<tool_id>' .atari/atari.log | jq .
To see recent events:
  tail -50 .atari/atari.log | jq -s .
To get bead details:
  bd show <bead-id>

---

User question: Why did Claude run the tests?
```

## Example Queries

Users might ask:
- "Why did Claude choose to modify that file?"
- "What error caused the last retry?"
- "Summarize what's happened so far"
- "Is the current approach likely to succeed?"
- "What's taking so long?"
- "Should I pause and intervene?"

## Testing

### Unit Tests (`internal/observer/observer_test.go`)

- `TestNewObserver`: Observer construction with configuration
- `TestObserver_Ask_Success`: Basic query execution
- `TestObserver_Ask_BrokerTimeout`: Session broker coordination
- `TestObserver_Ask_OutputTruncation`: Output limit enforcement (100KB)
- `TestObserver_Cancel`: Query cancellation via Cancel()
- `TestObserver_Reset`: Session state reset
- `TestObserver_BuildArgs`: CLI argument construction
- `TestObserver_WithStateProvider`: DrainState integration
- `TestLimitedWriter`: Output size limiting

### E2E Tests (`internal/integration/observer_test.go`)

- `TestObserverBasicQuery`: Full Q&A flow with mock claude
- `TestObserverBrokerCoordination`: Session broker mutex behavior
- `TestObserverCancel`: Query cancellation mid-execution
- `TestObserverTimeout`: Context timeout handling
- `TestObserverContextIncludesLogEvents`: Log event integration
- `TestObserverSessionHistory`: Multi-bead history tracking
- `TestObserverFailedBead`: Failed bead context
- `TestObserverErrorFromClaude`: Claude CLI error handling
- `TestObserverReset`: Session reset behavior
- `TestObserverModelConfiguration`: Model override testing
- `TestObserverEmptyLog`: Empty log file handling
- `TestObserverNoLogFile`: Missing log file handling

### Test Infrastructure

- `internal/testutil/mock_claude.go`: Mock claude script generation
- `internal/testutil/observer_fixtures.go`: Log event fixtures

## Error Handling

| Error | Action |
|-------|--------|
| Claude not available | Show error in observer pane, allow retry |
| Log file not found | Show "No events yet" message |
| Session resume fails | Clear session ID, rebuild context on next question |
| Response timeout | Show timeout message, allow retry |
| Parse error | Log warning, skip malformed events |

## Future Considerations

- **Streaming responses**: Show response as it generates (requires stream-json parsing)
- **Event filtering**: Focus context on specific event types
- **Model switching**: Change models mid-session for deeper analysis
- **Query history**: Show previous Q&A in the pane
- **Intervention actions**: Direct commands like "pause drain" from observer
