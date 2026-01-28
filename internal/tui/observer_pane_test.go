package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestNewObserverPane(t *testing.T) {
	pane := NewObserverPane(nil)

	if pane.loading {
		t.Error("expected loading to be false initially")
	}
	if len(pane.history) != 0 {
		t.Error("expected history to be empty initially")
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

	// Type some text (manually set for test)
	pane.input.SetValue("test question")

	// Press Esc to clear (in normal mode)
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	newPane, _ := pane.Update(msg)

	if newPane.input.Value() != "" {
		t.Errorf("expected input to be cleared, got %q", newPane.input.Value())
	}
}

func TestObserverPane_InsertMode_InitialState(t *testing.T) {
	pane := NewObserverPane(nil)
	pane.SetSize(80, 24)

	if pane.IsInsertMode() {
		t.Error("expected pane to start in normal mode, not insert mode")
	}
}

func TestObserverPane_InsertMode_EnterWithI(t *testing.T) {
	pane := NewObserverPane(nil)
	pane.SetSize(80, 24)
	pane.SetFocused(true)

	// Press 'i' to enter insert mode
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}}
	newPane, _ := pane.Update(msg)

	if !newPane.IsInsertMode() {
		t.Error("expected pane to be in insert mode after pressing 'i'")
	}
}

func TestObserverPane_InsertMode_ExitWithEsc(t *testing.T) {
	pane := NewObserverPane(nil)
	pane.SetSize(80, 24)
	pane.SetFocused(true)
	pane.insertMode = true

	// Press Esc to exit insert mode
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	newPane, _ := pane.Update(msg)

	if newPane.IsInsertMode() {
		t.Error("expected pane to exit insert mode after pressing Esc")
	}
}

func TestObserverPane_InsertMode_TypingWorks(t *testing.T) {
	pane := NewObserverPane(nil)
	pane.SetSize(80, 24)
	pane.SetFocused(true)

	// Enter insert mode
	iMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}}
	pane, _ = pane.Update(iMsg)

	// Type some characters
	for _, r := range "hello" {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		pane, _ = pane.Update(msg)
	}

	if pane.input.Value() != "hello" {
		t.Errorf("expected input='hello', got %q", pane.input.Value())
	}
}

func TestObserverPane_NormalMode_JKScrollsViewport(t *testing.T) {
	pane := NewObserverPane(nil)
	pane.SetSize(80, 24)
	pane.SetFocused(true)

	// Ensure in normal mode (not insert mode)
	if pane.IsInsertMode() {
		t.Fatal("expected to start in normal mode")
	}

	// Press 'j' - should NOT add to input
	jMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	newPane, _ := pane.Update(jMsg)

	if newPane.input.Value() != "" {
		t.Errorf("expected input to remain empty in normal mode, got %q", newPane.input.Value())
	}
}

func TestObserverPane_InsertMode_JKTypesInTextarea(t *testing.T) {
	pane := NewObserverPane(nil)
	pane.SetSize(80, 24)
	pane.SetFocused(true)

	// Enter insert mode first
	iMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}}
	pane, _ = pane.Update(iMsg)

	// Press 'j' - should add to input in insert mode
	jMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	newPane, _ := pane.Update(jMsg)

	if newPane.input.Value() != "j" {
		t.Errorf("expected input='j' in insert mode, got %q", newPane.input.Value())
	}
}

func TestObserverPane_InsertMode_ExitOnUnfocus(t *testing.T) {
	pane := NewObserverPane(nil)
	pane.SetSize(80, 24)
	pane.SetFocused(true)
	pane.insertMode = true

	// Unfocus the pane
	pane.SetFocused(false)

	if pane.IsInsertMode() {
		t.Error("expected insert mode to be disabled when pane is unfocused")
	}
}

