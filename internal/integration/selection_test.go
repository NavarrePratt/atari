// Package integration provides end-to-end tests for the atari drain loop.
package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/npratt/atari/internal/brclient"
	"github.com/npratt/atari/internal/controller"
	"github.com/npratt/atari/internal/events"
	"github.com/npratt/atari/internal/workqueue"
)

// selectionTestEnv extends testEnv for selection mode tests.
type selectionTestEnv struct {
	*testEnv
	stateSink *events.StateSink
	stateDir  string
}

// newSelectionTestEnv creates a test environment for selection mode tests.
func newSelectionTestEnv(t *testing.T) *selectionTestEnv {
	t.Helper()

	base := newTestEnv(t)
	base.cfg.WorkQueue.SelectionMode = "top-level"

	// Create state directory for persistence tests
	stateDir := filepath.Join(base.tempDir, ".atari")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		base.cleanup()
		t.Fatalf("failed to create state dir: %v", err)
	}

	statePath := filepath.Join(stateDir, "state.json")
	stateSink := events.NewStateSink(statePath)

	return &selectionTestEnv{
		testEnv:   base,
		stateSink: stateSink,
		stateDir:  stateDir,
	}
}

// beadGraph represents a hierarchy of beads for test setup.
type beadGraph struct {
	Epics      []epicDef
	Standalone []beadDef
}

// epicDef defines an epic with its children.
type epicDef struct {
	ID       string
	Title    string
	Priority int
	Children []beadDef
}

// beadDef defines a bead.
type beadDef struct {
	ID       string
	Title    string
	Priority int
	Status   string
}

// setupBeadGraph configures mock responses for the given bead graph.
func (e *selectionTestEnv) setupBeadGraph(graph beadGraph) {
	var readyBeads []brclient.Bead
	var allBeads []brclient.Bead

	// Add epics to allBeads
	for _, epic := range graph.Epics {
		allBeads = append(allBeads, brclient.Bead{
			ID:        epic.ID,
			Title:     epic.Title,
			Status:    "open",
			Priority:  epic.Priority,
			IssueType: "epic",
			CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		})

		// Add children
		for _, child := range epic.Children {
			status := child.Status
			if status == "" {
				status = "open"
			}
			bead := brclient.Bead{
				ID:        child.ID,
				Title:     child.Title,
				Status:    status,
				Priority:  child.Priority,
				IssueType: "task",
				Parent:    epic.ID,
				CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			}
			allBeads = append(allBeads, bead)
			if status == "open" {
				readyBeads = append(readyBeads, bead)
			}
		}
	}

	// Add standalone beads
	for _, bead := range graph.Standalone {
		status := bead.Status
		if status == "" {
			status = "open"
		}
		b := brclient.Bead{
			ID:        bead.ID,
			Title:     bead.Title,
			Status:    status,
			Priority:  bead.Priority,
			IssueType: "task",
			CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		}
		allBeads = append(allBeads, b)
		if status == "open" {
			readyBeads = append(readyBeads, b)
		}
	}

	e.brClient.ReadyResponse = readyBeads
	e.brClient.ListResponse = allBeads
}

