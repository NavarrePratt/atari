package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/npratt/atari/internal/events"
)

// TestCallback_PressP_TriggersPauseCallback verifies that pressing 'p' triggers
// the onPause callback and sets status to "pausing...".
func TestCallback_PressP_TriggersPauseCallback(t *testing.T) {
	env := newTUITestEnv(t)
	defer env.close()

	m := env.newModel()
	m.width = 100
	m.height = 30
	m.status = "idle"

	// Verify callback not called initially
	if env.pauseCalled {
		t.Fatal("pause callback should not be called initially")
	}

	// Send 'p' key press through Update
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	newM, _ := m.Update(keyMsg)
	resultM := newM.(model)

	// Verify onPause callback was invoked
	if !env.pauseCalled {
		t.Error("onPause callback should have been called")
	}

	// Verify status changed to "pausing..."
	if resultM.status != "pausing..." {
		t.Errorf("expected status 'pausing...', got %q", resultM.status)
	}
}

// TestCallback_PressR_TriggersResumeCallback verifies that pressing 'r' triggers
// the onResume callback and sets status to "resuming...".
func TestCallback_PressR_TriggersResumeCallback(t *testing.T) {
	env := newTUITestEnv(t)
	defer env.close()

	m := env.newModel()
	m.width = 100
	m.height = 30
	m.status = "paused"

	// Verify callback not called initially
	if env.resumeCalled {
		t.Fatal("resume callback should not be called initially")
	}

	// Send 'r' key press through Update
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
	newM, _ := m.Update(keyMsg)
	resultM := newM.(model)

	// Verify onResume callback was invoked
	if !env.resumeCalled {
		t.Error("onResume callback should have been called")
	}

	// Verify status changed to "resuming..."
	if resultM.status != "resuming..." {
		t.Errorf("expected status 'resuming...', got %q", resultM.status)
	}
}

// TestCallback_PressQ_TriggersQuitCallback verifies that pressing 'q' triggers
// the onQuit callback and returns tea.Quit.
func TestCallback_PressQ_TriggersQuitCallback(t *testing.T) {
	env := newTUITestEnv(t)
	defer env.close()

	m := env.newModel()
	m.width = 100
	m.height = 30
	m.status = "idle"

	// Verify callback not called initially
	if env.quitCalled {
		t.Fatal("quit callback should not be called initially")
	}

	// Send 'q' key press through Update
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd := m.Update(keyMsg)

	// Verify onQuit callback was invoked
	if !env.quitCalled {
		t.Error("onQuit callback should have been called")
	}

	// Verify tea.Quit command is returned
	if cmd == nil {
		t.Fatal("expected tea.Quit command, got nil")
	}

	// Execute the command and verify it returns tea.QuitMsg
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}

// TestCallback_DrainStateChangedEvent_PausedState verifies that a DrainStateChangedEvent
// with To="paused" updates the TUI status to "paused".
func TestCallback_DrainStateChangedEvent_PausedState(t *testing.T) {
	env := newTUITestEnv(t)
	defer env.close()

	m := env.newModel()
	m.width = 100
	m.height = 30
	m.status = "pausing..."

	// Create DrainStateChangedEvent indicating transition to paused
	evt := &events.DrainStateChangedEvent{
		BaseEvent: events.NewInternalEvent(events.EventDrainStateChanged),
		From:      "running",
		To:        "paused",
	}

	// Send through Update as eventMsg
	newM, cmd := m.Update(eventMsg(evt))
	resultM := newM.(model)

	// Verify status updated to paused
	if resultM.status != "paused" {
		t.Errorf("expected status 'paused', got %q", resultM.status)
	}

	// Verify event was logged
	if len(resultM.eventLines) == 0 {
		t.Error("expected event to be logged")
	}

	// Should return command to wait for next event
	if cmd == nil {
		t.Error("should return command to wait for next event")
	}
}

