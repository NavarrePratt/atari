package observer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/runner"
)

const (
	// defaultQueryTimeout is the default timeout for observer queries.
	defaultQueryTimeout = 60 * time.Second

	// maxOutputBytes is the maximum output size before truncation (100KB).
	maxOutputBytes = 100 * 1024

	// outputTruncationMarker is appended to truncated output.
	outputTruncationMarker = "\n\n[Output truncated - exceeded 100KB limit]"
)

var (
	// ErrCancelled is returned when a query is cancelled by the user.
	ErrCancelled = errors.New("observer: query cancelled")

	// ErrQueryTimeout is returned when a query exceeds the timeout.
	ErrQueryTimeout = errors.New("observer: query timeout")

	// ErrNoContext is returned when context building fails.
	ErrNoContext = errors.New("observer: failed to build context")
)

// DrainStateProvider provides current drain state for context building.
type DrainStateProvider interface {
	GetDrainState() DrainState
}

// Exchange represents a single Q&A exchange in the observer session.
type Exchange struct {
	Question string
	Answer   string
}

// Observer handles interactive Q&A queries using Claude CLI.
type Observer struct {
	config        *config.ObserverConfig
	broker        *SessionBroker
	builder       *ContextBuilder
	stateProvider DrainStateProvider
	runnerFactory func() runner.ProcessRunner

	mu        sync.Mutex
	sessionID string // Claude session ID for --resume
	runner    runner.ProcessRunner
	cancel    context.CancelFunc
	history   []Exchange // conversation history for session continuity
}

// NewObserver creates a new Observer with the given configuration.
func NewObserver(
	cfg *config.ObserverConfig,
	broker *SessionBroker,
	builder *ContextBuilder,
	stateProvider DrainStateProvider,
) *Observer {
	return &Observer{
		config:        cfg,
		broker:        broker,
		builder:       builder,
		stateProvider: stateProvider,
		runnerFactory: func() runner.ProcessRunner {
			return runner.NewExecProcessRunner()
		},
	}
}

// Ask executes a query and returns the response.
// It builds context and runs claude CLI. Observer runs independently of
// drain sessions - they use different models and processes.
func (o *Observer) Ask(ctx context.Context, question string) (string, error) {
	// Build context with conversation history
	state := DrainState{}
	if o.stateProvider != nil {
		state = o.stateProvider.GetDrainState()
	}

	o.mu.Lock()
	history := make([]Exchange, len(o.history))
	copy(history, o.history)
	o.mu.Unlock()

	contextStr, err := o.builder.Build(state, history)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrNoContext, err)
	}

	// Build prompt with context and question
	prompt := fmt.Sprintf("%s\n\nQuestion: %s", contextStr, question)

	// Execute query with retry on resume failure
	response, err := o.executeQuery(ctx, prompt)
	if err != nil && o.sessionID != "" {
		// Retry once with fresh session
		o.sessionID = ""
		response, err = o.executeQuery(ctx, prompt)
	}

	// Record successful exchange in history
	if err == nil && response != "" {
		o.mu.Lock()
		o.history = append(o.history, Exchange{
			Question: question,
			Answer:   response,
		})
		o.mu.Unlock()
	}

	return response, err
}