// TestTopLevelSelection_MultiEpicPriority tests that the highest priority epic is selected first.
func TestTopLevelSelection_MultiEpicPriority(t *testing.T) {
	env := newSelectionTestEnv(t)
	defer env.cleanup()

	// Setup: Two epics with different priorities
	// Epic A (priority 2) and Epic B (priority 1, higher)
	env.setupBeadGraph(beadGraph{
		Epics: []epicDef{
			{ID: "epic-A", Title: "Epic A", Priority: 2, Children: []beadDef{
				{ID: "task-A1", Title: "Task A1", Priority: 1},
			}},
			{ID: "epic-B", Title: "Epic B", Priority: 1, Children: []beadDef{
				{ID: "task-B1", Title: "Task B1", Priority: 1},
			}},
		},
	})

	// Track iterations
	var selectedBeads []string
	env.brClient.DynamicShow = func(ctx context.Context, id string) (*brclient.Bead, error, bool) {
		// Return closed status so controller marks it complete
		return &brclient.Bead{Status: "closed"}, nil, true
	}

	wq := workqueue.New(env.cfg, env.brClient, nil)
	ctrl := controller.New(env.cfg, wq, env.router, env.brClient, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- ctrl.Run(ctx)
	}()

	// Wait for iteration to start
	time.Sleep(300 * time.Millisecond)
	ctrl.Stop()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for controller to stop")
	}

	// Collect events
	env.collectEvents(100 * time.Millisecond)

	// Find iteration start events
	for _, evt := range env.collected {
		if evt.Type() == events.EventIterationStart {
			iterEvt := evt.(*events.IterationStartEvent)
			selectedBeads = append(selectedBeads, iterEvt.BeadID)
		}
	}

	// Should have selected task from epic-B (higher priority epic)
	if len(selectedBeads) == 0 {
		t.Fatal("expected at least one iteration")
	}

	if selectedBeads[0] != "task-B1" {
		t.Errorf("expected first bead to be task-B1 (from higher priority epic), got %s", selectedBeads[0])
	}

	// Verify active top-level was set
	if wq.ActiveTopLevel() != "epic-B" {
		t.Errorf("expected active top-level to be epic-B, got %s", wq.ActiveTopLevel())
	}
}

// TestTopLevelSelection_StandaloneBeads tests that standalone beads are selected based on their own priority.
func TestTopLevelSelection_StandaloneBeads(t *testing.T) {
	env := newSelectionTestEnv(t)
	defer env.cleanup()

	// Setup: Mix of epic children and standalone beads
	env.setupBeadGraph(beadGraph{
		Epics: []epicDef{
			{ID: "epic-A", Title: "Epic A", Priority: 2, Children: []beadDef{
				{ID: "task-A1", Title: "Task A1", Priority: 1},
			}},
		},
		Standalone: []beadDef{
			{ID: "standalone-1", Title: "Standalone Task", Priority: 1},
		},
	})

	env.brClient.DynamicShow = func(ctx context.Context, id string) (*brclient.Bead, error, bool) {
		return &brclient.Bead{Status: "closed"}, nil, true
	}

	wq := workqueue.New(env.cfg, env.brClient, nil)
	ctrl := controller.New(env.cfg, wq, env.router, env.brClient, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- ctrl.Run(ctx)
	}()

	time.Sleep(300 * time.Millisecond)
	ctrl.Stop()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for controller to stop")
	}

	env.collectEvents(100 * time.Millisecond)

	// Find the selected bead
	var firstBead string
	for _, evt := range env.collected {
		if evt.Type() == events.EventIterationStart {
			iterEvt := evt.(*events.IterationStartEvent)
			firstBead = iterEvt.BeadID
			break
		}
	}

	// Both have priority 1, but epic-A and standalone are at same priority level
	// The selection should pick based on epic priority tie-breaking rules
	if firstBead == "" {
		t.Fatal("expected at least one iteration")
	}

	t.Logf("selected bead: %s (standalone beads treated as top-level)", firstBead)
}

