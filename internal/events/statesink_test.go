package events

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewStateSink(t *testing.T) {
	sink := NewStateSink("/tmp/test-state.json")
	if sink == nil {
		t.Fatal("NewStateSink returned nil")
	}
	if sink.path != "/tmp/test-state.json" {
		t.Errorf("path = %q, want %q", sink.path, "/tmp/test-state.json")
	}
	if sink.state.Version != 1 {
		t.Errorf("state.Version = %d, want 1", sink.state.Version)
	}
	if sink.state.History == nil {
		t.Error("state.History is nil, want initialized map")
	}
}

func TestStateSinkCreatesDirectory(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "subdir", "nested", "state.json")

	sink := NewStateSink(path)
	events := make(chan Event, 10)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := sink.Start(ctx, events)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify directory was created
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("expected directory to be created")
	}

	cancel()
	_ = sink.Stop()
}

func TestStateSinkTracksIterations(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "state.json")

	sink := NewStateSink(path)
	sink.SetMinDelay(0) // Disable debounce for testing
	events := make(chan Event, 10)

	ctx, cancel := context.WithCancel(context.Background())

	err := sink.Start(ctx, events)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Send iteration start
	events <- &IterationStartEvent{
		BaseEvent: NewInternalEvent(EventIterationStart),
		BeadID:    "bd-001",
		Title:     "Test bead",
		Priority:  1,
		Attempt:   1,
	}

	time.Sleep(50 * time.Millisecond)

	state := sink.State()
	if state.Iteration != 1 {
		t.Errorf("Iteration = %d, want 1", state.Iteration)
	}
	if state.CurrentBead != "bd-001" {
		t.Errorf("CurrentBead = %q, want %q", state.CurrentBead, "bd-001")
	}

	// Send iteration end
	events <- &IterationEndEvent{
		BaseEvent:    NewInternalEvent(EventIterationEnd),
		BeadID:       "bd-001",
		Success:      true,
		NumTurns:     5,
		DurationMs:   1000,
		TotalCostUSD: 0.10,
	}

	time.Sleep(50 * time.Millisecond)

	state = sink.State()
	if state.CurrentBead != "" {
		t.Errorf("CurrentBead = %q, want empty", state.CurrentBead)
	}
	if state.TotalCost != 0.10 {
		t.Errorf("TotalCost = %f, want 0.10", state.TotalCost)
	}
	if state.TotalTurns != 5 {
		t.Errorf("TotalTurns = %d, want 5", state.TotalTurns)
	}

	// Check history
	h, ok := state.History["bd-001"]
	if !ok {
		t.Fatal("expected history for bd-001")
	}
	if h.Status != HistoryCompleted {
		t.Errorf("History status = %q, want %q", h.Status, HistoryCompleted)
	}

	cancel()
	_ = sink.Stop()
}

func TestStateSinkTracksDrainStatus(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "state.json")

	sink := NewStateSink(path)
	sink.SetMinDelay(0)
	events := make(chan Event, 10)

	ctx, cancel := context.WithCancel(context.Background())

	err := sink.Start(ctx, events)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Send drain start
	events <- &DrainStartEvent{
		BaseEvent: NewInternalEvent(EventDrainStart),
		WorkDir:   "/test/dir",
	}

	time.Sleep(50 * time.Millisecond)

	state := sink.State()
	if state.Status != "running" {
		t.Errorf("Status = %q, want %q", state.Status, "running")
	}

	// Send drain stop
	events <- &DrainStopEvent{
		BaseEvent: NewInternalEvent(EventDrainStop),
		Reason:    "user requested",
	}

	time.Sleep(50 * time.Millisecond)

	state = sink.State()
	if state.Status != "stopped" {
		t.Errorf("Status = %q, want %q", state.Status, "stopped")
	}

	cancel()
	_ = sink.Stop()
}

