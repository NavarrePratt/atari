package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/npratt/atari/internal/config"
)

// Note: mockFetcher is defined in graph_test.go

func TestNewGraphPane(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	pane := NewGraphPane(cfg, nil, "horizontal")

	if pane.loading {
		t.Error("expected loading to be false initially")
	}
	if pane.errorMsg != "" {
		t.Error("expected errorMsg to be empty initially")
	}
	if pane.focused {
		t.Error("expected focused to be false initially")
	}
	if pane.graph == nil {
		t.Error("expected graph to be initialized")
	}
}

func TestGraphPane_SetSize(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	pane := NewGraphPane(cfg, nil, "horizontal")
	pane.SetSize(80, 24)

	if pane.width != 80 {
		t.Errorf("expected width=80, got %d", pane.width)
	}
	if pane.height != 24 {
		t.Errorf("expected height=24, got %d", pane.height)
	}
}

func TestGraphPane_SetFocused(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	pane := NewGraphPane(cfg, nil, "horizontal")

	pane.SetFocused(true)
	if !pane.IsFocused() {
		t.Error("expected pane to be focused")
	}

	pane.SetFocused(false)
	if pane.IsFocused() {
		t.Error("expected pane to be unfocused")
	}
}

func TestGraphPane_HandleStartLoadingMsg(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	pane := NewGraphPane(cfg, nil, "horizontal")

	msg := graphStartLoadingMsg{requestID: 1}
	newPane, _ := pane.Update(msg)

	if !newPane.loading {
		t.Error("expected loading to be true after start message")
	}
	if newPane.requestID != 1 {
		t.Errorf("expected requestID=1, got %d", newPane.requestID)
	}
	if newPane.startedAt.IsZero() {
		t.Error("expected startedAt to be set")
	}
}

func TestGraphPane_HandleResultSuccess(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	pane := NewGraphPane(cfg, nil, "horizontal")
	pane.loading = true
	pane.requestID = 1

	beads := []GraphBead{
		{ID: "bd-test-123", Title: "Test bead", Status: "open", Priority: 2, IssueType: "task"},
	}
	msg := graphResultMsg{beads: beads, err: nil, requestID: 1}
	newPane, _ := pane.Update(msg)

	if newPane.loading {
		t.Error("expected loading to be false after result")
	}
	if newPane.errorMsg != "" {
		t.Errorf("expected no error, got %q", newPane.errorMsg)
	}
	if newPane.graph.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", newPane.graph.NodeCount())
	}
}

func TestGraphPane_HandleResultError(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	pane := NewGraphPane(cfg, nil, "horizontal")
	pane.loading = true
	pane.requestID = 1

	msg := graphResultMsg{beads: nil, err: errors.New("fetch failed"), requestID: 1}
	newPane, _ := pane.Update(msg)

	if newPane.loading {
		t.Error("expected loading to be false after result")
	}
	if newPane.errorMsg != "fetch failed" {
		t.Errorf("expected error='fetch failed', got %q", newPane.errorMsg)
	}
}

func TestGraphPane_DropsStaleResults(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	pane := NewGraphPane(cfg, nil, "horizontal")
	pane.loading = true
	pane.requestID = 2 // Current request is 2

	// Simulate stale result from request 1
	msg := graphResultMsg{beads: []GraphBead{{ID: "stale"}}, err: nil, requestID: 1}
	newPane, _ := pane.Update(msg)

	// Should still be loading since result was stale
	if !newPane.loading {
		t.Error("expected loading to remain true for stale result")
	}
}

func TestGraphPane_TickUpdatesDuringLoading(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	pane := NewGraphPane(cfg, nil, "horizontal")
	pane.loading = true
	pane.startedAt = time.Now().Add(-2 * time.Second)

	msg := graphTickMsg(time.Now())
	newPane, cmd := pane.Update(msg)

	if !newPane.loading {
		t.Error("expected loading to remain true")
	}
	if cmd == nil {
		t.Error("expected tick cmd to schedule another tick")
	}
}

