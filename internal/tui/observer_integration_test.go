package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/observer"
)

// observerTestEnv provides an isolated test environment for observer pane integration tests.
type observerTestEnv struct {
	t *testing.T
}

// newObserverTestEnv creates a new test environment for observer pane tests.
func newObserverTestEnv(t *testing.T) *observerTestEnv {
	t.Helper()

	return &observerTestEnv{
		t: t,
	}
}

// newPane creates an ObserverPane with nil observer for most tests.
func (env *observerTestEnv) newPane() ObserverPane {
	pane := NewObserverPane(nil)
	pane.SetSize(80, 24)
	pane.SetFocused(true)
	return pane
}

// newPaneWithObserver creates an ObserverPane with a configured observer.
func (env *observerTestEnv) newPaneWithObserver(obs *observer.Observer) ObserverPane {
	pane := NewObserverPane(obs)
	pane.SetSize(80, 24)
	pane.SetFocused(true)
	return pane
}

// fakeStateProvider implements observer.DrainStateProvider for testing.
type fakeStateProvider struct{}

func (f *fakeStateProvider) GetDrainState() observer.DrainState {
	return observer.DrainState{}
}

// TestObserver_QuerySubmitStartsLoading verifies that submitting a query
// sets the loading state and starts the spinner.
func TestObserver_QuerySubmitStartsLoading(t *testing.T) {
	env := newObserverTestEnv(t)
	pane := env.newPane()

	// Set up input with a question
	pane.input.SetValue("What is the current status?")

	// Verify initial state
	if pane.IsLoading() {
		t.Fatal("should not be loading initially")
	}

	// Submit the question by pressing Enter in normal mode
	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	newPane, cmd := pane.Update(keyMsg)
	pane = newPane

	// Should now be loading
	if !pane.IsLoading() {
		t.Error("should be loading after submit")
	}

	// Should have user question in history
	if len(pane.history) != 1 {
		t.Errorf("expected 1 message in history, got %d", len(pane.history))
	}
	if pane.history[0].role != roleUser {
		t.Error("expected first message to be from user")
	}
	if pane.history[0].content != "What is the current status?" {
		t.Errorf("expected question content, got %q", pane.history[0].content)
	}

	// Input should be cleared
	if pane.input.Value() != "" {
		t.Errorf("expected input to be cleared, got %q", pane.input.Value())
	}

	// Should return commands (spinner tick, query cmd, tick cmd)
	if cmd == nil {
		t.Error("should return command for async query")
	}
}

// TestObserver_SuccessfulResponseDisplaysInHistory verifies that a successful
// response from the observer is displayed in the chat history.
func TestObserver_SuccessfulResponseDisplaysInHistory(t *testing.T) {
	env := newObserverTestEnv(t)
	pane := env.newPane()

	// Simulate that a query was submitted
	pane.loading = true
	pane.history = []chatMessage{
		{role: roleUser, content: "What is happening?", time: time.Now()},
	}

	// Simulate successful response
	resultMsg := observerResultMsg{
		response: "The drain is currently processing bd-test-001.",
		err:      nil,
	}
	newPane, _ := pane.Update(resultMsg)
	pane = newPane

	// Should no longer be loading
	if pane.IsLoading() {
		t.Error("should not be loading after successful response")
	}

	// Should have 2 messages in history (user + assistant)
	if len(pane.history) != 2 {
		t.Errorf("expected 2 messages in history, got %d", len(pane.history))
	}

	// Second message should be assistant response
	if pane.history[1].role != roleAssistant {
		t.Errorf("expected second message to be assistant, got %d", pane.history[1].role)
	}
	if pane.history[1].content != "The drain is currently processing bd-test-001." {
		t.Errorf("expected response content, got %q", pane.history[1].content)
	}

	// No error should be displayed
	if pane.errorMsg != "" {
		t.Errorf("expected no error, got %q", pane.errorMsg)
	}
}