// executeQuery runs the claude CLI and captures output.
func (o *Observer) executeQuery(ctx context.Context, prompt string) (string, error) {
	// Create cancellable context with timeout
	queryCtx, cancel := context.WithTimeout(ctx, defaultQueryTimeout)

	o.mu.Lock()
	o.cancel = cancel
	o.runner = o.runnerFactory()
	o.mu.Unlock()

	defer func() {
		o.mu.Lock()
		o.cancel = nil
		o.runner = nil
		o.mu.Unlock()
		cancel()
	}()

	// Build command arguments
	args := o.buildArgs(prompt)

	// Start the process
	stdout, stderr, err := o.runner.Start(queryCtx, "claude", args...)
	if err != nil {
		return "", fmt.Errorf("failed to start claude: %w", err)
	}

	// Read output with size limit
	output, readErr := o.readOutput(stdout, stderr)

	// Wait for process to complete
	waitErr := o.runner.Wait()

	// Handle context cancellation
	if queryCtx.Err() == context.Canceled {
		return "", ErrCancelled
	}
	if queryCtx.Err() == context.DeadlineExceeded {
		_ = o.runner.Kill()
		return output, ErrQueryTimeout
	}

	// Return output even with errors (may have partial output)
	if readErr != nil && output == "" {
		return "", fmt.Errorf("failed to read output: %w", readErr)
	}
	if waitErr != nil && output == "" {
		return "", fmt.Errorf("claude exited with error: %w", waitErr)
	}

	// Extract session ID from output for future --resume calls
	o.extractSessionID(output)

	return output, nil
}

// buildArgs constructs the claude CLI arguments.
func (o *Observer) buildArgs(prompt string) []string {
	args := []string{
		"-p", prompt,
		"--output-format", "text",
	}

	// Add --resume if we have a session ID
	o.mu.Lock()
	if o.sessionID != "" {
		args = append([]string{"--resume", o.sessionID}, args...)
	}
	o.mu.Unlock()

	// Add model if specified
	if o.config != nil && o.config.Model != "" {
		args = append(args, "--model", o.config.Model)
	}

	return args
}

// readOutput reads from stdout and stderr with a size limit.
func (o *Observer) readOutput(stdout, stderr io.ReadCloser) (string, error) {
	var output bytes.Buffer
	limitedWriter := &limitedWriter{
		w:     &output,
		limit: maxOutputBytes,
	}

	// Read stdout
	_, err := io.Copy(limitedWriter, stdout)
	if err != nil && err != errLimitReached {
		return output.String(), err
	}

	// If we hit the limit, add truncation marker
	if limitedWriter.truncated {
		output.WriteString(outputTruncationMarker)
	}

	// Also capture stderr if there's any (for error messages)
	var stderrBuf bytes.Buffer
	_, _ = io.Copy(&stderrBuf, stderr)
	if stderrBuf.Len() > 0 && output.Len() == 0 {
		return stderrBuf.String(), nil
	}

	return output.String(), nil
}

// extractSessionID attempts to extract the session ID from Claude output.
// Claude typically outputs session info that we can use for --resume.
// For now, we don't have a reliable way to extract it, so we leave this
// as a placeholder for future enhancement.
func (o *Observer) extractSessionID(_ string) {
	// TODO: Extract session ID from output when Claude provides it
	// For now, each query starts fresh
}

// Cancel terminates the current query if one is running.
func (o *Observer) Cancel() {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.cancel != nil {
		o.cancel()
	}
	if o.runner != nil {
		_ = o.runner.Kill()
	}
}

// Reset clears the session state and conversation history for a fresh start.
func (o *Observer) Reset() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.sessionID = ""
	o.history = nil
}

// SetRunnerFactory allows injection of a mock runner factory for testing.
func (o *Observer) SetRunnerFactory(factory func() runner.ProcessRunner) {
	o.runnerFactory = factory
}

// limitedWriter wraps a writer with a size limit.
type limitedWriter struct {
	w         io.Writer
	limit     int
	written   int
	truncated bool
}

var errLimitReached = errors.New("output limit reached")

func (lw *limitedWriter) Write(p []byte) (int, error) {
	if lw.truncated {
		return len(p), nil // Discard but pretend we wrote it
	}

	remaining := lw.limit - lw.written
	if remaining <= 0 {
		lw.truncated = true
		return len(p), nil
	}

	toWrite := p
	if len(p) > remaining {
		toWrite = p[:remaining]
		lw.truncated = true
	}

	n, err := lw.w.Write(toWrite)
	lw.written += n

	if lw.truncated {
		return len(p), nil // Report full length consumed
	}
	return n, err
}
