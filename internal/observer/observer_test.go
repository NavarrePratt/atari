package observer

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/runner"
)

// mockRunner implements runner.ProcessRunner for testing.
type mockRunner struct {
	startFn func(ctx context.Context, name string, args ...string) (io.ReadCloser, io.ReadCloser, error)
	waitFn  func() error
	killFn  func() error
}

func (m *mockRunner) Start(ctx context.Context, name string, args ...string) (io.ReadCloser, io.ReadCloser, error) {
	if m.startFn != nil {
		return m.startFn(ctx, name, args...)
	}
	return io.NopCloser(strings.NewReader("")), io.NopCloser(strings.NewReader("")), nil
}

func (m *mockRunner) Wait() error {
	if m.waitFn != nil {
		return m.waitFn()
	}
	return nil
}

func (m *mockRunner) Kill() error {
	if m.killFn != nil {
		return m.killFn()
	}
	return nil
}

// mockStateProvider implements DrainStateProvider for testing.
type mockStateProvider struct {
	state DrainState
}

func (m *mockStateProvider) GetDrainState() DrainState {
	return m.state
}

func TestNewObserver(t *testing.T) {
	cfg := &config.ObserverConfig{
		Enabled: true,
		Model:   "haiku",
	}
	broker := NewSessionBroker()

	// Create a minimal log reader and context builder
	logReader := NewLogReader("/tmp/test.log")
	builder := NewContextBuilder(logReader, cfg)

	obs := NewObserver(cfg, broker, builder, nil)

	if obs == nil {
		t.Fatal("expected non-nil observer")
	}
	if obs.config != cfg {
		t.Error("expected config to be set")
	}
	if obs.broker != broker {
		t.Error("expected broker to be set")
	}
	if obs.builder != builder {
		t.Error("expected builder to be set")
	}
}

// makeStreamJSON creates a stream-json response with the given text and session ID.
func makeStreamJSON(text, sessionID string) string {
	lines := []string{
		`{"type":"system","subtype":"init","model":"haiku","tools":[]}`,
	}
	if text != "" {
		lines = append(lines, `{"type":"assistant","message":{"content":[{"type":"text","text":"`+text+`"}]}}`)
	}
	lines = append(lines, `{"type":"result","result":"`+text+`","session_id":"`+sessionID+`","num_turns":1}`)
	return strings.Join(lines, "\n") + "\n"
}

func TestObserver_Ask_Success(t *testing.T) {
	cfg := &config.ObserverConfig{Model: "haiku"}
	broker := NewSessionBroker()
	logReader := NewLogReader("/tmp/test.log")
	builder := NewContextBuilder(logReader, cfg)

	obs := NewObserver(cfg, broker, builder, nil)

	// Mock runner that returns stream-json output
	expectedOutput := "This is the answer to your question."
	streamJSON := makeStreamJSON(expectedOutput, "test-session-123")
	obs.SetRunnerFactory(func() runner.ProcessRunner {
		return &mockRunner{
			startFn: func(ctx context.Context, name string, args ...string) (io.ReadCloser, io.ReadCloser, error) {
				// Verify claude was called
				if name != "claude" {
					t.Errorf("expected 'claude', got %q", name)
				}

				// Verify -p flag is present
				hasPrompt := false
				for i, arg := range args {
					if arg == "-p" && i+1 < len(args) {
						hasPrompt = true
						break
					}
				}
				if !hasPrompt {
					t.Error("expected -p flag in args")
				}

				return io.NopCloser(strings.NewReader(streamJSON)), io.NopCloser(strings.NewReader("")), nil
			},
		}
	})

	result, err := obs.Ask(context.Background(), "What is happening?")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != expectedOutput {
		t.Errorf("expected %q, got %q", expectedOutput, result)
	}
}

