// Package session manages Claude Code process lifecycle.
package session

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/events"
)

// LimitedWriter caps output at maxSize bytes.
// Once the limit is reached, further writes are silently discarded.
type LimitedWriter struct {
	buf     []byte
	maxSize int
	mu      sync.Mutex
}

// NewLimitedWriter creates a LimitedWriter with the specified max size.
func NewLimitedWriter(maxSize int) *LimitedWriter {
	return &LimitedWriter{maxSize: maxSize}
}

// Write appends data to the buffer up to maxSize.
// Returns the input length to satisfy io.Writer (never returns error).
func (w *LimitedWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	remaining := w.maxSize - len(w.buf)
	if remaining <= 0 {
		return len(p), nil // Discard but report success
	}
	if len(p) > remaining {
		p = p[:remaining]
	}
	w.buf = append(w.buf, p...)
	return len(p), nil
}

// Bytes returns a copy of the captured data.
func (w *LimitedWriter) Bytes() []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	result := make([]byte, len(w.buf))
	copy(result, w.buf)
	return result
}

// String returns the captured data as a string.
func (w *LimitedWriter) String() string {
	return string(w.Bytes())
}

// Len returns the current buffer length.
func (w *LimitedWriter) Len() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.buf)
}

// DefaultStderrCap is the default maximum stderr capture size (64KB).
const DefaultStderrCap = 64 * 1024

// Manager manages a Claude Code session lifecycle.
type Manager struct {
	config         *config.Config
	events         *events.Router
	cmd            *exec.Cmd
	stdout         io.ReadCloser
	stdin          io.WriteCloser   // stdin pipe for prompt injection
	stderr         *LimitedWriter
	lastActive     atomic.Value // time.Time
	pauseRequested atomic.Bool  // graceful pause requested
	wrapUpSent     atomic.Bool  // wrap-up prompt has been sent
	done           chan struct{}
	mu             sync.Mutex
	started        bool
}

// New creates a Manager with the given config and event router.
func New(cfg *config.Config, router *events.Router) *Manager {
	m := &Manager{
		config: cfg,
		events: router,
		stderr: NewLimitedWriter(DefaultStderrCap),
		done:   make(chan struct{}),
	}
	m.lastActive.Store(time.Now())
	return m
}

// Start spawns the claude process with stream-json output format.
// The prompt is provided via stdin. Stdin remains open for prompt injection.
func (m *Manager) Start(ctx context.Context, prompt string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return fmt.Errorf("session already started")
	}

	args := []string{"-p", "--verbose", "--output-format", "stream-json"}
	if m.config.Claude.MaxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", m.config.Claude.MaxTurns))
	}
	args = append(args, m.config.Claude.ExtraArgs...)

	m.cmd = exec.CommandContext(ctx, "claude", args...)

	// Use pipe for stdin to allow prompt injection
	var err error
	m.stdin, err = m.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	m.stdout, err = m.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	m.cmd.Stderr = m.stderr

	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("start claude: %w", err)
	}

	// Write initial prompt to stdin
	if _, err := m.stdin.Write([]byte(prompt)); err != nil {
		_ = m.cmd.Process.Kill()
		return fmt.Errorf("write prompt: %w", err)
	}

	m.started = true

	// Start watchdog in background
	go m.watchdog(ctx)

	return nil
}

// watchdog monitors session activity and kills idle sessions.
func (m *Manager) watchdog(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// checkTimeout returns true if session should be terminated
	checkTimeout := func() bool {
		last := m.lastActive.Load().(time.Time)
		if time.Since(last) > m.config.Claude.Timeout {
			if m.events != nil {
				m.events.Emit(&events.SessionTimeoutEvent{
					BaseEvent: events.NewInternalEvent(events.EventSessionTimeout),
					Duration:  time.Since(last),
				})
			}
			m.Stop()
			return true
		}
		return false
	}

	// Check immediately on start
	if checkTimeout() {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.done:
			return
		case <-ticker.C:
			if checkTimeout() {
				return
			}
		}
	}
}

// UpdateActivity marks the session as active, resetting the timeout watchdog.
// This should be called whenever output is received from Claude.
func (m *Manager) UpdateActivity() {
	m.lastActive.Store(time.Now())
}

// Stdout returns the stdout reader for the claude process.
// Callers should read from this to process stream-json events.
func (m *Manager) Stdout() io.Reader {
	return m.stdout
}

// Stderr returns the captured stderr content.
func (m *Manager) Stderr() string {
	return m.stderr.String()
}

// Wait blocks until the claude process exits and returns its error.
// It closes the done channel to signal the watchdog to stop.
func (m *Manager) Wait() error {
	defer func() {
		select {
		case <-m.done:
			// Already closed
		default:
			close(m.done)
		}
	}()
	return m.cmd.Wait()
}

// Stop terminates the claude process.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd != nil && m.cmd.Process != nil {
		_ = m.cmd.Process.Kill()
	}
}

// Running returns true if the session has started and not yet completed.
func (m *Manager) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return false
	}

	select {
	case <-m.done:
		return false
	default:
		return true
	}
}

// RequestPause signals that the session should stop at the next turn boundary.
// This allows Claude to complete its current work before stopping.
func (m *Manager) RequestPause() {
	m.pauseRequested.Store(true)
}

// PauseRequested returns true if a graceful pause has been requested.
func (m *Manager) PauseRequested() bool {
	return m.pauseRequested.Load()
}

// SendWrapUp injects a wrap-up prompt and closes stdin to signal session end.
// This gives Claude a chance to save progress before the session terminates.
// Returns an error if stdin is not available or already closed.
func (m *Manager) SendWrapUp(prompt string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stdin == nil {
		return fmt.Errorf("stdin not available")
	}

	if m.wrapUpSent.Load() {
		return fmt.Errorf("wrap-up already sent")
	}

	// Write wrap-up prompt
	if _, err := m.stdin.Write([]byte("\n" + prompt + "\n")); err != nil {
		return fmt.Errorf("write wrap-up prompt: %w", err)
	}

	// Close stdin to signal EOF - Claude will finish processing and exit
	if err := m.stdin.Close(); err != nil {
		return fmt.Errorf("close stdin: %w", err)
	}

	m.wrapUpSent.Store(true)
	return nil
}

// WrapUpSent returns true if a wrap-up prompt has been sent.
func (m *Manager) WrapUpSent() bool {
	return m.wrapUpSent.Load()
}

// CloseStdin closes stdin without sending a wrap-up prompt.
// This is used for normal session completion.
func (m *Manager) CloseStdin() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stdin == nil {
		return nil
	}

	return m.stdin.Close()
}