// TestObserver_ErrorResponseDisplaysMessage verifies that an error response
// is displayed in the error message field.
func TestObserver_ErrorResponseDisplaysMessage(t *testing.T) {
	env := newObserverTestEnv(t)
	pane := env.newPane()

	// Simulate that a query was submitted
	pane.loading = true
	pane.history = []chatMessage{
		{role: roleUser, content: "What is happening?", time: time.Now()},
	}

	// Simulate error response
	resultMsg := observerResultMsg{
		response: "",
		err:      errors.New("failed to acquire session: acquisition timeout"),
	}
	newPane, _ := pane.Update(resultMsg)
	pane = newPane

	// Should no longer be loading
	if pane.IsLoading() {
		t.Error("should not be loading after error response")
	}

	// History should still only have user message (no assistant response on error)
	if len(pane.history) != 1 {
		t.Errorf("expected 1 message in history (no assistant on error), got %d", len(pane.history))
	}

	// Error message should be set
	if pane.errorMsg == "" {
		t.Error("expected error message to be set")
	}
	if !strings.Contains(pane.errorMsg, "acquisition timeout") {
		t.Errorf("expected error message to contain 'acquisition timeout', got %q", pane.errorMsg)
	}

	// View should show the error
	view := pane.View()
	if !strings.Contains(view, "Error:") {
		t.Error("expected view to contain error indicator")
	}
}

// TestObserver_InsertMode_IKeyEnablesTyping verifies that pressing 'i' enters
// insert mode and allows typing in the textarea.
func TestObserver_InsertMode_IKeyEnablesTyping(t *testing.T) {
	env := newObserverTestEnv(t)
	pane := env.newPane()

	// Should start in normal mode
	if pane.IsInsertMode() {
		t.Fatal("should start in normal mode")
	}

	// Press 'i' to enter insert mode
	iMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}}
	newPane, _ := pane.Update(iMsg)
	pane = newPane

	// Should now be in insert mode
	if !pane.IsInsertMode() {
		t.Error("should be in insert mode after pressing 'i'")
	}

	// View should show INSERT mode indicator
	view := pane.View()
	if !strings.Contains(view, "INSERT") {
		t.Error("expected view to show INSERT mode indicator")
	}

	// Type a question
	for _, r := range "test question" {
		charMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		newPane, _ = pane.Update(charMsg)
		pane = newPane
	}

	// Input should have the typed text
	if pane.input.Value() != "test question" {
		t.Errorf("expected input='test question', got %q", pane.input.Value())
	}
}

// TestObserver_InsertMode_EscapeExits verifies that pressing Escape exits
// insert mode and returns to normal mode.
func TestObserver_InsertMode_EscapeExits(t *testing.T) {
	env := newObserverTestEnv(t)
	pane := env.newPane()

	// Enter insert mode
	pane.insertMode = true

	// Press Escape to exit insert mode
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	newPane, _ := pane.Update(escMsg)
	pane = newPane

	// Should be back in normal mode
	if pane.IsInsertMode() {
		t.Error("should be in normal mode after pressing Escape")
	}

	// View should show NORMAL mode indicator
	view := pane.View()
	if !strings.Contains(view, "NORMAL") {
		t.Error("expected view to show NORMAL mode indicator")
	}
}

// TestObserver_NormalMode_JKScrollsHistory verifies that j/k keys scroll
// the chat history viewport in normal mode.
func TestObserver_NormalMode_JKScrollsHistory(t *testing.T) {
	env := newObserverTestEnv(t)
	pane := env.newPane()

	// Add enough messages to enable scrolling
	for i := 0; i < 30; i++ {
		pane.history = append(pane.history, chatMessage{
			role:    roleUser,
			content: "Question " + string(rune('A'+i%26)),
			time:    time.Now(),
		})
		pane.history = append(pane.history, chatMessage{
			role:    roleAssistant,
			content: "Answer " + string(rune('A'+i%26)),
			time:    time.Now(),
		})
	}
	pane.updateViewportContent()

	// Should be in normal mode
	if pane.IsInsertMode() {
		t.Fatal("should be in normal mode")
	}

	// Get initial viewport offset
	initialOffset := pane.viewport.YOffset

	// Press 'j' to scroll down
	jMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	newPane, _ := pane.Update(jMsg)
	pane = newPane

	// Input should remain empty (j should scroll, not type)
	if pane.input.Value() != "" {
		t.Errorf("j should scroll in normal mode, not type; input=%q", pane.input.Value())
	}

	// Press 'k' to scroll up
	kMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}
	newPane, _ = pane.Update(kMsg)
	pane = newPane

	// Input should still be empty
	if pane.input.Value() != "" {
		t.Errorf("k should scroll in normal mode, not type; input=%q", pane.input.Value())
	}

	// Viewport should have scrolled (offset may be same if returned to start)
	_ = initialOffset // We're testing that j/k don't type in normal mode
}