func TestObserver_Ask_ReturnsBusyWhenDrainHoldsBroker(t *testing.T) {
	cfg := &config.ObserverConfig{Model: "haiku"}
	broker := NewSessionBroker()
	logReader := NewLogReader("/tmp/test.log")
	builder := NewContextBuilder(logReader, cfg)

	// Acquire the broker to simulate drain holding it
	err := broker.Acquire(context.Background(), "drain", time.Second)
	if err != nil {
		t.Fatalf("failed to acquire broker: %v", err)
	}
	defer broker.Release()

	obs := NewObserver(cfg, broker, builder, nil)

	// Mock runner should never be called since broker acquisition fails
	runnerCalled := false
	obs.SetRunnerFactory(func() runner.ProcessRunner {
		runnerCalled = true
		return &mockRunner{}
	})

	// Observer should return ErrBusy when broker is held by drain
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = obs.Ask(ctx, "What is happening?")

	if !errors.Is(err, ErrBusy) {
		t.Errorf("expected ErrBusy, got %v", err)
	}
	if runnerCalled {
		t.Error("runner should not be called when broker acquisition fails")
	}
}

func TestObserver_Ask_AcquiresAndReleasesBroker(t *testing.T) {
	cfg := &config.ObserverConfig{Model: "haiku"}
	broker := NewSessionBroker()
	logReader := NewLogReader("/tmp/test.log")
	builder := NewContextBuilder(logReader, cfg)

	obs := NewObserver(cfg, broker, builder, nil)

	// Mock runner that returns stream-json output
	streamJSON := makeStreamJSON("response", "test-session-789")
	obs.SetRunnerFactory(func() runner.ProcessRunner {
		return &mockRunner{
			startFn: func(ctx context.Context, name string, args ...string) (io.ReadCloser, io.ReadCloser, error) {
				// Verify broker is held during query execution
				if broker.Holder() != "observer" {
					t.Errorf("expected broker holder to be 'observer', got %q", broker.Holder())
				}
				return io.NopCloser(strings.NewReader(streamJSON)), io.NopCloser(strings.NewReader("")), nil
			},
		}
	})

	// Verify broker is not held before query
	if broker.IsHeld() {
		t.Error("broker should not be held before Ask")
	}

	_, err := obs.Ask(context.Background(), "What is happening?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify broker is released after query
	if broker.IsHeld() {
		t.Error("broker should be released after Ask")
	}
}

func TestObserver_Ask_OutputTruncation(t *testing.T) {
	cfg := &config.ObserverConfig{Model: "haiku"}
	broker := NewSessionBroker()
	logReader := NewLogReader("/tmp/test.log")
	builder := NewContextBuilder(logReader, cfg)

	obs := NewObserver(cfg, broker, builder, nil)

	// Create stream-json with text larger than maxOutputBytes
	largeText := strings.Repeat("a", maxOutputBytes+1000)
	streamJSON := makeStreamJSON(largeText, "test-session-truncate")

	obs.SetRunnerFactory(func() runner.ProcessRunner {
		return &mockRunner{
			startFn: func(ctx context.Context, name string, args ...string) (io.ReadCloser, io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader(streamJSON)), io.NopCloser(strings.NewReader("")), nil
			},
		}
	})

	result, err := obs.Ask(context.Background(), "What is happening?")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, outputTruncationMarker) {
		t.Error("expected truncation marker in output")
	}
	if len(result) > maxOutputBytes+len(outputTruncationMarker)+100 {
		t.Errorf("output too large: %d bytes", len(result))
	}
}

