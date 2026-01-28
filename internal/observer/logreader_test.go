package observer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/npratt/atari/internal/events"
)

func TestNewLogReader(t *testing.T) {
	r := NewLogReader("/tmp/test.log")
	if r == nil {
		t.Fatal("expected non-nil reader")
	}
	if r.path != "/tmp/test.log" {
		t.Errorf("expected path /tmp/test.log, got %s", r.path)
	}
}

func TestReadRecent_FileNotFound(t *testing.T) {
	r := NewLogReader("/nonexistent/path/file.log")
	_, err := r.ReadRecent(10)
	if err != ErrFileNotFound {
		t.Errorf("expected ErrFileNotFound, got %v", err)
	}
}

func TestReadRecent_EmptyFile(t *testing.T) {
	tmpFile := createTempFile(t, "")

	r := NewLogReader(tmpFile)
	_, err := r.ReadRecent(10)
	if err != ErrEmptyFile {
		t.Errorf("expected ErrEmptyFile, got %v", err)
	}
}

func TestReadRecent_SingleEvent(t *testing.T) {
	event := events.SessionStartEvent{
		BaseEvent: events.NewInternalEvent(events.EventSessionStart),
		BeadID:    "bd-123",
		Title:     "Test bead",
	}
	tmpFile := createTempFileWithEvents(t, []events.Event{&event})

	r := NewLogReader(tmpFile)
	result, err := r.ReadRecent(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 event, got %d", len(result))
	}

	if result[0].Type() != events.EventSessionStart {
		t.Errorf("expected type %s, got %s", events.EventSessionStart, result[0].Type())
	}
}

func TestReadRecent_LimitResults(t *testing.T) {
	var evs []events.Event
	for i := 0; i < 20; i++ {
		evs = append(evs, &events.ClaudeTextEvent{
			BaseEvent: events.NewClaudeEvent(events.EventClaudeText),
			Text:      "Test text",
		})
	}
	tmpFile := createTempFileWithEvents(t, evs)

	r := NewLogReader(tmpFile)
	result, err := r.ReadRecent(5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 5 {
		t.Errorf("expected 5 events, got %d", len(result))
	}
}

func TestReadRecent_AllEventsWhenLessThanN(t *testing.T) {
	var evs []events.Event
	for i := 0; i < 3; i++ {
		evs = append(evs, &events.ClaudeTextEvent{
			BaseEvent: events.NewClaudeEvent(events.EventClaudeText),
			Text:      "Test text",
		})
	}
	tmpFile := createTempFileWithEvents(t, evs)

	r := NewLogReader(tmpFile)
	result, err := r.ReadRecent(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("expected 3 events, got %d", len(result))
	}
}

func TestReadRecent_ZeroOrNegative(t *testing.T) {
	r := NewLogReader("/tmp/test.log")

	result, err := r.ReadRecent(0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil for n=0")
	}

	result, err = r.ReadRecent(-1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil for n=-1")
	}
}

func TestReadByBeadID(t *testing.T) {
	evs := []events.Event{
		&events.IterationStartEvent{
			BaseEvent: events.NewInternalEvent(events.EventIterationStart),
			BeadID:    "bd-123",
			Title:     "First bead",
		},
		&events.ClaudeTextEvent{
			BaseEvent: events.NewClaudeEvent(events.EventClaudeText),
			Text:      "Some text",
		},
		&events.IterationStartEvent{
			BaseEvent: events.NewInternalEvent(events.EventIterationStart),
			BeadID:    "bd-456",
			Title:     "Second bead",
		},
		&events.IterationEndEvent{
			BaseEvent: events.NewInternalEvent(events.EventIterationEnd),
			BeadID:    "bd-123",
			Success:   true,
		},
	}
	tmpFile := createTempFileWithEvents(t, evs)

	r := NewLogReader(tmpFile)
	result, err := r.ReadByBeadID("bd-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 events for bd-123, got %d", len(result))
	}
}

func TestReadByBeadID_EmptyID(t *testing.T) {
	r := NewLogReader("/tmp/test.log")
	result, err := r.ReadByBeadID("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Error("expected nil for empty bead ID")
	}
}

func TestReadByBeadID_NotFound(t *testing.T) {
	evs := []events.Event{
		&events.IterationStartEvent{
			BaseEvent: events.NewInternalEvent(events.EventIterationStart),
			BeadID:    "bd-123",
			Title:     "Test bead",
		},
	}
	tmpFile := createTempFileWithEvents(t, evs)

	r := NewLogReader(tmpFile)
	result, err := r.ReadByBeadID("bd-nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 events, got %d", len(result))
	}
}

