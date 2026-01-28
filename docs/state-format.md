# State File Format

Atari persists state to `.atari/state.json` for crash recovery and session continuity.

## Format

```json
{
  "version": 1,
  "status": "running",
  "iteration": 5,
  "current_bead": "bd-123",
  "history": {
    "bd-001": {
      "id": "bd-001",
      "status": "completed",
      "attempts": 1,
      "last_attempt": "2024-01-15T10:30:00Z",
      "last_session_id": "sess-abc123"
    },
    "bd-002": {
      "id": "bd-002",
      "status": "failed",
      "attempts": 3,
      "last_attempt": "2024-01-15T11:00:00Z",
      "last_error": "tests failed"
    }
  },
  "total_cost": 1.25,
  "total_turns": 150,
  "updated_at": "2024-01-15T11:05:00Z",
  "active_top_level": "bd-epic-001",
  "active_top_level_title": "Feature Epic"
}
```

## Fields

| Field | Type | Description |
|-------|------|-------------|
| `version` | int | State format version for migration compatibility |
| `status` | string | Current drain status: "running", "paused", "stopped" |
| `iteration` | int | Total number of bead iterations (increments each work attempt) |
| `current_bead` | string | Bead ID currently being worked on (empty if idle) |
| `history` | object | Map of bead ID to history entry |
| `total_cost` | float | Cumulative API cost in USD |
| `total_turns` | int | Cumulative number of API turns |
| `updated_at` | string | ISO 8601 timestamp of last state update |
| `active_top_level` | string | ID of the active top-level item (epic or standalone bead) |
| `active_top_level_title` | string | Title of the active top-level item |

### History Entry Fields

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Bead ID |
| `status` | string | "working", "completed", "failed", "abandoned" |
| `attempts` | int | Number of attempts for this bead |
| `last_attempt` | string | ISO 8601 timestamp of last attempt |
| `last_error` | string | Error message from last failed attempt |
| `last_session_id` | string | Claude session ID for potential resume |

## Versioning

The `version` field tracks the state file format version. This enables safe upgrades when the format changes.

**Current version**: 1

### Version Handling on Load

When atari loads a state file:

1. **Compatible version**: State is loaded normally
2. **Missing version** (version=0): State is backed up and reset
3. **Incompatible version**: State is backed up and reset
4. **Corrupted JSON**: State is backed up and reset

### Backup Behavior

When the state file cannot be loaded due to version mismatch or corruption:

1. The existing file is moved to `.atari/state.json.backup`
2. A warning is logged with details about the incompatibility
3. A fresh state is initialized with the current version

This prevents data loss while ensuring atari can always start cleanly.

### Future Migration

When incrementing the version in future releases:

1. Update `CurrentStateVersion` constant in `internal/events/statesink.go`
2. Add migration logic if data can be preserved
3. Document changes in this file

For breaking changes where migration is not possible, the backup-and-reset behavior ensures a clean start.

## Atomic Writes

State is written atomically using a temp-file-and-rename pattern:

1. Write to `.atari/state.json.tmp`
2. Rename to `.atari/state.json`

This prevents corruption from incomplete writes during crashes.

## Save Frequency

State is saved:

- On drain stop (immediate)
- On significant events (debounced, default 5 seconds)
- On graceful shutdown (final flush)

The debounce delay prevents excessive disk writes during rapid event sequences.