func TestObserver_Cancel(t *testing.T) {
	cfg := &config.ObserverConfig{Model: "haiku"}
	broker := NewSessionBroker()
	logReader := NewLogReader("/tmp/test.log")
	builder := NewContextBuilder(logReader, cfg)

	obs := NewObserver(cfg, broker, builder, nil)

	killCalled := false
	obs.SetRunnerFactory(func() runner.ProcessRunner {
		return &mockRunner{
			startFn: func(ctx context.Context, name string, args ...string) (io.ReadCloser, io.ReadCloser, error) {
				// Block until context is cancelled
				<-ctx.Done()
				return io.NopCloser(strings.NewReader("")), io.NopCloser(strings.NewReader("")), ctx.Err()
			},
			killFn: func() error {
				killCalled = true
				return nil
			},
		}
	})

	// Start query in goroutine
	done := make(chan struct{})
	go func() {
		_, _ = obs.Ask(context.Background(), "What is happening?")
		close(done)
	}()

	// Give the goroutine time to start
	time.Sleep(10 * time.Millisecond)

	// Cancel the query
	obs.Cancel()

	// Wait for completion with timeout
	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Fatal("query did not complete after cancel")
	}

	if !killCalled {
		t.Error("expected Kill to be called")
	}
}

func TestObserver_Reset(t *testing.T) {
	cfg := &config.ObserverConfig{Model: "haiku"}
	broker := NewSessionBroker()
	logReader := NewLogReader("/tmp/test.log")
	builder := NewContextBuilder(logReader, cfg)

	obs := NewObserver(cfg, broker, builder, nil)

	// Set a session ID
	obs.mu.Lock()
	obs.sessionID = "test-session-123"
	obs.mu.Unlock()

	// Reset should clear it
	obs.Reset()

	obs.mu.Lock()
	if obs.sessionID != "" {
		t.Errorf("expected empty sessionID after reset, got %q", obs.sessionID)
	}
	obs.mu.Unlock()
}

func TestObserver_BuildArgs(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		sessionID string
		prompt    string
		wantArgs  []string
	}{
		{
			name:      "default model",
			model:     "haiku",
			sessionID: "",
			prompt:    "test prompt",
			wantArgs:  []string{"-p", "test prompt", "--verbose", "--output-format", "stream-json", "--model", "haiku"},
		},
		{
			name:      "custom model",
			model:     "sonnet",
			sessionID: "",
			prompt:    "test prompt",
			wantArgs:  []string{"-p", "test prompt", "--verbose", "--output-format", "stream-json", "--model", "sonnet"},
		},
		{
			name:      "with session ID",
			model:     "haiku",
			sessionID: "session-123",
			prompt:    "test prompt",
			wantArgs:  []string{"--resume", "session-123", "-p", "test prompt", "--verbose", "--output-format", "stream-json", "--model", "haiku"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.ObserverConfig{Model: tt.model}
			broker := NewSessionBroker()
			logReader := NewLogReader("/tmp/test.log")
			builder := NewContextBuilder(logReader, cfg)

			obs := NewObserver(cfg, broker, builder, nil)
			obs.mu.Lock()
			obs.sessionID = tt.sessionID
			obs.mu.Unlock()

			args := obs.buildArgs(tt.prompt)

			// Check all expected args are present
			for _, want := range tt.wantArgs {
				found := false
				for _, got := range args {
					if got == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected arg %q not found in %v", want, args)
				}
			}
		})
	}
}