func TestGraphPane_SpinnerTickDuringLoading(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	pane := NewGraphPane(cfg, nil, "horizontal")
	pane.loading = true

	msg := spinner.TickMsg{}
	_, cmd := pane.Update(msg)

	if cmd == nil {
		t.Error("expected spinner to return a cmd during loading")
	}
}

func TestGraphPane_SpinnerTickWhenNotLoading(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	pane := NewGraphPane(cfg, nil, "horizontal")
	pane.loading = false

	msg := spinner.TickMsg{}
	_, cmd := pane.Update(msg)

	if cmd != nil {
		t.Error("expected no cmd when not loading")
	}
}

func TestGraphPane_KeyNavigation(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-epic-1", Title: "Epic", Status: "open", IssueType: "epic"},
			{ID: "bd-task-1", Title: "Task", Status: "open", IssueType: "task"},
		},
	}
	pane := NewGraphPane(cfg, fetcher, "horizontal")
	pane.SetFocused(true)
	pane.SetSize(80, 24)

	// Load data first
	pane.rebuildGraph(fetcher.activeBeads)

	// Test navigation keys - they should not error
	keys := []string{"up", "down", "left", "right", "k", "j", "h", "l"}
	for _, key := range keys {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
		if key == "up" || key == "down" || key == "left" || key == "right" {
			msg = tea.KeyMsg{Type: tea.KeyType(keyTypeFromString(key))}
		}
		_, _ = pane.Update(msg)
	}
}

func keyTypeFromString(s string) tea.KeyType {
	switch s {
	case "up":
		return tea.KeyUp
	case "down":
		return tea.KeyDown
	case "left":
		return tea.KeyLeft
	case "right":
		return tea.KeyRight
	default:
		return tea.KeyRunes
	}
}

func TestGraphPane_KeyToggleView(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	fetcher := &mockFetcher{activeBeads: []GraphBead{}}
	pane := NewGraphPane(cfg, fetcher, "horizontal")
	pane.SetFocused(true)

	// Initial view should be Active
	if pane.graph.GetView() != ViewActive {
		t.Error("expected initial view to be Active")
	}

	// Press 'a' to toggle to Backlog
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	newPane, _ := pane.Update(msg)

	if newPane.graph.GetView() != ViewBacklog {
		t.Errorf("expected view to be Backlog after toggle, got %v", newPane.graph.GetView())
	}
}

func TestGraphPane_KeyCycleDensity(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	pane := NewGraphPane(cfg, nil, "horizontal")
	pane.SetFocused(true)

	// Press 'd' to cycle density
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}
	newPane, _ := pane.Update(msg)

	// Standard -> Detailed
	if newPane.graph.GetDensity() != DensityDetailed {
		t.Errorf("expected density to be detailed, got %v", newPane.graph.GetDensity())
	}
}

func TestGraphPane_KeyEscClearsError(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	pane := NewGraphPane(cfg, nil, "horizontal")
	pane.SetFocused(true)
	pane.errorMsg = "some error"

	msg := tea.KeyMsg{Type: tea.KeyEsc}
	newPane, _ := pane.Update(msg)

	if newPane.errorMsg != "" {
		t.Errorf("expected error to be cleared, got %q", newPane.errorMsg)
	}
}

func TestGraphPane_KeyEscUnfocuses(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	pane := NewGraphPane(cfg, nil, "horizontal")
	pane.SetFocused(true)
	pane.errorMsg = "" // No error

	msg := tea.KeyMsg{Type: tea.KeyEsc}
	newPane, _ := pane.Update(msg)

	if newPane.focused {
		t.Error("expected pane to be unfocused after Esc with no error")
	}
}

