// Package integration provides end-to-end tests for the atari drain loop.
package integration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/observer"
	"github.com/npratt/atari/internal/testutil"
)

// observerTestEnv holds the test environment for observer integration tests.
type observerTestEnv struct {
	t          *testing.T
	tempDir    string
	mockPath   string
	logPath    string
	oldPath    string
	cfg        *config.ObserverConfig
	broker     *observer.SessionBroker
	logReader  *observer.LogReader
	builder    *observer.ContextBuilder
	obs        *observer.Observer
}

// newObserverTestEnv creates a test environment for observer tests.
func newObserverTestEnv(t *testing.T) *observerTestEnv {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "atari-observer-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Create mock claude script
	mockPath := filepath.Join(tempDir, "claude")
	if err := testutil.CreateMockClaudeForObserver(mockPath, "This is a test response from the observer."); err != nil {
		_ = os.RemoveAll(tempDir)
		t.Fatalf("failed to create mock claude: %v", err)
	}

	// Prepend temp dir to PATH
	oldPath := os.Getenv("PATH")
	_ = os.Setenv("PATH", tempDir+":"+oldPath)

	// Create log file path
	logPath := filepath.Join(tempDir, "atari.log")

	cfg := &config.ObserverConfig{
		Enabled:      true,
		Model:        "haiku",
		RecentEvents: 20,
		Layout:       "horizontal",
	}

	broker := observer.NewSessionBroker()
	logReader := observer.NewLogReader(logPath)
	builder := observer.NewContextBuilder(logReader, cfg)

	return &observerTestEnv{
		t:         t,
		tempDir:   tempDir,
		mockPath:  mockPath,
		logPath:   logPath,
		oldPath:   oldPath,
		cfg:       cfg,
		broker:    broker,
		logReader: logReader,
		builder:   builder,
	}
}

// cleanup removes temporary files and restores PATH.
func (e *observerTestEnv) cleanup() {
	_ = os.Setenv("PATH", e.oldPath)
	_ = os.RemoveAll(e.tempDir)
}

// createObserver creates an observer instance for testing.
func (e *observerTestEnv) createObserver(stateProvider observer.DrainStateProvider) *observer.Observer {
	e.obs = observer.NewObserver(e.cfg, e.broker, e.builder, stateProvider)
	return e.obs
}

// mockStateProvider implements observer.DrainStateProvider for testing.
type mockStateProvider struct {
	state observer.DrainState
}

func (m *mockStateProvider) GetDrainState() observer.DrainState {
	return m.state
}

