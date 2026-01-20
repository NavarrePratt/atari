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
	FollowUp    FollowUpConfig    `yaml:"follow_up" mapstructure:"follow_up"`
	WrapUp      WrapUpConfig      `yaml:"wrap_up" mapstructure:"wrap_up"`
	Prompt     string `yaml:"prompt" mapstructure:"prompt"`
	PromptFile string `yaml:"prompt_file" mapstructure:"prompt_file"` // Path to prompt template file (takes priority over Prompt)
}

// ClaudeConfig holds Claude Code session settings.
type ClaudeConfig struct {
	Timeout   time.Duration `yaml:"timeout" mapstructure:"timeout"`
	MaxTurns  int           `yaml:"max_turns" mapstructure:"max_turns"`   // Max turns per session batch (0 = unlimited)
	ExtraArgs []string      `yaml:"extra_args" mapstructure:"extra_args"` // Additional CLI args (--max-turns handled separately)
}

// WorkQueueConfig holds work queue polling settings.
type WorkQueueConfig struct {
	PollInterval   time.Duration `yaml:"poll_interval" mapstructure:"poll_interval"`
	Label          string        `yaml:"label" mapstructure:"label"`
	Epic           string        `yaml:"epic" mapstructure:"epic"`                       // Restrict work to beads under this epic
	UnassignedOnly bool          `yaml:"unassigned_only" mapstructure:"unassigned_only"` // Only claim unassigned beads
	ExcludeLabels  []string      `yaml:"exclude_labels" mapstructure:"exclude_labels"`   // Labels to exclude from selection
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
	Enabled             bool          `yaml:"enabled" mapstructure:"enabled"`                             // Enable graph pane in TUI
	Density             string        `yaml:"density" mapstructure:"density"`                             // Node density: "compact", "standard", or "detailed"
	RefreshOnEvent      bool          `yaml:"refresh_on_event" mapstructure:"refresh_on_event"`           // Auto-refresh graph on events
	AutoRefreshInterval time.Duration `yaml:"auto_refresh_interval" mapstructure:"auto_refresh_interval"` // Interval for auto-refresh (0 = disabled, min 1s)
}

// FollowUpConfig holds settings for follow-up sessions when beads are not closed.
type FollowUpConfig struct {
	Enabled  bool `yaml:"enabled" mapstructure:"enabled"`     // Enable follow-up sessions (default: true)
	MaxTurns int  `yaml:"max_turns" mapstructure:"max_turns"` // Max turns for follow-up session (default: 5)
}

// WrapUpConfig holds settings for wrap-up prompts on graceful pause.
type WrapUpConfig struct {
	Enabled bool          `yaml:"enabled" mapstructure:"enabled"` // Enable wrap-up prompt on graceful pause (default: true)
	Timeout time.Duration `yaml:"timeout" mapstructure:"timeout"` // Timeout for wrap-up response (default: 60s)
}

// DefaultWrapUpPrompt is the prompt sent before graceful pause to save progress notes.
const DefaultWrapUpPrompt = `IMPORTANT: Atari is pausing this session. You must save your progress NOW.

Run this command immediately to record your current progress:
br update {{.BeadID}} --notes "WRAP-UP: <summarize what you completed and what remains to be done>"

Include in your notes:
- What tasks you completed
- What you were working on when paused
- Any blockers or issues discovered
- Suggested next steps

After running br update, your session will end.`

// DefaultFollowUpPrompt is the prompt sent to follow-up sessions to verify and close beads.
const DefaultFollowUpPrompt = `The previous session worked on bead {{.BeadID}} ("{{.BeadTitle}}") but did not close it.

## Your Task
Verify the work and either close or reset the bead.

## Steps

### 1. Check Current State
Run "br show {{.BeadID}} --json" to see the description and what work was done.
Review git status and recent commits to understand what changed.

### 2. Run Verification
Execute the verification commands listed in the bead's Verification section.

### 3. Complete or Reset
- If verification passes: "br close {{.BeadID}} --reason 'Work completed and verified: <brief description>'"
- If verification fails: "br update {{.BeadID}} --status open --notes 'Needs more work: <describe failures>'"

Either close the bead or reset it to open status. Do not leave it in_progress - this creates ambiguity about whether work is ongoing or abandoned.`

// DefaultPrompt is the default prompt sent to Claude Code sessions.
const DefaultPrompt = `You are an autonomous task-completion agent. Follow this workflow:

## 1. Your Assignment
You have been assigned bead {{.BeadID}}: "{{.BeadTitle}}"

### Description
{{.BeadDescription}}

### Claim
Run "br update {{.BeadID}} --status in_progress" to claim this bead before starting work.
This prevents duplicate work if other agents are running.

## 2. Execute the Task
- Read the task description above carefully
- Use available tools and agents for investigation and implementation
- Follow project documentation and existing patterns

## 3. Verify the Work
Run the verification commands listed in the bead's Verification section.
If no verification section exists, check CLAUDE.md for lint/test commands.
All checks must pass before closing. If verification fails, fix the issues before proceeding.

## 4. Track Discoveries
If you find bugs, TODOs, or related work during implementation:
- Create new issue with /issue-create or "br create"
- Link to current work: "br dep add <new-id> {{.BeadID}} --type discovered-from"
- Describe problems for investigation, not implementation instructions
This maintains context and traceability for future work.

## 5. Complete the Task
Close the bead before ending your session:
- Run "br close {{.BeadID}} --reason '<what was accomplished>'"
- Use /commit for atomic commits
Closing the bead marks the work as done and releases it from in_progress state.`

// Default returns a Config with sensible defaults for Phase 1 MVP.
func Default() *Config {
	return &Config{
		Claude: ClaudeConfig{
			Timeout:   60 * time.Minute,
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
		FollowUp: FollowUpConfig{
			Enabled:  true,
			MaxTurns: 5,
		},
		WrapUp: WrapUpConfig{
			Enabled: true,
			Timeout: 60 * time.Second,
		},
		Prompt: DefaultPrompt,
	}
}