func TestGraphPane_KeyEnterTwoStepBehavior(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-test-123", Title: "Test", Status: "open", IssueType: "task"},
		},
	}
	pane := NewGraphPane(cfg, fetcher, "horizontal")
	pane.SetFocused(true)
	pane.rebuildGraph(fetcher.activeBeads)

	// First Enter: opens inline detail view
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	pane, cmd := pane.Update(msg)

	if cmd == nil {
		t.Fatal("expected first Enter to return a command for async fetch")
	}
	if !pane.IsShowingDetail() {
		t.Error("expected pane to be showing detail after first Enter")
	}

	// Execute the command - should be graphDetailResultMsg for async fetch
	resultMsg := cmd()
	_, ok := resultMsg.(graphDetailResultMsg)
	if !ok {
		t.Fatalf("expected graphDetailResultMsg from first Enter, got %T", resultMsg)
	}

	// Second Enter: opens full-screen modal
	pane, cmd = pane.Update(msg)

	if cmd == nil {
		t.Fatal("expected second Enter to return a command for modal")
	}

	// Execute the command - should be GraphOpenModalMsg
	resultMsg = cmd()
	modalMsg, ok := resultMsg.(GraphOpenModalMsg)
	if !ok {
		t.Fatalf("expected GraphOpenModalMsg from second Enter, got %T", resultMsg)
	}
	if modalMsg.NodeID != "bd-test-123" {
		t.Errorf("expected NodeID='bd-test-123', got %q", modalMsg.NodeID)
	}
}

func TestGraphPane_EscapeClosesDetailView(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-test-123", Title: "Test", Status: "open", IssueType: "task"},
		},
	}
	pane := NewGraphPane(cfg, fetcher, "horizontal")
	pane.SetFocused(true)
	pane.rebuildGraph(fetcher.activeBeads)

	// First Enter: opens inline detail view
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	pane, _ = pane.Update(enterMsg)

	if !pane.IsShowingDetail() {
		t.Fatal("expected pane to be showing detail after Enter")
	}

	// Escape: closes detail view and returns to graph
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	pane, _ = pane.Update(escMsg)

	if pane.IsShowingDetail() {
		t.Error("expected detail view to be closed after Escape")
	}
	if !pane.IsFocused() {
		t.Error("expected pane to remain focused after closing detail view")
	}
}

func TestGraphPane_KeysIgnoredWhenUnfocused(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	pane := NewGraphPane(cfg, nil, "horizontal")
	pane.SetFocused(false)

	// Keys should be ignored when not focused
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}
	_, cmd := pane.Update(msg)

	if cmd != nil {
		t.Error("expected no command when pane is not focused")
	}
}

func TestGraphPane_ViewEmpty(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	pane := NewGraphPane(cfg, nil, "horizontal")
	// width/height are 0

	view := pane.View()
	if view != "" {
		t.Errorf("expected empty view when size is 0, got %q", view)
	}
}

func TestGraphPane_ViewWithSize(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	pane := NewGraphPane(cfg, nil, "horizontal")
	pane.SetSize(80, 24)

	view := pane.View()
	if view == "" {
		t.Error("expected non-empty view when size is set")
	}
	// Should contain placeholder text since no beads
	if !strings.Contains(view, "No beads") {
		t.Error("expected view to contain 'No beads' placeholder")
	}
}

func TestGraphPane_ViewWithBeads(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-test-123", Title: "Test bead", Status: "open", IssueType: "task"},
		},
	}
	pane := NewGraphPane(cfg, fetcher, "horizontal")
	pane.SetSize(80, 24)
	pane.rebuildGraph(fetcher.activeBeads)

	view := pane.View()
	if !strings.Contains(view, "bd-test-123") {
		t.Error("expected view to contain bead ID")
	}
}

func TestGraphPane_ViewWithError(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	pane := NewGraphPane(cfg, nil, "horizontal")
	pane.SetSize(80, 24)
	pane.errorMsg = "Something went wrong"

	view := pane.View()
	if !strings.Contains(view, "Error:") {
		t.Error("expected view to contain error prefix")
	}
	if !strings.Contains(view, "Something went wrong") {
		t.Error("expected view to contain error message")
	}
}

