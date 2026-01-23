# Configuration

Configuration file format and settings for atari.

## Configuration Sources

Configuration is loaded from multiple sources in this order (later overrides earlier):

1. Built-in defaults
2. Global config file: `~/.config/atari/config.yaml`
3. Project config file: `.atari/config.yaml`
4. Environment variables
5. Command-line flags

## Config File Discovery

Atari looks for config files in this order:

1. `--config` flag value
2. `ATARI_CONFIG` environment variable
3. `.atari/config.yaml` in current directory
4. `~/.config/atari/config.yaml`

## Config File Format

```yaml
# .atari/config.yaml

# Claude Code settings
# By default, atari relies on your global Claude config (~/.claude/settings.json)
# Only specify extra_args if you need to override global settings for atari sessions
claude:
  timeout: 60m                    # Session timeout (no activity)
  max_turns: 0                    # Max turns per session batch (0 = unlimited)
  extra_args: []                  # Extra CLI args to pass to claude

# Work queue settings
workqueue:
  poll_interval: 5s              # How often to check br ready
  label: ""                      # Filter beads by label (optional)
  epic: ""                       # Filter beads to specific epic (optional)
  unassigned_only: false         # Only claim unassigned beads
  exclude_labels: []             # Labels to exclude
  selection_mode: top-level      # "top-level" or "global"
  eager_switch: false            # Switch to higher priority beads mid-session

# Backoff settings for failed beads
backoff:
  initial: 1m                    # Initial backoff duration
  max: 1h                        # Maximum backoff duration
  multiplier: 2.0                # Backoff multiplier
  max_failures: 5                # Abandon bead after N failures (0 = unlimited)

# BD activity integration
bdactivity:
  enabled: true                  # Enable bd activity stream
  reconnect_delay: 5s            # Delay before reconnecting
  max_reconnect_delay: 5m        # Maximum reconnect delay

# Paths (relative to project root)
paths:
  state: .atari/state.json       # State file
  log: .atari/atari.log          # Log file
  socket: .atari/atari.sock      # Unix socket
  pid: .atari/atari.pid          # PID file

# Log rotation settings
log_rotation:
  max_size_mb: 100               # Max log file size in MB
  max_backups: 3                 # Number of old log files to retain
  max_age_days: 7                # Days to retain old log files
  compress: true                 # Compress rotated logs

# Prompt template (inline or file path)
prompt: |
  Run "br ready --json" to find available work...

# Or reference a file
# prompt_file: .atari/prompt.txt

# Observer settings (see tui.md for usage details)
observer:
  enabled: true                  # Enable observer mode in TUI
  model: haiku                   # Model for observer queries
  recent_events: 20              # Events for current bead context
  show_cost: true                # Display observer session cost
  layout: horizontal             # Pane layout: "horizontal" or "vertical"

# Graph pane settings (see tui.md for usage details)
graph:
  enabled: true                  # Enable graph pane in TUI
  density: standard              # Node density: minimal, compact, standard, verbose
  refresh_on_event: false        # Auto-refresh on events
  auto_refresh_interval: 5s      # Auto-refresh interval

# Follow-up sessions for unclosed beads
follow_up:
  enabled: true                  # Enable follow-up sessions
  max_turns: 5                   # Max turns for follow-up session

# Wrap-up prompts on graceful pause
wrap_up:
  enabled: true                  # Enable wrap-up prompt on pause
  timeout: 60s                   # Timeout for wrap-up response

# Logging
logging:
  level: info                    # debug, info, warn, error
  format: json                   # json or text
```

## Environment Variables

All config values can be overridden via environment variables:

| Variable | Config Path | Description |
|----------|-------------|-------------|
| `ATARI_CONFIG` | - | Config file path |
| `ATARI_LABEL` | `workqueue.label` | Bead label filter |
| `ATARI_LOG` | `paths.log` | Log file path |
| `ATARI_NO_TUI` | - | Disable TUI (set to "1") |
| `ATARI_DEBUG` | `logging.level` | Set to debug level |

Environment variable naming convention:
- Prefix: `ATARI_`
- Path separator: `_`
- Uppercase

Example:
```bash
export ATARI_LABEL=automated
export ATARI_DEBUG=1
```

Note: Claude model and settings come from your global Claude config (`~/.claude/settings.json`). Use `extra_args` in config if you need to override for atari sessions.

## Configuration Sections

### Claude Settings

