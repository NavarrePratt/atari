# Work Queue Manager

Discovers available work and manages bead selection with history tracking and backoff logic.

## Purpose

The Work Queue Manager is responsible for:
- Polling `br ready --json` for available beads
- Tracking bead history (attempts, failures, completions)
- Implementing exponential backoff for failed beads
- Selecting the next bead to work on based on priority and history

## Interface

```go
type Manager struct {
    config  *config.Config
    history map[string]*BeadHistory
    mu      sync.RWMutex
}

type Bead struct {
    ID          string
    Title       string
    Priority    int
    Labels      []string
    CreatedAt   time.Time
    Description string
}

type BeadHistory struct {
    ID          string
    Status      HistoryStatus  // pending, working, completed, failed
    Attempts    int
    LastAttempt time.Time
    LastError   string
}

type HistoryStatus string

const (
    HistoryPending   HistoryStatus = "pending"
    HistoryWorking   HistoryStatus = "working"
    HistoryCompleted HistoryStatus = "completed"
    HistoryFailed    HistoryStatus = "failed"
    HistoryAbandoned HistoryStatus = "abandoned"  // Hit max_failures limit
)

// Public API
func New(cfg *config.Config) *Manager
func (m *Manager) Next(ctx context.Context) (*Bead, error)
func (m *Manager) RecordSuccess(beadID string)
func (m *Manager) RecordFailure(beadID string, err error)
func (m *Manager) History() map[string]*BeadHistory
func (m *Manager) Stats() QueueStats
```

## Dependencies

| Component | Usage |
|-----------|-------|
| config.Config | Poll interval, backoff settings, label filters |

External:
- `br ready --json` command

## Dependency Handling

**Atari does NOT manage bead dependencies directly.** Dependencies are handled entirely by beads:

1. **bd-sequence skill** (or manual `br dep add`) sets dependencies between beads
2. **br ready** only returns beads with no unsatisfied blocking dependencies
3. When a blocking bead is closed, blocked beads automatically become "ready"

This means atari simply trusts `br ready` to return the correct set of workable beads. The work queue's job is to:
- Poll `br ready` for available work
- Apply backoff filtering for previously failed beads
- Select by priority among eligible beads

Example dependency flow:
```
# bd-sequence sets: bd-001 blocks bd-002 blocks bd-003

br ready --json
# Returns: [bd-001]  (bd-002, bd-003 blocked)

# Atari works on bd-001, closes it

br ready --json
# Returns: [bd-002]  (bd-003 still blocked)

# Atari works on bd-002, closes it

br ready --json
# Returns: [bd-003]  (now unblocked)
```

This design keeps atari simple - it doesn't need to understand the dependency graph.

## Race Condition Prevention

**Problem**: New beads created while atari is running could be picked up before being properly sequenced.

**Solution**: Label-based gating with the `workqueue.label` config.

### How It Works

1. Configure atari with a label filter:
   ```yaml
   workqueue:
     label: "automated"  # Only process beads with this label
   ```

2. `bd-plan` and `bd-plan-ultra` skills create issues WITHOUT the label

3. After `bd-sequence` orders all issues, add the label:
   ```bash
   br update bd-001 --labels automated
   br update bd-002 --labels automated
   # ... or batch update all sequenced issues
   ```

4. NOW they appear in `br ready --label automated`

### Why This Works

- **Zero race condition**: Label is only added AFTER all dependencies are set
- **Explicit opt-in**: Issues must be explicitly marked for automation
- **Visible state**: `br list --label automated` shows what atari will process
- **Easy debugging**: If issues aren't being processed, check if they have the label

### Alternative: Status-Based Gating

You can also use `status: deferred` as a gate:

1. Create issues, immediately update to `deferred`:
   ```bash
   br create "Title" --json && br update <id> --status deferred
   ```

2. After sequencing, update to `open`:
   ```bash
   br update bd-001 --status open
   ```

3. `br ready` only returns `open` or `in_progress` issues

**Caveat**: Small race window between create and status update. Label-based is preferred.

### Recommended Workflow

For automated atari runs, the full workflow is:

```bash
# 1. Run bd-plan to create issues (no label)
/bd-plan "Implement feature X"

# 2. Issues exist but NOT in atari's queue (missing label)

# 3. Run bd-sequence to order them
/bd-sequence

# 4. After sequencing, mark for automation
br list --json | jq -r '.[].id' | xargs -I{} br update {} --labels automated

# 5. NOW atari will pick them up in dependency order
```

