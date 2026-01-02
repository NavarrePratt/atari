// Package testutil provides test infrastructure for unit and integration testing.
package testutil

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
)

// Errors returned by MockProcessRunner.
var (
	ErrProcessAlreadyStarted = errors.New("process already started")
	ErrProcessNotStarted     = errors.New("process not started")
	ErrProcessKilled         = errors.New("process killed")
)

// MockProcess records a single Start call and its configuration.
type MockProcess struct {
	Name string
	Args []string
}

// StartCallback is called on each Start invocation.
// It can be used to simulate different behaviors on each attempt.
// Return the output, stderr, and error to use for this Start call.
type StartCallback func(attempt int, name string, args []string) (stdout string, stderr string, startErr error)

// MockProcessRunner implements runner.ProcessRunner for testing.
// It provides canned responses and records calls for assertion.
type MockProcessRunner struct {
	mu sync.Mutex

	// Configuration
	stdout     string
	stderr     string
	startErr   error
	waitErr    error
	onStart    StartCallback

	// State tracking
	started     bool
	killed      bool
	waitCalled  bool
	startCount  int
	processes   []MockProcess
	stdoutPipe  *mockPipe
	stderrPipe  *mockPipe
}

// NewMockProcessRunner creates a new mock for testing.
func NewMockProcessRunner() *MockProcessRunner {
	return &MockProcessRunner{
		processes: make([]MockProcess, 0),
	}
}

// SetOutput configures the stdout content to return.
func (m *MockProcessRunner) SetOutput(content string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stdout = content
}

// SetStderr configures the stderr content to return.
func (m *MockProcessRunner) SetStderr(content string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stderr = content
}

// SetStartError configures an error to return from Start.
func (m *MockProcessRunner) SetStartError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startErr = err
}

// SetWaitError configures an error to return from Wait.
func (m *MockProcessRunner) SetWaitError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.waitErr = err
}

// OnStart sets a callback for dynamic start behavior.
// The callback is invoked on each Start call and can return
// different responses based on the attempt number.
func (m *MockProcessRunner) OnStart(fn StartCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onStart = fn
}

// Start implements runner.ProcessRunner.Start.
func (m *MockProcessRunner) Start(ctx context.Context, name string, args ...string) (io.ReadCloser, io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return nil, nil, ErrProcessAlreadyStarted
	}

	m.startCount++
	m.processes = append(m.processes, MockProcess{Name: name, Args: args})

	// Use callback if set
	stdout, stderr, startErr := m.stdout, m.stderr, m.startErr
	if m.onStart != nil {
		stdout, stderr, startErr = m.onStart(m.startCount, name, args)
	}

	if startErr != nil {
		return nil, nil, startErr
	}

	m.started = true
	m.killed = false
	m.waitCalled = false

	m.stdoutPipe = newMockPipe(stdout)
	m.stderrPipe = newMockPipe(stderr)

	return m.stdoutPipe, m.stderrPipe, nil
}

// Wait implements runner.ProcessRunner.Wait.
func (m *MockProcessRunner) Wait() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return ErrProcessNotStarted
	}

	m.waitCalled = true

	if m.killed {
		return ErrProcessKilled
	}

	return m.waitErr
}

// Kill implements runner.ProcessRunner.Kill.
func (m *MockProcessRunner) Kill() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return nil // Safe to call if not started
	}

	m.killed = true

	// Close pipes to unblock readers
	if m.stdoutPipe != nil {
		_ = m.stdoutPipe.Close()
	}
	if m.stderrPipe != nil {
		_ = m.stderrPipe.Close()
	}

	return nil
}

// Reset clears state for reuse in multi-attempt tests.
// Call this between simulated process restarts.
func (m *MockProcessRunner) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.started = false
	m.killed = false
	m.waitCalled = false
	m.stdoutPipe = nil
	m.stderrPipe = nil
}

// StartCount returns the number of times Start was called.
func (m *MockProcessRunner) StartCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.startCount
}

// Processes returns a copy of all recorded process starts.
func (m *MockProcessRunner) Processes() []MockProcess {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]MockProcess, len(m.processes))
	copy(result, m.processes)
	return result
}

// Started returns whether a process is currently started.
func (m *MockProcessRunner) Started() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.started
}

// Killed returns whether the process was killed.
func (m *MockProcessRunner) Killed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.killed
}

// WaitCalled returns whether Wait was called.
func (m *MockProcessRunner) WaitCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.waitCalled
}

// mockPipe provides a simple io.ReadCloser for mock output.
type mockPipe struct {
	reader io.Reader
	closed bool
	mu     sync.Mutex
}

func newMockPipe(content string) *mockPipe {
	return &mockPipe{
		reader: strings.NewReader(content),
	}
}

func (p *mockPipe) Read(buf []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return 0, io.EOF
	}

	return p.reader.Read(buf)
}

func (p *mockPipe) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	return nil
}