func TestObserver_WithStateProvider(t *testing.T) {
	cfg := &config.ObserverConfig{Model: "haiku"}
	broker := NewSessionBroker()
	logReader := NewLogReader("/tmp/test.log")
	builder := NewContextBuilder(logReader, cfg)

	provider := &mockStateProvider{
		state: DrainState{
			Status:    "working",
			TotalCost: 1.23,
		},
	}

	obs := NewObserver(cfg, broker, builder, provider)

	// Mock runner to verify context is passed
	var capturedPrompt string
	streamJSON := makeStreamJSON("response", "test-session")
	obs.SetRunnerFactory(func() runner.ProcessRunner {
		return &mockRunner{
			startFn: func(ctx context.Context, name string, args ...string) (io.ReadCloser, io.ReadCloser, error) {
				for i, arg := range args {
					if arg == "-p" && i+1 < len(args) {
						capturedPrompt = args[i+1]
						break
					}
				}
				return io.NopCloser(strings.NewReader(streamJSON)), io.NopCloser(strings.NewReader("")), nil
			},
		}
	})

	_, err := obs.Ask(context.Background(), "What is happening?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the context includes drain status
	if !strings.Contains(capturedPrompt, "working") {
		t.Error("expected prompt to contain drain status 'working'")
	}
}

func TestLimitedWriter(t *testing.T) {
	tests := []struct {
		name        string
		limit       int
		writes      []string
		wantOutput  string
		wantTrunc   bool
		wantWritten int
	}{
		{
			name:        "under limit",
			limit:       100,
			writes:      []string{"hello", " world"},
			wantOutput:  "hello world",
			wantTrunc:   false,
			wantWritten: 11,
		},
		{
			name:        "exact limit",
			limit:       10,
			writes:      []string{"1234567890"},
			wantOutput:  "1234567890",
			wantTrunc:   false,
			wantWritten: 10,
		},
		{
			name:        "over limit single write",
			limit:       5,
			writes:      []string{"1234567890"},
			wantOutput:  "12345",
			wantTrunc:   true,
			wantWritten: 5,
		},
		{
			name:        "over limit multiple writes",
			limit:       10,
			writes:      []string{"hello", " world", "!"},
			wantOutput:  "hello worl",
			wantTrunc:   true,
			wantWritten: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			lw := &limitedWriter{w: &buf, limit: tt.limit}

			for _, s := range tt.writes {
				_, err := lw.Write([]byte(s))
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}

			if got := buf.String(); got != tt.wantOutput {
				t.Errorf("output = %q, want %q", got, tt.wantOutput)
			}
			if lw.truncated != tt.wantTrunc {
				t.Errorf("truncated = %v, want %v", lw.truncated, tt.wantTrunc)
			}
			if lw.written != tt.wantWritten {
				t.Errorf("written = %d, want %d", lw.written, tt.wantWritten)
			}
		})
	}
}

func TestObserver_HistoryTracking(t *testing.T) {
	cfg := &config.ObserverConfig{Model: "haiku"}
	broker := NewSessionBroker()
	logReader := NewLogReader("/tmp/test.log")
	builder := NewContextBuilder(logReader, cfg)

	obs := NewObserver(cfg, broker, builder, nil)

	// Set up mock runner that returns different stream-json responses
	responseNum := 0
	responses := []string{"First response", "Second response"}
	obs.SetRunnerFactory(func() runner.ProcessRunner {
		return &mockRunner{
			startFn: func(ctx context.Context, name string, args ...string) (io.ReadCloser, io.ReadCloser, error) {
				resp := makeStreamJSON(responses[responseNum], "session-"+responses[responseNum])
				responseNum++
				return io.NopCloser(strings.NewReader(resp)), io.NopCloser(strings.NewReader("")), nil
			},
		}
	})

	// First query
	resp1, err := obs.Ask(context.Background(), "First question")
	if err != nil {
		t.Fatalf("Ask failed: %v", err)
	}
	if resp1 != "First response" {
		t.Errorf("expected 'First response', got %q", resp1)
	}

	// Verify history has one exchange
	obs.mu.Lock()
	if len(obs.history) != 1 {
		t.Errorf("expected 1 exchange in history, got %d", len(obs.history))
	}
	if obs.history[0].Question != "First question" {
		t.Errorf("expected question 'First question', got %q", obs.history[0].Question)
	}
	if obs.history[0].Answer != "First response" {
		t.Errorf("expected answer 'First response', got %q", obs.history[0].Answer)
	}
	obs.mu.Unlock()

	// Second query
	resp2, err := obs.Ask(context.Background(), "Second question")
	if err != nil {
		t.Fatalf("Ask failed: %v", err)
	}
	if resp2 != "Second response" {
		t.Errorf("expected 'Second response', got %q", resp2)
	}

	// Verify history has two exchanges
	obs.mu.Lock()
	if len(obs.history) != 2 {
		t.Errorf("expected 2 exchanges in history, got %d", len(obs.history))
	}
	obs.mu.Unlock()
}

