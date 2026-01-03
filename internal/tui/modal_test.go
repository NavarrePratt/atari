package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/npratt/atari/internal/testutil"
)

func TestNewDetailModal(t *testing.T) {
	fetcher := &mockFetcher{}
	modal := NewDetailModal(fetcher)

	if modal == nil {
		t.Fatal("expected non-nil modal")
	}
	if modal.IsOpen() {
		t.Error("expected modal to be closed initially")
	}
	if modal.loading {
		t.Error("expected loading to be false initially")
	}
}

func TestDetailModal_OpenClose(t *testing.T) {
	fetcher := &mockFetcher{
		beadByID: map[string]GraphBead{
			"bd-test-123": {ID: "bd-test-123", Title: "Test bead", Status: "open"},
		},
	}
	modal := NewDetailModal(fetcher)

	node := &GraphNode{ID: "bd-test-123", Title: "Test bead", Status: "open"}
	cmd := modal.Open(node)

	if !modal.IsOpen() {
		t.Error("expected modal to be open after Open()")
	}
	if modal.node != node {
		t.Error("expected node to be set")
	}
	if !modal.loading {
		t.Error("expected loading to be true after Open()")
	}
	if cmd == nil {
		t.Error("expected Open to return a command for fetching")
	}

	modal.Close()

	if modal.IsOpen() {
		t.Error("expected modal to be closed after Close()")
	}
	if modal.node != nil {
		t.Error("expected node to be nil after Close()")
	}
	if modal.loading {
		t.Error("expected loading to be false after Close()")
	}
}

func TestDetailModal_OpenWithNilFetcher(t *testing.T) {
	modal := NewDetailModal(nil)
	node := &GraphNode{ID: "bd-test-123", Title: "Test"}

	cmd := modal.Open(node)

	if !modal.IsOpen() {
		t.Error("expected modal to be open")
	}
	if modal.loading {
		t.Error("expected loading to be false with nil fetcher")
	}
	if cmd != nil {
		t.Error("expected no command with nil fetcher")
	}
}

func TestDetailModal_OpenWithNilNode(t *testing.T) {
	fetcher := &mockFetcher{}
	modal := NewDetailModal(fetcher)

	cmd := modal.Open(nil)

	if !modal.IsOpen() {
		t.Error("expected modal to be open")
	}
	if modal.loading {
		t.Error("expected loading to be false with nil node")
	}
	if cmd != nil {
		t.Error("expected no command with nil node")
	}
}

func TestDetailModal_SetSize(t *testing.T) {
	modal := NewDetailModal(nil)
	modal.SetSize(80, 24)

	if modal.width != 80 {
		t.Errorf("expected width=80, got %d", modal.width)
	}
	if modal.height != 24 {
		t.Errorf("expected height=24, got %d", modal.height)
	}
}

func TestDetailModal_HandleFetchResult(t *testing.T) {
	fetcher := &mockFetcher{}
	modal := NewDetailModal(fetcher)
	modal.open = true
	modal.loading = true
	modal.requestID = 1

	bead := &GraphBead{
		ID:          "bd-test-123",
		Title:       "Test bead",
		Description: "Test description",
		Status:      "open",
	}

	msg := modalFetchResultMsg{bead: bead, err: nil, requestID: 1}
	modal.Update(msg)

	if modal.loading {
		t.Error("expected loading to be false after result")
	}
	if modal.errorMsg != "" {
		t.Errorf("expected no error, got %q", modal.errorMsg)
	}
	if modal.fullBead == nil {
		t.Error("expected fullBead to be set")
	}
	if modal.fullBead.ID != "bd-test-123" {
		t.Errorf("expected fullBead.ID=bd-test-123, got %q", modal.fullBead.ID)
	}
}

func TestDetailModal_HandleFetchError(t *testing.T) {
	fetcher := &mockFetcher{}
	modal := NewDetailModal(fetcher)
	modal.open = true
	modal.loading = true
	modal.requestID = 1

	msg := modalFetchResultMsg{bead: nil, err: errors.New("fetch failed"), requestID: 1}
	modal.Update(msg)

	if modal.loading {
		t.Error("expected loading to be false after result")
	}
	if modal.errorMsg != "fetch failed" {
		t.Errorf("expected error='fetch failed', got %q", modal.errorMsg)
	}
	if modal.fullBead != nil {
		t.Error("expected fullBead to be nil on error")
	}
}

