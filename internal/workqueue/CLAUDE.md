# Workqueue Package

Manages work discovery by polling `bd ready` with exponential backoff for failed beads.

## Key Types

### Manager

Discovers available work and tracks bead processing history.

```go
mgr := workqueue.New(cfg, client, logger)  // logger can be nil for default

// Get next eligible bead (highest priority, not in backoff)
// Returns (*Bead, SelectionReason, error)
bead, reason, err := mgr.Next(ctx)

// Check why no bead was selected
if bead == nil && reason == workqueue.ReasonBackoff {
    // All ready beads are in backoff period
}

// Record outcomes
mgr.RecordSuccess(beadID)
mgr.RecordFailure(beadID, err)

// Check statistics
stats := mgr.Stats()

// State persistence
history := mgr.History()
mgr.SetHistory(savedHistory)
```

### SelectionReason

Indicates why a bead selection returned the result it did:

- `ReasonSuccess` - Bead selected successfully
- `ReasonNoReady` - No ready beads available
- `ReasonBackoff` - All ready beads in backoff period
- `ReasonMaxFailure` - All ready beads hit max failures

### Bead

Issue from `bd ready --json` output:

```go
type Bead struct {
    ID          string
    Title       string
    Description string
    Status      string
    Priority    int      // Lower = higher priority
    IssueType   string
    Labels      []string
    CreatedAt   time.Time
    CreatedBy   string
    UpdatedAt   time.Time
}
```

## Selection Logic

`Next()` applies these filters in order:
1. Poll `bd ready --json` with optional label filter
2. Filter out completed and abandoned beads
3. Filter out beads still in backoff period
4. Sort by priority (ascending), then by creation time
5. Mark selected bead as working and increment attempts

## Backoff Behavior

Failed beads enter exponential backoff:
- First failure: `config.Backoff.Initial` (default 1m)
- Subsequent failures: Previous delay * `config.Backoff.Multiplier` (default 2.0)
- Maximum delay: `config.Backoff.Max` (default 1h)
- After `config.Backoff.MaxFailures` (default 5): bead marked abandoned

## History Statuses

Re-exported from events package for convenience:
- `HistoryPending` - Initial state
- `HistoryWorking` - Currently being processed
- `HistoryCompleted` - Successfully finished
- `HistoryFailed` - Failed, may retry after backoff
- `HistoryAbandoned` - Exceeded max failures

## Integration

Uses `testutil.CommandRunner` interface for bd command execution, enabling mocked tests.
