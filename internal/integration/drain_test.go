// Package integration provides end-to-end tests for the atari drain loop.
// These tests exercise the full controller with mocked external commands.
package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/controller"
	"github.com/npratt/atari/internal/events"
	"github.com/npratt/atari/internal/testutil"
	"github.com/npratt/atari/internal/workqueue"
)

// testEnv holds the test environment for integration tests.
type testEnv struct {
	t         *testing.T
	cfg       *config.Config
	runner    *testutil.MockRunner
	router    *events.Router
	tempDir   string
	mockPath  string
	oldPath   string
	sub       <-chan events.Event
	collected []events.Event
}

// newTestEnv creates a test environment with mock claude script.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "atari-integration-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Create mock claude script
	mockClaudePath := filepath.Join(tempDir, "claude")
	if err := createMockClaude(mockClaudePath); err != nil {
		_ = os.RemoveAll(tempDir)
		t.Fatalf("failed to create mock claude: %v", err)
	}

	// Prepend temp dir to PATH
	oldPath := os.Getenv("PATH")
	_ = os.Setenv("PATH", tempDir+":"+oldPath)

	cfg := testConfig()
	runner := testutil.NewMockRunner()
	setupAgentStateMocks(runner)

	router := events.NewRouter(1000)
	sub := router.Subscribe()

	return &testEnv{
		t:        t,
		cfg:      cfg,
		runner:   runner,
		router:   router,
		tempDir:  tempDir,
		mockPath: mockClaudePath,
		oldPath:  oldPath,
		sub:      sub,
	}
}

// cleanup removes temporary files and restores PATH.
func (e *testEnv) cleanup() {
	e.router.Close()
	_ = os.Setenv("PATH", e.oldPath)
	_ = os.RemoveAll(e.tempDir)
}

// collectEvents drains events from the subscription until timeout.
func (e *testEnv) collectEvents(timeout time.Duration) {
	deadline := time.After(timeout)
	for {
		select {
		case evt, ok := <-e.sub:
			if !ok {
				return
			}
			e.collected = append(e.collected, evt)
		case <-deadline:
			return
		}
	}
}

// findEvent returns the first event of the specified type.
func (e *testEnv) findEvent(eventType events.EventType) events.Event {
	for _, evt := range e.collected {
		if evt.Type() == eventType {
			return evt
		}
	}
	return nil
}

// countEvents returns the number of events of the specified type.
func (e *testEnv) countEvents(eventType events.EventType) int {
	count := 0
	for _, evt := range e.collected {
		if evt.Type() == eventType {
			count++
		}
	}
	return count
}

// testAgentID is the agent bead ID used in integration tests.
const testAgentID = "test-agent"

// testConfig returns a config suitable for fast integration tests.
func testConfig() *config.Config {
	cfg := config.Default()
	cfg.WorkQueue.PollInterval = 10 * time.Millisecond
	cfg.Claude.Timeout = 5 * time.Second
	cfg.Backoff.Initial = 10 * time.Millisecond
	cfg.Backoff.Max = 50 * time.Millisecond
	cfg.Backoff.MaxFailures = 3
	cfg.AgentID = testAgentID
	return cfg
}

// setupAgentStateMocks configures mock responses for bd agent state commands.
func setupAgentStateMocks(runner *testutil.MockRunner) {
	for _, state := range []string{"idle", "running", "stopped", "dead"} {
		runner.SetResponse("bd", []string{"agent", "state", testAgentID, state}, []byte(""))
	}
}