By default, atari uses your global Claude configuration from `~/.claude/settings.json`. This is the natural place for model selection, permissions, and other Claude settings.

```yaml
claude:
  timeout: 60m
  max_turns: 0
  extra_args: []
```

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `timeout` | duration | 60m | Kill session if no activity |
| `max_turns` | int | 0 | Max turns per batch (0 = unlimited) |
| `extra_args` | []string | [] | Extra CLI args passed to claude |

To override global settings for atari sessions only:

```yaml
claude:
  timeout: 10m
  max_turns: 100
  extra_args:
    - "--model"
    - "sonnet"
```

### Work Queue Settings

```yaml
workqueue:
  poll_interval: 5s
  label: "automated"       # Recommended: filter to explicitly-marked beads
  epic: ""                 # Restrict to beads under a specific epic
  unassigned_only: false   # Only claim unassigned beads
  exclude_labels: []       # Labels to exclude from selection
  selection_mode: top-level  # Selection strategy: "top-level" or "global"
  eager_switch: false      # Switch to higher priority beads mid-session
```

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `poll_interval` | duration | 5s | How often to poll `br ready` |
| `label` | string | "" | Filter beads by label (include only) |
| `epic` | string | "" | Restrict work to beads under this epic |
| `unassigned_only` | bool | false | Only claim beads with no assignee |
| `exclude_labels` | []string | [] | Beads with any of these labels will be skipped |
| `selection_mode` | string | "top-level" | Selection strategy (see below) |
| `eager_switch` | bool | false | Switch to higher priority bead when one becomes available |

**Important**: Setting `workqueue.label` is recommended for production use to prevent race conditions where new beads are picked up before being properly sequenced.

#### Human-Only Beads

To prevent atari from claiming certain beads (leaving them for humans), use `unassigned_only` and/or `exclude_labels`:

```yaml
workqueue:
  unassigned_only: true
  exclude_labels:
    - human
    - manual
    - needs-review
```

With this configuration:
- Beads assigned to a user are skipped (use `--assignee npratt` when creating)
- Beads with "human", "manual", or "needs-review" labels are skipped

Example creating human-only beads:
```bash
# Option 1: Assign to yourself
br create "Design decision needed" --assignee npratt

# Option 2: Add exclusion label
br create "Needs human review" --labels human
```

#### Workflow with label filtering
1. Create beads via `/issue-plan` (no label)
2. Sequence them via dependencies
3. Add the label: `br update <id> --labels automated`
4. Atari picks them up in correct dependency order

#### Selection Modes

The `selection_mode` setting controls how atari chooses the next bead to work on:

**top-level** (default): Groups work by top-level items (epics and standalone beads). Atari focuses on one epic at a time until all its work is complete, then moves to the next. This prevents context-switching between unrelated work.

- Top-level items are sorted by priority (lower number = higher priority), then creation time
- Epic priority matters: A P1 epic's work completes before a P2 epic begins
- Standalone beads (no parent) compete with epics at the top level

**global**: Pure priority-based selection across all beads. The highest-priority ready bead is always selected, regardless of epic grouping. This can cause frequent context switches between unrelated work.

**When to set epic priority**:
- P0-P1: Critical/blocking work that should complete before other epics
- P2: Normal priority (default), epics processed by creation time
- P3-P4: Lower priority work, processed after higher-priority epics

Example with two epics:
```yaml
# Epic A (P1): Urgent bug fixes
# Epic B (P2): New feature

# With top-level mode: All of Epic A's beads complete first
# With global mode: Beads interleave based on individual priorities
```

### Backoff Settings

```yaml
backoff:
  initial: 1m
  max: 1h
  multiplier: 2.0
  max_failures: 5
```

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `initial` | duration | 1m | Initial backoff after first failure |
| `max` | duration | 1h | Maximum backoff duration |
| `multiplier` | float | 2.0 | Multiply backoff each failure |
| `max_failures` | int | 5 | Abandon bead after N failures (0 = unlimited) |

Example progression with max_failures=5:
- Failure 1: 1 minute backoff
- Failure 2: 2 minutes backoff
- Failure 3: 4 minutes backoff
- Failure 4: 8 minutes backoff
- Failure 5: abandoned (triggers `bead.abandoned` event)

When a bead is abandoned, it will not be retried again.

### BD Activity Settings

