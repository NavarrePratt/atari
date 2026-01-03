package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func TestNewObserverPane(t *testing.T) {
	pane := NewObserverPane(nil)

	if pane.loading {
		t.Error("expected loading to be false initially")
	}
	if pane.response != "" {
		t.Error("expected response to be empty initially")
	}
	if pane.errorMsg != "" {
		t.Error("expected errorMsg to be empty initially")
	}
	if pane.focused {
		t.Error("expected focused to be false initially")
	}
}

func TestObserverPane_SetSize(t *testing.T) {
	pane := NewObserverPane(nil)
	pane.SetSize(80, 24)

	if pane.width != 80 {
		t.Errorf("expected width=80, got %d", pane.width)
	}
	if pane.height != 24 {
		t.Errorf("expected height=24, got %d", pane.height)
	}
}

func TestObserverPane_SetFocused(t *testing.T) {
	pane := NewObserverPane(nil)

	pane.SetFocused(true)
	if !pane.IsFocused() {
		t.Error("expected pane to be focused")
	}

	pane.SetFocused(false)
	if pane.IsFocused() {
		t.Error("expected pane to be unfocused")
	}
}

func TestObserverPane_EscClearsInput(t *testing.T) {
	pane := NewObserverPane(nil)
	pane.SetSize(80, 24)
	pane.SetFocused(true)

	// Type some text
	pane.input.SetValue("test question")

	// Press Esc to clear
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	newPane, _ := pane.Update(msg)

	if newPane.input.Value() != "" {
		t.Errorf("expected input to be cleared, got %q", newPane.input.Value())
	}
}

func TestObserverPane_EscClearsError(t *testing.T) {
	pane := NewObserverPane(nil)
	pane.SetSize(80, 24)
	pane.errorMsg = "some error"

	// Press Esc to clear error
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	newPane, _ := pane.Update(msg)

	if newPane.errorMsg != "" {
		t.Errorf("expected error to be cleared, got %q", newPane.errorMsg)
	}
}

func TestObserverPane_HandleResultSuccess(t *testing.T) {
	pane := NewObserverPane(nil)
	pane.loading = true

	msg := observerResultMsg{response: "test response", err: nil}
	newPane, _ := pane.Update(msg)

	if newPane.loading {
		t.Error("expected loading to be false after result")
	}
	if newPane.response != "test response" {
		t.Errorf("expected response='test response', got %q", newPane.response)
	}
	if newPane.errorMsg != "" {
		t.Errorf("expected no error, got %q", newPane.errorMsg)
	}
}

func TestObserverPane_HandleResultError(t *testing.T) {
	pane := NewObserverPane(nil)
	pane.loading = true

	msg := observerResultMsg{response: "", err: &testError{msg: "query failed"}}
	newPane, _ := pane.Update(msg)

	if newPane.loading {
		t.Error("expected loading to be false after result")
	}
	if newPane.response != "" {
		t.Errorf("expected empty response, got %q", newPane.response)
	}
	if newPane.errorMsg != "query failed" {
		t.Errorf("expected error='query failed', got %q", newPane.errorMsg)
	}
}

func TestObserverPane_TickUpdatesDuringLoading(t *testing.T) {
	pane := NewObserverPane(nil)
	pane.loading = true
	pane.startedAt = time.Now().Add(-2 * time.Second)

	msg := observerTickMsg(time.Now())
	newPane, cmd := pane.Update(msg)

	if !newPane.loading {
		t.Error("expected loading to remain true")
	}
	if cmd == nil {
		t.Error("expected tick cmd to schedule another tick")
	}
}

func TestObserverPane_SpinnerTickDuringLoading(t *testing.T) {
	pane := NewObserverPane(nil)
	pane.loading = true

	msg := spinner.TickMsg{}
	_, cmd := pane.Update(msg)

	// Spinner should continue ticking during loading
	if cmd == nil {
		t.Error("expected spinner to return a cmd during loading")
	}
}

func TestObserverPane_SpinnerTickWhenNotLoading(t *testing.T) {
	pane := NewObserverPane(nil)
	pane.loading = false

	msg := spinner.TickMsg{}
	_, cmd := pane.Update(msg)

	// Spinner should not tick when not loading
	if cmd != nil {
		t.Error("expected no cmd when not loading")
	}
}

func TestObserverPane_ClearResponse(t *testing.T) {
	pane := NewObserverPane(nil)
	pane.response = "some response"
	pane.errorMsg = "some error"

	pane.ClearResponse()

	if pane.response != "" {
		t.Errorf("expected response to be cleared, got %q", pane.response)
	}
	if pane.errorMsg != "" {
		t.Errorf("expected errorMsg to be cleared, got %q", pane.errorMsg)
	}
}

func TestObserverPane_ViewEmpty(t *testing.T) {
	pane := NewObserverPane(nil)
	// width/height are 0

	view := pane.View()
	if view != "" {
		t.Errorf("expected empty view when size is 0, got %q", view)
	}
}

func TestObserverPane_ViewWithSize(t *testing.T) {
	pane := NewObserverPane(nil)
	pane.SetSize(80, 24)
	pane.SetFocused(true)

	view := pane.View()
	if view == "" {
		t.Error("expected non-empty view when size is set")
	}
	// Should contain placeholder text
	if !strings.Contains(view, "Response will appear") {
		t.Error("expected view to contain placeholder text")
	}
}

func TestObserverPane_ViewWithResponse(t *testing.T) {
	pane := NewObserverPane(nil)
	pane.SetSize(80, 24)
	pane.response = "This is a test response"

	view := pane.View()
	if !strings.Contains(view, "This is a test response") {
		t.Error("expected view to contain response text")
	}
}

func TestObserverPane_ViewWithError(t *testing.T) {
	pane := NewObserverPane(nil)
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

func TestObserverPane_ViewWithLoading(t *testing.T) {
	pane := NewObserverPane(nil)
	pane.SetSize(80, 24)
	pane.loading = true
	pane.startedAt = time.Now()

	view := pane.View()
	if !strings.Contains(view, "Asking Claude") {
		t.Error("expected view to show loading status")
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 8, "hello..."},
		{"hello", 3, "hel"},
		{"hello", 5, "hello"},
		{"hi", 2, "hi"},
	}

	for _, tt := range tests {
		result := truncateString(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncateString(%q, %d) = %q, want %q",
				tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestWordWrap(t *testing.T) {
	tests := []struct {
		input    string
		width    int
		expected string
	}{
		{"hello world", 20, "hello world"},
		{"hello world", 5, "hello\nworld"},
		{"longword", 4, "long\nword"},
		// Short width wraps at word boundaries
		{"hello there", 8, "hello\nthere"},
		// Preserves existing newlines
		{"line1\nline2", 20, "line1\nline2"},
	}

	for _, tt := range tests {
		result := wordWrap(tt.input, tt.width)
		if result != tt.expected {
			t.Errorf("wordWrap(%q, %d) = %q, want %q",
				tt.input, tt.width, result, tt.expected)
		}
	}
}

// testError is a simple error type for testing.
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