func TestObserverPane_ViewShowsModeIndicator(t *testing.T) {
	pane := NewObserverPane(nil)
	pane.SetSize(80, 24)
	pane.SetFocused(true)

	// Normal mode
	view := pane.View()
	if !strings.Contains(view, "NORMAL") {
		t.Error("expected view to show NORMAL mode indicator")
	}

	// Enter insert mode
	pane.insertMode = true
	view = pane.View()
	if !strings.Contains(view, "INSERT") {
		t.Error("expected view to show INSERT mode indicator")
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
	pane.SetSize(80, 24)
	pane.loading = true

	msg := observerResultMsg{response: "test response", err: nil}
	newPane, _ := pane.Update(msg)

	if newPane.loading {
		t.Error("expected loading to be false after result")
	}
	if len(newPane.history) != 1 {
		t.Errorf("expected 1 message in history, got %d", len(newPane.history))
	}
	if newPane.history[0].content != "test response" {
		t.Errorf("expected response='test response', got %q", newPane.history[0].content)
	}
	if newPane.history[0].role != roleAssistant {
		t.Errorf("expected role=roleAssistant, got %d", newPane.history[0].role)
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
	if len(newPane.history) != 0 {
		t.Errorf("expected no messages in history on error, got %d", len(newPane.history))
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
	pane.SetSize(80, 24)
	pane.history = []chatMessage{
		{role: roleUser, content: "question"},
		{role: roleAssistant, content: "answer"},
	}
	pane.errorMsg = "some error"

	pane.ClearResponse()

	if len(pane.history) != 0 {
		t.Errorf("expected history to be cleared, got %d messages", len(pane.history))
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
	// Should contain placeholder text (updated for chat history)
	if !strings.Contains(view, "Conversation history") {
		t.Error("expected view to contain placeholder text about conversation history")
	}
}

func TestObserverPane_ViewWithHistory(t *testing.T) {
	pane := NewObserverPane(nil)
	pane.SetSize(80, 24)
	pane.history = []chatMessage{
		{role: roleUser, content: "What is happening?", time: time.Now()},
		{role: roleAssistant, content: "This is a test response", time: time.Now()},
	}
	pane.updateViewportContent()

	view := pane.View()
	if !strings.Contains(view, "You:") {
		t.Error("expected view to contain 'You:' prefix")
	}
	if !strings.Contains(view, "Claude:") {
		t.Error("expected view to contain 'Claude:' prefix")
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

func TestObserverPane_HistoryScrolling(t *testing.T) {
	pane := NewObserverPane(nil)
	pane.SetSize(80, 24)

	// Add multiple messages to have content to scroll
	for i := 0; i < 20; i++ {
		pane.history = append(pane.history, chatMessage{
			role:    roleUser,
			content: "Question " + string(rune('A'+i)),
			time:    time.Now(),
		})
		pane.history = append(pane.history, chatMessage{
			role:    roleAssistant,
			content: "Answer " + string(rune('A'+i)),
			time:    time.Now(),
		})
	}
	pane.updateViewportContent()

	// Test scroll up
	msg := tea.KeyMsg{Type: tea.KeyUp}
	newPane, _ := pane.Update(msg)
	_ = newPane // Scrolling should work without error

	// Test scroll down
	msg = tea.KeyMsg{Type: tea.KeyDown}
	newPane, _ = pane.Update(msg)
	_ = newPane

	// Test page up
	msg = tea.KeyMsg{Type: tea.KeyPgUp}
	newPane, _ = pane.Update(msg)
	_ = newPane

	// Test page down
	msg = tea.KeyMsg{Type: tea.KeyPgDown}
	newPane, _ = pane.Update(msg)
	_ = newPane
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

func TestObserverPane_ViewRenderedHeight(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{name: "standard terminal", width: 80, height: 24},
		{name: "small height", width: 80, height: 10},
		{name: "tall terminal", width: 80, height: 50},
		{name: "wide terminal", width: 120, height: 30},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pane := NewObserverPane(nil)
			pane.SetSize(tt.width, tt.height)
			pane.SetFocused(true)

			view := pane.View()
			renderedHeight := lipgloss.Height(view)

			if renderedHeight != tt.height {
				t.Errorf("rendered height=%d, expected=%d\nview:\n%s", renderedHeight, tt.height, view)
			}
		})
	}
}

func TestObserverPane_ViewRenderedHeightWithHistory(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{name: "standard terminal with history", width: 80, height: 24},
		{name: "small height with history", width: 80, height: 10},
		{name: "tall terminal with history", width: 80, height: 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pane := NewObserverPane(nil)
			pane.SetSize(tt.width, tt.height)
			pane.SetFocused(true)

			// Add some history
			pane.history = []chatMessage{
				{role: roleUser, content: "What is happening?", time: time.Now()},
				{role: roleAssistant, content: "This is a test response with some content.", time: time.Now()},
			}
			pane.updateViewportContent()

			view := pane.View()
			renderedHeight := lipgloss.Height(view)

			if renderedHeight != tt.height {
				t.Errorf("rendered height=%d, expected=%d\nview:\n%s", renderedHeight, tt.height, view)
			}
		})
	}
}

func TestObserverPane_SetSizeViewConsistency(t *testing.T) {
	tests := []struct {
		name    string
		sizes   [][2]int // sequence of [width, height] to apply
		withHistory bool
	}{
		{
			name:    "resize larger then smaller",
			sizes:   [][2]int{{80, 24}, {100, 30}, {80, 20}},
			withHistory: false,
		},
		{
			name:    "resize height only",
			sizes:   [][2]int{{80, 24}, {80, 30}, {80, 15}},
			withHistory: false,
		},
		{
			name:    "same size multiple times",
			sizes:   [][2]int{{80, 24}, {80, 24}, {80, 24}},
			withHistory: false,
		},
		{
			name:    "resize with history",
			sizes:   [][2]int{{80, 24}, {100, 30}, {80, 20}},
			withHistory: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pane := NewObserverPane(nil)
			pane.SetFocused(true)

			if tt.withHistory {
				pane.history = []chatMessage{
					{role: roleUser, content: "Test question", time: time.Now()},
					{role: roleAssistant, content: "Test response", time: time.Now()},
				}
			}

			for i, size := range tt.sizes {
				width, height := size[0], size[1]
				pane.SetSize(width, height)
				if tt.withHistory {
					pane.updateViewportContent()
				}

				view := pane.View()
				renderedHeight := lipgloss.Height(view)

				if renderedHeight != height {
					t.Errorf("iteration %d: after SetSize(%d, %d), rendered height=%d, expected=%d",
						i, width, height, renderedHeight, height)
				}
			}
		})
	}
}

func TestObserverPane_ViewRenderedHeightMinimum(t *testing.T) {
	pane := NewObserverPane(nil)
	pane.SetFocused(true)

	// ObserverPane structure: history area (1+) + status bar (1) + input area (3) = 5 minimum
	// Test heights starting at 6 to ensure stable behavior
	testHeights := []int{6, 7, 8, 10}
	for _, h := range testHeights {
		pane.SetSize(80, h)
		view := pane.View()
		renderedHeight := lipgloss.Height(view)

		if renderedHeight != h {
			t.Errorf("height=%d: rendered height=%d, expected=%d", h, renderedHeight, h)
		}
	}
}

func TestObserverPane_SetSize_PreservesAtBottom(t *testing.T) {
	pane := NewObserverPane(nil)
	pane.SetSize(80, 24)

	// Add enough history to require scrolling
	for i := 0; i < 30; i++ {
		pane.history = append(pane.history, chatMessage{
			role:    roleUser,
			content: "Question " + string(rune('A'+i%26)),
			time:    time.Now(),
		})
		pane.history = append(pane.history, chatMessage{
			role:    roleAssistant,
			content: "Answer " + string(rune('A'+i%26)) + " with some extra text to wrap",
			time:    time.Now(),
		})
	}
	pane.updateViewportContent()

	// Scroll to bottom
	pane.viewport.GotoBottom()
	if !pane.viewport.AtBottom() {
		t.Fatal("expected viewport to be at bottom after GotoBottom")
	}

	// Resize the pane
	pane.SetSize(100, 30)

	// Should still be at bottom
	if !pane.viewport.AtBottom() {
		t.Error("expected viewport to remain at bottom after resize")
	}

	// Resize again to a smaller size
	pane.SetSize(60, 20)

	// Should still be at bottom
	if !pane.viewport.AtBottom() {
		t.Error("expected viewport to remain at bottom after resize to smaller size")
	}
}

func TestObserverPane_SetSize_ClampsMidScrollPosition(t *testing.T) {
	pane := NewObserverPane(nil)
	pane.SetSize(80, 24)

	// Add enough history to require scrolling
	for i := 0; i < 30; i++ {
		pane.history = append(pane.history, chatMessage{
			role:    roleUser,
			content: "Question " + string(rune('A'+i%26)),
			time:    time.Now(),
		})
		pane.history = append(pane.history, chatMessage{
			role:    roleAssistant,
			content: "Answer " + string(rune('A'+i%26)) + " with some extra text",
			time:    time.Now(),
		})
	}
	pane.updateViewportContent()

	// Scroll to middle (not at bottom)
	pane.viewport.SetYOffset(50)
	initialOffset := pane.viewport.YOffset
	if pane.viewport.AtBottom() {
		t.Fatal("expected viewport to not be at bottom for mid-scroll test")
	}

	// Resize to much smaller (may invalidate old offset)
	pane.SetSize(40, 10)

	// YOffset should be clamped to valid range
	totalLines := pane.viewport.TotalLineCount()
	viewportHeight := pane.viewport.Height
	maxOffset := totalLines - viewportHeight
	if maxOffset < 0 {
		maxOffset = 0
	}

	if pane.viewport.YOffset < 0 {
		t.Errorf("YOffset should not be negative, got %d", pane.viewport.YOffset)
	}
	if pane.viewport.YOffset > maxOffset {
		t.Errorf("YOffset %d exceeds max valid offset %d", pane.viewport.YOffset, maxOffset)
	}

	// Verify offset was actually changed if it was out of bounds
	if initialOffset > maxOffset && pane.viewport.YOffset != maxOffset {
		t.Errorf("expected YOffset to be clamped to %d, got %d", maxOffset, pane.viewport.YOffset)
	}
}

func TestObserverPane_SetSize_EmptyHistory(t *testing.T) {
	pane := NewObserverPane(nil)
	pane.SetSize(80, 24)

	// No history - resize should not panic
	pane.SetSize(100, 30)
	pane.SetSize(40, 10)

	// YOffset should remain valid (0)
	if pane.viewport.YOffset != 0 {
		t.Errorf("expected YOffset=0 with empty history, got %d", pane.viewport.YOffset)
	}
}

func TestObserverPane_SetSize_VerySmallHeight(t *testing.T) {
	pane := NewObserverPane(nil)
	pane.SetSize(80, 24)

	// Add some history
	pane.history = []chatMessage{
		{role: roleUser, content: "Question", time: time.Now()},
		{role: roleAssistant, content: "Answer", time: time.Now()},
	}
	pane.updateViewportContent()
	pane.viewport.GotoBottom()

	// Resize to very small height - should not panic
	pane.SetSize(80, 5)

	// YOffset should be valid (non-negative)
	if pane.viewport.YOffset < 0 {
		t.Errorf("YOffset should not be negative, got %d", pane.viewport.YOffset)
	}
}