// createMockClaude creates a script that simulates claude's stream-json output.
func createMockClaude(path string) error {
	// This script outputs a successful session:
	// 1. system init event
	// 2. assistant message
	// 3. result success
	script := `#!/bin/bash
# Mock claude for integration testing
# Reads prompt from stdin and outputs stream-json

# Read prompt (discard it)
cat > /dev/null

# Check for MOCK_CLAUDE_FAIL environment variable
if [ "$MOCK_CLAUDE_FAIL" = "1" ]; then
    echo '{"type":"result","subtype":"error_tool_use","error":"simulated failure"}'
    exit 1
fi

# Output stream-json events
echo '{"type":"system","subtype":"init","session_id":"test-001","cwd":"/workspace","tools":["Bash","Read","Write"]}'
sleep 0.01
echo '{"type":"assistant","message":{"content":[{"type":"text","text":"Working on the task..."}]}}'
sleep 0.01
echo '{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tool_001","name":"Bash","input":{"command":"bd close test-bead-001 --reason done"}}]}}'
sleep 0.01
echo '{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tool_001","content":"closed"}]}}'
sleep 0.01
echo '{"type":"result","subtype":"success","total_cost_usd":0.05,"duration_ms":1000,"num_turns":3,"session_id":"test-001"}'

exit 0
`
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		return err
	}
	return nil
}

// createFailingMockClaude creates a script that simulates a failing session.
func createFailingMockClaude(path string) error {
	script := `#!/bin/bash
# Mock claude that fails
cat > /dev/null
echo '{"type":"system","subtype":"init","session_id":"fail-001","cwd":"/workspace","tools":[]}'
sleep 0.01
echo '{"type":"result","subtype":"error_tool_use","error":"simulated failure"}'
exit 1
`
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		return err
	}
	return nil
}

// singleBeadJSON returns bd ready response with a single bead.
func singleBeadJSON(id, title string) []byte {
	bead := []map[string]interface{}{
		{
			"id":          id,
			"title":       title,
			"description": "Test bead description",
			"status":      "open",
			"priority":    1,
			"issue_type":  "task",
			"created_at":  "2024-01-15T10:00:00Z",
			"created_by":  "user",
		},
	}
	data, _ := json.Marshal(bead)
	return data
}

// multipleBeadsJSON returns bd ready response with multiple beads.
func multipleBeadsJSON(count int) []byte {
	beads := make([]map[string]interface{}, count)
	for i := 0; i < count; i++ {
		beads[i] = map[string]interface{}{
			"id":          fmt.Sprintf("bd-%03d", i+1),
			"title":       fmt.Sprintf("Test bead %d", i+1),
			"description": fmt.Sprintf("Description for bead %d", i+1),
			"status":      "open",
			"priority":    i + 1,
			"issue_type":  "task",
			"created_at":  "2024-01-15T10:00:00Z",
			"created_by":  "user",
		}
	}
	data, _ := json.Marshal(beads)
	return data
}

func TestFullDrainCycle(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	// Configure mock to return one bead, then empty
	callCount := 0
	beadJSON := singleBeadJSON("bd-001", "Test bead 1")

	env.runner.DynamicResponse = func(ctx context.Context, name string, args []string) ([]byte, error, bool) {
		if name == "bd" && len(args) >= 2 && args[0] == "ready" {
			callCount++
			if callCount > 1 {
				return []byte("[]"), nil, true
			}
			return beadJSON, nil, true
		}
		return nil, nil, false
	}

	wq := workqueue.New(env.cfg, env.runner)
	ctrl := controller.New(env.cfg, wq, env.router, env.runner, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- ctrl.Run(ctx)
	}()

	// Give time for one iteration
	time.Sleep(500 * time.Millisecond)
	ctrl.Stop()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for controller to stop")
	}

	// Collect and verify events
	env.collectEvents(100 * time.Millisecond)

	// Should have drain start event
	if evt := env.findEvent(events.EventDrainStart); evt == nil {
		t.Error("expected DrainStartEvent")
	}

	// Should have iteration start event
	if evt := env.findEvent(events.EventIterationStart); evt == nil {
		t.Error("expected IterationStartEvent")
	} else {
		iterEvt := evt.(*events.IterationStartEvent)
		if iterEvt.BeadID != "bd-001" {
			t.Errorf("expected bead id bd-001, got %s", iterEvt.BeadID)
		}
	}

	// Should have session start event
	if evt := env.findEvent(events.EventSessionStart); evt == nil {
		t.Error("expected SessionStartEvent")
	}

	// Should have iteration end event
	if evt := env.findEvent(events.EventIterationEnd); evt == nil {
		t.Error("expected IterationEndEvent")
	}

	// Should have drain stop event
	if evt := env.findEvent(events.EventDrainStop); evt == nil {
		t.Error("expected DrainStopEvent")
	}
}