func TestReadAfterTimestamp(t *testing.T) {
	now := time.Now()
	evs := []events.Event{
		&events.ClaudeTextEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventClaudeText,
				Time:      now.Add(-2 * time.Hour),
				Src:       events.SourceClaude,
			},
			Text: "Old event",
		},
		&events.ClaudeTextEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventClaudeText,
				Time:      now.Add(-30 * time.Minute),
				Src:       events.SourceClaude,
			},
			Text: "Middle event",
		},
		&events.ClaudeTextEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventClaudeText,
				Time:      now,
				Src:       events.SourceClaude,
			},
			Text: "Recent event",
		},
	}
	tmpFile := createTempFileWithEvents(t, evs)

	r := NewLogReader(tmpFile)
	result, err := r.ReadAfterTimestamp(now.Add(-1 * time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 events after timestamp, got %d", len(result))
	}
}

func TestReadAfterTimestamp_NoMatches(t *testing.T) {
	evs := []events.Event{
		&events.ClaudeTextEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventClaudeText,
				Time:      time.Now().Add(-24 * time.Hour),
				Src:       events.SourceClaude,
			},
			Text: "Old event",
		},
	}
	tmpFile := createTempFileWithEvents(t, evs)

	r := NewLogReader(tmpFile)
	result, err := r.ReadAfterTimestamp(time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 events, got %d", len(result))
	}
}

func TestParseEvent_AllTypes(t *testing.T) {
	testCases := []struct {
		name      string
		event     events.Event
		eventType events.EventType
	}{
		{"SessionStart", &events.SessionStartEvent{BaseEvent: events.NewInternalEvent(events.EventSessionStart), BeadID: "bd-1"}, events.EventSessionStart},
		{"SessionEnd", &events.SessionEndEvent{BaseEvent: events.NewInternalEvent(events.EventSessionEnd)}, events.EventSessionEnd},
		{"SessionTimeout", &events.SessionTimeoutEvent{BaseEvent: events.NewInternalEvent(events.EventSessionTimeout)}, events.EventSessionTimeout},
		{"ClaudeText", &events.ClaudeTextEvent{BaseEvent: events.NewClaudeEvent(events.EventClaudeText)}, events.EventClaudeText},
		{"ClaudeToolUse", &events.ClaudeToolUseEvent{BaseEvent: events.NewClaudeEvent(events.EventClaudeToolUse)}, events.EventClaudeToolUse},
		{"ClaudeToolResult", &events.ClaudeToolResultEvent{BaseEvent: events.NewClaudeEvent(events.EventClaudeToolResult)}, events.EventClaudeToolResult},
		{"DrainStart", &events.DrainStartEvent{BaseEvent: events.NewInternalEvent(events.EventDrainStart)}, events.EventDrainStart},
		{"DrainStop", &events.DrainStopEvent{BaseEvent: events.NewInternalEvent(events.EventDrainStop)}, events.EventDrainStop},
		{"DrainStateChanged", &events.DrainStateChangedEvent{BaseEvent: events.NewInternalEvent(events.EventDrainStateChanged)}, events.EventDrainStateChanged},
		{"IterationStart", &events.IterationStartEvent{BaseEvent: events.NewInternalEvent(events.EventIterationStart), BeadID: "bd-1"}, events.EventIterationStart},
		{"IterationEnd", &events.IterationEndEvent{BaseEvent: events.NewInternalEvent(events.EventIterationEnd), BeadID: "bd-1"}, events.EventIterationEnd},
		{"BeadAbandoned", &events.BeadAbandonedEvent{BaseEvent: events.NewInternalEvent(events.EventBeadAbandoned), BeadID: "bd-1"}, events.EventBeadAbandoned},
		{"BeadCreated", &events.BeadCreatedEvent{BaseEvent: events.NewBDEvent(events.EventBeadCreated), BeadID: "bd-1"}, events.EventBeadCreated},
		{"BeadStatus", &events.BeadStatusEvent{BaseEvent: events.NewBDEvent(events.EventBeadStatus), BeadID: "bd-1"}, events.EventBeadStatus},
		{"BeadUpdated", &events.BeadUpdatedEvent{BaseEvent: events.NewBDEvent(events.EventBeadUpdated), BeadID: "bd-1"}, events.EventBeadUpdated},
		{"BeadComment", &events.BeadCommentEvent{BaseEvent: events.NewBDEvent(events.EventBeadComment), BeadID: "bd-1"}, events.EventBeadComment},
		{"BeadClosed", &events.BeadClosedEvent{BaseEvent: events.NewBDEvent(events.EventBeadClosed), BeadID: "bd-1"}, events.EventBeadClosed},
		{"Error", &events.ErrorEvent{BaseEvent: events.NewInternalEvent(events.EventError)}, events.EventError},
		{"ParseError", &events.ParseErrorEvent{BaseEvent: events.NewInternalEvent(events.EventParseError)}, events.EventParseError},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.event)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			parsed, err := events.ParseEvent(data)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}

			if parsed.Type() != tc.eventType {
				t.Errorf("expected type %s, got %s", tc.eventType, parsed.Type())
			}
		})
	}
}