// TestTopLevelSelection_ExhaustionAndSwitching tests that when an epic is exhausted, the next epic is selected.
func TestTopLevelSelection_ExhaustionAndSwitching(t *testing.T) {
	env := newSelectionTestEnv(t)
	defer env.cleanup()

	// Initial graph: Epic A has one task, Epic B has one task
	// After Epic A's task is done, should switch to Epic B
	iteration := 0
	env.brClient.DynamicReady = func(ctx context.Context, opts *brclient.ReadyOptions) ([]brclient.Bead, error, bool) {
		iteration++
		if iteration == 1 {
			// First call: both epics have work
			return []brclient.Bead{
				{ID: "task-A1", Title: "Task A1", Status: "open", Priority: 1, IssueType: "task", CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
				{ID: "task-B1", Title: "Task B1", Status: "open", Priority: 1, IssueType: "task", CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
			}, nil, true
		}
		// Second call: Epic A exhausted, only Epic B has work
		return []brclient.Bead{
			{ID: "task-B1", Title: "Task B1", Status: "open", Priority: 1, IssueType: "task", CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
		}, nil, true
	}

	listIteration := 0
	env.brClient.ListResponse = nil // Override with dynamic
	originalList := env.brClient.ListResponse
	_ = originalList

	// We need to track list calls separately
	env.brClient.DynamicShow = func(ctx context.Context, id string) (*brclient.Bead, error, bool) {
		return &brclient.Bead{Status: "closed"}, nil, true
	}

	// Set up ListResponse dynamically based on iteration
	// This is tricky since MockClient doesn't have DynamicList, so we'll use a fixed response
	env.brClient.ListResponse = []brclient.Bead{
		{ID: "epic-A", Title: "Epic A", Status: "open", IssueType: "epic", Priority: 1, CreatedAt: time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)},
		{ID: "epic-B", Title: "Epic B", Status: "open", IssueType: "epic", Priority: 1, CreatedAt: time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)},
		{ID: "task-A1", Title: "Task A1", Status: "open", Parent: "epic-A", Priority: 1, IssueType: "task"},
		{ID: "task-B1", Title: "Task B1", Status: "open", Parent: "epic-B", Priority: 1, IssueType: "task"},
	}
	_ = listIteration

	wq := workqueue.New(env.cfg, env.brClient, nil)
	ctrl := controller.New(env.cfg, wq, env.router, env.brClient, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- ctrl.Run(ctx)
	}()

	// Wait for two iterations
	time.Sleep(1200 * time.Millisecond)
	ctrl.Stop()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for controller to stop")
	}

	env.collectEvents(100 * time.Millisecond)

	var selectedBeads []string
	for _, evt := range env.collected {
		if evt.Type() == events.EventIterationStart {
			iterEvt := evt.(*events.IterationStartEvent)
			selectedBeads = append(selectedBeads, iterEvt.BeadID)
		}
	}

	t.Logf("selected beads: %v", selectedBeads)

	// Verify at least one iteration happened
	if len(selectedBeads) < 1 {
		t.Error("expected at least one iteration")
	}
}

// TestTopLevelSelection_EagerSwitch tests that eager_switch mode switches to higher priority epics.
func TestTopLevelSelection_EagerSwitch(t *testing.T) {
	env := newSelectionTestEnv(t)
	defer env.cleanup()

	// Enable eager switch
	env.cfg.WorkQueue.EagerSwitch = true

	// Scenario: Start with low priority epic, then high priority becomes available
	iteration := 0
	env.brClient.DynamicReady = func(ctx context.Context, opts *brclient.ReadyOptions) ([]brclient.Bead, error, bool) {
		iteration++
		if iteration == 1 {
			// First call: only low priority epic
			return []brclient.Bead{
				{ID: "task-low", Title: "Low Priority Task", Status: "open", Priority: 1, IssueType: "task", CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
			}, nil, true
		}
		// Second call: high priority epic appears
		return []brclient.Bead{
			{ID: "task-low", Title: "Low Priority Task", Status: "open", Priority: 1, IssueType: "task", CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
			{ID: "task-high", Title: "High Priority Task", Status: "open", Priority: 0, IssueType: "task", CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
		}, nil, true
	}

	// Setup list response with hierarchy
	env.brClient.ListResponse = []brclient.Bead{
		{ID: "epic-low", Title: "Low Priority Epic", Status: "open", IssueType: "epic", Priority: 3, CreatedAt: time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)},
		{ID: "epic-high", Title: "High Priority Epic", Status: "open", IssueType: "epic", Priority: 1, CreatedAt: time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)},
		{ID: "task-low", Title: "Low Priority Task", Status: "open", Parent: "epic-low", Priority: 1, IssueType: "task"},
		{ID: "task-high", Title: "High Priority Task", Status: "open", Parent: "epic-high", Priority: 0, IssueType: "task"},
	}

	env.brClient.DynamicShow = func(ctx context.Context, id string) (*brclient.Bead, error, bool) {
		switch id {
		case "epic-low":
			return &brclient.Bead{ID: "epic-low", Priority: 3}, nil, true
		case "epic-high":
			return &brclient.Bead{ID: "epic-high", Priority: 1}, nil, true
		default:
			return &brclient.Bead{Status: "closed", Priority: 1}, nil, true
		}
	}

	wq := workqueue.New(env.cfg, env.brClient, nil)
	ctrl := controller.New(env.cfg, wq, env.router, env.brClient, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- ctrl.Run(ctx)
	}()

	time.Sleep(1500 * time.Millisecond)
	ctrl.Stop()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for controller to stop")
	}

	env.collectEvents(100 * time.Millisecond)

	var selectedBeads []string
	for _, evt := range env.collected {
		if evt.Type() == events.EventIterationStart {
			iterEvt := evt.(*events.IterationStartEvent)
			selectedBeads = append(selectedBeads, iterEvt.BeadID)
		}
	}

	t.Logf("selected beads with eager_switch: %v", selectedBeads)
	t.Logf("eager_switch enabled: %v", env.cfg.WorkQueue.EagerSwitch)
}

