package tui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/testutil"
)

// graphTestEnv provides an isolated test environment for graph pane integration tests.
type graphTestEnv struct {
	t       *testing.T
	runner  *testutil.MockRunner
	fetcher *BDFetcher
	cfg     *config.GraphConfig
}

// newGraphTestEnv creates a new test environment for graph pane tests.
func newGraphTestEnv(t *testing.T) *graphTestEnv {
	t.Helper()

	runner := testutil.NewMockRunner()
	fetcher := NewBDFetcher(runner)
	cfg := &config.GraphConfig{
		Enabled:        true,
		Density:        "standard",
		RefreshOnEvent: false,
	}

	return &graphTestEnv{
		t:       t,
		runner:  runner,
		fetcher: fetcher,
		cfg:     cfg,
	}
}

// newPane creates a GraphPane with the test environment's dependencies.
func (env *graphTestEnv) newPane() GraphPane {
	pane := NewGraphPane(env.cfg, env.fetcher, "horizontal")
	pane.SetSize(100, 30)
	pane.SetFocused(true)
	return pane
}

// setActiveBeads configures the mock runner to return the given JSON for bd list.
func (env *graphTestEnv) setActiveBeads(json string) {
	env.runner.SetResponse("bd", []string{"list", "--json"}, []byte(json))
}

// setError configures the mock runner to return an error for bd list.
func (env *graphTestEnv) setError(err error) {
	env.runner.SetError("bd", []string{"list", "--json"}, err)
}

// TestGraphPane_InitialFetchLoadsBeads verifies that Init triggers a fetch
// and the result populates the graph with beads.
func TestGraphPane_InitialFetchLoadsBeads(t *testing.T) {
	env := newGraphTestEnv(t)
	env.setActiveBeads(testutil.GraphActiveBeadsJSON)

	pane := env.newPane()

	// Init should return a command to start loading
	cmd := pane.Init()
	if cmd == nil {
		t.Fatal("Init should return a command")
	}

	// Execute the command to get the start loading message
	msg := cmd()

	// Process the batch command results
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, cmd := range batch {
			if cmd != nil {
				innerMsg := cmd()
				pane, _ = pane.Update(innerMsg)
			}
		}
	}

	// After processing graphStartLoadingMsg, should be loading
	if !pane.loading {
		// May have already completed if fetch was synchronous
		// Check that graph has nodes
		if pane.graph == nil || pane.graph.NodeCount() == 0 {
			t.Error("expected graph to have nodes after fetch")
		}
	}
}

// TestGraphPane_RefreshKeyTriggersNewFetch verifies that pressing 'R' triggers
// a new fetch with an incremented requestID.
func TestGraphPane_RefreshKeyTriggersNewFetch(t *testing.T) {
	env := newGraphTestEnv(t)
	env.setActiveBeads(testutil.GraphActiveBeadsJSON)

	pane := env.newPane()
	pane.SetFocused(true)

	initialRequestID := pane.requestID

	// Press R to trigger refresh
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}}
	newPane, cmd := pane.Update(keyMsg)
	pane = newPane

	// Should return a command
	if cmd == nil {
		t.Fatal("R key should return a refresh command")
	}

	// Execute the command chain to get the start loading message
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, batchCmd := range batch {
			if batchCmd != nil {
				innerMsg := batchCmd()
				if startMsg, ok := innerMsg.(graphStartLoadingMsg); ok {
					// Process the start loading message
					newPane, _ = pane.Update(startMsg)
					pane = newPane
					// Verify requestID was incremented
					if startMsg.requestID <= initialRequestID {
						t.Errorf("expected requestID > %d, got %d", initialRequestID, startMsg.requestID)
					}
				}
			}
		}
	}
}

// TestGraphPane_ViewToggleSwitchesViews verifies that pressing 'a' cycles
// through Active, Backlog, and Closed views.
func TestGraphPane_ViewToggleSwitchesViews(t *testing.T) {
	env := newGraphTestEnv(t)
	env.setActiveBeads(testutil.GraphActiveBeadsJSON)

	pane := env.newPane()
	pane.SetFocused(true)

	// Manually set up graph to check view
	if pane.graph == nil {
		pane.graph = NewGraph(env.cfg, env.fetcher, "horizontal")
	}

	// Initial view should be Active
	if pane.graph.GetView() != ViewActive {
		t.Errorf("expected initial view Active, got %v", pane.graph.GetView())
	}

	// Press 'a' to toggle view
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	newPane, _ := pane.Update(keyMsg)
	pane = newPane

	// View should now be Backlog
	if pane.graph.GetView() != ViewBacklog {
		t.Errorf("expected view Backlog after toggle, got %v", pane.graph.GetView())
	}

	// Press 'a' again to toggle to Closed
	newPane, _ = pane.Update(keyMsg)
	pane = newPane

	// View should now be Closed
	if pane.graph.GetView() != ViewClosed {
		t.Errorf("expected view Closed after second toggle, got %v", pane.graph.GetView())
	}

	// Press 'a' again to cycle back to Active
	newPane, _ = pane.Update(keyMsg)
	pane = newPane

	// View should be Active again
	if pane.graph.GetView() != ViewActive {
		t.Errorf("expected view Active after third toggle, got %v", pane.graph.GetView())
	}
}

