package tui

import (
	"os"
	"testing"
	"time"

	"github.com/npratt/atari/internal/events"
	"github.com/npratt/atari/internal/testutil"
	"github.com/npratt/atari/internal/viewmodel"
)

// tuiTestEnv provides an isolated test environment for TUI integration tests.
// It wires together real components (router, event channel) with mocks where
// needed (runner, stats, observer) to test TUI behavior in realistic conditions.
type tuiTestEnv struct {
	t       *testing.T
	tempDir string
	runner  *testutil.MockRunner
	router  *events.Router

	// Event channel for TUI consumption
	eventChan chan events.Event

	// Callback tracking
	pauseCalled  bool
	resumeCalled bool
	quitCalled   bool

	// Fake dependencies
	statsGetter *fakeStatsGetter
}

// newTUITestEnv creates a new test environment with all dependencies wired up.
// Call env.close() in defer to clean up resources.
func newTUITestEnv(t *testing.T) *tuiTestEnv {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "tui-integration-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Create .atari directory structure
	if err := os.MkdirAll(tempDir+"/.atari", 0755); err != nil {
		_ = os.RemoveAll(tempDir)
		t.Fatalf("failed to create .atari dir: %v", err)
	}

	runner := testutil.NewMockRunner()
	router := events.NewRouter(100)

	// Create buffered event channel for TUI
	eventChan := make(chan events.Event, 100)

	env := &tuiTestEnv{
		t:           t,
		tempDir:     tempDir,
		runner:      runner,
		router:      router,
		eventChan:   eventChan,
		statsGetter: newFakeStatsGetter(),
	}

	return env
}

// close cleans up all test resources.
func (env *tuiTestEnv) close() {
	env.router.Close()
	close(env.eventChan)
	_ = os.RemoveAll(env.tempDir)
}

// emitEvent sends an event to the TUI event channel.
// Returns false if the channel is full (non-blocking send).
func (env *tuiTestEnv) emitEvent(evt events.Event) bool {
	select {
	case env.eventChan <- evt:
		return true
	default:
		return false
	}
}

// onPause returns a callback that tracks pause invocations.
func (env *tuiTestEnv) onPause() func() {
	return func() {
		env.pauseCalled = true
	}
}

// onResume returns a callback that tracks resume invocations.
func (env *tuiTestEnv) onResume() func() {
	return func() {
		env.resumeCalled = true
	}
}

// onQuit returns a callback that tracks quit invocations.
func (env *tuiTestEnv) onQuit() func() {
	return func() {
		env.quitCalled = true
	}
}

// resetCallbacks clears all callback tracking flags.
func (env *tuiTestEnv) resetCallbacks() {
	env.pauseCalled = false
	env.resumeCalled = false
	env.quitCalled = false
}

// newModel creates a model configured with the test environment's dependencies.
// This allows testing the model with real event flow and fake stats/observer.
func (env *tuiTestEnv) newModel() model {
	return newModel(
		env.eventChan,
		env.onPause(),
		env.onResume(),
		env.onQuit(),
		env.statsGetter,
		nil, // observer - nil is acceptable for most tests
		nil, // graph fetcher - nil is acceptable for most tests
		nil, // bead state getter - nil is acceptable for most tests
	)
}

// fakeStatsGetter implements StatsGetter with scripted values.
type fakeStatsGetter struct {
	iteration    int
	completed    int
	failed       int
	abandoned    int
	currentBead  string
	currentTurns int
}

// newFakeStatsGetter creates a fakeStatsGetter with default values.
func newFakeStatsGetter() *fakeStatsGetter {
	return &fakeStatsGetter{
		iteration:    0,
		completed:    0,
		failed:       0,
		abandoned:    0,
		currentBead:  "",
		currentTurns: 0,
	}
}

// Iteration implements StatsGetter.
func (f *fakeStatsGetter) Iteration() int {
	return f.iteration
}

// Completed implements StatsGetter.
func (f *fakeStatsGetter) Completed() int {
	return f.completed
}

// Failed implements StatsGetter.
func (f *fakeStatsGetter) Failed() int {
	return f.failed
}

// Abandoned implements StatsGetter.
func (f *fakeStatsGetter) Abandoned() int {
	return f.abandoned
}

// CurrentBead implements StatsGetter.
func (f *fakeStatsGetter) CurrentBead() string {
	return f.currentBead
}

// CurrentTurns implements StatsGetter.
func (f *fakeStatsGetter) CurrentTurns() int {
	return f.currentTurns
}

// GetStats implements StatsGetter.
func (f *fakeStatsGetter) GetStats() viewmodel.TUIStats {
	return viewmodel.TUIStats{
		Completed:    f.completed,
		Failed:       f.failed,
		Abandoned:    f.abandoned,
		CurrentBead:  f.currentBead,
		CurrentTurns: f.currentTurns,
	}
}

// SetStats updates all stats at once for testing.
func (f *fakeStatsGetter) SetStats(iteration, completed, failed, abandoned int, currentBead string, currentTurns int) {
	f.iteration = iteration
	f.completed = completed
	f.failed = failed
	f.abandoned = abandoned
	f.currentBead = currentBead
	f.currentTurns = currentTurns
}

