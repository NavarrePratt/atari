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

## Component Overview

| Component | Description |
|-----------|-------------|
| Observer | Main Q&A handler with Ask/Cancel/Reset methods |
| SessionBroker | Coordinates Claude CLI access between drain and observer |
| ContextBuilder | Builds structured prompts from log events and drain state |
| LogReader | Reads events from `.atari/atari.log` |
| DrainStateProvider | Interface for accessing current drain state |

## Dependencies

| Component | Usage |
|-----------|-------|
| config.ObserverConfig | Model, event count, display settings |
| SessionBroker | Available for future coordination features (observer runs independently of drain) |
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

The context sent to observer Claude sessions includes:

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
  br show <bead-id>

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

## Error Handling

| Error | Action |
|-------|--------|
| Claude not available | Show error in observer pane, allow retry |
| Log file not found | Show "No events yet" message |
| Session resume fails | Clear session ID, rebuild context on next question |
| Response timeout | Show timeout message, allow retry |
| Parse error | Log warning, skip malformed events |

## Testing

### Unit Tests (`internal/observer/*_test.go`)

- Observer construction and configuration
- Query execution with mock claude
- Output truncation (100KB limit)
- Query cancellation and reset
- CLI argument construction
- DrainState integration
- Session broker coordination

### E2E Tests (`internal/integration/observer_test.go`)

- Full Q&A flow with mock claude
- Session broker mutex behavior
- Query cancellation mid-execution
- Context timeout handling
- Log event integration
- Multi-bead history tracking
- Failed bead context
- Claude CLI error handling
- Model override testing
- Empty/missing log file handling

### Test Infrastructure

- `internal/testutil/mock_claude.go`: Mock claude script generation
- `internal/testutil/observer_fixtures.go`: Log event fixtures

## Future Considerations

- **Streaming responses**: Show response as it generates (requires stream-json parsing)
- **Event filtering**: Focus context on specific event types
- **Model switching**: Change models mid-session for deeper analysis
- **Query history**: Show previous Q&A in the pane
- **Intervention actions**: Direct commands like "pause drain" from observer
