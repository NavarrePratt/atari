package tui

import (
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

func TestGraphPane_KeyEnterEmitsModalMsg(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-test-123", Title: "Test", Status: "open", IssueType: "task"},
		},
	}
	pane := NewGraphPane(cfg, fetcher, "horizontal")
	pane.SetFocused(true)
	pane.rebuildGraph(fetcher.activeBeads)

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := pane.Update(msg)

	if cmd == nil {
		t.Fatal("expected Enter to return a command")
	}

	// Execute the command to get the message
	resultMsg := cmd()
	modalMsg, ok := resultMsg.(GraphOpenModalMsg)
	if !ok {
		t.Fatalf("expected GraphOpenModalMsg, got %T", resultMsg)
	}
	if modalMsg.NodeID != "bd-test-123" {
		t.Errorf("expected NodeID='bd-test-123', got %q", modalMsg.NodeID)
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
	if !strings.Contains(view, "Loading") {
		t.Error("expected view to show loading status")
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
