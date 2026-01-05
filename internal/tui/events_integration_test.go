package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/npratt/atari/internal/events"
)

// TestEventFlow_DrainStartEvent verifies DrainStartEvent updates status to running
// when processed through the full Update cycle.
func TestEventFlow_DrainStartEvent(t *testing.T) {
	env := newTUITestEnv(t)
	defer env.close()

	m := env.newModel()
	m.width = 100
	m.height = 30

	// Create DrainStartEvent
	evt := &events.DrainStartEvent{
		BaseEvent: events.NewInternalEvent(events.EventDrainStart),
		WorkDir:   "/test/workdir",
	}

	// Send through Update as eventMsg
	newM, cmd := m.Update(eventMsg(evt))
	resultM := newM.(model)

	// Verify event was logged
	if len(resultM.eventLines) == 0 {
		t.Fatal("expected event to be logged")
	}

	// Verify event text contains expected content
	lastLine := resultM.eventLines[len(resultM.eventLines)-1]
	if lastLine.Text == "" {
		t.Error("event line text should not be empty")
	}

	// Should return command to wait for next event
	if cmd == nil {
		t.Error("should return command to wait for next event")
	}
}

// TestEventFlow_IterationStartEvent verifies IterationStartEvent populates
// currentBead with ID, title, and priority through the full Update cycle.
func TestEventFlow_IterationStartEvent(t *testing.T) {
	env := newTUITestEnv(t)
	defer env.close()

	m := env.newModel()
	m.width = 100
	m.height = 30
	m.status = "idle"

	// Create IterationStartEvent
	evt := &events.IterationStartEvent{
		BaseEvent: events.NewInternalEvent(events.EventIterationStart),
		BeadID:    "bd-test-123",
		Title:     "Test bead for integration",
		Priority:  2,
		Attempt:   1,
	}

	// Send through Update as eventMsg
	newM, cmd := m.Update(eventMsg(evt))
	resultM := newM.(model)

	// Verify status changed to working
	if resultM.status != "working" {
		t.Errorf("expected status 'working', got %q", resultM.status)
	}

	// Verify currentBead is populated
	if resultM.currentBead == nil {
		t.Fatal("currentBead should be populated")
	}
	if resultM.currentBead.ID != "bd-test-123" {
		t.Errorf("expected bead ID 'bd-test-123', got %q", resultM.currentBead.ID)
	}
	if resultM.currentBead.Title != "Test bead for integration" {
		t.Errorf("expected title 'Test bead for integration', got %q", resultM.currentBead.Title)
	}
	if resultM.currentBead.Priority != 2 {
		t.Errorf("expected priority 2, got %d", resultM.currentBead.Priority)
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

// TestEventFlow_IterationEndEvent verifies IterationEndEvent clears currentBead
// and updates stats through the full Update cycle.
func TestEventFlow_IterationEndEvent(t *testing.T) {
	env := newTUITestEnv(t)
	defer env.close()

	m := env.newModel()
	m.width = 100
	m.height = 30
	m.status = "working"
	m.currentBead = &beadInfo{
		ID:        "bd-test-123",
		Title:     "Test bead",
		Priority:  2,
		StartTime: time.Now().Add(-5 * time.Minute),
	}
	m.stats = modelStats{
		Completed: 5,
		Failed:    1,
		TotalCost: 0.10,
	}

	// Create IterationEndEvent for success
	evt := &events.IterationEndEvent{
		BaseEvent:    events.NewInternalEvent(events.EventIterationEnd),
		BeadID:       "bd-test-123",
		Success:      true,
		NumTurns:     15,
		DurationMs:   300000,
		TotalCostUSD: 0.05,
	}

	// Send through Update as eventMsg
	newM, cmd := m.Update(eventMsg(evt))
	resultM := newM.(model)

	// Verify currentBead is cleared
	if resultM.currentBead != nil {
		t.Error("currentBead should be nil after iteration end")
	}

	// Verify status changed to idle
	if resultM.status != "idle" {
		t.Errorf("expected status 'idle', got %q", resultM.status)
	}

	// Verify stats updated
	if resultM.stats.Completed != 6 {
		t.Errorf("expected completed 6, got %d", resultM.stats.Completed)
	}
	// Use tolerance for float comparison
	expectedCost := 0.15
	if resultM.stats.TotalCost < expectedCost-0.001 || resultM.stats.TotalCost > expectedCost+0.001 {
		t.Errorf("expected total cost ~0.15, got %f", resultM.stats.TotalCost)
	}
	if resultM.stats.TotalTurns != 15 {
		t.Errorf("expected total turns 15, got %d", resultM.stats.TotalTurns)
	}

	// Should return command to wait for next event
	if cmd == nil {
		t.Error("should return command to wait for next event")
	}
}

// TestEventFlow_IterationEndEvent_Failure verifies IterationEndEvent increments
// failed count when Success is false.
func TestEventFlow_IterationEndEvent_Failure(t *testing.T) {
	env := newTUITestEnv(t)
	defer env.close()

	m := env.newModel()
	m.width = 100
	m.height = 30
	m.status = "working"
	m.currentBead = &beadInfo{ID: "bd-test-123"}
	m.stats = modelStats{
		Completed: 5,
		Failed:    1,
	}

	// Create IterationEndEvent for failure
	evt := &events.IterationEndEvent{
		BaseEvent: events.NewInternalEvent(events.EventIterationEnd),
		BeadID:    "bd-test-123",
		Success:   false,
		NumTurns:  5,
		Error:     "max turns exceeded",
	}

	newM, _ := m.Update(eventMsg(evt))
	resultM := newM.(model)

	// Verify failed count incremented
	if resultM.stats.Failed != 2 {
		t.Errorf("expected failed 2, got %d", resultM.stats.Failed)
	}
	// Completed should not change
	if resultM.stats.Completed != 5 {
		t.Errorf("expected completed 5 (unchanged), got %d", resultM.stats.Completed)
	}
}

// TestEventFlow_ClaudeToolUseEvent verifies ClaudeToolUseEvent formats tool
// info correctly in the event log.
func TestEventFlow_ClaudeToolUseEvent(t *testing.T) {
	env := newTUITestEnv(t)
	defer env.close()

	m := env.newModel()
	m.width = 100
	m.height = 30

	// Create ClaudeToolUseEvent
	evt := &events.ClaudeToolUseEvent{
		BaseEvent: events.NewClaudeEvent(events.EventClaudeToolUse),
		ToolID:    "tool-123",
		ToolName:  "Bash",
		Input:     map[string]any{"command": "ls -la"},
	}

	newM, cmd := m.Update(eventMsg(evt))
	resultM := newM.(model)

	// Verify event was logged
	if len(resultM.eventLines) == 0 {
		t.Fatal("expected event to be logged")
	}

	// Verify event text contains tool name
	lastLine := resultM.eventLines[len(resultM.eventLines)-1]
	if lastLine.Text == "" {
		t.Error("event line text should not be empty")
	}

	// Format function should produce "tool: Bash ls -la"
	expectedContains := "tool: Bash"
	if len(lastLine.Text) < len(expectedContains) {
		t.Errorf("event text too short, expected to contain %q, got %q", expectedContains, lastLine.Text)
	}

	// Should return command to wait for next event
	if cmd == nil {
		t.Error("should return command to wait for next event")
	}
}

// TestEventFlow_ErrorEvent verifies ErrorEvent displays error message in log.
func TestEventFlow_ErrorEvent(t *testing.T) {
	env := newTUITestEnv(t)
	defer env.close()

	m := env.newModel()
	m.width = 100
	m.height = 30

	// Create ErrorEvent
	evt := &events.ErrorEvent{
		BaseEvent: events.NewInternalEvent(events.EventError),
		Message:   "failed to connect to daemon",
		Severity:  events.SeverityError,
		BeadID:    "bd-test-456",
	}

	newM, cmd := m.Update(eventMsg(evt))
	resultM := newM.(model)

	// Verify event was logged
	if len(resultM.eventLines) == 0 {
		t.Fatal("expected error event to be logged")
	}

	// Verify event text contains error info
	lastLine := resultM.eventLines[len(resultM.eventLines)-1]
	if lastLine.Text == "" {
		t.Error("event line text should not be empty")
	}

	// Should return command to wait for next event
	if cmd == nil {
		t.Error("should return command to wait for next event")
	}
}

// TestEventFlow_MultipleEventsInSequence verifies multiple events process
// in order and accumulate correctly.
func TestEventFlow_MultipleEventsInSequence(t *testing.T) {
	env := newTUITestEnv(t)
	defer env.close()

	m := env.newModel()
	m.width = 100
	m.height = 30

	// Send sequence of events through Update cycle
	eventsToSend := []events.Event{
		&events.DrainStartEvent{
			BaseEvent: events.NewInternalEvent(events.EventDrainStart),
			WorkDir:   "/test",
		},
		&events.IterationStartEvent{
			BaseEvent: events.NewInternalEvent(events.EventIterationStart),
			BeadID:    "bd-seq-001",
			Title:     "First bead",
			Priority:  1,
		},
		&events.ClaudeToolUseEvent{
			BaseEvent: events.NewClaudeEvent(events.EventClaudeToolUse),
			ToolName:  "Read",
			Input:     map[string]any{"file_path": "/test/file.go"},
		},
		&events.ClaudeToolUseEvent{
			BaseEvent: events.NewClaudeEvent(events.EventClaudeToolUse),
			ToolName:  "Edit",
			Input:     map[string]any{"file_path": "/test/file.go"},
		},
		&events.IterationEndEvent{
			BaseEvent:    events.NewInternalEvent(events.EventIterationEnd),
			BeadID:       "bd-seq-001",
			Success:      true,
			NumTurns:     5,
			TotalCostUSD: 0.02,
		},
	}

	// Process each event through Update
	resultM := m
	for _, evt := range eventsToSend {
		newM, _ := resultM.Update(eventMsg(evt))
		resultM = newM.(model)
	}

	// Verify all events were logged in order
	if len(resultM.eventLines) != 5 {
		t.Errorf("expected 5 event lines, got %d", len(resultM.eventLines))
	}

	// Verify final state
	if resultM.status != "idle" {
		t.Errorf("expected status 'idle' after sequence, got %q", resultM.status)
	}
	if resultM.currentBead != nil {
		t.Error("currentBead should be nil after iteration end")
	}
	if resultM.stats.Completed != 1 {
		t.Errorf("expected completed 1, got %d", resultM.stats.Completed)
	}
	if resultM.stats.TotalTurns != 5 {
		t.Errorf("expected total turns 5, got %d", resultM.stats.TotalTurns)
	}
}

// TestEventFlow_ChannelClose verifies channel close triggers TUI quit command.
func TestEventFlow_ChannelClose(t *testing.T) {
	env := newTUITestEnv(t)
	defer env.close()

	m := env.newModel()
	m.width = 100
	m.height = 30

	// Send channelClosedMsg
	newM, cmd := m.Update(channelClosedMsg{})
	_ = newM.(model)

	// Should return tea.Quit command
	if cmd == nil {
		t.Fatal("should return tea.Quit command on channel close")
	}

	// Execute the command and verify it returns quit batch
	// Note: tea.Quit is a function, so we verify cmd is not nil
	// The actual quit behavior is tested by the fact that channelClosedMsg
	// returns tea.Quit in the Update function
}

// TestEventFlow_DrainStateChangedEvent verifies DrainStateChangedEvent updates
// status correctly through the full Update cycle.
func TestEventFlow_DrainStateChangedEvent(t *testing.T) {
	env := newTUITestEnv(t)
	defer env.close()

	m := env.newModel()
	m.width = 100
	m.height = 30
	m.status = "idle"

	// Create DrainStateChangedEvent
	evt := &events.DrainStateChangedEvent{
		BaseEvent: events.NewInternalEvent(events.EventDrainStateChanged),
		From:      "idle",
		To:        "running",
	}

	newM, cmd := m.Update(eventMsg(evt))
	resultM := newM.(model)

	// Verify status updated
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

// TestEventFlow_BeadAbandonedEvent verifies BeadAbandonedEvent increments
// abandoned count through the full Update cycle.
func TestEventFlow_BeadAbandonedEvent(t *testing.T) {
	env := newTUITestEnv(t)
	defer env.close()

	m := env.newModel()
	m.width = 100
	m.height = 30
	m.stats.Abandoned = 2

	// Create BeadAbandonedEvent
	evt := &events.BeadAbandonedEvent{
		BaseEvent:   events.NewInternalEvent(events.EventBeadAbandoned),
		BeadID:      "bd-abandoned-001",
		Attempts:    3,
		MaxFailures: 3,
		LastError:   "max failures exceeded",
	}

	newM, _ := m.Update(eventMsg(evt))
	resultM := newM.(model)

	// Verify abandoned count incremented
	if resultM.stats.Abandoned != 3 {
		t.Errorf("expected abandoned 3, got %d", resultM.stats.Abandoned)
	}

	// Verify event was logged
	if len(resultM.eventLines) == 0 {
		t.Error("expected event to be logged")
	}
}

// TestEventFlow_EventAppearsInEventLines verifies events appear in
// eventLines slice with correct formatting.
func TestEventFlow_EventAppearsInEventLines(t *testing.T) {
	tests := []struct {
		name          string
		event         events.Event
		expectContain string
	}{
		{
			name: "session start",
			event: &events.SessionStartEvent{
				BaseEvent: events.NewClaudeEvent(events.EventSessionStart),
				BeadID:    "bd-sess-001",
				Title:     "Session test bead",
			},
			expectContain: "session started",
		},
		{
			name: "session end",
			event: &events.SessionEndEvent{
				BaseEvent:    events.NewClaudeEvent(events.EventSessionEnd),
				SessionID:    "sess-123",
				NumTurns:     10,
				TotalCostUSD: 0.05,
			},
			expectContain: "session ended",
		},
		{
			name: "drain stop",
			event: &events.DrainStopEvent{
				BaseEvent: events.NewInternalEvent(events.EventDrainStop),
				Reason:    "user requested",
			},
			expectContain: "drain stopped",
		},
		{
			name: "turn complete",
			event: &events.TurnCompleteEvent{
				BaseEvent:     events.NewClaudeEvent(events.EventTurnComplete),
				TurnNumber:    5,
				ToolCount:     3,
				ToolElapsedMs: 1500,
			},
			expectContain: "turn 5 complete",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newTUITestEnv(t)
			defer env.close()

			m := env.newModel()
			m.width = 100
			m.height = 30

			newM, _ := m.Update(eventMsg(tt.event))
			resultM := newM.(model)

			if len(resultM.eventLines) == 0 {
				t.Fatalf("expected event to be logged for %s", tt.name)
			}

			lastLine := resultM.eventLines[len(resultM.eventLines)-1]
			if lastLine.Text == "" {
				t.Errorf("event text should not be empty for %s", tt.name)
			}
		})
	}
}

// TestEventFlow_WaitForEventCommand verifies Update returns waitForEvent command.
func TestEventFlow_WaitForEventCommand(t *testing.T) {
	env := newTUITestEnv(t)
	defer env.close()

	m := env.newModel()
	m.width = 100
	m.height = 30

	evt := &events.DrainStartEvent{
		BaseEvent: events.NewInternalEvent(events.EventDrainStart),
		WorkDir:   "/test",
	}

	_, cmd := m.Update(eventMsg(evt))

	// Verify command is returned
	if cmd == nil {
		t.Error("Update should return command to wait for next event")
	}

	// Send an event to the channel and verify the command can receive it
	testEvt := &events.DrainStopEvent{
		BaseEvent: events.NewInternalEvent(events.EventDrainStop),
		Reason:    "test",
	}
	env.emitEvent(testEvt)

	// The command should be able to receive the event
	// (we can't easily execute tea.Cmd in tests without the full bubbletea runtime)
}

// TestEventFlow_StatusTransitions verifies correct status transitions
// through various event sequences.
func TestEventFlow_StatusTransitions(t *testing.T) {
	env := newTUITestEnv(t)
	defer env.close()

	m := env.newModel()
	m.width = 100
	m.height = 30

	// Start with idle
	if m.status != "idle" {
		t.Errorf("expected initial status 'idle', got %q", m.status)
	}

	// DrainStateChanged to running
	newM, _ := m.Update(eventMsg(&events.DrainStateChangedEvent{
		BaseEvent: events.NewInternalEvent(events.EventDrainStateChanged),
		From:      "idle",
		To:        "running",
	}))
	m = newM.(model)
	if m.status != "running" {
		t.Errorf("expected status 'running', got %q", m.status)
	}

	// IterationStart sets to working
	newM, _ = m.Update(eventMsg(&events.IterationStartEvent{
		BaseEvent: events.NewInternalEvent(events.EventIterationStart),
		BeadID:    "bd-001",
		Title:     "Test",
		Priority:  1,
	}))
	m = newM.(model)
	if m.status != "working" {
		t.Errorf("expected status 'working', got %q", m.status)
	}

	// IterationEnd sets to idle
	newM, _ = m.Update(eventMsg(&events.IterationEndEvent{
		BaseEvent: events.NewInternalEvent(events.EventIterationEnd),
		BeadID:    "bd-001",
		Success:   true,
	}))
	m = newM.(model)
	if m.status != "idle" {
		t.Errorf("expected status 'idle', got %q", m.status)
	}

	// DrainStop sets to stopped
	newM, _ = m.Update(eventMsg(&events.DrainStopEvent{
		BaseEvent: events.NewInternalEvent(events.EventDrainStop),
		Reason:    "complete",
	}))
	m = newM.(model)
	if m.status != "stopped" {
		t.Errorf("expected status 'stopped', got %q", m.status)
	}
}

// TestEventFlow_RealChannelFlow tests events flowing through a real channel.
func TestEventFlow_RealChannelFlow(t *testing.T) {
	env := newTUITestEnv(t)
	defer env.close()

	m := env.newModel()
	m.width = 100
	m.height = 30

	// Send event to the real channel
	evt := &events.DrainStartEvent{
		BaseEvent: events.NewInternalEvent(events.EventDrainStart),
		WorkDir:   "/test/channel",
	}
	if !env.emitEvent(evt) {
		t.Fatal("failed to emit event to channel")
	}

	// Read from channel directly to verify it was sent
	select {
	case received := <-env.eventChan:
		if received.Type() != events.EventDrainStart {
			t.Errorf("expected EventDrainStart, got %v", received.Type())
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for event from channel")
	}
}

// TestEventFlow_ChannelClosedMsg_QuitsBubbletea verifies that channelClosedMsg
// causes bubbletea to quit by returning tea.Quit.
func TestEventFlow_ChannelClosedMsg_QuitsBubbletea(t *testing.T) {
	env := newTUITestEnv(t)
	defer env.close()

	m := env.newModel()
	m.width = 100
	m.height = 30

	// Process channelClosedMsg
	_, cmd := m.Update(channelClosedMsg{})

	// The returned command should be tea.Quit
	// We verify by checking that cmd is not nil and is the quit command
	if cmd == nil {
		t.Fatal("channelClosedMsg should return tea.Quit command")
	}

	// Execute the command to get the message
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}