func TestStateSinkTracksFailedBeads(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "state.json")

	sink := NewStateSink(path)
	sink.SetMinDelay(0)
	events := make(chan Event, 10)

	ctx, cancel := context.WithCancel(context.Background())

	err := sink.Start(ctx, events)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Send iteration start
	events <- &IterationStartEvent{
		BaseEvent: NewInternalEvent(EventIterationStart),
		BeadID:    "bd-002",
		Title:     "Failing bead",
		Priority:  1,
		Attempt:   1,
	}

	time.Sleep(50 * time.Millisecond)

	// Send iteration end with failure
	events <- &IterationEndEvent{
		BaseEvent:    NewInternalEvent(EventIterationEnd),
		BeadID:       "bd-002",
		Success:      false,
		NumTurns:     3,
		DurationMs:   500,
		TotalCostUSD: 0.05,
		Error:        "tests failed",
	}

	time.Sleep(50 * time.Millisecond)

	state := sink.State()
	h, ok := state.History["bd-002"]
	if !ok {
		t.Fatal("expected history for bd-002")
	}
	if h.Status != HistoryFailed {
		t.Errorf("History status = %q, want %q", h.Status, HistoryFailed)
	}
	if h.LastError != "tests failed" {
		t.Errorf("History LastError = %q, want %q", h.LastError, "tests failed")
	}

	cancel()
	_ = sink.Stop()
}

func TestStateSinkPersistsAndLoads(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "state.json")

	// First sink - write some state
	sink1 := NewStateSink(path)
	sink1.SetMinDelay(0)
	events1 := make(chan Event, 10)

	ctx1, cancel1 := context.WithCancel(context.Background())

	err := sink1.Start(ctx1, events1)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	events1 <- &DrainStartEvent{
		BaseEvent: NewInternalEvent(EventDrainStart),
		WorkDir:   "/test/dir",
	}
	events1 <- &IterationStartEvent{
		BaseEvent: NewInternalEvent(EventIterationStart),
		BeadID:    "bd-003",
		Title:     "Persisted bead",
		Priority:  1,
		Attempt:   1,
	}
	events1 <- &IterationEndEvent{
		BaseEvent:    NewInternalEvent(EventIterationEnd),
		BeadID:       "bd-003",
		Success:      true,
		NumTurns:     10,
		DurationMs:   2000,
		TotalCostUSD: 0.25,
	}

	time.Sleep(50 * time.Millisecond)
	cancel1()
	_ = sink1.Stop()

	// Verify file exists and contains valid JSON
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read state file: %v", err)
	}

	var savedState State
	if err := json.Unmarshal(data, &savedState); err != nil {
		t.Fatalf("state file is not valid JSON: %v", err)
	}

	// Second sink - load and verify
	sink2 := NewStateSink(path)
	events2 := make(chan Event, 10)

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	err = sink2.Start(ctx2, events2)
	if err != nil {
		t.Fatalf("Start (second sink) failed: %v", err)
	}

	state := sink2.State()
	if state.Status != "running" {
		t.Errorf("loaded Status = %q, want %q", state.Status, "running")
	}
	if state.Iteration != 1 {
		t.Errorf("loaded Iteration = %d, want 1", state.Iteration)
	}
	if state.TotalCost != 0.25 {
		t.Errorf("loaded TotalCost = %f, want 0.25", state.TotalCost)
	}
	if state.TotalTurns != 10 {
		t.Errorf("loaded TotalTurns = %d, want 10", state.TotalTurns)
	}

	h, ok := state.History["bd-003"]
	if !ok {
		t.Fatal("expected history for bd-003 after load")
	}
	if h.Status != HistoryCompleted {
		t.Errorf("loaded History status = %q, want %q", h.Status, HistoryCompleted)
	}

	cancel2()
	_ = sink2.Stop()
}

func TestStateSinkAtomicWrite(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "state.json")

	sink := NewStateSink(path)
	sink.SetMinDelay(0)
	events := make(chan Event, 10)

	ctx, cancel := context.WithCancel(context.Background())

	err := sink.Start(ctx, events)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Send event to trigger save
	events <- &DrainStartEvent{
		BaseEvent: NewInternalEvent(EventDrainStart),
		WorkDir:   "/test/dir",
	}

	time.Sleep(50 * time.Millisecond)

	// Verify no .tmp file exists
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("expected .tmp file to be cleaned up")
	}

	// Verify main file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected state file to exist")
	}

	cancel()
	_ = sink.Stop()
}