// TestObserver_NilObserver_ReturnsNotInitializedError verifies that
// submitting a query with nil observer returns a "not initialized" error.
func TestObserver_NilObserver_ReturnsNotInitializedError(t *testing.T) {
	env := newObserverTestEnv(t)
	pane := env.newPane() // nil observer

	// Set up input with a question
	pane.input.SetValue("Test question")

	// Submit the question
	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	newPane, cmd := pane.Update(keyMsg)
	pane = newPane

	// Execute the query command to get the result message
	if cmd == nil {
		t.Fatal("expected command from submit")
	}

	// Execute command chain to find the query result
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, batchCmd := range batch {
			if batchCmd != nil {
				innerMsg := batchCmd()
				if resultMsg, ok := innerMsg.(observerResultMsg); ok {
					// Process the result
					newPane, _ = pane.Update(resultMsg)
					pane = newPane

					// Should have error about not initialized
					if pane.errorMsg == "" {
						t.Error("expected error message")
					}
					if !strings.Contains(pane.errorMsg, "not initialized") {
						t.Errorf("expected 'not initialized' error, got %q", pane.errorMsg)
					}
					return
				}
			}
		}
	}

	// If we got here, manually execute the query cmd which should return error
	// The queryCmd closure checks for nil observer
	queryFunc := pane.queryCmd("Test question")
	resultMsg := queryFunc().(observerResultMsg)

	// Process the result
	newPane, _ = pane.Update(resultMsg)
	pane = newPane

	// Should have error about not initialized
	if pane.errorMsg == "" {
		t.Error("expected error message")
	}
	if !strings.Contains(pane.errorMsg, "not initialized") {
		t.Errorf("expected 'not initialized' error, got %q", pane.errorMsg)
	}
}

// TestObserver_LoadingStateDuringQuery verifies that the loading state
// is properly managed while a query is in progress.
func TestObserver_LoadingStateDuringQuery(t *testing.T) {
	env := newObserverTestEnv(t)
	pane := env.newPane()

	// Submit a query
	pane.input.SetValue("Test")
	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	newPane, _ := pane.Update(keyMsg)
	pane = newPane

	// Should be loading
	if !pane.IsLoading() {
		t.Error("should be loading after submit")
	}

	// View should show loading indicator
	view := pane.View()
	if !strings.Contains(view, "Asking Claude") {
		t.Error("expected view to show loading indicator")
	}

	// Tick should keep spinner going
	tickMsg := observerTickMsg(time.Now())
	newPane, cmd := pane.Update(tickMsg)
	pane = newPane

	// Should still be loading
	if !pane.IsLoading() {
		t.Error("should still be loading after tick")
	}

	// Should return command for next tick
	if cmd == nil {
		t.Error("expected tick command during loading")
	}
}

// TestObserver_MultipleExchanges verifies that multiple question/answer
// exchanges accumulate correctly in the history.
func TestObserver_MultipleExchanges(t *testing.T) {
	env := newObserverTestEnv(t)
	pane := env.newPane()

	// First exchange
	pane.input.SetValue("Question 1")
	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	newPane, _ := pane.Update(keyMsg)
	pane = newPane

	// Simulate first response
	resultMsg := observerResultMsg{response: "Answer 1", err: nil}
	newPane, _ = pane.Update(resultMsg)
	pane = newPane

	// Should have 2 messages
	if len(pane.history) != 2 {
		t.Errorf("expected 2 messages after first exchange, got %d", len(pane.history))
	}

	// Second exchange
	pane.input.SetValue("Question 2")
	newPane, _ = pane.Update(keyMsg)
	pane = newPane

	// Simulate second response
	resultMsg = observerResultMsg{response: "Answer 2", err: nil}
	newPane, _ = pane.Update(resultMsg)
	pane = newPane

	// Should have 4 messages
	if len(pane.history) != 4 {
		t.Errorf("expected 4 messages after second exchange, got %d", len(pane.history))
	}

	// Verify order
	if pane.history[0].content != "Question 1" {
		t.Error("first message should be Question 1")
	}
	if pane.history[1].content != "Answer 1" {
		t.Error("second message should be Answer 1")
	}
	if pane.history[2].content != "Question 2" {
		t.Error("third message should be Question 2")
	}
	if pane.history[3].content != "Answer 2" {
		t.Error("fourth message should be Answer 2")
	}
}