func TestDetailModal_DropsStaleResults(t *testing.T) {
	fetcher := &mockFetcher{}
	modal := NewDetailModal(fetcher)
	modal.open = true
	modal.loading = true
	modal.requestID = 2

	// Stale result from request 1
	msg := modalFetchResultMsg{
		bead:      &GraphBead{ID: "stale"},
		err:       nil,
		requestID: 1,
	}
	modal.Update(msg)

	if !modal.loading {
		t.Error("expected loading to remain true for stale result")
	}
	if modal.fullBead != nil {
		t.Error("expected fullBead to remain nil for stale result")
	}
}

func TestDetailModal_KeyClose(t *testing.T) {
	modal := NewDetailModal(nil)
	modal.open = true
	modal.node = &GraphNode{ID: "test"}

	closeKeys := []string{"esc", "enter", "q"}
	for _, key := range closeKeys {
		modal.open = true
		modal.node = &GraphNode{ID: "test"}

		var msg tea.KeyMsg
		switch key {
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "q":
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
		}

		modal.Update(msg)

		if modal.IsOpen() {
			t.Errorf("expected modal to close on %s key", key)
		}
	}
}

func TestDetailModal_KeyScroll(t *testing.T) {
	modal := NewDetailModal(nil)
	modal.open = true
	modal.node = &GraphNode{ID: "test"}
	modal.scrollPos = 5

	// Test scroll up
	msg := tea.KeyMsg{Type: tea.KeyUp}
	modal.Update(msg)
	if modal.scrollPos != 4 {
		t.Errorf("expected scrollPos=4 after up, got %d", modal.scrollPos)
	}

	// Test scroll down
	msg = tea.KeyMsg{Type: tea.KeyDown}
	modal.Update(msg)
	if modal.scrollPos != 5 {
		t.Errorf("expected scrollPos=5 after down, got %d", modal.scrollPos)
	}

	// Test home key
	modal.scrollPos = 10
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}
	modal.Update(msg)
	if modal.scrollPos != 0 {
		t.Errorf("expected scrollPos=0 after home, got %d", modal.scrollPos)
	}

	// Test end key
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}}
	modal.Update(msg)
	if modal.scrollPos != 9999 {
		t.Errorf("expected scrollPos=9999 after end, got %d", modal.scrollPos)
	}
}

func TestDetailModal_KeyScrollVim(t *testing.T) {
	modal := NewDetailModal(nil)
	modal.open = true
	modal.node = &GraphNode{ID: "test"}
	modal.scrollPos = 5

	// Test 'k' for up
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	modal.Update(msg)
	if modal.scrollPos != 4 {
		t.Errorf("expected scrollPos=4 after k, got %d", modal.scrollPos)
	}

	// Test 'j' for down
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	modal.Update(msg)
	if modal.scrollPos != 5 {
		t.Errorf("expected scrollPos=5 after j, got %d", modal.scrollPos)
	}
}

func TestDetailModal_ScrollPosClamp(t *testing.T) {
	modal := NewDetailModal(nil)
	modal.open = true
	modal.node = &GraphNode{ID: "test"}
	modal.scrollPos = 0

	// Try to scroll up past 0
	msg := tea.KeyMsg{Type: tea.KeyUp}
	modal.Update(msg)
	if modal.scrollPos != 0 {
		t.Errorf("expected scrollPos=0 (clamped), got %d", modal.scrollPos)
	}
}

func TestDetailModal_ViewWhenClosed(t *testing.T) {
	modal := NewDetailModal(nil)
	modal.open = false

	view := modal.View(80, 24)
	if view != "" {
		t.Errorf("expected empty view when closed, got %q", view)
	}
}

func TestDetailModal_ViewWhenOpenNoNode(t *testing.T) {
	modal := NewDetailModal(nil)
	modal.open = true
	modal.node = nil

	view := modal.View(80, 24)
	if view != "" {
		t.Errorf("expected empty view with nil node, got %q", view)
	}
}

func TestDetailModal_ViewWithNode(t *testing.T) {
	modal := NewDetailModal(nil)
	modal.open = true
	modal.node = &GraphNode{
		ID:       "bd-test-123",
		Title:    "Test bead title",
		Status:   "open",
		Priority: 2,
		Type:     "task",
	}

	view := modal.View(80, 24)
	if view == "" {
		t.Error("expected non-empty view")
	}
	if !strings.Contains(view, "bd-test-123") {
		t.Error("expected view to contain bead ID")
	}
	if !strings.Contains(view, "open") {
		t.Error("expected view to contain status")
	}
}