// FastTestConfig returns timing values suitable for fast integration tests.
// These values are designed to keep tests quick while still exercising real behavior.
//   - pollInterval: 10ms for fast polling
//   - timeout: 5s to allow operations to complete
//   - maxFailures: 3 retries before abandoning
func FastTestConfig() (pollInterval, timeout time.Duration, maxFailures int) {
	return 10 * time.Millisecond, 5 * time.Second, 3
}

// TestTUITestEnvSetup verifies the test environment can be created and closed.
func TestTUITestEnvSetup(t *testing.T) {
	env := newTUITestEnv(t)
	defer env.close()

	// Verify temp directory exists
	if _, err := os.Stat(env.tempDir); os.IsNotExist(err) {
		t.Error("temp directory was not created")
	}

	// Verify .atari subdirectory exists
	if _, err := os.Stat(env.tempDir + "/.atari"); os.IsNotExist(err) {
		t.Error(".atari directory was not created")
	}

	// Verify components are initialized
	if env.runner == nil {
		t.Error("runner is nil")
	}
	if env.router == nil {
		t.Error("router is nil")
	}
	if env.eventChan == nil {
		t.Error("eventChan is nil")
	}
	if env.statsGetter == nil {
		t.Error("statsGetter is nil")
	}
}

// TestTUITestEnvEmitEvent verifies events can be sent to the TUI channel.
func TestTUITestEnvEmitEvent(t *testing.T) {
	env := newTUITestEnv(t)
	defer env.close()

	// Create a test event
	evt := &events.DrainStartEvent{
		BaseEvent: events.BaseEvent{
			EventType: events.EventDrainStart,
			Time:      time.Now(),
		},
	}

	// Emit should succeed
	if !env.emitEvent(evt) {
		t.Error("emitEvent returned false")
	}

	// Event should be receivable
	select {
	case received := <-env.eventChan:
		if received.Type() != events.EventDrainStart {
			t.Errorf("expected EventDrainStart, got %v", received.Type())
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for event")
	}
}

// TestTUITestEnvCallbacks verifies callback tracking works correctly.
func TestTUITestEnvCallbacks(t *testing.T) {
	env := newTUITestEnv(t)
	defer env.close()

	// Initially no callbacks called
	if env.pauseCalled || env.resumeCalled || env.quitCalled {
		t.Error("callbacks should not be called initially")
	}

	// Invoke callbacks
	env.onPause()()
	if !env.pauseCalled {
		t.Error("pause callback not tracked")
	}

	env.onResume()()
	if !env.resumeCalled {
		t.Error("resume callback not tracked")
	}

	env.onQuit()()
	if !env.quitCalled {
		t.Error("quit callback not tracked")
	}

	// Reset and verify
	env.resetCallbacks()
	if env.pauseCalled || env.resumeCalled || env.quitCalled {
		t.Error("callbacks should be reset")
	}
}

// TestFakeStatsGetter verifies the fake stats getter works correctly.
func TestFakeStatsGetter(t *testing.T) {
	stats := newFakeStatsGetter()

	// Default values
	if stats.Iteration() != 0 {
		t.Errorf("expected iteration 0, got %d", stats.Iteration())
	}
	if stats.CurrentBead() != "" {
		t.Errorf("expected empty currentBead, got %q", stats.CurrentBead())
	}

	// Set values
	stats.SetStats(5, 10, 2, 1, "bd-test-001", 7)

	if stats.Iteration() != 5 {
		t.Errorf("expected iteration 5, got %d", stats.Iteration())
	}
	if stats.Completed() != 10 {
		t.Errorf("expected completed 10, got %d", stats.Completed())
	}
	if stats.Failed() != 2 {
		t.Errorf("expected failed 2, got %d", stats.Failed())
	}
	if stats.Abandoned() != 1 {
		t.Errorf("expected abandoned 1, got %d", stats.Abandoned())
	}
	if stats.CurrentBead() != "bd-test-001" {
		t.Errorf("expected currentBead bd-test-001, got %q", stats.CurrentBead())
	}
	if stats.CurrentTurns() != 7 {
		t.Errorf("expected currentTurns 7, got %d", stats.CurrentTurns())
	}
}

// TestTUITestEnvNewModel verifies the model can be created from the test env.
func TestTUITestEnvNewModel(t *testing.T) {
	env := newTUITestEnv(t)
	defer env.close()

	m := env.newModel()

	// Verify model is initialized with expected defaults
	if m.eventChan == nil {
		t.Error("model eventChan is nil")
	}
	if m.status != "idle" {
		t.Errorf("expected status idle, got %q", m.status)
	}
	if !m.autoScroll {
		t.Error("expected autoScroll to be true")
	}
	if m.statsGetter != env.statsGetter {
		t.Error("statsGetter not wired correctly")
	}
}

// TestFastTestConfig verifies the fast test config returns expected values.
func TestFastTestConfig(t *testing.T) {
	pollInterval, timeout, maxFailures := FastTestConfig()

	if pollInterval != 10*time.Millisecond {
		t.Errorf("expected pollInterval 10ms, got %v", pollInterval)
	}
	if timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", timeout)
	}
	if maxFailures != 3 {
		t.Errorf("expected maxFailures 3, got %d", maxFailures)
	}
}