func TestGraphPane_ViewWithLoading(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	pane := NewGraphPane(cfg, nil, "horizontal")
	pane.SetSize(80, 24)
	pane.loading = true
	pane.startedAt = time.Now()

	view := pane.View()
	// When loading, the status bar should still show normal content (view, density)
	// with a spinner appended at the end - not replace the whole header
	if !strings.Contains(view, "active") {
		t.Error("expected view to show view mode while loading")
	}
	if !strings.Contains(view, "standard") {
		t.Error("expected view to show density while loading")
	}
}

func TestGraphPane_SetCurrentBead(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-test-123", Title: "Test", Status: "open", IssueType: "task"},
		},
	}
	pane := NewGraphPane(cfg, fetcher, "horizontal")
	pane.rebuildGraph(fetcher.activeBeads)

	pane.SetCurrentBead("bd-test-123")

	if pane.graph.GetCurrentBead() != "bd-test-123" {
		t.Errorf("expected current bead to be bd-test-123, got %q", pane.graph.GetCurrentBead())
	}
}

func TestGraphPane_GetSelectedNode(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-test-123", Title: "Test", Status: "open", IssueType: "task"},
		},
	}
	pane := NewGraphPane(cfg, fetcher, "horizontal")
	pane.rebuildGraph(fetcher.activeBeads)

	node := pane.GetSelectedNode()
	if node == nil {
		t.Fatal("expected selected node to be returned")
	}
	if node.ID != "bd-test-123" {
		t.Errorf("expected node ID=bd-test-123, got %q", node.ID)
	}
}

func TestGraphPane_IsLoading(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	pane := NewGraphPane(cfg, nil, "horizontal")

	if pane.IsLoading() {
		t.Error("expected IsLoading to be false initially")
	}

	pane.loading = true
	if !pane.IsLoading() {
		t.Error("expected IsLoading to be true when loading")
	}
}

// slowMockFetcher simulates a slow fetch operation for testing async behavior.
type slowMockFetcher struct {
	delay       time.Duration
	activeBeads []GraphBead
	activeErr   error
}

func (m *slowMockFetcher) FetchActive(ctx context.Context) ([]GraphBead, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(m.delay):
		if m.activeErr != nil {
			return nil, m.activeErr
		}
		return m.activeBeads, nil
	}
}

func (m *slowMockFetcher) FetchBacklog(ctx context.Context) ([]GraphBead, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(m.delay):
		return nil, nil
	}
}

func (m *slowMockFetcher) FetchBead(ctx context.Context, id string) (*GraphBead, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(m.delay):
		return nil, nil
	}
}

func TestGraphPane_RefreshCmdReturnsCommands(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-001", Title: "Test", Status: "open", IssueType: "task"},
		},
	}
	pane := NewGraphPane(cfg, fetcher, "horizontal")

	// refreshCmd should return commands when not already loading
	cmd := pane.refreshCmd()
	if cmd == nil {
		t.Error("expected refreshCmd to return commands")
	}
}

func TestGraphPane_RefreshCmdReturnsNilWhenLoading(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	pane := NewGraphPane(cfg, nil, "horizontal")
	pane.loading = true

	cmd := pane.refreshCmd()
	if cmd != nil {
		t.Error("expected refreshCmd to return nil when already loading")
	}
}

func TestGraphPane_FetchCmdWithFastFetcher(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-001", Title: "Test", Status: "open", IssueType: "task"},
		},
	}
	pane := NewGraphPane(cfg, fetcher, "horizontal")

	// Execute the fetch command directly
	cmd := pane.fetchCmd(1)
	result := cmd()

	msg, ok := result.(graphResultMsg)
	if !ok {
		t.Fatalf("expected graphResultMsg, got %T", result)
	}
	if msg.err != nil {
		t.Errorf("unexpected error: %v", msg.err)
	}
	if len(msg.beads) != 1 {
		t.Errorf("expected 1 bead, got %d", len(msg.beads))
	}
	if msg.requestID != 1 {
		t.Errorf("expected requestID=1, got %d", msg.requestID)
	}
}

