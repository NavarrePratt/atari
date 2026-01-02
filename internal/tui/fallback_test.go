package tui

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/npratt/atari/internal/events"
)

func TestIsTerminal_ReturnsBoolean(t *testing.T) {
	// isTerminal should return a boolean without panicking
	// The actual value depends on how the test is run
	result := isTerminal()
	_ = result // just verify it returns without error
}

func TestTerminalSize_ReturnsInts(t *testing.T) {
	// terminalSize should return integers without panicking
	// May return 0,0 if not a terminal
	width, height := terminalSize()
	if width < 0 || height < 0 {
		t.Errorf("terminalSize returned negative values: %d, %d", width, height)
	}
}

func TestTerminalTooSmall_ReturnsBool(t *testing.T) {
	// terminalTooSmall should return a boolean based on size check
	result := terminalTooSmall()
	_ = result // just verify it returns without error
}

func TestRunSimple_ExitsOnChannelClose(t *testing.T) {
	eventChan := make(chan events.Event)
	tui := New(eventChan)

	done := make(chan error, 1)
	go func() {
		done <- tui.runSimple()
	}()

	// Close the channel to signal exit
	close(eventChan)

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("runSimple returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("runSimple did not exit after channel close")
	}
}

func TestRunSimple_FormatsEvents(t *testing.T) {
	eventChan := make(chan events.Event, 1)
	tui := New(eventChan)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	done := make(chan error, 1)
	go func() {
		done <- tui.runSimple()
	}()

	// Send a test event
	eventChan <- &events.SessionStartEvent{
		BaseEvent: events.NewInternalEvent(events.EventSessionStart),
		BeadID:    "test-123",
		Title:     "Test session",
	}

	// Small delay to allow processing
	time.Sleep(50 * time.Millisecond)

	// Close channel to stop runSimple
	close(eventChan)

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("runSimple returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("runSimple did not exit")
	}

	// Restore stdout and read captured output
	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Verify output contains expected format
	if output == "" {
		t.Error("expected output but got empty string")
	}
	if !bytes.Contains([]byte(output), []byte("test-123")) {
		t.Errorf("output should contain bead ID: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("Test session")) {
		t.Errorf("output should contain title: %s", output)
	}
}

func TestRunSimple_SkipsEmptyFormat(t *testing.T) {
	eventChan := make(chan events.Event, 1)
	tui := New(eventChan)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	done := make(chan error, 1)
	go func() {
		done <- tui.runSimple()
	}()

	// Send nil event (Format returns empty string)
	eventChan <- nil

	// Small delay
	time.Sleep(50 * time.Millisecond)

	close(eventChan)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("runSimple did not exit")
	}

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Output should be empty since nil events produce empty format
	if output != "" {
		t.Errorf("expected empty output for nil event, got: %s", output)
	}
}
