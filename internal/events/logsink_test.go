package events

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewLogSink(t *testing.T) {
	sink := NewLogSink("/tmp/test.log")
	if sink == nil {
		t.Fatal("NewLogSink returned nil")
	}
	if sink.path != "/tmp/test.log" {
		t.Errorf("path = %q, want %q", sink.path, "/tmp/test.log")
	}
}

func TestLogSinkCreatesDirectory(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "subdir", "nested", "test.log")

	sink := NewLogSink(path)
	events := make(chan Event, 10)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := sink.Start(ctx, events)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify directory was created
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("expected directory to be created")
	}

	cancel()
	_ = sink.Stop()
}

func TestLogSinkWritesJSONLines(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.log")

	sink := NewLogSink(path)
	events := make(chan Event, 10)

	ctx, cancel := context.WithCancel(context.Background())

	err := sink.Start(ctx, events)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Send a drain start event
	events <- &DrainStartEvent{
		BaseEvent: NewInternalEvent(EventDrainStart),
		WorkDir:   "/test/dir",
	}

	// Send an iteration start event
	events <- &IterationStartEvent{
		BaseEvent: NewInternalEvent(EventIterationStart),
		BeadID:    "bd-001",
		Title:     "Test bead",
		Priority:  1,
		Attempt:   1,
	}

	// Give time for events to be written
	time.Sleep(50 * time.Millisecond)

	cancel()
	_ = sink.Stop()

	// Read and verify log contents
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	content := string(data)

	// Check drain.start event
	if !strings.Contains(content, `"type":"drain.start"`) {
		t.Error("expected drain.start event in log")
	}

	// Check iteration.start event
	if !strings.Contains(content, `"type":"iteration.start"`) {
		t.Error("expected iteration.start event in log")
	}

	// Verify each line is valid JSON
	lines := strings.Split(strings.TrimSpace(content), "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i, err)
		}
	}
}

func TestLogSinkAppendsToExistingFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.log")

	// Write initial content
	initial := `{"type":"initial","timestamp":"2024-01-01T00:00:00Z","source":"test"}` + "\n"
	if err := os.WriteFile(path, []byte(initial), 0644); err != nil {
		t.Fatalf("failed to write initial content: %v", err)
	}

	sink := NewLogSink(path)
	events := make(chan Event, 10)

	ctx, cancel := context.WithCancel(context.Background())

	err := sink.Start(ctx, events)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	events <- &DrainStopEvent{
		BaseEvent: NewInternalEvent(EventDrainStop),
		Reason:    "test",
	}

	time.Sleep(50 * time.Millisecond)
	cancel()
	_ = sink.Stop()

	// Verify both initial and new content exist
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, `"type":"initial"`) {
		t.Error("expected initial content to be preserved")
	}
	if !strings.Contains(content, `"type":"drain.stop"`) {
		t.Error("expected new event to be appended")
	}
}

func TestLogSinkHandlesClosedChannel(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.log")

	sink := NewLogSink(path)
	events := make(chan Event, 10)

	ctx := context.Background()

	err := sink.Start(ctx, events)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Close the channel
	close(events)

	// Stop should return without hanging
	done := make(chan struct{})
	go func() {
		_ = sink.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Error("Stop timed out after channel close")
	}
}

func TestLogSinkPath(t *testing.T) {
	sink := NewLogSink("/path/to/file.log")
	if sink.Path() != "/path/to/file.log" {
		t.Errorf("Path() = %q, want %q", sink.Path(), "/path/to/file.log")
	}
}
