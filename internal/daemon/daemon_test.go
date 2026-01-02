package daemon

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/npratt/atari/internal/config"
)

func TestNew(t *testing.T) {
	cfg := config.Default()
	cfg.Paths.Socket = "/tmp/test.sock"

	d := New(cfg, nil, nil)

	if d == nil {
		t.Fatal("New() returned nil")
	}
	if d.config != cfg {
		t.Error("config not set")
	}
	if d.sockPath != cfg.Paths.Socket {
		t.Errorf("expected sockPath %s, got %s", cfg.Paths.Socket, d.sockPath)
	}
	if d.logger == nil {
		t.Error("logger should default to slog.Default()")
	}
}

func TestNew_WithLogger(t *testing.T) {
	cfg := config.Default()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	d := New(cfg, nil, logger)

	if d.logger != logger {
		t.Error("logger not set correctly")
	}
}

func TestDaemon_Running_InitialState(t *testing.T) {
	cfg := config.Default()
	d := New(cfg, nil, nil)

	if d.Running() {
		t.Error("daemon should not be running initially")
	}
}

func TestDaemon_SetRunning(t *testing.T) {
	cfg := config.Default()
	d := New(cfg, nil, nil)

	d.setRunning(true)
	if !d.Running() {
		t.Error("expected Running() to return true after setRunning(true)")
	}

	d.setRunning(false)
	if d.Running() {
		t.Error("expected Running() to return false after setRunning(false)")
	}
}

func TestDaemon_SocketPath(t *testing.T) {
	cfg := config.Default()
	cfg.Paths.Socket = "/custom/path/atari.sock"

	d := New(cfg, nil, nil)

	if d.SocketPath() != "/custom/path/atari.sock" {
		t.Errorf("expected socket path /custom/path/atari.sock, got %s", d.SocketPath())
	}
}

func TestDaemon_Controller(t *testing.T) {
	cfg := config.Default()
	// Controller() returns nil when none is set
	d := New(cfg, nil, nil)

	if d.Controller() != nil {
		t.Error("Controller() should return nil when no controller is set")
	}
}

func TestDaemon_StartTime_ZeroInitially(t *testing.T) {
	cfg := config.Default()
	d := New(cfg, nil, nil)

	if !d.StartTime().IsZero() {
		t.Error("StartTime() should be zero initially")
	}
}

func TestDaemon_ThreadSafety(t *testing.T) {
	cfg := config.Default()
	d := New(cfg, nil, nil)

	// Run concurrent reads and writes
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				d.setRunning(true)
				_ = d.Running()
				d.setRunning(false)
			}
			done <- struct{}{}
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestDaemon_DefaultPaths(t *testing.T) {
	cfg := config.Default()

	// Verify default paths from config
	if cfg.Paths.Socket != ".atari/atari.sock" {
		t.Errorf("expected default socket path .atari/atari.sock, got %s", cfg.Paths.Socket)
	}
	if cfg.Paths.PID != ".atari/atari.pid" {
		t.Errorf("expected default PID path .atari/atari.pid, got %s", cfg.Paths.PID)
	}
}

func TestDaemon_ConfigIntegration(t *testing.T) {
	tmp := t.TempDir()
	cfg := config.Default()
	cfg.Paths.Socket = filepath.Join(tmp, "test.sock")
	cfg.Paths.PID = filepath.Join(tmp, "test.pid")

	d := New(cfg, nil, nil)

	if d.SocketPath() != cfg.Paths.Socket {
		t.Errorf("expected socket path %s, got %s", cfg.Paths.Socket, d.SocketPath())
	}
}