func TestStateSinkTracksAbandonedBeads(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "state.json")

	sink := NewStateSink(path)
	sink.SetMinDelay(0)
	events := make(chan Event, 10)

	ctx, cancel := context.WithCancel(context.Background())

	err := sink.Start(ctx, events)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Initialize history via iteration start
	events <- &IterationStartEvent{
		BaseEvent: NewInternalEvent(EventIterationStart),
		BeadID:    "bd-004",
		Title:     "Abandoned bead",
		Priority:  1,
		Attempt:   5,
	}

	time.Sleep(50 * time.Millisecond)

	// Send abandoned event
	events <- &BeadAbandonedEvent{
		BaseEvent:   NewInternalEvent(EventBeadAbandoned),
		BeadID:      "bd-004",
		Attempts:    5,
		MaxFailures: 5,
		LastError:   "max retries exceeded",
	}

	time.Sleep(50 * time.Millisecond)

	state := sink.State()
	h, ok := state.History["bd-004"]
	if !ok {
		t.Fatal("expected history for bd-004")
	}
	if h.Status != HistoryAbandoned {
		t.Errorf("History status = %q, want %q", h.Status, HistoryAbandoned)
	}
	if h.LastError != "max retries exceeded" {
		t.Errorf("History LastError = %q, want %q", h.LastError, "max retries exceeded")
	}

	cancel()
	_ = sink.Stop()
}

func TestStateSinkHandlesClosedChannel(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "state.json")

	sink := NewStateSink(path)
	sink.SetMinDelay(0)
	events := make(chan Event, 10)

	ctx := context.Background()

	err := sink.Start(ctx, events)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Send an event to make state dirty
	events <- &DrainStartEvent{
		BaseEvent: NewInternalEvent(EventDrainStart),
		WorkDir:   "/test/dir",
	}

	time.Sleep(50 * time.Millisecond)

	// Close channel
	close(events)

	// Stop should return without hanging and save dirty state
	done := make(chan struct{})
	go func() {
		_ = sink.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Error("Stop timed out after channel close")
	}

	// Verify state was saved
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected state file to be saved on channel close")
	}
}

func TestStateSinkPath(t *testing.T) {
	sink := NewStateSink("/path/to/state.json")
	if sink.Path() != "/path/to/state.json" {
		t.Errorf("Path() = %q, want %q", sink.Path(), "/path/to/state.json")
	}
}

func TestStateSinkTracksDrainStateChanged(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "state.json")

	sink := NewStateSink(path)
	sink.SetMinDelay(0)
	events := make(chan Event, 10)

	ctx, cancel := context.WithCancel(context.Background())

	err := sink.Start(ctx, events)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Send drain start first to initialize
	events <- &DrainStartEvent{
		BaseEvent: NewInternalEvent(EventDrainStart),
		WorkDir:   "/test/dir",
	}

	time.Sleep(50 * time.Millisecond)

	state := sink.State()
	if state.Status != "running" {
		t.Errorf("Status = %q, want %q", state.Status, "running")
	}

	// Send state change to paused
	events <- &DrainStateChangedEvent{
		BaseEvent: NewInternalEvent(EventDrainStateChanged),
		From:      "idle",
		To:        "paused",
	}

	time.Sleep(50 * time.Millisecond)

	state = sink.State()
	if state.Status != "paused" {
		t.Errorf("Status = %q, want %q", state.Status, "paused")
	}

	// Send state change to working
	events <- &DrainStateChangedEvent{
		BaseEvent: NewInternalEvent(EventDrainStateChanged),
		From:      "paused",
		To:        "working",
	}

	time.Sleep(50 * time.Millisecond)

	state = sink.State()
	if state.Status != "working" {
		t.Errorf("Status = %q, want %q", state.Status, "working")
	}

	cancel()
	_ = sink.Stop()
}

func TestStateSinkNoCostDoubleCount(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "state.json")

	sink := NewStateSink(path)
	sink.SetMinDelay(0)
	events := make(chan Event, 10)

	ctx, cancel := context.WithCancel(context.Background())

	err := sink.Start(ctx, events)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Simulate the normal flow where both SessionEndEvent and IterationEndEvent fire
	// with the same cost data - only one should be counted

	// 1. IterationStartEvent sets CurrentBead
	events <- &IterationStartEvent{
		BaseEvent: NewInternalEvent(EventIterationStart),
		BeadID:    "bd-100",
		Title:     "Test bead",
		Priority:  1,
		Attempt:   1,
	}

	time.Sleep(50 * time.Millisecond)

	// 2. SessionEndEvent (from parser) - this fires first in normal flow
	events <- &SessionEndEvent{
		BaseEvent:    NewInternalEvent(EventSessionEnd),
		SessionID:    "sess-100",
		NumTurns:     10,
		DurationMs:   5000,
		TotalCostUSD: 0.50,
	}

	time.Sleep(50 * time.Millisecond)

	// 3. IterationEndEvent (from controller) - this fires second
	events <- &IterationEndEvent{
		BaseEvent:    NewInternalEvent(EventIterationEnd),
		BeadID:       "bd-100",
		SessionID:    "sess-100",
		Success:      true,
		NumTurns:     10,
		DurationMs:   5000,
		TotalCostUSD: 0.50,
	}

	time.Sleep(50 * time.Millisecond)

	state := sink.State()

	// Cost should only be counted once, not twice
	if state.TotalCost != 0.50 {
		t.Errorf("TotalCost = %f, want 0.50 (got double-counted if 1.00)", state.TotalCost)
	}
	if state.TotalTurns != 10 {
		t.Errorf("TotalTurns = %d, want 10 (got double-counted if 20)", state.TotalTurns)
	}

	cancel()
	_ = sink.Stop()
}

