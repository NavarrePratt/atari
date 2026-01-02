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
	Prompt      string            `yaml:"prompt" mapstructure:"prompt"`
	AgentID     string            `yaml:"agent_id" mapstructure:"agent_id"` // Bead ID for agent state reporting (empty = disabled)
}

// ClaudeConfig holds Claude Code session settings.
type ClaudeConfig struct {
	Timeout   time.Duration `yaml:"timeout" mapstructure:"timeout"`
	ExtraArgs []string      `yaml:"extra_args" mapstructure:"extra_args"`
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

// DefaultPrompt is the default prompt sent to Claude Code sessions.
const DefaultPrompt = `Run "bd ready --json" to find available work. Review your skills (bd-issue-tracking, git-commit), MCPs (codex for verification), and agents (Explore, Plan). Implement the highest-priority ready issue completely, including all tests and linting. When you discover bugs or issues during implementation, create new bd issues with exact context of what you were doing and what you found - describe the problem for investigation, not as implementation instructions. Use the Explore and Plan subagents to investigate new issues before creating implementation tasks. Use /commit for atomic commits.`

// Default returns a Config with sensible defaults for Phase 1 MVP.
func Default() *Config {
	return &Config{
		Claude: ClaudeConfig{
			Timeout:   5 * time.Minute,
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
		Prompt: DefaultPrompt,
	}
}
