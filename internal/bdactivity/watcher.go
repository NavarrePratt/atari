// Package bdactivity provides BD activity stream watching and event parsing.
package bdactivity

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/events"
	"github.com/npratt/atari/internal/runner"
)

const (
	// maxLineSize is the maximum line size for bd activity output (1MB).
	maxLineSize = 1024 * 1024

	// parseWarningInterval is the minimum time between parse warning events.
	parseWarningInterval = 5 * time.Second
)

// Watcher monitors bd activity --follow --json and emits events to the Router.
type Watcher struct {
	config *config.BDActivityConfig
	router *events.Router
	runner runner.ProcessRunner
	logger *slog.Logger

	running      atomic.Bool
	done         chan struct{}
	ctx          context.Context
	cancel       context.CancelFunc
	mu           sync.Mutex
	lastWarning  time.Time
	backoff      time.Duration
	successCount int
}

// New creates a new BD Activity Watcher.
func New(cfg *config.BDActivityConfig, router *events.Router, r runner.ProcessRunner, logger *slog.Logger) *Watcher {
	if logger == nil {
		logger = slog.Default()
	}
	return &Watcher{
		config:  cfg,
		router:  router,
		runner:  r,
		logger:  logger.With("component", "bdactivity"),
		backoff: cfg.ReconnectDelay,
	}
}

// Start begins watching bd activity in a background goroutine.
// Returns immediately. Use Stop() to terminate.
func (w *Watcher) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running.Load() {
		return fmt.Errorf("watcher already running")
	}

	w.ctx, w.cancel = context.WithCancel(ctx)
	w.done = make(chan struct{})
	w.running.Store(true)
	w.backoff = w.config.ReconnectDelay

	go w.runLoop()

	return nil
}

// Stop terminates the watcher gracefully.
func (w *Watcher) Stop() error {
	w.mu.Lock()
	if !w.running.Load() {
		w.mu.Unlock()
		return nil
	}
	w.mu.Unlock()

	// Signal shutdown
	w.cancel()

	// Kill the process
	if err := w.runner.Kill(); err != nil {
		w.logger.Warn("failed to kill bd activity process", "error", err)
	}

	// Wait for runLoop to exit
	<-w.done

	w.running.Store(false)
	return nil
}

// Running returns whether the watcher is currently active.
func (w *Watcher) Running() bool {
	return w.running.Load()
}

// runLoop is the main reconnection loop. It runs iteratively (not recursively)
// to prevent goroutine buildup.
func (w *Watcher) runLoop() {
	defer close(w.done)

	for {
		select {
		case <-w.ctx.Done():
			return
		default:
		}

		err := w.watch()

		// Check if we should stop
		select {
		case <-w.ctx.Done():
			return
		default:
		}

		if err != nil {
			w.emitWarning(fmt.Sprintf("bd activity exited: %v, reconnecting in %v", err, w.backoff))
		}

		// Backoff before reconnecting
		select {
		case <-w.ctx.Done():
			return
		case <-time.After(w.backoff):
		}

		// Increase backoff for next failure
		w.backoff = time.Duration(float64(w.backoff) * 2)
		if w.backoff > w.config.MaxReconnectDelay {
			w.backoff = w.config.MaxReconnectDelay
		}
	}
}

// watch starts the bd activity process and reads events.
// Returns when the process exits or context is cancelled.
func (w *Watcher) watch() error {
	stdout, stderr, err := w.runner.Start(w.ctx, "bd", "activity", "--follow", "--json")
	if err != nil {
		return fmt.Errorf("start bd activity: %w", err)
	}

	// Drain stderr in background to prevent blocking
	go w.drainStderr(stderr)

	// Read stdout line by line
	reader := bufio.NewReaderSize(stdout, maxLineSize)
	for {
		select {
		case <-w.ctx.Done():
			return w.ctx.Err()
		default:
		}

		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			// Check if context was cancelled
			select {
			case <-w.ctx.Done():
				return w.ctx.Err()
			default:
			}
			return fmt.Errorf("read error: %w", err)
		}

		// Remove trailing newline
		if len(line) > 0 && line[len(line)-1] == '\n' {
			line = line[:len(line)-1]
		}

		// Skip empty lines
		if len(line) == 0 {
			continue
		}

		// Parse and emit event
		event, parseErr := ParseLine(line)
		if parseErr != nil {
			w.emitParseWarning(parseErr, string(line))
			continue
		}

		if event != nil {
			w.router.Emit(event)
			w.resetBackoff()
		}
	}

	// Wait for process to exit
	return w.runner.Wait()
}

// drainStderr reads and discards stderr to prevent blocking.
func (w *Watcher) drainStderr(stderr io.ReadCloser) {
	defer func() { _ = stderr.Close() }()

	buf := make([]byte, 4096)
	for {
		n, err := stderr.Read(buf)
		if n > 0 {
			// Log stderr content at debug level
			w.logger.Debug("bd activity stderr", "content", string(buf[:n]))
		}
		if err != nil {
			return
		}
	}
}

// resetBackoff resets the backoff duration after a successful event.
func (w *Watcher) resetBackoff() {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.successCount++
	// Reset backoff after receiving events successfully
	if w.successCount >= 3 {
		w.backoff = w.config.ReconnectDelay
		w.successCount = 0
	}
}

// emitWarning emits a warning event to the router.
func (w *Watcher) emitWarning(msg string) {
	w.logger.Warn(msg)
	w.router.Emit(&events.ErrorEvent{
		BaseEvent: events.BaseEvent{
			EventType: events.EventError,
			Time:      time.Now(),
			Src:       events.SourceInternal,
		},
		Message:  msg,
		Severity: "warning",
	})
}

// emitParseWarning emits a rate-limited parse warning.
func (w *Watcher) emitParseWarning(err error, line string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	if now.Sub(w.lastWarning) < parseWarningInterval {
		return // Rate limited
	}
	w.lastWarning = now

	w.logger.Warn("bd activity parse error", "error", err, "line", line)
	w.router.Emit(&events.ErrorEvent{
		BaseEvent: events.BaseEvent{
			EventType: events.EventError,
			Time:      now,
			Src:       events.SourceInternal,
		},
		Message:  fmt.Sprintf("bd activity parse error: %v", err),
		Severity: "warning",
	})
}