This workflow ensures issues are never processed out of order.

## Implementation

### Polling br ready

```go
func (m *Manager) poll(ctx context.Context) ([]Bead, error) {
    args := []string{"ready", "--json"}

    if m.config.Label != "" {
        args = append(args, "--label", m.config.Label)
    }

    cmd := exec.CommandContext(ctx, "br", args...)
    output, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("br ready failed: %w", err)
    }

    var beads []Bead
    if err := json.Unmarshal(output, &beads); err != nil {
        return nil, fmt.Errorf("parse br ready output: %w", err)
    }

    return beads, nil
}
```

### br ready JSON Format

Expected output from `br ready --json`:

```json
[
  {
    "id": "bd-042",
    "title": "Fix authentication bug",
    "priority": 1,
    "labels": ["bug", "auth"],
    "created_at": "2024-01-15T10:00:00Z",
    "description": "Users are getting logged out unexpectedly..."
  },
  {
    "id": "bd-043",
    "title": "Add rate limiting",
    "priority": 2,
    "labels": ["feature"],
    "created_at": "2024-01-15T11:00:00Z",
    "description": "Implement rate limiting for API endpoints..."
  }
]
```

### Bead Selection

The `Next` method selects the best bead to work on:

```go
func (m *Manager) Next(ctx context.Context) (*Bead, error) {
    beads, err := m.poll(ctx)
    if err != nil {
        return nil, err
    }

    if len(beads) == 0 {
        return nil, nil
    }

    // Filter out beads that are in backoff
    eligible := m.filterEligible(beads)
    if len(eligible) == 0 {
        return nil, nil
    }

    // Sort by priority (lower is higher priority), then by created_at
    sort.Slice(eligible, func(i, j int) bool {
        if eligible[i].Priority != eligible[j].Priority {
            return eligible[i].Priority < eligible[j].Priority
        }
        return eligible[i].CreatedAt.Before(eligible[j].CreatedAt)
    })

    selected := eligible[0]

    // Mark as working
    m.mu.Lock()
    m.history[selected.ID] = &BeadHistory{
        ID:          selected.ID,
        Status:      HistoryWorking,
        Attempts:    m.history[selected.ID].Attempts + 1,
        LastAttempt: time.Now(),
    }
    m.mu.Unlock()

    return &selected, nil
}
```

### Backoff Logic

Beads that fail repeatedly get exponential backoff:

```go
func (m *Manager) filterEligible(beads []Bead) []Bead {
    m.mu.RLock()
    defer m.mu.RUnlock()

    var eligible []Bead
    now := time.Now()

    for _, bead := range beads {
        history, exists := m.history[bead.ID]
        if !exists {
            eligible = append(eligible, bead)
            continue
        }

        if history.Status == HistoryCompleted {
            // Already completed, skip
            continue
        }

        if history.Status == HistoryAbandoned {
            // Hit max failures, skip permanently
            continue
        }

        if history.Status == HistoryFailed {
            // Check if we've hit max failures
            if m.config.Backoff.MaxFailures > 0 && history.Attempts >= m.config.Backoff.MaxFailures {
                // Mark as abandoned and emit notification
                m.markAbandoned(bead.ID, history)
                continue
            }

            backoff := m.calculateBackoff(history.Attempts)
            if now.Sub(history.LastAttempt) < backoff {
                // Still in backoff period
                continue
            }
        }

        eligible = append(eligible, bead)
    }

    return eligible
}

func (m *Manager) markAbandoned(beadID string, history *BeadHistory) {
    // This is called while holding the read lock, so we need to upgrade
    // In practice, this would be handled differently to avoid lock issues
    history.Status = HistoryAbandoned

    // Emit event for notification system
    m.events.Emit(&events.BeadAbandoned{
        BeadID:      beadID,
        Attempts:    history.Attempts,
        LastError:   history.LastError,
        MaxFailures: m.config.Backoff.MaxFailures,
    })
}

func (m *Manager) calculateBackoff(attempts int) time.Duration {
    if attempts <= 1 {
        return 0
    }

    backoff := m.config.Backoff.Initial
    for i := 1; i < attempts; i++ {
        backoff = time.Duration(float64(backoff) * m.config.Backoff.Multiplier)
        if backoff > m.config.Backoff.Max {
            return m.config.Backoff.Max
        }
    }

    return backoff
}
```