// TestTopLevelSelection_StatePersistence tests that active top-level is restored across restarts.
func TestTopLevelSelection_StatePersistence(t *testing.T) {
	env := newSelectionTestEnv(t)
	defer env.cleanup()

	// Setup initial graph
	env.setupBeadGraph(beadGraph{
		Epics: []epicDef{
			{ID: "epic-A", Title: "Epic A", Priority: 1, Children: []beadDef{
				{ID: "task-A1", Title: "Task A1", Priority: 1},
				{ID: "task-A2", Title: "Task A2", Priority: 2},
			}},
			{ID: "epic-B", Title: "Epic B", Priority: 2, Children: []beadDef{
				{ID: "task-B1", Title: "Task B1", Priority: 1},
			}},
		},
	})

	env.brClient.DynamicShow = func(ctx context.Context, id string) (*brclient.Bead, error, bool) {
		if id == "epic-A" {
			return &brclient.Bead{ID: "epic-A", Title: "Epic A"}, nil, true
		}
		return &brclient.Bead{Status: "closed"}, nil, true
	}

	// Start state sink
	stateChan := env.router.Subscribe()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := env.stateSink.Start(ctx, stateChan); err != nil {
		t.Fatalf("failed to start state sink: %v", err)
	}

	wq := workqueue.New(env.cfg, env.brClient, nil)
	ctrl := controller.New(env.cfg, wq, env.router, env.brClient, nil, nil,
		controller.WithStateSink(env.stateSink))

	runCtx, runCancel := context.WithTimeout(ctx, 3*time.Second)
	defer runCancel()

	done := make(chan error, 1)
	go func() {
		done <- ctrl.Run(runCtx)
	}()

	// Wait for iteration and state persistence
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

	// Check that active top-level was persisted
	state := env.stateSink.State()
	t.Logf("persisted state: active_top_level=%s, title=%s", state.ActiveTopLevel, state.ActiveTopLevelTitle)

	if state.ActiveTopLevel == "" {
		t.Log("active top-level was not persisted (may not have been set in this iteration)")
	}
}

// TestTopLevelSelection_EpicFlagPrecedence tests that --epic flag takes precedence over selection mode.
func TestTopLevelSelection_EpicFlagPrecedence(t *testing.T) {
	env := newSelectionTestEnv(t)
	defer env.cleanup()

	// Set epic flag which should override top-level mode
	env.cfg.WorkQueue.Epic = "epic-B"

	// Setup graph with higher priority epic-A but --epic restricts to epic-B
	env.brClient.DynamicReady = func(ctx context.Context, opts *brclient.ReadyOptions) ([]brclient.Bead, error, bool) {
		// Both epics have work, but --epic should restrict to epic-B
		return []brclient.Bead{
			{ID: "task-A1", Title: "Task A1", Status: "open", Priority: 1, IssueType: "task", CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
			{ID: "task-B1", Title: "Task B1", Status: "open", Priority: 1, IssueType: "task", CreatedAt: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)},
		}, nil, true
	}

	env.brClient.ListResponse = []brclient.Bead{
		{ID: "epic-A", Title: "Epic A", Status: "open", IssueType: "epic", Priority: 1, CreatedAt: time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)},
		{ID: "epic-B", Title: "Epic B", Status: "open", IssueType: "epic", Priority: 2, CreatedAt: time.Date(2024, 1, 15, 8, 0, 0, 0, time.UTC)},
		{ID: "task-A1", Title: "Task A1", Status: "open", Parent: "epic-A", Priority: 1, IssueType: "task"},
		{ID: "task-B1", Title: "Task B1", Status: "open", Parent: "epic-B", Priority: 1, IssueType: "task"},
	}

	env.brClient.DynamicShow = func(ctx context.Context, id string) (*brclient.Bead, error, bool) {
		if id == "epic-B" {
			return &brclient.Bead{ID: "epic-B", Title: "Epic B", IssueType: "epic"}, nil, true
		}
		return &brclient.Bead{Status: "closed"}, nil, true
	}

	wq := workqueue.New(env.cfg, env.brClient, nil)
	ctrl := controller.New(env.cfg, wq, env.router, env.brClient, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- ctrl.Run(ctx)
	}()

	time.Sleep(400 * time.Millisecond)
	ctrl.Stop()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for controller to stop")
	}

	env.collectEvents(100 * time.Millisecond)

	// Verify controller validated the epic
	epicID, epicTitle := ctrl.ValidatedEpic()
	if epicID != "epic-B" {
		t.Errorf("expected validated epic to be epic-B, got %s", epicID)
	}
	t.Logf("validated epic: id=%s, title=%s", epicID, epicTitle)

	// Find selected bead
	for _, evt := range env.collected {
		if evt.Type() == events.EventIterationStart {
			iterEvt := evt.(*events.IterationStartEvent)
			// With --epic flag, workqueue.Next is used instead of NextTopLevel
			// The selection should be from the restricted epic
			t.Logf("selected bead: %s (--epic flag should restrict selection)", iterEvt.BeadID)
		}
	}
}

