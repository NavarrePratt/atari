package config

import (
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg == nil {
		t.Fatal("Default() returned nil")
	}
}

func TestDefaultClaudeConfig(t *testing.T) {
	cfg := Default()

	if cfg.Claude.Timeout != 5*time.Minute {
		t.Errorf("Claude.Timeout = %v, want %v", cfg.Claude.Timeout, 5*time.Minute)
	}

	if cfg.Claude.ExtraArgs == nil {
		t.Error("Claude.ExtraArgs is nil, want empty slice")
	}

	if len(cfg.Claude.ExtraArgs) != 0 {
		t.Errorf("Claude.ExtraArgs has %d elements, want 0", len(cfg.Claude.ExtraArgs))
	}
}

func TestDefaultWorkQueueConfig(t *testing.T) {
	cfg := Default()

	if cfg.WorkQueue.PollInterval != 5*time.Second {
		t.Errorf("WorkQueue.PollInterval = %v, want %v", cfg.WorkQueue.PollInterval, 5*time.Second)
	}

	if cfg.WorkQueue.Label != "" {
		t.Errorf("WorkQueue.Label = %q, want empty string", cfg.WorkQueue.Label)
	}
}

func TestDefaultBackoffConfig(t *testing.T) {
	cfg := Default()

	if cfg.Backoff.Initial != time.Minute {
		t.Errorf("Backoff.Initial = %v, want %v", cfg.Backoff.Initial, time.Minute)
	}

	if cfg.Backoff.Max != time.Hour {
		t.Errorf("Backoff.Max = %v, want %v", cfg.Backoff.Max, time.Hour)
	}

	if cfg.Backoff.Multiplier != 2.0 {
		t.Errorf("Backoff.Multiplier = %v, want %v", cfg.Backoff.Multiplier, 2.0)
	}

	if cfg.Backoff.MaxFailures != 5 {
		t.Errorf("Backoff.MaxFailures = %d, want %d", cfg.Backoff.MaxFailures, 5)
	}
}

func TestDefaultPathsConfig(t *testing.T) {
	cfg := Default()

	paths := []struct {
		name string
		got  string
		want string
	}{
		{"State", cfg.Paths.State, ".atari/state.json"},
		{"Log", cfg.Paths.Log, ".atari/atari.log"},
		{"Socket", cfg.Paths.Socket, ".atari/atari.sock"},
		{"PID", cfg.Paths.PID, ".atari/atari.pid"},
	}

	for _, tc := range paths {
		if tc.got != tc.want {
			t.Errorf("Paths.%s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}

func TestDefaultPrompt(t *testing.T) {
	cfg := Default()

	if cfg.Prompt == "" {
		t.Error("Prompt is empty, want non-empty default prompt")
	}

	if cfg.Prompt != DefaultPrompt {
		t.Error("Prompt does not match DefaultPrompt constant")
	}
}

func TestDefaultBDActivityConfig(t *testing.T) {
	cfg := Default()

	if !cfg.BDActivity.Enabled {
		t.Error("BDActivity.Enabled = false, want true")
	}

	if cfg.BDActivity.ReconnectDelay != 5*time.Second {
		t.Errorf("BDActivity.ReconnectDelay = %v, want %v", cfg.BDActivity.ReconnectDelay, 5*time.Second)
	}

	if cfg.BDActivity.MaxReconnectDelay != 5*time.Minute {
		t.Errorf("BDActivity.MaxReconnectDelay = %v, want %v", cfg.BDActivity.MaxReconnectDelay, 5*time.Minute)
	}
}
