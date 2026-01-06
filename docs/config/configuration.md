# Configuration

Configuration file format and settings for atari.

## Configuration Sources

Configuration is loaded from multiple sources in this order (later overrides earlier):

1. Built-in defaults
2. Global config file: `~/.config/atari/config.yaml`
3. Project config file: `.atari/config.yaml`
4. Environment variables
5. Command-line flags

## Config File Format

```yaml
# .atari/config.yaml

# Claude Code settings
# By default, atari relies on your global Claude config (~/.claude/settings.json)
# Only specify extra_args if you need to override global settings for atari sessions
claude:
  timeout: 60m                    # Session timeout (no activity)
  extra_args: []                 # Extra CLI args to pass to claude (override global config)

# Work queue settings
workqueue:
  poll_interval: 5s              # How often to check bd ready
  label: ""                      # Filter beads by label (optional)

# Backoff settings for failed beads
backoff:
  initial: 1m                    # Initial backoff duration
  max: 1h                        # Maximum backoff duration
  multiplier: 2.0                # Backoff multiplier
  max_failures: 5                # Abandon bead after N failures (0 = unlimited)

# BD activity integration
bd_activity:
  enabled: true                  # Enable bd activity stream
  reconnect_delay: 5s            # Delay before reconnecting

# Paths (relative to project root)
paths:
  state: .atari/state.json       # State file
  log: .atari/atari.log          # Log file
  socket: .atari/atari.sock      # Unix socket
  pid: .atari/atari.pid          # PID file

# Prompt template (inline or file path)
prompt: |
  Run "bd ready --json" to find available work...

# Or reference a file
# prompt_file: .atari/prompt.txt

# Notifications
notifications:
  # Rate limiting
  rate_limit:
    min_delay: 30s               # Minimum time between same event type
    batch_window: 5s             # Batch events within this window

  # IFTTT webhooks
  ifttt:
    enabled: false
    key: ""                      # Your IFTTT webhook key
    event_name: "atari_event"
    triggers:
      - iteration.end
      - error

  # Slack webhooks
  slack:
    enabled: false
    webhook_url: ""
    channel: "#atari"
    triggers:
      - iteration.end
      - error

  # Discord webhooks
  discord:
    enabled: false
    webhook_url: ""
    triggers:
      - iteration.end
      - error

  # Generic webhook
  webhook:
    enabled: false
    url: ""
    method: POST
    headers: {}
    triggers:
      - iteration.end

# TUI settings
tui:
  enabled: true                  # Enable TUI by default
  colors: true                   # Enable colors

# Observer settings
observer:
  enabled: true                  # Enable observer mode in TUI
  model: haiku                   # Model for observer queries
  recent_events: 20              # Events for current bead context
  show_cost: true                # Display observer session cost
  layout: horizontal             # Pane layout: "horizontal" or "vertical"

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
| `ATARI_NO_TUI` | `tui.enabled` | Disable TUI (set to "1") |
| `ATARI_DEBUG` | `logging.level` | Set to debug level |
| `ATARI_IFTTT_KEY` | `notifications.ifttt.key` | IFTTT webhook key |
| `ATARI_SLACK_WEBHOOK` | `notifications.slack.webhook_url` | Slack webhook URL |

Environment variable naming convention:
- Prefix: `ATARI_`
- Path separator: `_`
- Uppercase

Example:
```bash
export ATARI_LABEL=automated
export ATARI_NOTIFICATIONS_IFTTT_ENABLED=true
```

Note: Claude model and settings come from your global Claude config (`~/.claude/settings.json`). Use `extra_args` in config if you need to override for atari sessions.

## Configuration Sections

### Claude Settings

By default, atari uses your global Claude configuration from `~/.claude/settings.json`. This is the natural place for model selection, permissions, and other Claude settings.

```yaml
claude:
  timeout: 5m
  extra_args: []
```

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `timeout` | duration | 5m | Kill session if no activity |
| `extra_args` | []string | [] | Extra CLI args passed to claude |

To override global settings for atari sessions only:

```yaml
claude:
  timeout: 10m
  extra_args:
    - "--model"
    - "sonnet"
    - "--max-turns"
    - "100"
```

### Work Queue Settings

```yaml
workqueue:
  poll_interval: 5s
  label: "automated"       # Recommended: filter to explicitly-marked beads
  unassigned_only: false   # Only claim unassigned beads
  exclude_labels: []       # Labels to exclude from selection
```

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `poll_interval` | duration | 5s | How often to poll `bd ready` |
| `label` | string | "" | Filter beads by label (include only) |
| `unassigned_only` | bool | false | Only claim beads with no assignee |
| `exclude_labels` | []string | [] | Beads with any of these labels will be skipped |

**Important**: Setting `workqueue.label` is recommended for production use to prevent race conditions where new beads are picked up before being properly sequenced. See [workqueue.md](../components/workqueue.md#race-condition-prevention) for details.

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
bd create "Design decision needed" --assignee npratt

# Option 2: Add exclusion label
bd create "Needs human review" --labels human
```

#### Workflow with label filtering
1. Create beads via `/bd-plan` (no label)
2. Sequence them via `/bd-sequence`
3. Add the label: `bd update <id> --labels automated`
4. Atari picks them up in correct dependency order

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
- Failure 5: **abandoned** - triggers `bead.abandoned` notification

When a bead is abandoned, it won't be retried again. Configure notifications to alert on `bead.abandoned` events to catch these overnight.

### Notification Settings

```yaml
notifications:
  rate_limit:
    min_delay: 30s
    batch_window: 5s

  ifttt:
    enabled: true
    key: "abc123"
    event_name: "atari_notification"
    triggers:
      - iteration.end
      - error
```