### Backoff Configuration

```go
type BackoffConfig struct {
    Initial     time.Duration // e.g., 1 minute
    Max         time.Duration // e.g., 1 hour
    Multiplier  float64       // e.g., 2.0
    MaxFailures int           // e.g., 5 (0 = unlimited)
}
```

Example progression with Initial=1m, Multiplier=2, Max=1h, MaxFailures=5:
- Attempt 1: no backoff
- Attempt 2: 1 minute
- Attempt 3: 2 minutes
- Attempt 4: 4 minutes
- Attempt 5: 8 minutes, then **abandoned** (triggers `bead.abandoned` notification)

Setting `MaxFailures=0` disables the limit (unlimited retries with backoff).

When a bead is abandoned:
1. Status changes to `HistoryAbandoned`
2. A `bead.abandoned` event is emitted
3. The bead is skipped in future polling
4. Notification triggers can alert the user

### Recording Results

```go
func (m *Manager) RecordSuccess(beadID string) {
    m.mu.Lock()
    defer m.mu.Unlock()

    m.history[beadID] = &BeadHistory{
        ID:          beadID,
        Status:      HistoryCompleted,
        Attempts:    m.history[beadID].Attempts,
        LastAttempt: time.Now(),
    }
}

func (m *Manager) RecordFailure(beadID string, err error) {
    m.mu.Lock()
    defer m.mu.Unlock()

    history := m.history[beadID]
    history.Status = HistoryFailed
    history.LastError = err.Error()
    history.LastAttempt = time.Now()
}
```

### Statistics

```go
type QueueStats struct {
    TotalSeen      int
    Completed      int
    Failed         int
    InBackoff      int
    CurrentlyReady int
}

func (m *Manager) Stats() QueueStats {
    m.mu.RLock()
    defer m.mu.RUnlock()

    stats := QueueStats{}
    now := time.Now()

    for _, h := range m.history {
        stats.TotalSeen++
        switch h.Status {
        case HistoryCompleted:
            stats.Completed++
        case HistoryFailed:
            stats.Failed++
            backoff := m.calculateBackoff(h.Attempts)
            if now.Sub(h.LastAttempt) < backoff {
                stats.InBackoff++
            }
        }
    }

    return stats
}
```

## Persistence

History is persisted as part of the main state file. See [sinks.md](sinks.md) for state persistence details.

```json
{
  "history": {
    "bd-040": {"status": "completed", "attempts": 1},
    "bd-041": {"status": "completed", "attempts": 1},
    "bd-039": {"status": "failed", "attempts": 3, "last_error": "tests failing"}
  }
}
```

## Testing

### Unit Tests

- `filterEligible`: verify backoff filtering works correctly
- `calculateBackoff`: verify exponential growth and max cap
- `Next`: verify priority ordering
- Concurrent access: verify mutex protection

### Test Cases

```go
func TestBackoffProgression(t *testing.T) {
    cfg := &config.Config{
        Backoff: BackoffConfig{
            Initial:    time.Minute,
            Max:        time.Hour,
            Multiplier: 2.0,
        },
    }
    m := New(cfg)

    cases := []struct {
        attempts int
        expected time.Duration
    }{
        {1, 0},
        {2, time.Minute},
        {3, 2 * time.Minute},
        {4, 4 * time.Minute},
        {10, time.Hour}, // capped
    }

    for _, tc := range cases {
        got := m.calculateBackoff(tc.attempts)
        if got != tc.expected {
            t.Errorf("attempts=%d: got %v, want %v", tc.attempts, got, tc.expected)
        }
    }
}

func TestPriorityOrdering(t *testing.T) {
    // Verify P0 beads selected before P1, etc.
}

func TestBackoffFiltering(t *testing.T) {
    // Verify beads in backoff period are skipped
}
```

## Error Handling

| Error | Action |
|-------|--------|
| `br ready` command fails | Return error, caller retries with backoff |
| JSON parse error | Return error with context |
| Empty result | Return nil bead (not an error) |

## Future Considerations

- **Label filtering**: Support multiple labels, exclusions
- **Custom priority logic**: Pluggable priority functions
- **Bead grouping**: Work on related beads together

Note: Dependency handling is NOT a future consideration - it's handled by `br ready`. See [Dependency Handling](#dependency-handling).