func TestDrainWithMultipleBeads(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	// Configure mock to return 3 beads initially, then empty
	callCount := 0
	beadsJSON := multipleBeadsJSON(3)

	env.runner.DynamicResponse = func(ctx context.Context, name string, args []string) ([]byte, error, bool) {
		if name == "bd" && len(args) >= 2 && args[0] == "ready" {
			callCount++
			// Return beads for first 3 calls, then empty
			if callCount <= 3 {
				return beadsJSON, nil, true
			}
			return []byte("[]"), nil, true
		}
		return nil, nil, false
	}

	wq := workqueue.New(env.cfg, env.runner)
	ctrl := controller.New(env.cfg, wq, env.router, env.runner, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- ctrl.Run(ctx)
	}()

	// Give time for multiple iterations
	time.Sleep(2 * time.Second)
	ctrl.Stop()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for controller to stop")
	}

	// Collect events
	env.collectEvents(100 * time.Millisecond)

	// Count iteration events - should have at least one
	iterCount := env.countEvents(events.EventIterationStart)
	if iterCount < 1 {
		t.Errorf("expected at least 1 iteration, got %d", iterCount)
	}

	t.Logf("processed %d iterations", iterCount)
}

func TestDrainWithFailedBead(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	// Replace mock claude with failing version
	if err := createFailingMockClaude(env.mockPath); err != nil {
		t.Fatalf("failed to create failing mock: %v", err)
	}

	// Return one bead
	env.runner.SetResponse("bd", []string{"ready", "--json"}, singleBeadJSON("bd-fail-001", "Failing bead"))

	wq := workqueue.New(env.cfg, env.runner)
	ctrl := controller.New(env.cfg, wq, env.router, env.runner, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- ctrl.Run(ctx)
	}()

	// Give time for one iteration
	time.Sleep(500 * time.Millisecond)
	ctrl.Stop()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for controller to stop")
	}

	// Verify failure was recorded in history
	history := wq.History()
	if h, ok := history["bd-fail-001"]; ok {
		if h.Status != workqueue.HistoryFailed && h.Status != workqueue.HistoryAbandoned {
			t.Errorf("expected failed or abandoned status, got %s", h.Status)
		}
		if h.Attempts < 1 {
			t.Errorf("expected at least 1 attempt, got %d", h.Attempts)
		}
		t.Logf("bead status: %s, attempts: %d", h.Status, h.Attempts)
	} else {
		t.Error("expected bead to be in history")
	}
}

func TestGracefulShutdown(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	// Return beads continuously
	env.runner.SetResponse("bd", []string{"ready", "--json"}, singleBeadJSON("bd-shutdown-001", "Shutdown test"))

	wq := workqueue.New(env.cfg, env.runner)
	ctrl := controller.New(env.cfg, wq, env.router, env.runner, nil, nil)

	ctx := context.Background()

	done := make(chan error, 1)
	go func() {
		done <- ctrl.Run(ctx)
	}()

	// Wait for controller to start working
	time.Sleep(100 * time.Millisecond)

	// Request stop
	start := time.Now()
	ctrl.Stop()

	// Should stop gracefully
	select {
	case err := <-done:
		elapsed := time.Since(start)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		// Should complete within a reasonable time
		if elapsed > 3*time.Second {
			t.Errorf("shutdown took too long: %v", elapsed)
		}
		t.Logf("graceful shutdown completed in %v", elapsed)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for graceful shutdown")
	}

	// Verify final state
	if ctrl.State() != controller.StateStopped {
		t.Errorf("expected state %s, got %s", controller.StateStopped, ctrl.State())
	}

	// Verify drain stop event was emitted
	env.collectEvents(100 * time.Millisecond)
	if evt := env.findEvent(events.EventDrainStop); evt == nil {
		t.Error("expected DrainStopEvent")
	} else {
		stopEvt := evt.(*events.DrainStopEvent)
		if stopEvt.Reason == "" {
			t.Error("expected non-empty stop reason")
		}
		t.Logf("stop reason: %s", stopEvt.Reason)
	}
}