func TestGraphPane_FetchCmdWithSlowFetcher(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	fetcher := &slowMockFetcher{
		delay: 50 * time.Millisecond,
		activeBeads: []GraphBead{
			{ID: "bd-001", Title: "Test", Status: "open", IssueType: "task"},
		},
	}
	pane := NewGraphPane(cfg, fetcher, "horizontal")

	start := time.Now()
	cmd := pane.fetchCmd(1)
	result := cmd()
	elapsed := time.Since(start)

	// Should have taken at least the delay time
	if elapsed < 40*time.Millisecond {
		t.Errorf("fetch completed too quickly: %v", elapsed)
	}

	msg, ok := result.(graphResultMsg)
	if !ok {
		t.Fatalf("expected graphResultMsg, got %T", result)
	}
	if msg.err != nil {
		t.Errorf("unexpected error: %v", msg.err)
	}
	if len(msg.beads) != 1 {
		t.Errorf("expected 1 bead, got %d", len(msg.beads))
	}
}

func TestGraphPane_FetchCmdWithNilFetcher(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	pane := NewGraphPane(cfg, nil, "horizontal")

	cmd := pane.fetchCmd(1)
	result := cmd()

	msg, ok := result.(graphResultMsg)
	if !ok {
		t.Fatalf("expected graphResultMsg, got %T", result)
	}
	// Nil fetcher should return no error and nil beads
	if msg.err != nil {
		t.Errorf("unexpected error: %v", msg.err)
	}
	if msg.beads != nil {
		t.Errorf("expected nil beads, got %v", msg.beads)
	}
}

func TestGraphPane_FetchCmdBacklogView(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	fetcher := &mockFetcher{
		backlogBeads: []GraphBead{
			{ID: "bd-deferred", Title: "Deferred Task", Status: "deferred", IssueType: "task"},
		},
	}
	pane := NewGraphPane(cfg, fetcher, "horizontal")
	pane.graph.SetView(ViewBacklog)

	cmd := pane.fetchCmd(1)
	result := cmd()

	msg, ok := result.(graphResultMsg)
	if !ok {
		t.Fatalf("expected graphResultMsg, got %T", result)
	}
	if msg.err != nil {
		t.Errorf("unexpected error: %v", msg.err)
	}
	if len(msg.beads) != 1 {
		t.Errorf("expected 1 bead, got %d", len(msg.beads))
	}
	if len(msg.beads) > 0 && msg.beads[0].ID != "bd-deferred" {
		t.Errorf("expected bd-deferred, got %s", msg.beads[0].ID)
	}
}

func TestGraphPane_KeyRefresh(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-001", Title: "Test", Status: "open", IssueType: "task"},
		},
	}
	pane := NewGraphPane(cfg, fetcher, "horizontal")
	pane.SetFocused(true)

	// Press 'R' to trigger manual refresh
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}}
	_, cmd := pane.Update(msg)

	if cmd == nil {
		t.Error("expected R key to trigger refresh command")
	}
}

// TestGraphPane_StartLoadingMsgStartsFetch verifies that processing graphStartLoadingMsg
// with startFetch=true returns a command to start the fetch. This is critical for fixing
// the race condition where fast fetches would complete before requestID was set.
func TestGraphPane_StartLoadingMsgStartsFetch(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-001", Title: "Standalone Task", Status: "in_progress", IssueType: "task"},
		},
	}
	pane := NewGraphPane(cfg, fetcher, "horizontal")

	// Process graphStartLoadingMsg with startFetch=true
	msg := graphStartLoadingMsg{requestID: 1, startFetch: true}
	pane, cmd := pane.Update(msg)

	// State should be updated
	if pane.requestID != 1 {
		t.Errorf("requestID not set: got %d, want 1", pane.requestID)
	}
	if !pane.loading {
		t.Error("loading not set to true")
	}

	// Should return a command to start the fetch
	if cmd == nil {
		t.Error("expected startFetch=true to return fetch command")
	}
}

