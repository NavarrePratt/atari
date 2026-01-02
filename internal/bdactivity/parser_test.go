package bdactivity

import (
	"testing"
	"time"

	"github.com/npratt/atari/internal/events"
)

func TestParseLine_CreateEvent(t *testing.T) {
	line := []byte(`{"timestamp":"2026-01-02T12:45:34.121001-05:00","type":"create","issue_id":"bd-drain-jbk","symbol":"+","message":"bd-drain-jbk created · Fix CurrentBead issue"}`)

	event, err := ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event == nil {
		t.Fatal("expected event, got nil")
	}

	created, ok := event.(*events.BeadCreatedEvent)
	if !ok {
		t.Fatalf("expected BeadCreatedEvent, got %T", event)
	}

	if created.Type() != events.EventBeadCreated {
		t.Errorf("expected type %s, got %s", events.EventBeadCreated, created.Type())
	}
	if created.BeadID != "bd-drain-jbk" {
		t.Errorf("expected bead_id bd-drain-jbk, got %s", created.BeadID)
	}
	if created.Title != "Fix CurrentBead issue" {
		t.Errorf("expected title 'Fix CurrentBead issue', got %s", created.Title)
	}
	if created.Source() != events.SourceBD {
		t.Errorf("expected source %s, got %s", events.SourceBD, created.Source())
	}
}

func TestParseLine_StatusEvent(t *testing.T) {
	line := []byte(`{"timestamp":"2026-01-02T12:12:54.275921-05:00","type":"status","issue_id":"bd-drain-4no","symbol":"→","message":"bd-drain-4no status changed","old_status":"open","new_status":"in_progress"}`)

	event, err := ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event == nil {
		t.Fatal("expected event, got nil")
	}

	status, ok := event.(*events.BeadStatusEvent)
	if !ok {
		t.Fatalf("expected BeadStatusEvent, got %T", event)
	}

	if status.Type() != events.EventBeadStatus {
		t.Errorf("expected type %s, got %s", events.EventBeadStatus, status.Type())
	}
	if status.BeadID != "bd-drain-4no" {
		t.Errorf("expected bead_id bd-drain-4no, got %s", status.BeadID)
	}
	if status.OldStatus != "open" {
		t.Errorf("expected old_status open, got %s", status.OldStatus)
	}
	if status.NewStatus != "in_progress" {
		t.Errorf("expected new_status in_progress, got %s", status.NewStatus)
	}
}

func TestParseLine_StatusToClosedProducesBeadClosedEvent(t *testing.T) {
	tests := []struct {
		name      string
		newStatus string
	}{
		{"closed status", "closed"},
		{"completed status", "completed"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			line := []byte(`{"timestamp":"2026-01-02T12:12:54.275921-05:00","type":"status","issue_id":"bd-drain-xyz","symbol":"✓","message":"bd-drain-xyz completed","old_status":"open","new_status":"` + tc.newStatus + `"}`)

			event, err := ParseLine(line)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if event == nil {
				t.Fatal("expected event, got nil")
			}

			closed, ok := event.(*events.BeadClosedEvent)
			if !ok {
				t.Fatalf("expected BeadClosedEvent, got %T", event)
			}

			if closed.Type() != events.EventBeadClosed {
				t.Errorf("expected type %s, got %s", events.EventBeadClosed, closed.Type())
			}
			if closed.BeadID != "bd-drain-xyz" {
				t.Errorf("expected bead_id bd-drain-xyz, got %s", closed.BeadID)
			}
		})
	}
}

func TestParseLine_UpdateEvent(t *testing.T) {
	line := []byte(`{"timestamp":"2026-01-02T12:48:09.011244-05:00","type":"update","issue_id":"bd-drain-0oe","symbol":"→","message":"bd-drain-0oe updated"}`)

	event, err := ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event == nil {
		t.Fatal("expected event, got nil")
	}

	updated, ok := event.(*events.BeadUpdatedEvent)
	if !ok {
		t.Fatalf("expected BeadUpdatedEvent, got %T", event)
	}

	if updated.Type() != events.EventBeadUpdated {
		t.Errorf("expected type %s, got %s", events.EventBeadUpdated, updated.Type())
	}
	if updated.BeadID != "bd-drain-0oe" {
		t.Errorf("expected bead_id bd-drain-0oe, got %s", updated.BeadID)
	}
}

func TestParseLine_CommentEvent(t *testing.T) {
	line := []byte(`{"timestamp":"2026-01-02T12:48:09.011244-05:00","type":"comment","issue_id":"bd-drain-abc","symbol":"💬","message":"bd-drain-abc comment added"}`)

	event, err := ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event == nil {
		t.Fatal("expected event, got nil")
	}

	comment, ok := event.(*events.BeadCommentEvent)
	if !ok {
		t.Fatalf("expected BeadCommentEvent, got %T", event)
	}

	if comment.Type() != events.EventBeadComment {
		t.Errorf("expected type %s, got %s", events.EventBeadComment, comment.Type())
	}
	if comment.BeadID != "bd-drain-abc" {
		t.Errorf("expected bead_id bd-drain-abc, got %s", comment.BeadID)
	}
}

