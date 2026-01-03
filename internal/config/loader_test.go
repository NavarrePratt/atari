package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestLoadConfig_Defaults(t *testing.T) {
	v := viper.New()
	cfg, err := LoadConfig(v)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Check defaults are applied
	if cfg.Claude.Timeout != 5*time.Minute {
		t.Errorf("Claude.Timeout = %v, want %v", cfg.Claude.Timeout, 5*time.Minute)
	}
	if cfg.WorkQueue.PollInterval != 5*time.Second {
		t.Errorf("WorkQueue.PollInterval = %v, want %v", cfg.WorkQueue.PollInterval, 5*time.Second)
	}
	if cfg.Backoff.Initial != time.Minute {
		t.Errorf("Backoff.Initial = %v, want %v", cfg.Backoff.Initial, time.Minute)
	}
	if cfg.Backoff.Multiplier != 2.0 {
		t.Errorf("Backoff.Multiplier = %v, want %v", cfg.Backoff.Multiplier, 2.0)
	}
}

func TestLoadConfig_ProjectFile(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	// Create .atari directory and config file
	if err := os.MkdirAll(ProjectConfigDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	configContent := `
claude:
  timeout: 10m
workqueue:
  poll_interval: 30s
  label: "test-label"
backoff:
  initial: 2m
  max: 30m
  multiplier: 1.5
  max_failures: 3
`
	configPath := filepath.Join(ProjectConfigDir, ProjectConfigFile)
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	v := viper.New()
	cfg, err := LoadConfig(v)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Check values from file
	if cfg.Claude.Timeout != 10*time.Minute {
		t.Errorf("Claude.Timeout = %v, want %v", cfg.Claude.Timeout, 10*time.Minute)
	}
	if cfg.WorkQueue.PollInterval != 30*time.Second {
		t.Errorf("WorkQueue.PollInterval = %v, want %v", cfg.WorkQueue.PollInterval, 30*time.Second)
	}
	if cfg.WorkQueue.Label != "test-label" {
		t.Errorf("WorkQueue.Label = %q, want %q", cfg.WorkQueue.Label, "test-label")
	}
	if cfg.Backoff.Initial != 2*time.Minute {
		t.Errorf("Backoff.Initial = %v, want %v", cfg.Backoff.Initial, 2*time.Minute)
	}
	if cfg.Backoff.Max != 30*time.Minute {
		t.Errorf("Backoff.Max = %v, want %v", cfg.Backoff.Max, 30*time.Minute)
	}
	if cfg.Backoff.Multiplier != 1.5 {
		t.Errorf("Backoff.Multiplier = %v, want %v", cfg.Backoff.Multiplier, 1.5)
	}
	if cfg.Backoff.MaxFailures != 3 {
		t.Errorf("Backoff.MaxFailures = %v, want %v", cfg.Backoff.MaxFailures, 3)
	}
}

func TestLoadConfig_ExplicitFile(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `
claude:
  timeout: 15m
agent_id: "bd-test-agent"
`
	configPath := filepath.Join(tmpDir, "custom-config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	v := viper.New()
	v.Set("config", configPath)

	cfg, err := LoadConfig(v)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Claude.Timeout != 15*time.Minute {
		t.Errorf("Claude.Timeout = %v, want %v", cfg.Claude.Timeout, 15*time.Minute)
	}
	if cfg.AgentID != "bd-test-agent" {
		t.Errorf("AgentID = %q, want %q", cfg.AgentID, "bd-test-agent")
	}
}

func TestLoadConfig_ExplicitFileMissing(t *testing.T) {
	v := viper.New()
	v.Set("config", "/nonexistent/path/config.yaml")

	_, err := LoadConfig(v)
	if err == nil {
		t.Error("LoadConfig should fail for missing explicit config")
	}
}

func TestLoadConfig_EnvOverride(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	// Create project config with one value
	if err := os.MkdirAll(ProjectConfigDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	configContent := `
workqueue:
  label: "from-file"
`
	configPath := filepath.Join(ProjectConfigDir, ProjectConfigFile)
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	v := viper.New()
	v.SetEnvPrefix("ATARI")
	v.AutomaticEnv()

	// Simulate env var by setting directly in viper (env binding happens in CLI)
	v.Set("workqueue.label", "from-env")

	cfg, err := LoadConfig(v)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Env should override file
	if cfg.WorkQueue.Label != "from-env" {
		t.Errorf("WorkQueue.Label = %q, want %q", cfg.WorkQueue.Label, "from-env")
	}
}

func TestLoadConfig_DurationParsing(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		yaml     string
		wantDur  time.Duration
		field    string
	}{
		{
			name:    "seconds",
			yaml:    "claude:\n  timeout: 30s",
			wantDur: 30 * time.Second,
			field:   "claude.timeout",
		},
		{
			name:    "minutes",
			yaml:    "claude:\n  timeout: 5m",
			wantDur: 5 * time.Minute,
			field:   "claude.timeout",
		},
		{
			name:    "hours",
			yaml:    "backoff:\n  max: 2h",
			wantDur: 2 * time.Hour,
			field:   "backoff.max",
		},
		{
			name:    "combined",
			yaml:    "claude:\n  timeout: 1h30m",
			wantDur: 90 * time.Minute,
			field:   "claude.timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(tmpDir, tt.name+".yaml")
			if err := os.WriteFile(configPath, []byte(tt.yaml), 0644); err != nil {
				t.Fatalf("write config failed: %v", err)
			}

			v := viper.New()
			v.Set("config", configPath)

			cfg, err := LoadConfig(v)
			if err != nil {
				t.Fatalf("LoadConfig failed: %v", err)
			}

			var got time.Duration
			switch tt.field {
			case "claude.timeout":
				got = cfg.Claude.Timeout
			case "backoff.max":
				got = cfg.Backoff.Max
			}

			if got != tt.wantDur {
				t.Errorf("got %v, want %v", got, tt.wantDur)
			}
		})
	}
}

func TestLoadConfig_PartialOverride(t *testing.T) {
	tmpDir := t.TempDir()

	// Only override some fields
	configContent := `
claude:
  timeout: 20m
# Don't set extra_args - should keep default empty slice
`
	configPath := filepath.Join(tmpDir, "partial.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	v := viper.New()
	v.Set("config", configPath)

	cfg, err := LoadConfig(v)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Overridden value
	if cfg.Claude.Timeout != 20*time.Minute {
		t.Errorf("Claude.Timeout = %v, want %v", cfg.Claude.Timeout, 20*time.Minute)
	}

	// Default values should remain
	if cfg.Backoff.Initial != time.Minute {
		t.Errorf("Backoff.Initial = %v, want %v (default)", cfg.Backoff.Initial, time.Minute)
	}
	if cfg.Paths.State != ".atari/state.json" {
		t.Errorf("Paths.State = %q, want %q (default)", cfg.Paths.State, ".atari/state.json")
	}
}

func TestGlobalConfigPath(t *testing.T) {
	// Just test that it doesn't panic and returns empty for non-existent
	path := globalConfigPath()
	if path != "" {
		// If it returns a path, it should exist
		if _, err := os.Stat(path); err != nil {
			t.Errorf("globalConfigPath returned %q but file doesn't exist", path)
		}
	}
}

func TestProjectConfigPath(t *testing.T) {
	// Test with no config file
	path := projectConfigPath()
	// Should return empty since we're not in a directory with .atari/config.yaml
	// (unless we happen to be running tests from such a directory)
	if path != "" {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("projectConfigPath returned %q but file doesn't exist", path)
		}
	}
}

func TestLoadConfig_ObserverSettings(t *testing.T) {
	tmpDir := t.TempDir()

	configContent := `
observer:
  enabled: false
  model: sonnet
  recent_events: 50
  show_cost: false
  layout: vertical
`
	configPath := filepath.Join(tmpDir, "observer-config.yaml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	v := viper.New()
	v.Set("config", configPath)

	cfg, err := LoadConfig(v)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Check observer values from file
	if cfg.Observer.Enabled {
		t.Error("Observer.Enabled = true, want false")
	}
	if cfg.Observer.Model != "sonnet" {
		t.Errorf("Observer.Model = %q, want %q", cfg.Observer.Model, "sonnet")
	}
	if cfg.Observer.RecentEvents != 50 {
		t.Errorf("Observer.RecentEvents = %d, want %d", cfg.Observer.RecentEvents, 50)
	}
	if cfg.Observer.ShowCost {
		t.Error("Observer.ShowCost = true, want false")
	}
	if cfg.Observer.Layout != "vertical" {
		t.Errorf("Observer.Layout = %q, want %q", cfg.Observer.Layout, "vertical")
	}
}

func TestLoadConfig_ObserverDefaults(t *testing.T) {
	// When no observer config is provided, defaults should apply
	v := viper.New()
	cfg, err := LoadConfig(v)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Check defaults
	if !cfg.Observer.Enabled {
		t.Error("Observer.Enabled = false, want true (default)")
	}
	if cfg.Observer.Model != "haiku" {
		t.Errorf("Observer.Model = %q, want %q (default)", cfg.Observer.Model, "haiku")
	}
	if cfg.Observer.RecentEvents != 20 {
		t.Errorf("Observer.RecentEvents = %d, want %d (default)", cfg.Observer.RecentEvents, 20)
	}
	if !cfg.Observer.ShowCost {
		t.Error("Observer.ShowCost = false, want true (default)")
	}
	if cfg.Observer.Layout != "horizontal" {
		t.Errorf("Observer.Layout = %q, want %q (default)", cfg.Observer.Layout, "horizontal")
	}
}