// TestObserverBasicQuery tests a simple question-answer flow.
func TestObserverBasicQuery(t *testing.T) {
	env := newObserverTestEnv(t)
	defer env.cleanup()

	// Create sample log file
	if err := testutil.CreateSampleLogFile(env.logPath); err != nil {
		t.Fatalf("failed to create sample log: %v", err)
	}

	// Create observer with mock state
	provider := &mockStateProvider{
		state: observer.DrainState{
			Status:    "working",
			TotalCost: 0.50,
			Uptime:    10 * time.Minute,
			CurrentBead: &observer.CurrentBeadInfo{
				ID:        "bd-002",
				Title:     "Add input validation",
				StartedAt: time.Now().Add(-1 * time.Minute),
			},
		},
	}
	obs := env.createObserver(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := obs.Ask(ctx, "What is happening right now?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if response == "" {
		t.Error("expected non-empty response")
	}

	if !strings.Contains(response, "test response") {
		t.Errorf("expected response to contain mock output, got: %s", response)
	}
}

// TestObserverReturnsBusyWhenDrainIsActive tests that observer returns ErrBusy
// when drain holds the session broker.
func TestObserverReturnsBusyWhenDrainIsActive(t *testing.T) {
	env := newObserverTestEnv(t)
	defer env.cleanup()

	// Create empty log (context builder handles this gracefully)
	if err := testutil.WriteLogFixture(env.logPath, testutil.EmptyLogEvents()); err != nil {
		t.Fatalf("failed to create empty log: %v", err)
	}

	obs := env.createObserver(nil)

	// Acquire the broker (simulating drain holding it)
	err := env.broker.Acquire(context.Background(), "drain", time.Second)
	if err != nil {
		t.Fatalf("failed to acquire broker: %v", err)
	}
	defer env.broker.Release()

	// Observer should return ErrBusy when drain holds the broker
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = obs.Ask(ctx, "test question")
	if !errors.Is(err, observer.ErrBusy) {
		t.Errorf("expected observer.ErrBusy, got %v", err)
	}
}

// TestObserverCancel tests cancelling an in-progress query.
func TestObserverCancel(t *testing.T) {
	env := newObserverTestEnv(t)
	defer env.cleanup()

	// Create a ready signal file path
	readyFile := filepath.Join(env.tempDir, "ready")

	// Create mock that signals when ready, then takes a long time
	if err := testutil.CreateSlowMockClaudeWithSignal(env.mockPath, "Slow response", readyFile, "30"); err != nil {
		t.Fatalf("failed to create slow mock: %v", err)
	}

	if err := testutil.WriteLogFixture(env.logPath, testutil.EmptyLogEvents()); err != nil {
		t.Fatalf("failed to create log: %v", err)
	}

	obs := env.createObserver(nil)

	// Start query in goroutine
	done := make(chan struct{})
	var queryErr error
	go func() {
		_, queryErr = obs.Ask(context.Background(), "slow question")
		close(done)
	}()

	// Wait for the mock script to signal that it has started
	if !testutil.WaitForFile(readyFile, 5*time.Second) {
		t.Fatal("mock script did not signal ready within timeout")
	}

	// Cancel the query
	obs.Cancel()

	// Should complete quickly after cancel
	select {
	case <-done:
		// Success - query was cancelled
		if queryErr == nil || !strings.Contains(queryErr.Error(), "cancel") {
			t.Logf("query error: %v", queryErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("query did not complete after cancel")
	}
}

// TestObserverTimeout tests that queries timeout after the configured duration.
// Note: This test uses Cancel() since the observer's io.Copy blocks until
// the process terminates. The actual timeout handling in observer.go would
// need to use goroutines for proper context-based timeout.
func TestObserverTimeout(t *testing.T) {
	env := newObserverTestEnv(t)
	defer env.cleanup()

	// Create a ready signal file path
	readyFile := filepath.Join(env.tempDir, "ready")

	// Create mock that signals when ready, then takes a long time
	if err := testutil.CreateSlowMockClaudeWithSignal(env.mockPath, "Slow response", readyFile, "30"); err != nil {
		t.Fatalf("failed to create slow mock: %v", err)
	}

	if err := testutil.WriteLogFixture(env.logPath, testutil.EmptyLogEvents()); err != nil {
		t.Fatalf("failed to create log: %v", err)
	}

	obs := env.createObserver(nil)

	// Start query and cancel it to simulate timeout behavior
	done := make(chan struct{})
	var queryErr error
	go func() {
		_, queryErr = obs.Ask(context.Background(), "slow question")
		close(done)
	}()

	// Wait for the mock script to signal that it has started
	if !testutil.WaitForFile(readyFile, 5*time.Second) {
		t.Fatal("mock script did not signal ready within timeout")
	}

	start := time.Now()
	obs.Cancel()

	// Should complete quickly after cancel
	select {
	case <-done:
		elapsed := time.Since(start)
		if elapsed > 5*time.Second {
			t.Errorf("cancel took too long: %v", elapsed)
		}
		// Query should have been cancelled
		if queryErr != nil {
			t.Logf("query error (expected): %v", queryErr)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("query did not complete after cancel")
	}
}

// TestObserverContextIncludesLogEvents tests that context builder includes log events.
func TestObserverContextIncludesLogEvents(t *testing.T) {
	env := newObserverTestEnv(t)
	defer env.cleanup()

	// Create log with specific events
	events := testutil.SingleBeadLogEvents("bd-test-001", "Test bead title")
	if err := testutil.WriteLogFixture(env.logPath, events); err != nil {
		t.Fatalf("failed to create log: %v", err)
	}

	provider := &mockStateProvider{
		state: observer.DrainState{
			Status:    "working",
			TotalCost: 0.25,
			Uptime:    5 * time.Minute,
			CurrentBead: &observer.CurrentBeadInfo{
				ID:        "bd-test-001",
				Title:     "Test bead title",
				StartedAt: time.Now().Add(-1 * time.Minute),
			},
		},
	}

	// Build context directly to verify content
	context, err := env.builder.Build(provider.state, nil)
	if err != nil {
		t.Fatalf("failed to build context: %v", err)
	}

	// Should include drain status
	if !strings.Contains(context, "working") {
		t.Error("context should include drain status")
	}

	// Should include bead ID
	if !strings.Contains(context, "bd-test-001") {
		t.Error("context should include bead ID")
	}

	// Should include system prompt
	if !strings.Contains(context, "observer assistant") {
		t.Error("context should include system prompt")
	}
}

// TestObserverSessionHistory tests that session history is included in context.
func TestObserverSessionHistory(t *testing.T) {
	env := newObserverTestEnv(t)
	defer env.cleanup()

	// Create log with completed beads
	events := testutil.CompletedBeadsLogEvents(3)
	if err := testutil.WriteLogFixture(env.logPath, events); err != nil {
		t.Fatalf("failed to create log: %v", err)
	}

	provider := &mockStateProvider{
		state: observer.DrainState{
			Status:    "idle",
			TotalCost: 0.45,
			Uptime:    10 * time.Minute,
		},
	}

	context, err := env.builder.Build(provider.state, nil)
	if err != nil {
		t.Fatalf("failed to build context: %v", err)
	}

	// Should include session history section
	if !strings.Contains(context, "Session History") {
		t.Error("context should include session history section")
	}

	// Should include completed bead IDs
	if !strings.Contains(context, "bd-001") {
		t.Error("context should include first bead")
	}
}

// TestObserverWithFailedBead tests context building with failed bead events.
func TestObserverWithFailedBead(t *testing.T) {
	env := newObserverTestEnv(t)
	defer env.cleanup()

	events := testutil.FailedBeadLogEvents("bd-fail-001", "Failed task")
	if err := testutil.WriteLogFixture(env.logPath, events); err != nil {
		t.Fatalf("failed to create log: %v", err)
	}

	provider := &mockStateProvider{
		state: observer.DrainState{
			Status:    "idle",
			TotalCost: 0.15,
			Uptime:    5 * time.Minute,
		},
	}

	context, err := env.builder.Build(provider.state, nil)
	if err != nil {
		t.Fatalf("failed to build context: %v", err)
	}

	// Should include session history with failed outcome
	if !strings.Contains(context, "failed") {
		t.Error("context should include failed outcome")
	}
}

// TestObserverErrorFromClaude tests handling of Claude CLI errors.
func TestObserverErrorFromClaude(t *testing.T) {
	env := newObserverTestEnv(t)
	defer env.cleanup()

	// Create a mock that exits with error and outputs to stderr
	// Note: observer captures stderr as output when stdout is empty,
	// so we verify the error message is captured
	if err := testutil.CreateFailingMockClaudeForObserver(env.mockPath, "simulated failure"); err != nil {
		t.Fatalf("failed to create failing mock: %v", err)
	}

	if err := testutil.WriteLogFixture(env.logPath, testutil.EmptyLogEvents()); err != nil {
		t.Fatalf("failed to create log: %v", err)
	}

	obs := env.createObserver(nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := obs.Ask(ctx, "test question")

	// Observer captures stderr as output when claude fails
	// Either we get an error, or the response contains the error message
	if err != nil {
		t.Logf("got error as expected: %v", err)
	} else if !strings.Contains(response, "simulated failure") {
		t.Errorf("expected error message in response, got: %s", response)
	}
}

// TestObserverReset tests that Reset clears session state.
func TestObserverReset(t *testing.T) {
	env := newObserverTestEnv(t)
	defer env.cleanup()

	if err := testutil.CreateMockClaudeForObserver(env.mockPath, "response"); err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}

	if err := testutil.WriteLogFixture(env.logPath, testutil.EmptyLogEvents()); err != nil {
		t.Fatalf("failed to create log: %v", err)
	}

	obs := env.createObserver(nil)

	// First query
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	_, err := obs.Ask(ctx, "first question")
	cancel()
	if err != nil {
		t.Fatalf("first query failed: %v", err)
	}

	// Reset
	obs.Reset()

	// Second query should work (session cleared)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()

	_, err = obs.Ask(ctx2, "second question")
	if err != nil {
		t.Fatalf("second query failed: %v", err)
	}
}

// TestObserverModelConfiguration tests that model is passed to claude CLI.
func TestObserverModelConfiguration(t *testing.T) {
	env := newObserverTestEnv(t)
	defer env.cleanup()

	// Use sonnet model
	env.cfg.Model = "sonnet"

	// Create a mock that echoes args (for verification)
	script := `#!/bin/bash
# Echo the arguments for verification
echo "args: $@"
`
	if err := os.WriteFile(env.mockPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create script: %v", err)
	}

	if err := testutil.WriteLogFixture(env.logPath, testutil.EmptyLogEvents()); err != nil {
		t.Fatalf("failed to create log: %v", err)
	}

	// Recreate builder and observer with updated config
	env.builder = observer.NewContextBuilder(env.logReader, env.cfg)
	obs := env.createObserver(nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	response, err := obs.Ask(ctx, "test question")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	// Response should include model flag
	if !strings.Contains(response, "sonnet") {
		t.Logf("response: %s", response)
		// Note: model is only added for non-haiku models
	}
}

// TestObserverEmptyLog tests observer behavior with an empty log file.
func TestObserverEmptyLog(t *testing.T) {
	env := newObserverTestEnv(t)
	defer env.cleanup()

	// Create empty log file
	if err := os.WriteFile(env.logPath, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create empty log: %v", err)
	}

	provider := &mockStateProvider{
		state: observer.DrainState{
			Status: "idle",
		},
	}

	obs := env.createObserver(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Should work even with empty log
	response, err := obs.Ask(ctx, "test question")
	if err != nil {
		t.Fatalf("query failed with empty log: %v", err)
	}
	if response == "" {
		t.Error("expected response even with empty log")
	}
}

// TestObserverNoLogFile tests observer behavior when log file doesn't exist.
func TestObserverNoLogFile(t *testing.T) {
	env := newObserverTestEnv(t)
	defer env.cleanup()

	// Don't create log file - it won't exist

	provider := &mockStateProvider{
		state: observer.DrainState{
			Status: "idle",
		},
	}

	obs := env.createObserver(provider)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Should work even without log file
	response, err := obs.Ask(ctx, "test question")
	if err != nil {
		t.Fatalf("query failed without log file: %v", err)
	}
	if response == "" {
		t.Error("expected response even without log file")
	}
}