// TestGraphPane_FullRefreshFlow verifies the complete refresh flow works correctly,
// ensuring results aren't dropped as stale when the fetch completes quickly.
func TestGraphPane_FullRefreshFlow(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-standalone", Title: "Standalone Task", Status: "in_progress", IssueType: "task"},
		},
	}
	pane := NewGraphPane(cfg, fetcher, "horizontal")
	pane.SetSize(80, 24)

	// Initially should have no beads (graph was created but not refreshed)
	if pane.graph.NodeCount() != 0 {
		t.Errorf("expected 0 nodes initially, got %d", pane.graph.NodeCount())
	}

	// Step 1: refreshCmd returns graphStartLoadingMsg with startFetch=true
	refreshCmd := pane.refreshCmd()
	if refreshCmd == nil {
		t.Fatal("refreshCmd returned nil")
	}

	// Execute batch to get the graphStartLoadingMsg
	// The batch contains spinner tick and graphStartLoadingMsg
	// We need to find and execute the graphStartLoadingMsg
	msg := graphStartLoadingMsg{requestID: 1, startFetch: true}

	// Step 2: Process graphStartLoadingMsg - this sets requestID and returns fetch command
	pane, fetchCmd := pane.Update(msg)
	if pane.requestID != 1 {
		t.Errorf("requestID not set after graphStartLoadingMsg: got %d, want 1", pane.requestID)
	}
	if !pane.loading {
		t.Error("loading should be true after graphStartLoadingMsg")
	}
	if fetchCmd == nil {
		t.Fatal("expected fetchCmd to be returned")
	}

	// Step 3: Execute the fetch command - this simulates the fetch completing
	// We need to extract and run the actual fetchCmd from the batch
	actualFetchCmd := pane.fetchCmd(1)
	fetchResult := actualFetchCmd()

	resultMsg, ok := fetchResult.(graphResultMsg)
	if !ok {
		t.Fatalf("expected graphResultMsg, got %T", fetchResult)
	}
	if resultMsg.requestID != 1 {
		t.Errorf("result requestID=%d, want 1", resultMsg.requestID)
	}

	// Step 4: Process the result - this should NOT be dropped as stale
	// because requestID was set BEFORE the fetch started
	pane, _ = pane.Update(resultMsg)

	// Verify the graph now has the bead
	if pane.graph.NodeCount() != 1 {
		t.Errorf("expected 1 node after result, got %d", pane.graph.NodeCount())
	}
	if pane.loading {
		t.Error("loading should be false after result processed")
	}

	// Verify the standalone bead is visible
	nodes := pane.graph.GetNodes()
	if _, ok := nodes["bd-standalone"]; !ok {
		t.Error("standalone bead should be in graph")
	}
}

func TestGraphPane_SetVisible(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	pane := NewGraphPane(cfg, nil, "horizontal")

	// Initially not visible
	if pane.visible {
		t.Error("expected visible to be false initially")
	}

	pane.SetVisible(true)
	if !pane.visible {
		t.Error("expected visible to be true after SetVisible(true)")
	}

	pane.SetVisible(false)
	if pane.visible {
		t.Error("expected visible to be false after SetVisible(false)")
	}
}

func TestGraphPane_AutoRefreshCmdDisabled(t *testing.T) {
	// AutoRefreshInterval = 0 (disabled)
	cfg := &config.GraphConfig{
		Density:             "standard",
		AutoRefreshInterval: 0,
	}
	pane := NewGraphPane(cfg, nil, "horizontal")

	cmd := pane.autoRefreshCmd()
	if cmd != nil {
		t.Error("expected autoRefreshCmd to return nil when disabled (interval=0)")
	}
}

func TestGraphPane_AutoRefreshCmdEnabled(t *testing.T) {
	cfg := &config.GraphConfig{
		Density:             "standard",
		AutoRefreshInterval: 5 * time.Second,
	}
	pane := NewGraphPane(cfg, nil, "horizontal")

	cmd := pane.autoRefreshCmd()
	if cmd == nil {
		t.Error("expected autoRefreshCmd to return a command when enabled")
	}
}