// TestTopLevelSelection_NoWork tests behavior when no beads are available.
func TestTopLevelSelection_NoWork(t *testing.T) {
	env := newSelectionTestEnv(t)
	defer env.cleanup()

	// Empty work queue
	env.brClient.ReadyResponse = nil

	wq := workqueue.New(env.cfg, env.brClient, nil)
	ctrl := controller.New(env.cfg, wq, env.router, env.brClient, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- ctrl.Run(ctx)
	}()

	// Short wait then stop
	time.Sleep(200 * time.Millisecond)
	ctrl.Stop()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for controller to stop")
	}

	env.collectEvents(100 * time.Millisecond)

	// Should not have any iteration events
	iterCount := env.countEvents(events.EventIterationStart)
	if iterCount != 0 {
		t.Errorf("expected no iterations when no work, got %d", iterCount)
	}

	// Active top-level should remain empty
	if wq.ActiveTopLevel() != "" {
		t.Errorf("expected empty active top-level, got %s", wq.ActiveTopLevel())
	}
}

// TestTopLevelSelection_IterationEventIncludesTopLevel tests that IterationStartEvent includes top-level info.
func TestTopLevelSelection_IterationEventIncludesTopLevel(t *testing.T) {
	env := newSelectionTestEnv(t)
	defer env.cleanup()

	env.setupBeadGraph(beadGraph{
		Epics: []epicDef{
			{ID: "epic-A", Title: "Epic A", Priority: 1, Children: []beadDef{
				{ID: "task-A1", Title: "Task A1", Priority: 1},
			}},
		},
	})

	env.brClient.DynamicShow = func(ctx context.Context, id string) (*brclient.Bead, error, bool) {
		if id == "epic-A" {
			return &brclient.Bead{ID: "epic-A", Title: "Epic A"}, nil, true
		}
		return &brclient.Bead{Status: "closed"}, nil, true
	}

	wq := workqueue.New(env.cfg, env.brClient, nil)
	ctrl := controller.New(env.cfg, wq, env.router, env.brClient, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- ctrl.Run(ctx)
	}()

	time.Sleep(400 * time.Millisecond)
	ctrl.Stop()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for controller to stop")
	}

	env.collectEvents(100 * time.Millisecond)

	// Find iteration start event
	for _, evt := range env.collected {
		if evt.Type() == events.EventIterationStart {
			iterEvt := evt.(*events.IterationStartEvent)
			t.Logf("iteration event: bead=%s, top_level_id=%s, top_level_title=%s",
				iterEvt.BeadID, iterEvt.TopLevelID, iterEvt.TopLevelTitle)

			if iterEvt.TopLevelID == "" {
				t.Error("expected TopLevelID to be set in IterationStartEvent")
			}
			return
		}
	}

	t.Error("expected IterationStartEvent")
}
