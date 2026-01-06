package observer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/runner"
)

// streamEvent represents a Claude stream-json event (simplified for Observer).
type streamEvent struct {
	Type    string `json:"type"`
	Result  string `json:"result,omitempty"`
	Message *struct {
		Content []json.RawMessage `json:"content"`
	} `json:"message,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

// contentBlock represents a content item within a message.
type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

const (
	// defaultQueryTimeout is the default timeout for observer queries.
	defaultQueryTimeout = 60 * time.Second

	// maxOutputBytes is the maximum output size before truncation (100KB).
	maxOutputBytes = 100 * 1024

	// outputTruncationMarker is appended to truncated output.
	outputTruncationMarker = "\n\n[Output truncated - exceeded 100KB limit]"

	// minSessionIDLength is the minimum valid session ID length.
	// Claude session IDs are UUIDs (36 chars) but we use a conservative minimum.
	minSessionIDLength = 8
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
	if err != nil {
		// Check if we have a session ID to clear (thread-safe)
		o.mu.Lock()
		hasSession := o.sessionID != ""
		if hasSession {
			o.sessionID = ""
		}
		o.mu.Unlock()

		// Retry once with fresh session
		if hasSession {
			response, err = o.executeQuery(ctx, prompt)
		}
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
		"--verbose",
		"--output-format", "stream-json",
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

// parseResult holds the parsed output from stream-json.
type parseResult struct {
	text      string
	sessionID string
}

// readOutput reads stream-json from stdout and parses it.
// It extracts session_id from result events and text from assistant events.
// The session_id is captured BEFORE truncation applies.
func (o *Observer) readOutput(stdout, stderr io.ReadCloser) (string, error) {
	result := o.parseStreamJSON(stdout)

	// Store session_id if extracted and valid (thread-safe)
	// Validate minimum length to prevent --resume with invalid values
	if len(result.sessionID) >= minSessionIDLength {
		o.mu.Lock()
		o.sessionID = result.sessionID
		o.mu.Unlock()
	}

	// Check stderr if no text was extracted
	if result.text == "" {
		var stderrBuf bytes.Buffer
		_, _ = io.Copy(&stderrBuf, stderr)
		if stderrBuf.Len() > 0 {
			return stderrBuf.String(), nil
		}
	}

	return result.text, nil
}

// parseStreamJSON parses Claude's stream-json output.
// Returns extracted text and session_id separately.
func (o *Observer) parseStreamJSON(r io.Reader) parseResult {
	var result parseResult
	var textBuilder bytes.Buffer
	truncated := false

	scanner := bufio.NewScanner(r)
	// Use larger buffer for potentially large JSON lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event streamEvent
		if err := json.Unmarshal(line, &event); err != nil {
			// Skip invalid JSON lines
			continue
		}

		switch event.Type {
		case "assistant":
			// Extract text from assistant messages
			if event.Message != nil {
				for _, rawContent := range event.Message.Content {
					var block contentBlock
					if err := json.Unmarshal(rawContent, &block); err != nil {
						continue
					}
					if block.Type == "text" && block.Text != "" {
						// Apply truncation limit to text only
						if !truncated {
							remaining := maxOutputBytes - textBuilder.Len()
							if remaining > 0 {
								if len(block.Text) > remaining {
									textBuilder.WriteString(block.Text[:remaining])
									truncated = true
								} else {
									textBuilder.WriteString(block.Text)
								}
							}
						}
					}
				}
			}

		case "result":
			// Extract session_id from result event (BEFORE truncation check)
			// Session ID is captured regardless of text truncation
			if event.SessionID != "" {
				result.sessionID = event.SessionID
			}
			// Use result text if no assistant text was collected
			if textBuilder.Len() == 0 && event.Result != "" {
				textBuilder.WriteString(event.Result)
			}
		}
	}

	// Add truncation marker if needed
	if truncated {
		textBuilder.WriteString(outputTruncationMarker)
	}

	result.text = textBuilder.String()
	return result
}

// extractSessionID is a no-op placeholder for backward compatibility.
// Session ID extraction is now handled directly in parseStreamJSON.
func (o *Observer) extractSessionID(_ string) {
	// Session ID extraction moved to parseStreamJSON/readOutput
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
// Note: This is still used by tests (TestLimitedWriter).
type limitedWriter struct {
	w         io.Writer
	limit     int
	written   int
	truncated bool
}

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