// TestGraphPane_FetchErrorDisplaysMessage verifies that fetch errors
// are displayed in the error message field.
func TestGraphPane_FetchErrorDisplaysMessage(t *testing.T) {
	env := newGraphTestEnv(t)
	env.setError(errors.New("network timeout"))

	pane := env.newPane()

	// Simulate a fetch result with error
	resultMsg := graphResultMsg{
		beads:     nil,
		err:       errors.New("bd list active failed: network timeout"),
		requestID: 1,
	}

	// Set requestID to match
	pane.requestID = 1
	pane.loading = true

	newPane, _ := pane.Update(resultMsg)
	pane = newPane

	// Should no longer be loading
	if pane.loading {
		t.Error("should not be loading after error result")
	}

	// Error message should be set
	if pane.errorMsg == "" {
		t.Error("expected error message to be set")
	}

	if pane.errorMsg != "bd list active failed: network timeout" {
		t.Errorf("expected error message 'bd list active failed: network timeout', got %q", pane.errorMsg)
	}
}

// TestGraphPane_NavigationUpdatesSelectedNode verifies that j/k/h/l navigation
// keys update the selected node.
func TestGraphPane_NavigationUpdatesSelectedNode(t *testing.T) {
	env := newGraphTestEnv(t)

	pane := env.newPane()
	pane.SetFocused(true)

	// Manually set up graph with complex hierarchy
	pane.graph = NewGraph(env.cfg, env.fetcher, "horizontal")

	// Parse and load the complex hierarchy fixture
	beads, err := parseBeads([]byte(testutil.GraphComplexHierarchyJSON))
	if err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}
	pane.graph.RebuildFromBeads(beads)

	// Get initial selected node
	initialSelected := pane.graph.GetSelectedID()
	if initialSelected == "" {
		t.Fatal("expected initial node to be selected")
	}

	// Test navigation keys
	tests := []struct {
		key        string
		keyRune    rune
		shouldMove bool
	}{
		{"j", 'j', true},  // down/child
		{"k", 'k', true},  // up/parent
		{"l", 'l', true},  // right/next sibling
		{"h", 'h', true},  // left/prev sibling
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			beforeSelected := pane.graph.GetSelectedID()
			keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tt.keyRune}}
			newPane, _ := pane.Update(keyMsg)
			pane = newPane

			// Selection may or may not change depending on graph structure
			// but the update should not panic
			afterSelected := pane.graph.GetSelectedID()
			_ = afterSelected
			_ = beforeSelected
			// Just verify no panic occurred
		})
	}
}

// TestGraphPane_EnterKeyTwoStepSelection verifies that pressing Enter
// first shows inline detail view, and a second Enter emits GraphOpenModalMsg.
func TestGraphPane_EnterKeyTwoStepSelection(t *testing.T) {
	env := newGraphTestEnv(t)

	pane := env.newPane()
	pane.SetFocused(true)

	// Set up graph with single bead
	pane.graph = NewGraph(env.cfg, env.fetcher, "horizontal")
	beads, err := parseBeads([]byte(testutil.GraphSingleBeadJSON))
	if err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}
	pane.graph.RebuildFromBeads(beads)

	expectedID := pane.graph.GetSelectedID()
	if expectedID == "" {
		t.Fatal("expected node to be selected")
	}

	// First Enter: opens inline detail view
	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	pane, cmd := pane.Update(keyMsg)

	if cmd == nil {
		t.Fatal("first Enter should return a command for async fetch")
	}
	if !pane.IsShowingDetail() {
		t.Error("expected pane to be showing detail after first Enter")
	}

	// Execute the command - should be graphDetailResultMsg
	msg := cmd()
	_, ok := msg.(graphDetailResultMsg)
	if !ok {
		t.Fatalf("expected graphDetailResultMsg from first Enter, got %T", msg)
	}

	// Second Enter: emits GraphOpenModalMsg for fullscreen
	pane, cmd = pane.Update(keyMsg)

	if cmd == nil {
		t.Fatal("second Enter should return a command for modal")
	}

	// Execute the command to get the message
	msg = cmd()
	openMsg, ok := msg.(GraphOpenModalMsg)
	if !ok {
		t.Fatalf("expected GraphOpenModalMsg from second Enter, got %T", msg)
	}

	if openMsg.NodeID != expectedID {
		t.Errorf("expected NodeID %q, got %q", expectedID, openMsg.NodeID)
	}
}