func TestStateSinkCostCountedOncePerBead(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "state.json")

	sink := NewStateSink(path)
	sink.SetMinDelay(0)
	events := make(chan Event, 20)

	ctx, cancel := context.WithCancel(context.Background())

	err := sink.Start(ctx, events)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Process two beads with full event sequences

	// Bead 1
	events <- &IterationStartEvent{
		BaseEvent: NewInternalEvent(EventIterationStart),
		BeadID:    "bd-200",
		Title:     "Bead 1",
		Priority:  1,
		Attempt:   1,
	}
	time.Sleep(30 * time.Millisecond)

	events <- &SessionEndEvent{
		BaseEvent:    NewInternalEvent(EventSessionEnd),
		SessionID:    "sess-200",
		NumTurns:     5,
		TotalCostUSD: 0.25,
	}
	time.Sleep(30 * time.Millisecond)

	events <- &IterationEndEvent{
		BaseEvent:    NewInternalEvent(EventIterationEnd),
		BeadID:       "bd-200",
		SessionID:    "sess-200",
		Success:      true,
		NumTurns:     5,
		TotalCostUSD: 0.25,
	}
	time.Sleep(30 * time.Millisecond)

	// Bead 2
	events <- &IterationStartEvent{
		BaseEvent: NewInternalEvent(EventIterationStart),
		BeadID:    "bd-201",
		Title:     "Bead 2",
		Priority:  1,
		Attempt:   1,
	}
	time.Sleep(30 * time.Millisecond)

	events <- &SessionEndEvent{
		BaseEvent:    NewInternalEvent(EventSessionEnd),
		SessionID:    "sess-201",
		NumTurns:     8,
		TotalCostUSD: 0.40,
	}
	time.Sleep(30 * time.Millisecond)

	events <- &IterationEndEvent{
		BaseEvent:    NewInternalEvent(EventIterationEnd),
		BeadID:       "bd-201",
		SessionID:    "sess-201",
		Success:      true,
		NumTurns:     8,
		TotalCostUSD: 0.40,
	}
	time.Sleep(30 * time.Millisecond)

	state := sink.State()

	// Total should be sum of both beads, not double-counted
	expectedCost := 0.25 + 0.40
	if state.TotalCost != expectedCost {
		t.Errorf("TotalCost = %f, want %f", state.TotalCost, expectedCost)
	}
	expectedTurns := 5 + 8
	if state.TotalTurns != expectedTurns {
		t.Errorf("TotalTurns = %d, want %d", state.TotalTurns, expectedTurns)
	}

	cancel()
	_ = sink.Stop()
}

func TestStateSinkVersionMismatch(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "state.json")

	// Write a state file with a different version
	oldState := State{
		Version:    999, // Future incompatible version
		Status:     "running",
		Iteration:  5,
		TotalCost:  1.50,
		TotalTurns: 100,
		History: map[string]*BeadHistory{
			"bd-old": {ID: "bd-old", Status: HistoryCompleted},
		},
	}
	data, err := json.MarshalIndent(oldState, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal old state: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write old state: %v", err)
	}

	// Load with new sink
	sink := NewStateSink(path)
	err = sink.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// State should be fresh (version mismatch triggers reset)
	state := sink.State()
	if state.Version != CurrentStateVersion {
		t.Errorf("Version = %d, want %d", state.Version, CurrentStateVersion)
	}
	if state.Iteration != 0 {
		t.Errorf("Iteration = %d, want 0 (fresh state)", state.Iteration)
	}
	if state.TotalCost != 0 {
		t.Errorf("TotalCost = %f, want 0 (fresh state)", state.TotalCost)
	}
	if len(state.History) != 0 {
		t.Errorf("History has %d entries, want 0 (fresh state)", len(state.History))
	}

	// Backup file should exist
	backupPath := path + ".backup"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Error("expected backup file to exist")
	}
}