func TestObserver_ResetClearsHistory(t *testing.T) {
	cfg := &config.ObserverConfig{Model: "haiku"}
	broker := NewSessionBroker()
	logReader := NewLogReader("/tmp/test.log")
	builder := NewContextBuilder(logReader, cfg)

	obs := NewObserver(cfg, broker, builder, nil)

	// Set up mock runner with stream-json
	streamJSON := makeStreamJSON("response", "test-session")
	obs.SetRunnerFactory(func() runner.ProcessRunner {
		return &mockRunner{
			startFn: func(ctx context.Context, name string, args ...string) (io.ReadCloser, io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader(streamJSON)), io.NopCloser(strings.NewReader("")), nil
			},
		}
	})

	// Make a query to populate history
	_, err := obs.Ask(context.Background(), "test question")
	if err != nil {
		t.Fatalf("Ask failed: %v", err)
	}

	// Verify history is populated
	obs.mu.Lock()
	if len(obs.history) != 1 {
		t.Errorf("expected 1 exchange in history, got %d", len(obs.history))
	}
	obs.mu.Unlock()

	// Reset and verify history is cleared
	obs.Reset()

	obs.mu.Lock()
	if len(obs.history) != 0 {
		t.Errorf("expected empty history after Reset, got %d exchanges", len(obs.history))
	}
	obs.mu.Unlock()
}

func TestObserver_HistoryNotRecordedOnError(t *testing.T) {
	cfg := &config.ObserverConfig{Model: "haiku"}
	broker := NewSessionBroker()
	logReader := NewLogReader("/tmp/test.log")
	builder := NewContextBuilder(logReader, cfg)

	obs := NewObserver(cfg, broker, builder, nil)

	// Set up mock runner that fails
	obs.SetRunnerFactory(func() runner.ProcessRunner {
		return &mockRunner{
			startFn: func(ctx context.Context, name string, args ...string) (io.ReadCloser, io.ReadCloser, error) {
				return nil, nil, errors.New("connection failed")
			},
		}
	})

	// Make a query that will fail
	_, err := obs.Ask(context.Background(), "test question")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Verify history is empty (failed queries should not be recorded)
	obs.mu.Lock()
	if len(obs.history) != 0 {
		t.Errorf("expected empty history after failed query, got %d exchanges", len(obs.history))
	}
	obs.mu.Unlock()
}

func TestObserver_SessionIDExtraction(t *testing.T) {
	cfg := &config.ObserverConfig{Model: "haiku"}
	broker := NewSessionBroker()
	logReader := NewLogReader("/tmp/test.log")
	builder := NewContextBuilder(logReader, cfg)

	obs := NewObserver(cfg, broker, builder, nil)

	expectedSessionID := "abc123-def456-ghi789"
	streamJSON := makeStreamJSON("test response", expectedSessionID)

	obs.SetRunnerFactory(func() runner.ProcessRunner {
		return &mockRunner{
			startFn: func(ctx context.Context, name string, args ...string) (io.ReadCloser, io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader(streamJSON)), io.NopCloser(strings.NewReader("")), nil
			},
		}
	})

	_, err := obs.Ask(context.Background(), "test question")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify session ID was extracted
	obs.mu.Lock()
	actualSessionID := obs.sessionID
	obs.mu.Unlock()

	if actualSessionID != expectedSessionID {
		t.Errorf("expected sessionID %q, got %q", expectedSessionID, actualSessionID)
	}
}