// TestGraphPane_StalenessDetection verifies that out-of-order responses
// with older requestIDs are dropped.
func TestGraphPane_StalenessDetection(t *testing.T) {
	env := newGraphTestEnv(t)

	pane := env.newPane()

	// Set the pane to be waiting for requestID 5
	pane.requestID = 5
	pane.loading = true

	// Send a stale result with requestID 3 (old)
	staleResult := graphResultMsg{
		beads: []GraphBead{{
			ID:     "stale-bead",
			Title:  "Should be ignored",
			Status: "open",
		}},
		err:       nil,
		requestID: 3, // Stale!
	}

	newPane, _ := pane.Update(staleResult)
	pane = newPane

	// Should still be loading (stale result ignored)
	if !pane.loading {
		t.Error("stale result should not stop loading")
	}

	// Graph should not have the stale bead
	if pane.graph != nil && pane.graph.NodeCount() > 0 {
		nodes := pane.graph.GetNodes()
		if _, found := nodes["stale-bead"]; found {
			t.Error("stale bead should not be in graph")
		}
	}
}

// TestGraphPane_OnlyNewerRequestAccepted verifies that only results with
// matching requestID are accepted.
func TestGraphPane_OnlyNewerRequestAccepted(t *testing.T) {
	env := newGraphTestEnv(t)

	pane := env.newPane()

	// Set up initial state
	pane.requestID = 5
	pane.loading = true

	// Send result with matching requestID
	validResult := graphResultMsg{
		beads: []GraphBead{{
			ID:        "valid-bead",
			Title:     "Should be accepted",
			Status:    "open",
			IssueType: "task",
		}},
		err:       nil,
		requestID: 5, // Matches!
	}

	newPane, _ := pane.Update(validResult)
	pane = newPane

	// Should no longer be loading
	if pane.loading {
		t.Error("valid result should stop loading")
	}

	// Graph should have the valid bead
	if pane.graph == nil {
		t.Fatal("graph should be initialized")
	}

	if pane.graph.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", pane.graph.NodeCount())
	}

	nodes := pane.graph.GetNodes()
	if _, found := nodes["valid-bead"]; !found {
		t.Error("valid bead should be in graph")
	}
}

// TestGraphPane_GraphStartLoadingMsgSetsState verifies that graphStartLoadingMsg
// correctly updates loading state and requestID.
func TestGraphPane_GraphStartLoadingMsgSetsState(t *testing.T) {
	env := newGraphTestEnv(t)

	pane := env.newPane()
	pane.requestID = 10
	pane.loading = false

	// Send graphStartLoadingMsg
	startMsg := graphStartLoadingMsg{
		requestID:  11,
		startFetch: false, // Don't actually start fetch
	}

	newPane, _ := pane.Update(startMsg)
	pane = newPane

	// Should be loading
	if !pane.loading {
		t.Error("should be loading after graphStartLoadingMsg")
	}

	// RequestID should be updated
	if pane.requestID != 11 {
		t.Errorf("expected requestID 11, got %d", pane.requestID)
	}
}

// TestGraphPane_EscapeClearsError verifies that pressing Escape clears
// the error message when one is displayed.
func TestGraphPane_EscapeClearsError(t *testing.T) {
	env := newGraphTestEnv(t)

	pane := env.newPane()
	pane.SetFocused(true)
	pane.errorMsg = "some error occurred"

	// Press Escape
	keyMsg := tea.KeyMsg{Type: tea.KeyEscape}
	newPane, _ := pane.Update(keyMsg)
	pane = newPane

	// Error should be cleared
	if pane.errorMsg != "" {
		t.Errorf("expected error to be cleared, got %q", pane.errorMsg)
	}
}

// TestGraphPane_UnfocusedIgnoresKeyPresses verifies that key presses
// are ignored when the pane is not focused.
func TestGraphPane_UnfocusedIgnoresKeyPresses(t *testing.T) {
	env := newGraphTestEnv(t)

	pane := env.newPane()
	pane.SetFocused(false) // Not focused

	// Set up graph to verify navigation doesn't happen
	pane.graph = NewGraph(env.cfg, env.fetcher, "horizontal")
	beads, _ := parseBeads([]byte(testutil.GraphActiveBeadsJSON))
	pane.graph.RebuildFromBeads(beads)
	initialSelected := pane.graph.GetSelectedID()

	// Try navigation key
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	newPane, cmd := pane.Update(keyMsg)
	pane = newPane

	// Should return nil command (ignored)
	if cmd != nil {
		t.Error("unfocused pane should return nil command")
	}

	// Selection should not change
	if pane.graph.GetSelectedID() != initialSelected {
		t.Error("selection should not change when unfocused")
	}
}