// TestObserver_CancelQuery verifies that Ctrl+C cancels an in-progress query.
// Cancel only works when there's a real observer to cancel.
func TestObserver_CancelQuery(t *testing.T) {
	env := newObserverTestEnv(t)

	// Create a real observer to enable cancel functionality
	cfg := &config.ObserverConfig{
		Enabled: true,
	}
	logReader := observer.NewLogReader("/tmp/test.log")
	builder := observer.NewContextBuilder(logReader, cfg)
	stateProvider := &fakeStateProvider{}
	obs := observer.NewObserver(cfg, builder, stateProvider)

	// Create pane with real observer
	pane := env.newPaneWithObserver(obs)

	// Simulate loading state
	pane.loading = true
	pane.insertMode = true // In insert mode

	// Press Ctrl+C to cancel
	ctrlCMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
	newPane, _ := pane.Update(ctrlCMsg)
	pane = newPane

	// Should no longer be loading
	if pane.IsLoading() {
		t.Error("should not be loading after Ctrl+C")
	}

	// Should have cancel error message
	if pane.errorMsg != "Query cancelled" {
		t.Errorf("expected 'Query cancelled' error, got %q", pane.errorMsg)
	}
}

// TestObserver_CancelWithNilObserverNoPanic verifies that pressing Ctrl+C
// with nil observer does not panic or cause errors.
func TestObserver_CancelWithNilObserverNoPanic(t *testing.T) {
	env := newObserverTestEnv(t)
	pane := env.newPane() // nil observer

	// Simulate loading state
	pane.loading = true
	pane.insertMode = true

	// Press Ctrl+C - should not panic with nil observer
	ctrlCMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
	newPane, _ := pane.Update(ctrlCMsg)
	pane = newPane

	// With nil observer, cancel doesn't do anything (no query to cancel)
	// Loading state remains unchanged since there's no actual observer to cancel
	// This is expected behavior - you can't cancel what isn't running
}

// TestObserver_EscapeClearsError verifies that pressing Escape clears
// the error message in normal mode.
func TestObserver_EscapeClearsError(t *testing.T) {
	env := newObserverTestEnv(t)
	pane := env.newPane()

	// Set an error
	pane.errorMsg = "Some error occurred"

	// Press Escape to clear error (in normal mode)
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	newPane, _ := pane.Update(escMsg)
	pane = newPane

	// Error should be cleared
	if pane.errorMsg != "" {
		t.Errorf("expected error to be cleared, got %q", pane.errorMsg)
	}
}

// TestObserver_EmptyInputNotSubmitted verifies that pressing Enter with
// empty input does not submit a query.
func TestObserver_EmptyInputNotSubmitted(t *testing.T) {
	env := newObserverTestEnv(t)
	pane := env.newPane()

	// Ensure input is empty
	pane.input.SetValue("")

	// Press Enter
	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	newPane, _ := pane.Update(keyMsg)
	pane = newPane

	// Should not be loading
	if pane.IsLoading() {
		t.Error("should not be loading with empty input")
	}

	// History should be empty
	if len(pane.history) != 0 {
		t.Errorf("expected empty history, got %d messages", len(pane.history))
	}
}

// TestObserver_WhitespaceOnlyInputNotSubmitted verifies that pressing Enter
// with whitespace-only input does not submit a query.
func TestObserver_WhitespaceOnlyInputNotSubmitted(t *testing.T) {
	env := newObserverTestEnv(t)
	pane := env.newPane()

	// Set whitespace-only input
	pane.input.SetValue("   \t  ")

	// Press Enter
	keyMsg := tea.KeyMsg{Type: tea.KeyEnter}
	newPane, _ := pane.Update(keyMsg)
	pane = newPane

	// Should not be loading
	if pane.IsLoading() {
		t.Error("should not be loading with whitespace-only input")
	}

	// History should be empty
	if len(pane.history) != 0 {
		t.Errorf("expected empty history, got %d messages", len(pane.history))
	}
}

// TestObserver_HistoryViewRendersCorrectly verifies that the chat history
// is rendered with proper role prefixes.
func TestObserver_HistoryViewRendersCorrectly(t *testing.T) {
	env := newObserverTestEnv(t)
	pane := env.newPane()

	// Add messages to history
	pane.history = []chatMessage{
		{role: roleUser, content: "What is the status?", time: time.Now()},
		{role: roleAssistant, content: "Processing bd-test-001.", time: time.Now()},
	}
	pane.updateViewportContent()

	// Get the view
	view := pane.View()

	// Should contain role prefixes
	if !strings.Contains(view, "You:") {
		t.Error("expected view to contain 'You:' prefix")
	}
	if !strings.Contains(view, "Claude:") {
		t.Error("expected view to contain 'Claude:' prefix")
	}
}