func TestDetailModal_ViewWithLoading(t *testing.T) {
	modal := NewDetailModal(nil)
	modal.open = true
	modal.loading = true
	modal.node = &GraphNode{ID: "bd-test-123", Title: "Test"}

	view := modal.View(80, 24)
	if !strings.Contains(view, "Loading") {
		t.Error("expected view to show loading state")
	}
}

func TestDetailModal_ViewWithError(t *testing.T) {
	modal := NewDetailModal(nil)
	modal.open = true
	modal.node = &GraphNode{ID: "bd-test-123", Title: "Test"}
	modal.errorMsg = "Something went wrong"

	view := modal.View(80, 24)
	if !strings.Contains(view, "Error:") {
		t.Error("expected view to show error prefix")
	}
	if !strings.Contains(view, "Something went wrong") {
		t.Error("expected view to show error message")
	}
}

func TestDetailModal_ViewWithFullBead(t *testing.T) {
	modal := NewDetailModal(nil)
	modal.open = true
	modal.node = &GraphNode{ID: "bd-test-123", Title: "Test"}
	modal.fullBead = &GraphBead{
		ID:          "bd-test-123",
		Title:       "Full bead title",
		Description: "This is a test description",
		Status:      "open",
		Priority:    2,
		IssueType:   "task",
		Notes:       "Some notes here",
		CreatedAt:   "2024-01-01",
		CreatedBy:   "user",
		UpdatedAt:   "2024-01-02",
		Dependencies: []BeadReference{
			{ID: "bd-dep-1", Title: "Dependency 1", Status: "closed"},
		},
		Dependents: []BeadReference{
			{ID: "bd-dependent-1", Title: "Dependent 1", Status: "open"},
		},
	}

	view := modal.View(100, 40)
	if !strings.Contains(view, "Full bead title") {
		t.Error("expected view to show full bead title")
	}
	if !strings.Contains(view, "This is a test description") {
		t.Error("expected view to show description")
	}
	if !strings.Contains(view, "Dependencies:") {
		t.Error("expected view to show dependencies section")
	}
	if !strings.Contains(view, "Dependents:") {
		t.Error("expected view to show dependents section")
	}
	if !strings.Contains(view, "Notes:") {
		t.Error("expected view to show notes section")
	}
}

// Test FetchBead using testutil.MockRunner
func TestBDFetcher_FetchBead_Success(t *testing.T) {
	runner := testutil.NewMockRunner()
	runner.SetResponse("bd", []string{"show", "bd-test-123", "--json"}, []byte(`[{
		"id": "bd-test-123",
		"title": "Test bead",
		"description": "Test description",
		"status": "open",
		"priority": 2,
		"issue_type": "task"
	}]`))

	fetcher := NewBDFetcher(runner)

	bead, err := fetcher.FetchBead(t.Context(), "bd-test-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bead == nil {
		t.Fatal("expected non-nil bead")
	}
	if bead.ID != "bd-test-123" {
		t.Errorf("expected ID=bd-test-123, got %q", bead.ID)
	}
	if bead.Title != "Test bead" {
		t.Errorf("expected Title='Test bead', got %q", bead.Title)
	}
}

func TestBDFetcher_FetchBead_Error(t *testing.T) {
	runner := testutil.NewMockRunner()
	runner.SetError("bd", []string{"show", "bd-test-123", "--json"}, errors.New("command failed"))

	fetcher := NewBDFetcher(runner)

	_, err := fetcher.FetchBead(t.Context(), "bd-test-123")
	if err == nil {
		t.Error("expected error")
	}
	if !strings.Contains(err.Error(), "bd show") {
		t.Errorf("expected error to mention 'bd show', got %q", err.Error())
	}
}

func TestBDFetcher_FetchBead_NotFound(t *testing.T) {
	runner := testutil.NewMockRunner()
	runner.SetResponse("bd", []string{"show", "bd-nonexistent", "--json"}, []byte(`[]`))

	fetcher := NewBDFetcher(runner)

	_, err := fetcher.FetchBead(t.Context(), "bd-nonexistent")
	if err == nil {
		t.Error("expected error for not found")
	}
	if !strings.Contains(err.Error(), "bead not found") {
		t.Errorf("expected 'bead not found' error, got %q", err.Error())
	}
}