// TestCallback_DrainStateChangedEvent_ResumedState verifies that a DrainStateChangedEvent
// with To="running" updates the TUI status correctly after a resume.
func TestCallback_DrainStateChangedEvent_ResumedState(t *testing.T) {
	env := newTUITestEnv(t)
	defer env.close()

	m := env.newModel()
	m.width = 100
	m.height = 30
	m.status = "resuming..."

	// Create DrainStateChangedEvent indicating transition to running
	evt := &events.DrainStateChangedEvent{
		BaseEvent: events.NewInternalEvent(events.EventDrainStateChanged),
		From:      "paused",
		To:        "running",
	}

	// Send through Update as eventMsg
	newM, cmd := m.Update(eventMsg(evt))
	resultM := newM.(model)

	// Verify status updated to running
	if resultM.status != "running" {
		t.Errorf("expected status 'running', got %q", resultM.status)
	}

	// Verify event was logged
	if len(resultM.eventLines) == 0 {
		t.Error("expected event to be logged")
	}

	// Should return command to wait for next event
	if cmd == nil {
		t.Error("should return command to wait for next event")
	}
}

// TestCallback_NilCallbacks_NoPanic verifies that pressing p/r/q with nil callbacks
// does not cause a panic.
func TestCallback_NilCallbacks_NoPanic(t *testing.T) {
	env := newTUITestEnv(t)
	defer env.close()

	// Create model with nil callbacks
	m := newModel(
		env.eventChan,
		nil, // onPause
		nil, // onResume
		nil, // onQuit
		env.statsGetter,
		nil, // observer
		nil, // graph fetcher
	)
	m.width = 100
	m.height = 30

	// These should not panic even with nil callbacks
	tests := []struct {
		name string
		key  rune
	}{
		{"press p with nil onPause", 'p'},
		{"press r with nil onResume", 'r'},
		{"press q with nil onQuit", 'q'},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic occurred: %v", r)
				}
			}()

			keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tt.key}}
			_, _ = m.Update(keyMsg)
		})
	}
}

// TestCallback_RoundTrip_PauseToConfirmation tests the full round-trip:
// key press -> callback -> event -> status update.
func TestCallback_RoundTrip_PauseToConfirmation(t *testing.T) {
	env := newTUITestEnv(t)
	defer env.close()

	m := env.newModel()
	m.width = 100
	m.height = 30
	m.status = "running"

	// Step 1: Press 'p' to initiate pause
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	newM, _ := m.Update(keyMsg)
	m = newM.(model)

	// Verify intermediate state
	if m.status != "pausing..." {
		t.Errorf("after press p, expected status 'pausing...', got %q", m.status)
	}
	if !env.pauseCalled {
		t.Error("onPause callback should have been called")
	}

	// Step 2: Controller confirms pause via DrainStateChangedEvent
	evt := &events.DrainStateChangedEvent{
		BaseEvent: events.NewInternalEvent(events.EventDrainStateChanged),
		From:      "running",
		To:        "paused",
	}
	newM, _ = m.Update(eventMsg(evt))
	m = newM.(model)

	// Verify final state
	if m.status != "paused" {
		t.Errorf("after confirmation event, expected status 'paused', got %q", m.status)
	}
}

// TestCallback_RoundTrip_ResumeToConfirmation tests the full round-trip:
// key press -> callback -> event -> status update for resume.
func TestCallback_RoundTrip_ResumeToConfirmation(t *testing.T) {
	env := newTUITestEnv(t)
	defer env.close()

	m := env.newModel()
	m.width = 100
	m.height = 30
	m.status = "paused"

	// Step 1: Press 'r' to initiate resume
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}
	newM, _ := m.Update(keyMsg)
	m = newM.(model)

	// Verify intermediate state
	if m.status != "resuming..." {
		t.Errorf("after press r, expected status 'resuming...', got %q", m.status)
	}
	if !env.resumeCalled {
		t.Error("onResume callback should have been called")
	}

	// Step 2: Controller confirms resume via DrainStateChangedEvent
	evt := &events.DrainStateChangedEvent{
		BaseEvent: events.NewInternalEvent(events.EventDrainStateChanged),
		From:      "paused",
		To:        "running",
	}
	newM, _ = m.Update(eventMsg(evt))
	m = newM.(model)

	// Verify final state
	if m.status != "running" {
		t.Errorf("after confirmation event, expected status 'running', got %q", m.status)
	}
}
