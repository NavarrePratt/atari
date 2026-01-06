# Observer Package

Provides the TUI observer mode for real-time Q&A about drain activity.

## Overview

The observer package implements an interactive Q&A system that allows users to ask questions about what's happening during an Atari drain session. It uses Claude CLI to answer questions with context from the event log and current drain state.

## Components

### Observer (observer.go)

The main Q&A handler that manages Claude sessions for answering user questions.

```go
// DrainStateProvider provides current drain state for context building.
type DrainStateProvider interface {
    GetDrainState() DrainState
}

// Exchange represents a single Q&A exchange in the observer session.
type Exchange struct {
    Question string
    Answer   string
}

// Observer handles interactive Q&A queries using Claude CLI.
type Observer struct {
    config        *config.ObserverConfig
    broker        *SessionBroker
    builder       *ContextBuilder
    stateProvider DrainStateProvider
    runnerFactory func() runner.ProcessRunner

    mu        sync.Mutex
    sessionID string      // Claude session ID for --resume
    runner    runner.ProcessRunner
    cancel    context.CancelFunc
    history   []Exchange  // conversation history for session continuity
}

// Public API
func NewObserver(cfg *config.ObserverConfig, broker *SessionBroker, builder *ContextBuilder, stateProvider DrainStateProvider) *Observer
func (o *Observer) Ask(ctx context.Context, question string) (string, error)
func (o *Observer) Cancel()   // Cancel in-progress query
func (o *Observer) Reset()    // Clear session and history for fresh start
func (o *Observer) SetRunnerFactory(factory func() runner.ProcessRunner)  // For testing
```

**Key behaviors:**
- Uses `claude -p --output-format stream-json` for queries
- Supports `--resume` for follow-up questions
- Maintains conversation history in memory
- Output limited to 100KB with truncation marker
- 60-second default query timeout

**Errors:**
- `ErrCancelled`: Query cancelled by user
- `ErrQueryTimeout`: Query exceeded timeout
- `ErrNoContext`: Failed to build context

### SessionBroker (broker.go)

Coordinates access to Claude CLI between drain and observer sessions.

```go
// SessionBroker coordinates access to the Claude CLI process.
// Only one Claude process can run at a time.
type SessionBroker struct {
    mu     sync.RWMutex
    holder string        // "drain", "observer", or "" if unlocked
    sem    chan struct{} // semaphore channel
}

// Public API
func NewSessionBroker() *SessionBroker
func (b *SessionBroker) Acquire(ctx context.Context, holder string, timeout time.Duration) error
func (b *SessionBroker) TryAcquire(holder string) bool
func (b *SessionBroker) Release()
func (b *SessionBroker) Holder() string
func (b *SessionBroker) IsHeld() bool
```

**Key behaviors:**
- Thread-safe semaphore-based coordination
- Context cancellation support
- Configurable timeout on acquisition
- Safe to call Release multiple times
- Holder tracking for debugging

**Note:** Observer currently runs independently of drain - they use different models and are separate processes. The broker is available for future coordination if needed.

### ContextBuilder (context.go)

Builds structured context prompts from log events and drain state.

```go
// DrainState holds the current state of the drain for context building.
type DrainState struct {
    Status       string
    Uptime       time.Duration
    TotalCost    float64
    CurrentBead  *CurrentBeadInfo
    CurrentTurns int
}

// CurrentBeadInfo holds information about the currently active bead.
type CurrentBeadInfo struct {
    ID        string
    Title     string
    StartedAt time.Time
}

// SessionHistory holds information about a completed bead session.
type SessionHistory struct {
    BeadID  string
    Title   string
    Outcome string
    Cost    float64
    Turns   int
}

// ContextBuilder assembles structured context from log events.
type ContextBuilder struct {
    logReader *LogReader
    config    *config.ObserverConfig
}

// Public API
func NewContextBuilder(logReader *LogReader, cfg *config.ObserverConfig) *ContextBuilder
func (b *ContextBuilder) Build(state DrainState, conversation []Exchange) (string, error)
func FormatEvent(e events.Event) string  // Package-level function
```

**Context sections:**
1. System prompt (observer role description)
2. Drain status (state, uptime, cost, current turn)
3. Session history (last 5 completed beads)
4. Current bead (ID, title, recent events)
5. Tips (how to retrieve full event details)
6. Conversation history (prior Q&A exchanges)

### LogReader (logreader.go)

Reads events from `.atari/atari.log` with rotation detection.

```go
// LogReader reads events from the atari log file with rotation detection.
type LogReader struct {
    path      string
    lastInode uint64
    lastSize  int64
}

// Public API
func NewLogReader(path string) *LogReader
func (r *LogReader) ReadRecent(n int) ([]events.Event, error)
func (r *LogReader) ReadByBeadID(beadID string) ([]events.Event, error)
func (r *LogReader) ReadAfterTimestamp(t time.Time) ([]events.Event, error)
```

**Key behaviors:**
- Detects log rotation via inode/size changes
- Handles lines up to 1MB
- Truncates oversized lines with marker
- Parses all event types defined in events package
- Returns typed events for filtering

**Errors:**
- `ErrFileNotFound`: Log file does not exist
- `ErrEmptyFile`: Log file is empty

## Configuration

Observer uses `config.ObserverConfig`:

```go
type ObserverConfig struct {
    Enabled      bool   // Default: true
    Model        string // Default: "haiku"
    RecentEvents int    // Default: 20
    ShowCost     bool   // Default: true
    Layout       string // "horizontal" or "vertical"
}
```

## Testing

### Unit Tests

- `observer_test.go`: Observer construction, Ask, Cancel, Reset, argument building
- `broker_test.go`: SessionBroker Acquire, TryAcquire, Release, concurrent access
- `context_test.go`: ContextBuilder sections, event formatting, truncation
- `logreader_test.go`: LogReader parsing, filtering, rotation detection

### Test Utilities

- Use `SetRunnerFactory()` to inject mock ProcessRunner
- Create mock log files with test events
- Test fixtures in `testutil/observer_fixtures.go`

## Design Notes

1. **Independent sessions**: Observer uses separate Claude process from drain
2. **Session continuity**: Uses `--resume` for follow-up questions
3. **Memory-based history**: Conversation history stored in Observer struct
4. **Output truncation**: 100KB limit prevents runaway responses
5. **Log-based context**: Reads from existing `.atari/atari.log` file
6. **Event summarization**: Truncates text to fit context limits

## Usage Example

```go
// Create components
logReader := observer.NewLogReader(".atari/atari.log")
builder := observer.NewContextBuilder(logReader, cfg.Observer)
broker := observer.NewSessionBroker()

// Create observer with drain state provider
obs := observer.NewObserver(cfg.Observer, broker, builder, drainStateProvider)

// Ask a question
response, err := obs.Ask(ctx, "What is Claude doing right now?")
if err != nil {
    log.Printf("Observer error: %v", err)
}

// Follow-up question (uses --resume)
response, err = obs.Ask(ctx, "Why did it choose that approach?")

// Reset for fresh start
obs.Reset()
```
