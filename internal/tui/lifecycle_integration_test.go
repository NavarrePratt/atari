package tui

import (
	"bytes"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/npratt/atari/internal/events"
)

// TestTUILifecycleSmoke verifies the full bubbletea program lifecycle:
// start, receive events, handle keyboard input, and quit cleanly.
// This test uses teatest to run the TUI headlessly without a real TTY.
func TestTUILifecycleSmoke(t *testing.T) {
	// Create event channel with some events to display
	eventChan := make(chan events.Event, 10)

	// Pre-populate with a drain start event
	eventChan <- &events.DrainStartEvent{
		BaseEvent: events.BaseEvent{
			EventType: events.EventDrainStart,
			Time:      time.Now(),
		},
	}

	// Track callbacks
	var quitCalled bool
	onQuit := func() { quitCalled = true }

	// Create model with minimal dependencies
	m := newModel(
		eventChan,
		nil, // onPause
		nil, // onResume
		onQuit,
		nil, // statsGetter
		nil, // observer
		nil, // graphFetcher
		nil, // beadStateGetter
	)

	// Create headless test model with initial terminal size
	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(80, 24),
	)

	// Wait briefly for Init to complete and process initial events
	time.Sleep(50 * time.Millisecond)

	// Send scroll down key
	tm.Send(tea.KeyMsg{Type: tea.KeyDown})

	// Send scroll up key
	tm.Send(tea.KeyMsg{Type: tea.KeyUp})

	// Send quit key to trigger clean exit
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	// Wait for program to finish with timeout
	fm := tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second))

	// Verify we got a model back (not nil)
	if fm == nil {
		t.Fatal("FinalModel returned nil")
	}

	// Verify quit callback was invoked
	if !quitCalled {
		t.Error("quit callback was not invoked")
	}

	// Get final output and verify it contains expected elements
	out := tm.FinalOutput(t, teatest.WithFinalTimeout(5*time.Second))
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(out)
	output := buf.String()

	// Verify output is non-empty (TUI rendered something)
	if len(output) == 0 {
		t.Error("expected non-empty output from TUI")
	}

	// Close event channel to clean up
	close(eventChan)
}

// TestTUILifecycleCtrlCQuit verifies that ctrl+c also triggers quit.
func TestTUILifecycleCtrlCQuit(t *testing.T) {
	eventChan := make(chan events.Event, 10)

	var quitCalled bool
	onQuit := func() { quitCalled = true }

	m := newModel(
		eventChan,
		nil, // onPause
		nil, // onResume
		onQuit,
		nil, // statsGetter
		nil, // observer
		nil, // graphFetcher
		nil, // beadStateGetter
	)

	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(80, 24),
	)

	// Wait for Init
	time.Sleep(50 * time.Millisecond)

	// Send ctrl+c to quit
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})

	// Wait for program to finish
	fm := tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second))

	if fm == nil {
		t.Fatal("FinalModel returned nil")
	}

	if !quitCalled {
		t.Error("quit callback was not invoked on ctrl+c")
	}

	close(eventChan)
}

// TestTUILifecycleChannelClose verifies that closing the event channel
// causes the TUI to exit gracefully.
func TestTUILifecycleChannelClose(t *testing.T) {
	eventChan := make(chan events.Event, 10)

	m := newModel(
		eventChan,
		nil, // onPause
		nil, // onResume
		nil, // onQuit
		nil, // statsGetter
		nil, // observer
		nil, // graphFetcher
		nil, // beadStateGetter
	)

	tm := teatest.NewTestModel(
		t,
		m,
		teatest.WithInitialTermSize(80, 24),
	)

	// Wait for Init to complete
	time.Sleep(50 * time.Millisecond)

	// Close the event channel to trigger graceful shutdown
	close(eventChan)

	// Wait for program to finish - it should exit due to channel close
	fm := tm.FinalModel(t, teatest.WithFinalTimeout(5*time.Second))

	if fm == nil {
		t.Fatal("FinalModel returned nil after channel close")
	}

	// Cast to model to verify state
	finalModel, ok := fm.(model)
	if !ok {
		t.Fatalf("FinalModel is not of type model: %T", fm)
	}

	// Status should be idle (default state, no quit callback invoked)
	if finalModel.status != "idle" {
		t.Errorf("expected status idle, got %q", finalModel.status)
	}
}