func TestParseLine_UnknownTypesReturnNilWithoutError(t *testing.T) {
	unknownTypes := []string{"bonded", "squashed", "burned", "delete", "unknown_type"}

	for _, mutationType := range unknownTypes {
		t.Run(mutationType, func(t *testing.T) {
			line := []byte(`{"timestamp":"2026-01-02T12:48:09.011244-05:00","type":"` + mutationType + `","issue_id":"bd-drain-xyz","symbol":"?","message":"some message"}`)

			event, err := ParseLine(line)
			if err != nil {
				t.Errorf("expected no error for unknown type %s, got %v", mutationType, err)
			}
			if event != nil {
				t.Errorf("expected nil event for unknown type %s, got %T", mutationType, event)
			}
		})
	}
}

func TestParseLine_InvalidJSONReturnsError(t *testing.T) {
	invalidLines := []struct {
		name string
		line []byte
	}{
		{"malformed json", []byte(`{invalid json}`)},
		{"truncated json", []byte(`{"timestamp":"2026`)},
		{"not json", []byte(`hello world`)},
	}

	for _, tc := range invalidLines {
		t.Run(tc.name, func(t *testing.T) {
			event, err := ParseLine(tc.line)
			if err == nil {
				t.Error("expected error for invalid JSON, got nil")
			}
			if event != nil {
				t.Errorf("expected nil event for invalid JSON, got %T", event)
			}
		})
	}
}

func TestParseLine_EmptyLinesSkipped(t *testing.T) {
	emptyLines := [][]byte{
		{},
		[]byte(""),
		[]byte("   "),
		[]byte("\t"),
		[]byte("\n"),
		[]byte("  \t  "),
	}

	for i, line := range emptyLines {
		event, err := ParseLine(line)
		if err != nil {
			t.Errorf("case %d: unexpected error for empty line: %v", i, err)
		}
		if event != nil {
			t.Errorf("case %d: expected nil event for empty line, got %T", i, event)
		}
	}
}

func TestParseLine_MissingTimestampUsesCurrentTime(t *testing.T) {
	line := []byte(`{"type":"create","issue_id":"bd-drain-jbk","symbol":"+","message":"bd-drain-jbk created · Test"}`)

	before := time.Now()
	event, err := ParseLine(line)
	after := time.Now()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event == nil {
		t.Fatal("expected event, got nil")
	}

	ts := event.Timestamp()
	if ts.Before(before) || ts.After(after) {
		t.Errorf("expected timestamp between %v and %v, got %v", before, after, ts)
	}
}

func TestParseLine_TimestampParsing(t *testing.T) {
	// Test RFC3339 format
	line := []byte(`{"timestamp":"2026-01-02T12:45:34-05:00","type":"create","issue_id":"bd-drain-jbk","symbol":"+","message":"bd-drain-jbk created · Test"}`)

	event, err := ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := time.Date(2026, 1, 2, 12, 45, 34, 0, time.FixedZone("", -5*3600))
	if !event.Timestamp().Equal(expected) {
		t.Errorf("expected timestamp %v, got %v", expected, event.Timestamp())
	}
}

func TestParseLine_TimestampWithNanoseconds(t *testing.T) {
	// Test RFC3339Nano format
	line := []byte(`{"timestamp":"2026-01-02T12:45:34.123456789-05:00","type":"create","issue_id":"bd-drain-jbk","symbol":"+","message":"bd-drain-jbk created · Test"}`)

	event, err := ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ts := event.Timestamp()
	if ts.Year() != 2026 || ts.Month() != 1 || ts.Day() != 2 {
		t.Errorf("unexpected date: %v", ts)
	}
	if ts.Hour() != 12 || ts.Minute() != 45 || ts.Second() != 34 {
		t.Errorf("unexpected time: %v", ts)
	}
	if ts.Nanosecond() != 123456789 {
		t.Errorf("expected nanoseconds 123456789, got %d", ts.Nanosecond())
	}
}

func TestParseTimestamp_InvalidTimestampUsesCurrentTime(t *testing.T) {
	before := time.Now()
	ts := parseTimestamp("not-a-timestamp")
	after := time.Now()

	if ts.Before(before) || ts.After(after) {
		t.Errorf("expected timestamp between %v and %v, got %v", before, after, ts)
	}
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		message  string
		issueID  string
		expected string
	}{
		{"bd-drain-jbk created · Fix CurrentBead issue", "bd-drain-jbk", "Fix CurrentBead issue"},
		{"bd-drain-xyz created · ", "bd-drain-xyz", ""},
		{"bd-drain-abc updated", "bd-drain-abc", ""},
		{"some other message", "bd-drain-def", ""},
	}

	for _, tc := range tests {
		result := extractTitle(tc.message, tc.issueID)
		if result != tc.expected {
			t.Errorf("extractTitle(%q, %q) = %q, expected %q", tc.message, tc.issueID, result, tc.expected)
		}
	}
}

func TestMapStatusEventType(t *testing.T) {
	tests := []struct {
		newStatus string
		expected  events.EventType
	}{
		{"closed", events.EventBeadClosed},
		{"completed", events.EventBeadClosed},
		{"open", events.EventBeadStatus},
		{"in_progress", events.EventBeadStatus},
		{"ready", events.EventBeadStatus},
		{"blocked", events.EventBeadStatus},
	}

	for _, tc := range tests {
		result := mapStatusEventType(tc.newStatus)
		if result != tc.expected {
			t.Errorf("mapStatusEventType(%q) = %s, expected %s", tc.newStatus, result, tc.expected)
		}
	}
}