See [notifications.md](../components/notifications.md) for detailed configuration.

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
  Run "bd ready --json" to find available work.
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
- `{{.Label}}` - Configured label filter

### Observer Settings

Configuration for the Observer Mode feature:

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

See [components/observer.md](../components/observer.md) for Observer Mode details.

## Default Prompt

The default prompt references the user's Claude configuration including skills, agents, and MCPs. See [EXISTING_IMPLEMENTATION.md](../EXISTING_IMPLEMENTATION.md) for the shell-based implementation this is based on.

```
Run "bd ready --json" to find available work. Review your skills (bd-issue-tracking, git-commit), MCPs (codex for verification), and agents (Explore, Plan). Implement the highest-priority ready issue completely, including all tests and linting. When you discover bugs or issues during implementation, create new bd issues with exact context of what you were doing and what you found. Use the Explore and Plan subagents to investigate new issues before creating implementation tasks. Use /commit for atomic commits.
```

## Implementation

### Config Loading

```go
type Config struct {
    Claude        ClaudeConfig       `yaml:"claude"`
    WorkQueue     WorkQueueConfig    `yaml:"workqueue"`
    Backoff       BackoffConfig      `yaml:"backoff"`
    BDActivity    BDActivityConfig   `yaml:"bd_activity"`
    Paths         PathsConfig        `yaml:"paths"`
    Prompt        string             `yaml:"prompt"`
    PromptFile    string             `yaml:"prompt_file"`
    Notifications NotificationConfig `yaml:"notifications"`
    TUI           TUIConfig          `yaml:"tui"`
    Observer      ObserverConfig     `yaml:"observer"`
    Logging       LoggingConfig      `yaml:"logging"`
}

type ClaudeConfig struct {
    Timeout   time.Duration `yaml:"timeout"`
    ExtraArgs []string      `yaml:"extra_args"`
}

type ObserverConfig struct {
    Enabled      bool   `yaml:"enabled"`
    Model        string `yaml:"model"`
    RecentEvents int    `yaml:"recent_events"`
    ShowCost     bool   `yaml:"show_cost"`
    Layout       string `yaml:"layout"`
}

func Default() *Config {
    return &Config{
        Claude: ClaudeConfig{
            Timeout:   5 * time.Minute,
            ExtraArgs: []string{},
        },
        Observer: ObserverConfig{
            Enabled:      true,
            Model:        "haiku",
            RecentEvents: 20,
            ShowCost:     true,
            Layout:       "horizontal",
        },
        WorkQueue: WorkQueueConfig{
            PollInterval: 5 * time.Second,
        },
        Backoff: BackoffConfig{
            Initial:     time.Minute,
            Max:         time.Hour,
            Multiplier:  2.0,
            MaxFailures: 5,
        },
        BDActivity: BDActivityConfig{
            Enabled:        true,
            ReconnectDelay: 5 * time.Second,
        },
        Paths: PathsConfig{
            State:  ".atari/state.json",
            Log:    ".atari/atari.log",
            Socket: ".atari/atari.sock",
            PID:    ".atari/atari.pid",
        },
        TUI: TUIConfig{
            Enabled: true,
            Colors:  true,
        },
        Logging: LoggingConfig{
            Level:  "info",
            Format: "json",
        },
    }
}

func (c *Config) LoadFile(path string) error {
    data, err := os.ReadFile(path)
    if os.IsNotExist(err) {
        return nil // No config file is OK
    }
    if err != nil {
        return err
    }
    return yaml.Unmarshal(data, c)
}

func (c *Config) LoadEnv() {
    if v := os.Getenv("ATARI_MODEL"); v != "" {
        c.Claude.Model = v
    }
    if v := os.Getenv("ATARI_MAX_TURNS"); v != "" {
        if n, err := strconv.Atoi(v); err == nil {
            c.Claude.MaxTurns = n
        }
    }
    // ... etc
}
```

### Config Validation

```go
func (c *Config) Validate() error {
    if c.Claude.MaxTurns < 1 {
        return fmt.Errorf("max_turns must be positive")
    }
    if c.Claude.Model == "" {
        return fmt.Errorf("model is required")
    }
    if c.Backoff.Multiplier < 1 {
        return fmt.Errorf("backoff multiplier must be >= 1")
    }
    // Validate notification URLs if enabled
    if c.Notifications.IFTTT.Enabled && c.Notifications.IFTTT.Key == "" {
        return fmt.Errorf("ifttt.key is required when ifttt is enabled")
    }
    return nil
}
```

## Config File Discovery

Atari looks for config files in this order:

1. `--config` flag value
2. `ATARI_CONFIG` environment variable
3. `.atari/config.yaml` in current directory
4. `~/.config/atari/config.yaml`

## Example Configurations

### Minimal

```yaml
# Just override what you need
claude:
  max_turns: 100
```

### With Notifications

```yaml
claude:
  model: opus
  max_turns: 50

notifications:
  slack:
    enabled: true
    webhook_url: "https://hooks.slack.com/services/..."
    channel: "#ci-notifications"
    triggers:
      - iteration.end
      - error
      - drain.stop
```

### CI/CD Environment

```yaml
claude:
  model: sonnet  # Faster, cheaper for CI
  max_turns: 25

tui:
  enabled: false

logging:
  level: info
  format: text

notifications:
  webhook:
    enabled: true
    url: "${CI_WEBHOOK_URL}"
    triggers:
      - drain.stop
```

### Development

```yaml
claude:
  model: opus
  max_turns: 100

backoff:
  initial: 30s
  max: 10m
  multiplier: 1.5

logging:
  level: debug
  format: text
```