func TestBackoffProgression(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	// Set low max failures for faster test
	env.cfg.Backoff.MaxFailures = 2

	// Replace mock claude with failing version
	if err := createFailingMockClaude(env.mockPath); err != nil {
		t.Fatalf("failed to create failing mock: %v", err)
	}

	// Return one bead repeatedly
	env.runner.SetResponse("bd", []string{"ready", "--json"}, singleBeadJSON("bd-backoff-001", "Backoff test"))

	wq := workqueue.New(env.cfg, env.runner)
	ctrl := controller.New(env.cfg, wq, env.router, env.runner, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- ctrl.Run(ctx)
	}()

	// Give time for multiple failure attempts
	time.Sleep(1 * time.Second)
	ctrl.Stop()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for controller to stop")
	}

	// Verify bead was eventually abandoned
	history := wq.History()
	if h, ok := history["bd-backoff-001"]; ok {
		if h.Attempts < 2 {
			t.Errorf("expected at least 2 attempts, got %d", h.Attempts)
		}
		// After max failures, should be abandoned
		if h.Attempts >= env.cfg.Backoff.MaxFailures && h.Status != workqueue.HistoryAbandoned {
			t.Errorf("expected abandoned status after %d failures, got %s", h.Attempts, h.Status)
		}
		t.Logf("bead status: %s, attempts: %d", h.Status, h.Attempts)
	} else {
		t.Error("expected bead to be in history")
	}

	// Collect events and check for abandoned event
	env.collectEvents(100 * time.Millisecond)
	if evt := env.findEvent(events.EventBeadAbandoned); evt != nil {
		abandonEvt := evt.(*events.BeadAbandonedEvent)
		t.Logf("bead abandoned: id=%s, attempts=%d", abandonEvt.BeadID, abandonEvt.Attempts)
	}
}

func TestContextCancellation(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	// Return beads
	env.runner.SetResponse("bd", []string{"ready", "--json"}, singleBeadJSON("bd-cancel-001", "Cancel test"))

	wq := workqueue.New(env.cfg, env.runner)
	ctrl := controller.New(env.cfg, wq, env.router, env.runner, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- ctrl.Run(ctx)
	}()

	// Wait for controller to start
	time.Sleep(100 * time.Millisecond)

	// Cancel context
	start := time.Now()
	cancel()

	// Should exit cleanly
	select {
	case err := <-done:
		elapsed := time.Since(start)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		t.Logf("context cancellation handled in %v", elapsed)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for context cancellation handling")
	}

	if ctrl.State() != controller.StateStopped {
		t.Errorf("expected state %s, got %s", controller.StateStopped, ctrl.State())
	}
}

func TestPauseResumeDuringDrain(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	// Always return empty beads to keep controller in idle state
	env.runner.SetResponse("bd", []string{"ready", "--json"}, []byte("[]"))

	wq := workqueue.New(env.cfg, env.runner)
	ctrl := controller.New(env.cfg, wq, env.router, env.runner, nil, nil)

	ctx := context.Background()

	done := make(chan error, 1)
	go func() {
		done <- ctrl.Run(ctx)
	}()

	// Wait for controller to start and poll (staying in idle)
	time.Sleep(50 * time.Millisecond)

	// Pause from idle state - should work immediately
	ctrl.Pause()

	// Wait for state to transition
	deadline := time.After(500 * time.Millisecond)
	paused := false
	for !paused {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for paused state, got %s", ctrl.State())
		default:
			if ctrl.State() == controller.StatePaused {
				paused = true
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Resume
	ctrl.Resume()

	// Wait for state to change from paused
	deadline = time.After(500 * time.Millisecond)
	resumed := false
	for !resumed {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for resume, still in %s", ctrl.State())
		default:
			if ctrl.State() != controller.StatePaused {
				resumed = true
			}
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Verify we resumed to idle
	if ctrl.State() != controller.StateIdle {
		t.Logf("state after resume: %s (may be working or idle)", ctrl.State())
	}

	// Stop
	ctrl.Stop()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for controller to stop")
	}

	t.Logf("final state: %s", ctrl.State())
}