```yaml
bdactivity:
  enabled: true
  reconnect_delay: 5s
  max_reconnect_delay: 5m
```

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `enabled` | bool | true | Enable watching `.beads/issues.jsonl` for changes |
| `reconnect_delay` | duration | 5s | Initial delay before reconnecting on error |
| `max_reconnect_delay` | duration | 5m | Maximum reconnect delay |

### Log Rotation Settings

```yaml
log_rotation:
  max_size_mb: 100
  max_backups: 3
  max_age_days: 7
  compress: true
```

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `max_size_mb` | int | 100 | Max log file size in MB before rotation |
| `max_backups` | int | 3 | Number of old log files to retain |
| `max_age_days` | int | 7 | Days to retain old log files |
| `compress` | bool | true | Compress rotated log files |

### Path Settings

```yaml
paths:
  state: .atari/state.json
  log: .atari/atari.log
  socket: .atari/atari.sock
  pid: .atari/atari.pid
```

All paths are relative to the project root unless absolute.

### Prompt Configuration

Inline prompt:
```yaml
prompt: |
  Run "br ready --json" to find available work.
  Work on the highest-priority ready issue.
  ...
```

Or reference a file:
```yaml
prompt_file: .atari/prompt.txt
```

Prompt template variables:
- `{{.BeadID}}` - Current bead ID
- `{{.BeadTitle}}` - Current bead title
- `{{.BeadDescription}}` - Current bead description
- `{{.Label}}` - Configured label filter

### Observer Settings

Configuration for the TUI observer mode. See [tui.md](../tui.md) for usage details.

```yaml
observer:
  enabled: true
  model: haiku
  recent_events: 20
  show_cost: true
  layout: horizontal
```

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `enabled` | bool | true | Enable observer mode in TUI |
| `model` | string | "haiku" | Claude model for observer queries |
| `recent_events` | int | 20 | Events for current bead context |
| `show_cost` | bool | true | Display observer session cost in TUI |
| `layout` | string | "horizontal" | Pane layout: "horizontal" or "vertical" |

### Graph Settings

Configuration for the TUI graph pane. See [tui.md](../tui.md) for usage details.

```yaml
graph:
  enabled: true
  density: standard
  refresh_on_event: false
  auto_refresh_interval: 5s
```

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `enabled` | bool | true | Enable graph pane in TUI |
| `density` | string | "standard" | Node density: minimal, compact, standard, verbose |
| `refresh_on_event` | bool | false | Auto-refresh graph on events |
| `auto_refresh_interval` | duration | 5s | Interval for auto-refresh |

### Follow-Up Settings

When a bead is left in_progress after a session ends (not closed or reset), atari can run a follow-up session to verify and close it.

```yaml
follow_up:
  enabled: true
  max_turns: 5
```

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `enabled` | bool | true | Enable follow-up sessions |
| `max_turns` | int | 5 | Max turns for follow-up session |

### Wrap-Up Settings

When atari pauses gracefully (via `p` key or signal), it can prompt the current session to save progress notes before terminating.

```yaml
wrap_up:
  enabled: true
  timeout: 60s
```

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `enabled` | bool | true | Enable wrap-up prompt on graceful pause |
| `timeout` | duration | 60s | Timeout waiting for wrap-up response |

### Logging Settings

```yaml
logging:
  level: info
  format: json
```

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `level` | string | "info" | Log level: debug, info, warn, error |
| `format` | string | "json" | Output format: json or text |

## Default Prompt

The default prompt references the user's Claude configuration including skills, agents, and MCPs.

```
Run "br ready --json" to find available work. Review your skills (issue-tracking, git-commit), MCPs (codex for verification), and agents (Explore, Plan). Implement the highest-priority ready issue completely, including all tests and linting. When you discover bugs or issues during implementation, create new br issues with exact context of what you were doing and what you found. Use the Explore and Plan subagents to investigate new issues before creating implementation tasks. Use /commit for atomic commits.
```

## Example Configurations

### Minimal

```yaml
# Just override what you need
claude:
  max_turns: 100
```

### Production with Label Filter

```yaml
workqueue:
  label: automated
  poll_interval: 10s
  selection_mode: top-level

backoff:
  max_failures: 3

logging:
  level: info
  format: json
```

### Development

```yaml
claude:
  max_turns: 50

backoff:
  initial: 30s
  max: 10m
  multiplier: 1.5

logging:
  level: debug
  format: text
```

### CI/CD Environment

```yaml
claude:
  extra_args:
    - "--model"
    - "sonnet"
  max_turns: 25

logging:
  level: info
  format: text
```
