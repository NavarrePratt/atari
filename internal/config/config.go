// Package config provides configuration types and defaults for atari.
package config

import "time"

// Config holds all configuration for atari.
type Config struct {
	Claude      ClaudeConfig      `yaml:"claude" mapstructure:"claude"`
	WorkQueue   WorkQueueConfig   `yaml:"workqueue" mapstructure:"workqueue"`
	Backoff     BackoffConfig     `yaml:"backoff" mapstructure:"backoff"`
	Paths       PathsConfig       `yaml:"paths" mapstructure:"paths"`
	BDActivity  BDActivityConfig  `yaml:"bdactivity" mapstructure:"bdactivity"`
	LogRotation LogRotationConfig `yaml:"log_rotation" mapstructure:"log_rotation"`
	Observer    ObserverConfig    `yaml:"observer" mapstructure:"observer"`
	Graph       GraphConfig       `yaml:"graph" mapstructure:"graph"`
	Prompt      string            `yaml:"prompt" mapstructure:"prompt"`
	PromptFile  string            `yaml:"prompt_file" mapstructure:"prompt_file"` // Path to prompt template file (takes priority over Prompt)
	AgentID     string            `yaml:"agent_id" mapstructure:"agent_id"`       // Bead ID for agent state reporting (empty = disabled)
}

// ClaudeConfig holds Claude Code session settings.
type ClaudeConfig struct {
	Timeout   time.Duration `yaml:"timeout" mapstructure:"timeout"`
	MaxTurns  int           `yaml:"max_turns" mapstructure:"max_turns"`   // Max turns per session batch (0 = unlimited)
	ExtraArgs []string      `yaml:"extra_args" mapstructure:"extra_args"` // Additional CLI args (--max-turns handled separately)
}

// WorkQueueConfig holds work queue polling settings.
type WorkQueueConfig struct {
	PollInterval time.Duration `yaml:"poll_interval" mapstructure:"poll_interval"`
	Label        string        `yaml:"label" mapstructure:"label"`
}

// BackoffConfig holds exponential backoff settings for failed beads.
type BackoffConfig struct {
	Initial     time.Duration `yaml:"initial" mapstructure:"initial"`
	Max         time.Duration `yaml:"max" mapstructure:"max"`
	Multiplier  float64       `yaml:"multiplier" mapstructure:"multiplier"`
	MaxFailures int           `yaml:"max_failures" mapstructure:"max_failures"`
}

// PathsConfig holds file paths for state, logs, and socket.
type PathsConfig struct {
	State  string `yaml:"state" mapstructure:"state"`
	Log    string `yaml:"log" mapstructure:"log"`
	Socket string `yaml:"socket" mapstructure:"socket"`
	PID    string `yaml:"pid" mapstructure:"pid"`
}

// BDActivityConfig holds BD activity watcher settings.
type BDActivityConfig struct {
	Enabled           bool          `yaml:"enabled" mapstructure:"enabled"`
	ReconnectDelay    time.Duration `yaml:"reconnect_delay" mapstructure:"reconnect_delay"`
	MaxReconnectDelay time.Duration `yaml:"max_reconnect_delay" mapstructure:"max_reconnect_delay"`
}

// LogRotationConfig holds settings for log file rotation.
// Used for the TUI debug log (lumberjack-based automatic rotation).
type LogRotationConfig struct {
	MaxSizeMB  int  `yaml:"max_size_mb" mapstructure:"max_size_mb"`
	MaxBackups int  `yaml:"max_backups" mapstructure:"max_backups"`
	MaxAgeDays int  `yaml:"max_age_days" mapstructure:"max_age_days"`
	Compress   bool `yaml:"compress" mapstructure:"compress"`
}

// ObserverConfig holds settings for the TUI observer mode.
type ObserverConfig struct {
	Enabled      bool   `yaml:"enabled" mapstructure:"enabled"`             // Enable observer mode in TUI
	Model        string `yaml:"model" mapstructure:"model"`                 // Claude model for observer queries (default: haiku)
	RecentEvents int    `yaml:"recent_events" mapstructure:"recent_events"` // Events for current bead context
	ShowCost     bool   `yaml:"show_cost" mapstructure:"show_cost"`         // Display observer session cost
	Layout       string `yaml:"layout" mapstructure:"layout"`               // Pane layout: "horizontal" or "vertical"
}

// GraphConfig holds settings for the TUI graph pane.
type GraphConfig struct {
	Enabled             bool          `yaml:"enabled" mapstructure:"enabled"`                           // Enable graph pane in TUI
	Density             string        `yaml:"density" mapstructure:"density"`                           // Node density: "compact", "standard", or "detailed"
	RefreshOnEvent      bool          `yaml:"refresh_on_event" mapstructure:"refresh_on_event"`         // Auto-refresh graph on events
	AutoRefreshInterval time.Duration `yaml:"auto_refresh_interval" mapstructure:"auto_refresh_interval"` // Interval for auto-refresh (0 = disabled, min 1s)
}

// DefaultPrompt is the default prompt sent to Claude Code sessions.
const DefaultPrompt = `Run "bd ready --json" to find available work. Review your skills (bd-issue-tracking, git-commit), MCPs (codex for verification), and agents (Explore, Plan). Implement the highest-priority ready issue completely, including all tests and linting. CRITICAL: When you have fully completed the work and verified it passes lint and tests, you MUST close the bead with "bd close <bead-id> --reason <description>" before ending your session - this is required for the work to be tracked as complete. When you discover bugs or issues during implementation, create new bd issues with exact context of what you were doing and what you found - describe the problem for investigation, not as implementation instructions. Use the Explore and Plan subagents to investigate new issues before creating implementation tasks. Use /commit for atomic commits.`

// Default returns a Config with sensible defaults for Phase 1 MVP.
func Default() *Config {
	return &Config{
		Claude: ClaudeConfig{
			Timeout:   5 * time.Minute,
			MaxTurns:  0, // 0 = unlimited; set to 10 for faster graceful pause
			ExtraArgs: []string{},
		},
		WorkQueue: WorkQueueConfig{
			PollInterval: 5 * time.Second,
			Label:        "",
		},
		Backoff: BackoffConfig{
			Initial:     time.Minute,
			Max:         time.Hour,
			Multiplier:  2.0,
			MaxFailures: 5,
		},
		Paths: PathsConfig{
			State:  ".atari/state.json",
			Log:    ".atari/atari.log",
			Socket: ".atari/atari.sock",
			PID:    ".atari/atari.pid",
		},
		BDActivity: BDActivityConfig{
			Enabled:           true,
			ReconnectDelay:    5 * time.Second,
			MaxReconnectDelay: 5 * time.Minute,
		},
		LogRotation: LogRotationConfig{
			MaxSizeMB:  100,
			MaxBackups: 3,
			MaxAgeDays: 7,
			Compress:   true,
		},
		Observer: ObserverConfig{
			Enabled:      true,
			Model:        "haiku",
			RecentEvents: 20,
			ShowCost:     true,
			Layout:       "horizontal",
		},
		Graph: GraphConfig{
			Enabled:             true,
			Density:             "standard",
			RefreshOnEvent:      false,
			AutoRefreshInterval: 5 * time.Second,
		},
		Prompt: DefaultPrompt,
	}
}