func TestObserver_SessionIDMinLengthValidation(t *testing.T) {
	cfg := &config.ObserverConfig{Model: "haiku"}
	broker := NewSessionBroker()
	logReader := NewLogReader("/tmp/test.log")
	builder := NewContextBuilder(logReader, cfg)

	tests := []struct {
		name           string
		sessionID      string
		shouldBeStored bool
	}{
		{
			name:           "valid long session ID",
			sessionID:      "abc123-def456-ghi789",
			shouldBeStored: true,
		},
		{
			name:           "exactly minimum length",
			sessionID:      "12345678",
			shouldBeStored: true,
		},
		{
			name:           "too short",
			sessionID:      "short",
			shouldBeStored: false,
		},
		{
			name:           "empty",
			sessionID:      "",
			shouldBeStored: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obs := NewObserver(cfg, broker, builder, nil)
			streamJSON := makeStreamJSON("test response", tt.sessionID)

			obs.SetRunnerFactory(func() runner.ProcessRunner {
				return &mockRunner{
					startFn: func(ctx context.Context, name string, args ...string) (io.ReadCloser, io.ReadCloser, error) {
						return io.NopCloser(strings.NewReader(streamJSON)), io.NopCloser(strings.NewReader("")), nil
					},
				}
			})

			_, err := obs.Ask(context.Background(), "test question")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			obs.mu.Lock()
			actualSessionID := obs.sessionID
			obs.mu.Unlock()

			if tt.shouldBeStored {
				if actualSessionID != tt.sessionID {
					t.Errorf("expected sessionID %q, got %q", tt.sessionID, actualSessionID)
				}
			} else {
				if actualSessionID != "" {
					t.Errorf("expected empty sessionID, got %q", actualSessionID)
				}
			}
		})
	}
}

func TestObserver_ResumeOnSubsequentQuery(t *testing.T) {
	cfg := &config.ObserverConfig{Model: "haiku"}
	broker := NewSessionBroker()
	logReader := NewLogReader("/tmp/test.log")
	builder := NewContextBuilder(logReader, cfg)

	obs := NewObserver(cfg, broker, builder, nil)

	sessionID := "session-for-resume-test"
	queryCount := 0
	var capturedArgs [][]string

	obs.SetRunnerFactory(func() runner.ProcessRunner {
		return &mockRunner{
			startFn: func(ctx context.Context, name string, args ...string) (io.ReadCloser, io.ReadCloser, error) {
				queryCount++
				// Capture args for verification
				argsCopy := make([]string, len(args))
				copy(argsCopy, args)
				capturedArgs = append(capturedArgs, argsCopy)

				streamJSON := makeStreamJSON("response "+string(rune('0'+queryCount)), sessionID)
				return io.NopCloser(strings.NewReader(streamJSON)), io.NopCloser(strings.NewReader("")), nil
			},
		}
	})

	// First query - should not have --resume
	_, err := obs.Ask(context.Background(), "first question")
	if err != nil {
		t.Fatalf("first query failed: %v", err)
	}

	// Second query - should have --resume with session ID
	_, err = obs.Ask(context.Background(), "second question")
	if err != nil {
		t.Fatalf("second query failed: %v", err)
	}

	if len(capturedArgs) != 2 {
		t.Fatalf("expected 2 queries, got %d", len(capturedArgs))
	}

	// First query should not have --resume
	hasResumeFirst := false
	for _, arg := range capturedArgs[0] {
		if arg == "--resume" {
			hasResumeFirst = true
			break
		}
	}
	if hasResumeFirst {
		t.Error("first query should not have --resume flag")
	}

	// Second query should have --resume with correct session ID
	hasResumeSecond := false
	resumeValue := ""
	for i, arg := range capturedArgs[1] {
		if arg == "--resume" && i+1 < len(capturedArgs[1]) {
			hasResumeSecond = true
			resumeValue = capturedArgs[1][i+1]
			break
		}
	}
	if !hasResumeSecond {
		t.Error("second query should have --resume flag")
	}
	if resumeValue != sessionID {
		t.Errorf("expected --resume value %q, got %q", sessionID, resumeValue)
	}
}