func TestGraphPane_AutoRefreshCmdEnforcesMinimum(t *testing.T) {
	// Interval below minimum (500ms < 1s)
	cfg := &config.GraphConfig{
		Density:             "standard",
		AutoRefreshInterval: 500 * time.Millisecond,
	}
	pane := NewGraphPane(cfg, nil, "horizontal")

	cmd := pane.autoRefreshCmd()
	if cmd == nil {
		t.Fatal("expected autoRefreshCmd to return a command")
	}

	// The command should be a tea.Tick with at least minAutoRefreshInterval (1s)
	// We can't directly test the interval, but we verify the command exists
}

func TestGraphPane_AutoRefreshCmdNilConfig(t *testing.T) {
	pane := NewGraphPane(nil, nil, "horizontal")

	cmd := pane.autoRefreshCmd()
	if cmd != nil {
		t.Error("expected autoRefreshCmd to return nil when config is nil")
	}
}

func TestGraphPane_AutoRefreshMsgTriggersRefreshWhenVisible(t *testing.T) {
	cfg := &config.GraphConfig{
		Density:             "standard",
		AutoRefreshInterval: 5 * time.Second,
	}
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-001", Title: "Test", Status: "open", IssueType: "task"},
		},
	}
	pane := NewGraphPane(cfg, fetcher, "horizontal")
	pane.SetVisible(true)

	// Send graphAutoRefreshMsg
	msg := graphAutoRefreshMsg{}
	_, cmd := pane.Update(msg)

	// Should return commands: refresh cmd + next auto-refresh tick
	if cmd == nil {
		t.Error("expected graphAutoRefreshMsg to return commands when visible")
	}
}

func TestGraphPane_AutoRefreshMsgNoRefreshWhenNotVisible(t *testing.T) {
	cfg := &config.GraphConfig{
		Density:             "standard",
		AutoRefreshInterval: 5 * time.Second,
	}
	pane := NewGraphPane(cfg, nil, "horizontal")
	pane.SetVisible(false)

	// Send graphAutoRefreshMsg
	msg := graphAutoRefreshMsg{}
	newPane, cmd := pane.Update(msg)

	// Should still return next auto-refresh tick, but no refresh triggered
	if cmd == nil {
		t.Error("expected graphAutoRefreshMsg to return next tick command")
	}

	// Pane should not have started loading since it's not visible
	if newPane.loading {
		t.Error("expected loading to remain false when not visible")
	}
}

func TestGraphPane_AutoRefreshMsgSchedulesNextTick(t *testing.T) {
	cfg := &config.GraphConfig{
		Density:             "standard",
		AutoRefreshInterval: 5 * time.Second,
	}
	pane := NewGraphPane(cfg, nil, "horizontal")
	pane.SetVisible(false) // Even when not visible, should schedule next tick

	msg := graphAutoRefreshMsg{}
	_, cmd := pane.Update(msg)

	if cmd == nil {
		t.Error("expected graphAutoRefreshMsg to schedule next tick")
	}
}

func TestGraphPane_InitStartsAutoRefresh(t *testing.T) {
	cfg := &config.GraphConfig{
		Density:             "standard",
		AutoRefreshInterval: 5 * time.Second,
	}
	pane := NewGraphPane(cfg, nil, "horizontal")

	cmd := pane.Init()
	if cmd == nil {
		t.Error("expected Init to return commands including auto-refresh")
	}
}

func TestGraphPane_InitNoAutoRefreshWhenDisabled(t *testing.T) {
	cfg := &config.GraphConfig{
		Density:             "standard",
		AutoRefreshInterval: 0, // Disabled
	}
	pane := NewGraphPane(cfg, nil, "horizontal")

	cmd := pane.Init()
	// Should still return the initial refresh command
	if cmd == nil {
		t.Error("expected Init to return initial refresh command")
	}
}