func TestParseEvent_InvalidJSON(t *testing.T) {
	_, err := events.ParseEvent([]byte("not valid json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseEvent_UnknownType(t *testing.T) {
	data := []byte(`{"type":"unknown.event","timestamp":"2024-01-01T00:00:00Z","source":"test"}`)
	ev, err := events.ParseEvent(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev != nil {
		t.Error("expected nil for unknown event type")
	}
}

func TestReadRecent_InvalidJSONLine(t *testing.T) {
	// Mix of valid and invalid lines
	content := `{"type":"session.start","timestamp":"2024-01-01T00:00:00Z","source":"atari","bead_id":"bd-1","title":"Test"}
invalid json line
{"type":"session.end","timestamp":"2024-01-01T00:00:01Z","source":"atari","session_id":"s1","num_turns":5}`

	tmpFile := createTempFile(t, content)

	r := NewLogReader(tmpFile)
	result, err := r.ReadRecent(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 2 valid events (invalid line skipped)
	if len(result) != 2 {
		t.Errorf("expected 2 events, got %d", len(result))
	}
}

func TestReadRecent_LargeLine(t *testing.T) {
	// Create a line that exceeds maxLineSize
	largeText := strings.Repeat("x", maxLineSize+1000)
	content := `{"type":"claude.text","timestamp":"2024-01-01T00:00:00Z","source":"claude","text":"` + largeText + `"}`

	tmpFile := createTempFile(t, content)

	r := NewLogReader(tmpFile)
	result, err := r.ReadRecent(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The truncated line should fail to parse as valid JSON
	// but shouldn't crash the reader
	if len(result) != 0 {
		t.Errorf("expected 0 events (truncated line invalid), got %d", len(result))
	}
}

func TestGetBeadID(t *testing.T) {
	testCases := []struct {
		name     string
		event    events.Event
		expected string
	}{
		{"SessionStart", &events.SessionStartEvent{BeadID: "bd-1"}, "bd-1"},
		{"IterationStart", &events.IterationStartEvent{BeadID: "bd-2"}, "bd-2"},
		{"IterationEnd", &events.IterationEndEvent{BeadID: "bd-3"}, "bd-3"},
		{"BeadAbandoned", &events.BeadAbandonedEvent{BeadID: "bd-4"}, "bd-4"},
		{"BeadCreated", &events.BeadCreatedEvent{BeadID: "bd-5"}, "bd-5"},
		{"BeadStatus", &events.BeadStatusEvent{BeadID: "bd-6"}, "bd-6"},
		{"BeadUpdated", &events.BeadUpdatedEvent{BeadID: "bd-7"}, "bd-7"},
		{"BeadComment", &events.BeadCommentEvent{BeadID: "bd-8"}, "bd-8"},
		{"BeadClosed", &events.BeadClosedEvent{BeadID: "bd-9"}, "bd-9"},
		{"Error", &events.ErrorEvent{BeadID: "bd-10"}, "bd-10"},
		{"ClaudeText (no bead)", &events.ClaudeTextEvent{}, ""},
		{"DrainStart (no bead)", &events.DrainStartEvent{}, ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := events.GetBeadID(tc.event)
			if result != tc.expected {
				t.Errorf("expected bead ID %q, got %q", tc.expected, result)
			}
		})
	}
}

func TestTruncateForLog(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is longer", 10, "this is lo..."},
	}

	for _, tc := range tests {
		result := truncateForLog(tc.input, tc.maxLen)
		if result != tc.expected {
			t.Errorf("truncateForLog(%q, %d) = %q, want %q", tc.input, tc.maxLen, result, tc.expected)
		}
	}
}

// Helper functions

func createTempFile(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.log")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	return tmpFile
}

func createTempFileWithEvents(t *testing.T, evs []events.Event) string {
	t.Helper()
	var lines []string
	for _, ev := range evs {
		data, err := json.Marshal(ev)
		if err != nil {
			t.Fatalf("failed to marshal event: %v", err)
		}
		lines = append(lines, string(data))
	}
	return createTempFile(t, strings.Join(lines, "\n"))
}
