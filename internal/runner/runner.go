// Package runner provides abstractions for streaming process execution.
// It enables testability by allowing mock implementations to be substituted
// for real process execution.
package runner

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// ProcessRunner abstracts streaming subprocess execution.
// Unlike CommandRunner which returns output after completion,
// ProcessRunner provides streaming access to stdout/stderr.
type ProcessRunner interface {
	// Start spawns a process and returns readers for stdout and stderr.
	// The process runs until Wait is called or the context is cancelled.
	Start(ctx context.Context, name string, args ...string) (stdout, stderr io.ReadCloser, err error)

	// Wait blocks until the process exits and returns the exit error.
	// Must be called after Start to avoid resource leaks.
	Wait() error

	// Kill terminates the process immediately with SIGKILL.
	// Safe to call multiple times or if process already exited.
	Kill() error
}

// ExecProcessRunner implements ProcessRunner using os/exec.
// It is the production implementation for running real processes.
type ExecProcessRunner struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	started bool
}

// NewExecProcessRunner creates a new ExecProcessRunner.
func NewExecProcessRunner() *ExecProcessRunner {
	return &ExecProcessRunner{}
}

// Start spawns the named process with the given arguments.
// Returns readers for stdout and stderr that can be read concurrently.
func (r *ExecProcessRunner) Start(ctx context.Context, name string, args ...string) (io.ReadCloser, io.ReadCloser, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.started {
		return nil, nil, fmt.Errorf("process already started")
	}

	r.cmd = exec.CommandContext(ctx, name, args...)

	stdout, err := r.cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, err := r.cmd.StderrPipe()
	if err != nil {
		_ = stdout.Close()
		return nil, nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := r.cmd.Start(); err != nil {
		_ = stdout.Close()
		_ = stderr.Close()
		return nil, nil, fmt.Errorf("start process: %w", err)
	}

	r.started = true
	return stdout, stderr, nil
}

// Wait blocks until the process exits and returns the exit error.
func (r *ExecProcessRunner) Wait() error {
	r.mu.Lock()
	cmd := r.cmd
	r.mu.Unlock()

	if cmd == nil {
		return fmt.Errorf("process not started")
	}

	return cmd.Wait()
}

// Kill terminates the process immediately with SIGKILL.
func (r *ExecProcessRunner) Kill() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cmd == nil || r.cmd.Process == nil {
		return nil // Not started or already cleaned up
	}

	return r.cmd.Process.Kill()
}
