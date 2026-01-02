// Package daemon provides background execution with external control via Unix socket RPC.
package daemon

import (
	"log/slog"
	"sync"
	"time"

	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/controller"
)

// Daemon manages background execution with external control via Unix socket.
type Daemon struct {
	config     *config.Config
	controller *controller.Controller
	sockPath   string
	startTime  time.Time
	logger     *slog.Logger

	running bool
	mu      sync.RWMutex
}

// New creates a new Daemon with the given configuration and controller.
func New(cfg *config.Config, ctrl *controller.Controller, logger *slog.Logger) *Daemon {
	if logger == nil {
		logger = slog.Default()
	}
	return &Daemon{
		config:     cfg,
		controller: ctrl,
		sockPath:   cfg.Paths.Socket,
		logger:     logger,
	}
}

// Running returns whether the daemon is currently running.
func (d *Daemon) Running() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.running
}

// setRunning updates the running state (thread-safe).
func (d *Daemon) setRunning(running bool) {
	d.mu.Lock()
	d.running = running
	d.mu.Unlock()
}

// Controller returns the underlying controller for testing.
func (d *Daemon) Controller() *controller.Controller {
	return d.controller
}

// StartTime returns when the daemon was started.
func (d *Daemon) StartTime() time.Time {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.startTime
}

// SocketPath returns the Unix socket path.
func (d *Daemon) SocketPath() string {
	return d.sockPath
}