func TestStateSinkMissingVersion(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "state.json")

	// Write a state file with no version (version=0)
	oldState := `{
		"status": "running",
		"iteration": 3,
		"total_cost": 0.75,
		"history": {}
	}`
	if err := os.WriteFile(path, []byte(oldState), 0644); err != nil {
		t.Fatalf("failed to write old state: %v", err)
	}

	// Load with new sink
	sink := NewStateSink(path)
	err := sink.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// State should be fresh (missing version triggers reset)
	state := sink.State()
	if state.Version != CurrentStateVersion {
		t.Errorf("Version = %d, want %d", state.Version, CurrentStateVersion)
	}
	if state.Iteration != 0 {
		t.Errorf("Iteration = %d, want 0 (fresh state)", state.Iteration)
	}

	// Backup file should exist
	backupPath := path + ".backup"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Error("expected backup file to exist")
	}
}

func TestStateSinkCorruptedJSON(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "state.json")

	// Write corrupted JSON
	if err := os.WriteFile(path, []byte(`{"version": 1, "status": "run`), 0644); err != nil {
		t.Fatalf("failed to write corrupted state: %v", err)
	}

	// Load with new sink
	sink := NewStateSink(path)
	err := sink.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// State should be fresh (corrupted JSON triggers reset)
	state := sink.State()
	if state.Version != CurrentStateVersion {
		t.Errorf("Version = %d, want %d", state.Version, CurrentStateVersion)
	}
	if state.Status != "" {
		t.Errorf("Status = %q, want empty (fresh state)", state.Status)
	}

	// Backup file should exist
	backupPath := path + ".backup"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Error("expected backup file to exist")
	}
}

func TestStateSinkCompatibleVersion(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "state.json")

	// Write a state file with the current version
	existingState := State{
		Version:    CurrentStateVersion,
		Status:     "paused",
		Iteration:  10,
		TotalCost:  2.50,
		TotalTurns: 200,
		History: map[string]*BeadHistory{
			"bd-existing": {ID: "bd-existing", Status: HistoryCompleted},
		},
	}
	data, err := json.MarshalIndent(existingState, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal state: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write state: %v", err)
	}

	// Load with new sink
	sink := NewStateSink(path)
	err = sink.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// State should be loaded (compatible version)
	state := sink.State()
	if state.Version != CurrentStateVersion {
		t.Errorf("Version = %d, want %d", state.Version, CurrentStateVersion)
	}
	if state.Status != "paused" {
		t.Errorf("Status = %q, want %q", state.Status, "paused")
	}
	if state.Iteration != 10 {
		t.Errorf("Iteration = %d, want 10", state.Iteration)
	}
	if state.TotalCost != 2.50 {
		t.Errorf("TotalCost = %f, want 2.50", state.TotalCost)
	}
	if len(state.History) != 1 {
		t.Errorf("History has %d entries, want 1", len(state.History))
	}

	// No backup file should exist
	backupPath := path + ".backup"
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Error("backup file should not exist for compatible version")
	}
}

