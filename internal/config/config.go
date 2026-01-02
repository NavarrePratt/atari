// Package config provides configuration types and defaults for atari.
package config

import "time"

// Config holds all configuration for atari.
type Config struct {
	Claude     ClaudeConfig
	WorkQueue  WorkQueueConfig
	Backoff    BackoffConfig
	Paths      PathsConfig
	BDActivity BDActivityConfig
	Prompt     string
	AgentID    string // Bead ID for agent state reporting (empty = disabled)
}

// ClaudeConfig holds Claude Code session settings.
type ClaudeConfig struct {
	Timeout   time.Duration
	ExtraArgs []string
}

// WorkQueueConfig holds work queue polling settings.
type WorkQueueConfig struct {
	PollInterval time.Duration
	Label        string
}

// BackoffConfig holds exponential backoff settings for failed beads.
type BackoffConfig struct {
	Initial     time.Duration
	Max         time.Duration
	Multiplier  float64
	MaxFailures int
}

// PathsConfig holds file paths for state, logs, and socket.
type PathsConfig struct {
	State  string
	Log    string
	Socket string
	PID    string
}

// BDActivityConfig holds BD activity watcher settings.
type BDActivityConfig struct {
	Enabled           bool
	ReconnectDelay    time.Duration
	MaxReconnectDelay time.Duration
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
		Prompt: DefaultPrompt,
	}
}