func TestStateSinkCostCountedForRetries(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "state.json")

	sink := NewStateSink(path)
	sink.SetMinDelay(0)
	events := make(chan Event, 20)

	ctx, cancel := context.WithCancel(context.Background())

	err := sink.Start(ctx, events)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Simulate a bead that fails and is retried.
	// Each attempt should have its cost counted.

	// Attempt 1: fails
	events <- &IterationStartEvent{
		BaseEvent: NewInternalEvent(EventIterationStart),
		BeadID:    "bd-retry",
		Title:     "Retry bead",
		Priority:  1,
		Attempt:   1,
	}
	time.Sleep(30 * time.Millisecond)

	events <- &IterationEndEvent{
		BaseEvent:    NewInternalEvent(EventIterationEnd),
		BeadID:       "bd-retry",
		SessionID:    "sess-retry-1",
		Success:      false,
		NumTurns:     5,
		DurationMs:   1000,
		TotalCostUSD: 0.10,
		Error:        "tests failed",
	}
	time.Sleep(30 * time.Millisecond)

	// Attempt 2: also fails
	events <- &IterationStartEvent{
		BaseEvent: NewInternalEvent(EventIterationStart),
		BeadID:    "bd-retry",
		Title:     "Retry bead",
		Priority:  1,
		Attempt:   2,
	}
	time.Sleep(30 * time.Millisecond)

	events <- &IterationEndEvent{
		BaseEvent:    NewInternalEvent(EventIterationEnd),
		BeadID:       "bd-retry",
		SessionID:    "sess-retry-2",
		Success:      false,
		NumTurns:     8,
		DurationMs:   2000,
		TotalCostUSD: 0.15,
		Error:        "still failing",
	}
	time.Sleep(30 * time.Millisecond)

	// Attempt 3: succeeds
	events <- &IterationStartEvent{
		BaseEvent: NewInternalEvent(EventIterationStart),
		BeadID:    "bd-retry",
		Title:     "Retry bead",
		Priority:  1,
		Attempt:   3,
	}
	time.Sleep(30 * time.Millisecond)

	events <- &IterationEndEvent{
		BaseEvent:    NewInternalEvent(EventIterationEnd),
		BeadID:       "bd-retry",
		SessionID:    "sess-retry-3",
		Success:      true,
		NumTurns:     10,
		DurationMs:   3000,
		TotalCostUSD: 0.20,
	}
	time.Sleep(30 * time.Millisecond)

	state := sink.State()

	// Total should include all three attempts
	expectedCost := 0.10 + 0.15 + 0.20
	if state.TotalCost != expectedCost {
		t.Errorf("TotalCost = %f, want %f (all attempts should be counted)", state.TotalCost, expectedCost)
	}
	expectedTurns := 5 + 8 + 10
	if state.TotalTurns != expectedTurns {
		t.Errorf("TotalTurns = %d, want %d (all attempts should be counted)", state.TotalTurns, expectedTurns)
	}

	cancel()
	_ = sink.Stop()
}

func TestStateSinkSessionEndCostCountedForRetries(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "state.json")

	sink := NewStateSink(path)
	sink.SetMinDelay(0)
	events := make(chan Event, 30)

	ctx, cancel := context.WithCancel(context.Background())

	err := sink.Start(ctx, events)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Simulate retries where SessionEndEvent fires (normal flow with both events)

	// Attempt 1
	events <- &IterationStartEvent{
		BaseEvent: NewInternalEvent(EventIterationStart),
		BeadID:    "bd-300",
		Title:     "Retry test",
		Priority:  1,
		Attempt:   1,
	}
	time.Sleep(30 * time.Millisecond)

	events <- &SessionEndEvent{
		BaseEvent:    NewInternalEvent(EventSessionEnd),
		SessionID:    "sess-300-1",
		NumTurns:     5,
		TotalCostUSD: 0.10,
	}
	time.Sleep(30 * time.Millisecond)

	events <- &IterationEndEvent{
		BaseEvent:    NewInternalEvent(EventIterationEnd),
		BeadID:       "bd-300",
		SessionID:    "sess-300-1",
		Success:      false,
		NumTurns:     5,
		TotalCostUSD: 0.10,
		Error:        "failed",
	}
	time.Sleep(30 * time.Millisecond)

	// Attempt 2
	events <- &IterationStartEvent{
		BaseEvent: NewInternalEvent(EventIterationStart),
		BeadID:    "bd-300",
		Title:     "Retry test",
		Priority:  1,
		Attempt:   2,
	}
	time.Sleep(30 * time.Millisecond)

	events <- &SessionEndEvent{
		BaseEvent:    NewInternalEvent(EventSessionEnd),
		SessionID:    "sess-300-2",
		NumTurns:     7,
		TotalCostUSD: 0.15,
	}
	time.Sleep(30 * time.Millisecond)

	events <- &IterationEndEvent{
		BaseEvent:    NewInternalEvent(EventIterationEnd),
		BeadID:       "bd-300",
		SessionID:    "sess-300-2",
		Success:      true,
		NumTurns:     7,
		TotalCostUSD: 0.15,
	}
	time.Sleep(30 * time.Millisecond)

	state := sink.State()

	// Both attempts should be counted (total = 0.10 + 0.15)
	expectedCost := 0.10 + 0.15
	if state.TotalCost != expectedCost {
		t.Errorf("TotalCost = %f, want %f", state.TotalCost, expectedCost)
	}
	expectedTurns := 5 + 7
	if state.TotalTurns != expectedTurns {
		t.Errorf("TotalTurns = %d, want %d", state.TotalTurns, expectedTurns)
	}

	cancel()
	_ = sink.Stop()
}
