package events

import (
	"encoding/json"
	"testing"
	"time"
)

// TestEventInterfaceCompliance verifies all concrete event types implement Event.
func TestEventInterfaceCompliance(t *testing.T) {
	// Compile-time interface compliance checks
	var _ Event = (*SessionStartEvent)(nil)
	var _ Event = (*SessionEndEvent)(nil)
	var _ Event = (*ClaudeTextEvent)(nil)
	var _ Event = (*ClaudeToolUseEvent)(nil)
	var _ Event = (*ClaudeToolResultEvent)(nil)
	var _ Event = (*DrainStartEvent)(nil)
	var _ Event = (*DrainStopEvent)(nil)
	var _ Event = (*IterationStartEvent)(nil)
	var _ Event = (*IterationEndEvent)(nil)
	var _ Event = (*BeadAbandonedEvent)(nil)
	var _ Event = (*ErrorEvent)(nil)

	// BD activity event types
	var _ Event = (*BeadCreatedEvent)(nil)
	var _ Event = (*BeadStatusEvent)(nil)
	var _ Event = (*BeadUpdatedEvent)(nil)
	var _ Event = (*BeadCommentEvent)(nil)
	var _ Event = (*BeadClosedEvent)(nil)

	// Also test that BaseEvent itself implements Event
	var _ Event = (*BaseEvent)(nil)
}

// TestBaseEventMethods verifies BaseEvent interface method implementations.
func TestBaseEventMethods(t *testing.T) {
	now := time.Now()
	event := BaseEvent{
		EventType: EventSessionStart,
		Time:      now,
		Src:       SourceClaude,
	}

	if event.Type() != EventSessionStart {
		t.Errorf("Type() = %v, want %v", event.Type(), EventSessionStart)
	}
	if event.Timestamp() != now {
		t.Errorf("Timestamp() = %v, want %v", event.Timestamp(), now)
	}
	if event.Source() != SourceClaude {
		t.Errorf("Source() = %v, want %v", event.Source(), SourceClaude)
	}
}

// TestNewEventPopulatesTimestamp verifies constructor sets timestamp.
func TestNewEventPopulatesTimestamp(t *testing.T) {
	before := time.Now()
	event := NewEvent(EventSessionStart, SourceClaude)
	after := time.Now()

	if event.Time.Before(before) || event.Time.After(after) {
		t.Errorf("NewEvent timestamp %v not between %v and %v", event.Time, before, after)
	}
	if event.EventType != EventSessionStart {
		t.Errorf("NewEvent type = %v, want %v", event.EventType, EventSessionStart)
	}
	if event.Src != SourceClaude {
		t.Errorf("NewEvent source = %v, want %v", event.Src, SourceClaude)
	}
}

// TestNewClaudeEvent verifies NewClaudeEvent sets Claude source.
func TestNewClaudeEvent(t *testing.T) {
	event := NewClaudeEvent(EventClaudeText)

	if event.Src != SourceClaude {
		t.Errorf("NewClaudeEvent source = %v, want %v", event.Src, SourceClaude)
	}
	if event.EventType != EventClaudeText {
		t.Errorf("NewClaudeEvent type = %v, want %v", event.EventType, EventClaudeText)
	}
}

// TestNewBDEvent verifies NewBDEvent sets BD source.
func TestNewBDEvent(t *testing.T) {
	event := NewBDEvent(EventBeadAbandoned)

	if event.Src != SourceBD {
		t.Errorf("NewBDEvent source = %v, want %v", event.Src, SourceBD)
	}
}

// TestNewInternalEvent verifies NewInternalEvent sets Atari source.
func TestNewInternalEvent(t *testing.T) {
	event := NewInternalEvent(EventDrainStart)

	if event.Src != SourceInternal {
		t.Errorf("NewInternalEvent source = %v, want %v", event.Src, SourceInternal)
	}
}

// TestSessionStartEventJSON tests JSON round-trip for SessionStartEvent.
func TestSessionStartEventJSON(t *testing.T) {
	original := SessionStartEvent{
		BaseEvent: NewInternalEvent(EventSessionStart),
		BeadID:    "bd-001",
		Title:     "Test bead",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded SessionStartEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.BeadID != original.BeadID {
		t.Errorf("BeadID = %v, want %v", decoded.BeadID, original.BeadID)
	}
	if decoded.Title != original.Title {
		t.Errorf("Title = %v, want %v", decoded.Title, original.Title)
	}
	if decoded.Type() != EventSessionStart {
		t.Errorf("Type = %v, want %v", decoded.Type(), EventSessionStart)
	}
	if decoded.Source() != SourceInternal {
		t.Errorf("Source = %v, want %v", decoded.Source(), SourceInternal)
	}
}

// TestSessionEndEventJSON tests JSON round-trip for SessionEndEvent.
func TestSessionEndEventJSON(t *testing.T) {
	original := SessionEndEvent{
		BaseEvent:    NewClaudeEvent(EventSessionEnd),
		SessionID:    "session-123",
		NumTurns:     10,
		DurationMs:   5000,
		TotalCostUSD: 0.42,
		Result:       "success",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded SessionEndEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.SessionID != original.SessionID {
		t.Errorf("SessionID = %v, want %v", decoded.SessionID, original.SessionID)
	}
	if decoded.NumTurns != original.NumTurns {
		t.Errorf("NumTurns = %v, want %v", decoded.NumTurns, original.NumTurns)
	}
	if decoded.DurationMs != original.DurationMs {
		t.Errorf("DurationMs = %v, want %v", decoded.DurationMs, original.DurationMs)
	}
	if decoded.TotalCostUSD != original.TotalCostUSD {
		t.Errorf("TotalCostUSD = %v, want %v", decoded.TotalCostUSD, original.TotalCostUSD)
	}
}

// TestClaudeToolUseEventJSON tests JSON round-trip for ClaudeToolUseEvent.
func TestClaudeToolUseEventJSON(t *testing.T) {
	original := ClaudeToolUseEvent{
		BaseEvent: NewClaudeEvent(EventClaudeToolUse),
		ToolID:    "tool_abc123",
		ToolName:  "Bash",
		Input:     map[string]any{"command": "git status"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded ClaudeToolUseEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.ToolID != original.ToolID {
		t.Errorf("ToolID = %v, want %v", decoded.ToolID, original.ToolID)
	}
	if decoded.ToolName != original.ToolName {
		t.Errorf("ToolName = %v, want %v", decoded.ToolName, original.ToolName)
	}
	if decoded.Input["command"] != "git status" {
		t.Errorf("Input[command] = %v, want %v", decoded.Input["command"], "git status")
	}
}

// TestClaudeToolResultEventJSON tests JSON round-trip for ClaudeToolResultEvent.
func TestClaudeToolResultEventJSON(t *testing.T) {
	original := ClaudeToolResultEvent{
		BaseEvent: NewClaudeEvent(EventClaudeToolResult),
		ToolID:    "tool_abc123",
		Content:   "command output here",
		IsError:   false,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded ClaudeToolResultEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.ToolID != original.ToolID {
		t.Errorf("ToolID = %v, want %v", decoded.ToolID, original.ToolID)
	}
	if decoded.Content != original.Content {
		t.Errorf("Content = %v, want %v", decoded.Content, original.Content)
	}
}

// TestIterationStartEventJSON tests JSON round-trip for IterationStartEvent.
func TestIterationStartEventJSON(t *testing.T) {
	original := IterationStartEvent{
		BaseEvent: NewInternalEvent(EventIterationStart),
		BeadID:    "bd-042",
		Title:     "Fix bug in auth",
		Priority:  1,
		Attempt:   2,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded IterationStartEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.BeadID != original.BeadID {
		t.Errorf("BeadID = %v, want %v", decoded.BeadID, original.BeadID)
	}
	if decoded.Priority != original.Priority {
		t.Errorf("Priority = %v, want %v", decoded.Priority, original.Priority)
	}
	if decoded.Attempt != original.Attempt {
		t.Errorf("Attempt = %v, want %v", decoded.Attempt, original.Attempt)
	}
}

// TestIterationEndEventJSON tests JSON round-trip for IterationEndEvent.
func TestIterationEndEventJSON(t *testing.T) {
	original := IterationEndEvent{
		BaseEvent:    NewInternalEvent(EventIterationEnd),
		BeadID:       "bd-042",
		Success:      true,
		NumTurns:     8,
		DurationMs:   115000,
		TotalCostUSD: 0.42,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded IterationEndEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Success != original.Success {
		t.Errorf("Success = %v, want %v", decoded.Success, original.Success)
	}
	if decoded.NumTurns != original.NumTurns {
		t.Errorf("NumTurns = %v, want %v", decoded.NumTurns, original.NumTurns)
	}
}

// TestBeadAbandonedEventJSON tests JSON round-trip for BeadAbandonedEvent.
func TestBeadAbandonedEventJSON(t *testing.T) {
	original := BeadAbandonedEvent{
		BaseEvent:   NewInternalEvent(EventBeadAbandoned),
		BeadID:      "bd-042",
		Attempts:    5,
		MaxFailures: 5,
		LastError:   "tests failed",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded BeadAbandonedEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.BeadID != original.BeadID {
		t.Errorf("BeadID = %v, want %v", decoded.BeadID, original.BeadID)
	}
	if decoded.Attempts != original.Attempts {
		t.Errorf("Attempts = %v, want %v", decoded.Attempts, original.Attempts)
	}
	if decoded.LastError != original.LastError {
		t.Errorf("LastError = %v, want %v", decoded.LastError, original.LastError)
	}
}

// TestBeadCreatedEventJSON tests JSON round-trip for BeadCreatedEvent.
func TestBeadCreatedEventJSON(t *testing.T) {
	original := BeadCreatedEvent{
		BaseEvent: NewBDEvent(EventBeadCreated),
		BeadID:    "bd-123",
		Title:     "New feature request",
		Actor:     "user@example.com",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded BeadCreatedEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.BeadID != original.BeadID {
		t.Errorf("BeadID = %v, want %v", decoded.BeadID, original.BeadID)
	}
	if decoded.Title != original.Title {
		t.Errorf("Title = %v, want %v", decoded.Title, original.Title)
	}
	if decoded.Actor != original.Actor {
		t.Errorf("Actor = %v, want %v", decoded.Actor, original.Actor)
	}
	if decoded.Source() != SourceBD {
		t.Errorf("Source = %v, want %v", decoded.Source(), SourceBD)
	}
}

// TestBeadStatusEventJSON tests JSON round-trip for BeadStatusEvent.
func TestBeadStatusEventJSON(t *testing.T) {
	original := BeadStatusEvent{
		BaseEvent: NewBDEvent(EventBeadStatus),
		BeadID:    "bd-456",
		OldStatus: "ready",
		NewStatus: "in_progress",
		Actor:     "atari",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded BeadStatusEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.BeadID != original.BeadID {
		t.Errorf("BeadID = %v, want %v", decoded.BeadID, original.BeadID)
	}
	if decoded.OldStatus != original.OldStatus {
		t.Errorf("OldStatus = %v, want %v", decoded.OldStatus, original.OldStatus)
	}
	if decoded.NewStatus != original.NewStatus {
		t.Errorf("NewStatus = %v, want %v", decoded.NewStatus, original.NewStatus)
	}
	if decoded.Actor != original.Actor {
		t.Errorf("Actor = %v, want %v", decoded.Actor, original.Actor)
	}
	if decoded.Source() != SourceBD {
		t.Errorf("Source = %v, want %v", decoded.Source(), SourceBD)
	}
}

// TestBeadUpdatedEventJSON tests JSON round-trip for BeadUpdatedEvent.
func TestBeadUpdatedEventJSON(t *testing.T) {
	original := BeadUpdatedEvent{
		BaseEvent: NewBDEvent(EventBeadUpdated),
		BeadID:    "bd-789",
		Actor:     "developer",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded BeadUpdatedEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.BeadID != original.BeadID {
		t.Errorf("BeadID = %v, want %v", decoded.BeadID, original.BeadID)
	}
	if decoded.Actor != original.Actor {
		t.Errorf("Actor = %v, want %v", decoded.Actor, original.Actor)
	}
	if decoded.Source() != SourceBD {
		t.Errorf("Source = %v, want %v", decoded.Source(), SourceBD)
	}
}

// TestBeadCommentEventJSON tests JSON round-trip for BeadCommentEvent.
func TestBeadCommentEventJSON(t *testing.T) {
	original := BeadCommentEvent{
		BaseEvent: NewBDEvent(EventBeadComment),
		BeadID:    "bd-101",
		Actor:     "reviewer",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded BeadCommentEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.BeadID != original.BeadID {
		t.Errorf("BeadID = %v, want %v", decoded.BeadID, original.BeadID)
	}
	if decoded.Actor != original.Actor {
		t.Errorf("Actor = %v, want %v", decoded.Actor, original.Actor)
	}
	if decoded.Source() != SourceBD {
		t.Errorf("Source = %v, want %v", decoded.Source(), SourceBD)
	}
}

// TestBeadClosedEventJSON tests JSON round-trip for BeadClosedEvent.
func TestBeadClosedEventJSON(t *testing.T) {
	original := BeadClosedEvent{
		BaseEvent: NewBDEvent(EventBeadClosed),
		BeadID:    "bd-202",
		Actor:     "atari",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded BeadClosedEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.BeadID != original.BeadID {
		t.Errorf("BeadID = %v, want %v", decoded.BeadID, original.BeadID)
	}
	if decoded.Actor != original.Actor {
		t.Errorf("Actor = %v, want %v", decoded.Actor, original.Actor)
	}
	if decoded.Source() != SourceBD {
		t.Errorf("Source = %v, want %v", decoded.Source(), SourceBD)
	}
}

// TestErrorEventJSON tests JSON round-trip for ErrorEvent.
func TestErrorEventJSON(t *testing.T) {
	original := ErrorEvent{
		BaseEvent: NewInternalEvent(EventError),
		Message:   "failed to spawn claude",
		Severity:  SeverityError,
		BeadID:    "bd-042",
		Context: map[string]string{
			"component": "session",
			"action":    "spawn",
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded ErrorEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Message != original.Message {
		t.Errorf("Message = %v, want %v", decoded.Message, original.Message)
	}
	if decoded.Severity != original.Severity {
		t.Errorf("Severity = %v, want %v", decoded.Severity, original.Severity)
	}
	if decoded.Context["component"] != "session" {
		t.Errorf("Context[component] = %v, want %v", decoded.Context["component"], "session")
	}
}

// TestDrainStartEventJSON tests JSON round-trip for DrainStartEvent.
func TestDrainStartEventJSON(t *testing.T) {
	original := DrainStartEvent{
		BaseEvent: NewInternalEvent(EventDrainStart),
		WorkDir:   "/home/user/project",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded DrainStartEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.WorkDir != original.WorkDir {
		t.Errorf("WorkDir = %v, want %v", decoded.WorkDir, original.WorkDir)
	}
}

// TestDrainStopEventJSON tests JSON round-trip for DrainStopEvent.
func TestDrainStopEventJSON(t *testing.T) {
	original := DrainStopEvent{
		BaseEvent: NewInternalEvent(EventDrainStop),
		Reason:    "user requested",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded DrainStopEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Reason != original.Reason {
		t.Errorf("Reason = %v, want %v", decoded.Reason, original.Reason)
	}
}

// TestClaudeTextEventJSON tests JSON round-trip for ClaudeTextEvent.
func TestClaudeTextEventJSON(t *testing.T) {
	original := ClaudeTextEvent{
		BaseEvent: NewClaudeEvent(EventClaudeText),
		Text:      "I'll help you with that",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded ClaudeTextEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Text != original.Text {
		t.Errorf("Text = %v, want %v", decoded.Text, original.Text)
	}
}

// TestBeadHistoryJSON tests JSON round-trip for BeadHistory.
func TestBeadHistoryJSON(t *testing.T) {
	now := time.Now().Truncate(time.Second) // Truncate for JSON comparison
	original := BeadHistory{
		ID:          "bd-042",
		Status:      HistoryWorking,
		Attempts:    2,
		LastAttempt: now,
		LastError:   "previous error",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded BeadHistory
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID = %v, want %v", decoded.ID, original.ID)
	}
	if decoded.Status != original.Status {
		t.Errorf("Status = %v, want %v", decoded.Status, original.Status)
	}
	if decoded.Attempts != original.Attempts {
		t.Errorf("Attempts = %v, want %v", decoded.Attempts, original.Attempts)
	}
	if decoded.LastError != original.LastError {
		t.Errorf("LastError = %v, want %v", decoded.LastError, original.LastError)
	}
}

// TestHistoryStatusConstants verifies HistoryStatus constants.
func TestHistoryStatusConstants(t *testing.T) {
	tests := []struct {
		status HistoryStatus
		want   string
	}{
		{HistoryPending, "pending"},
		{HistoryWorking, "working"},
		{HistoryCompleted, "completed"},
		{HistoryFailed, "failed"},
		{HistoryAbandoned, "abandoned"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("HistoryStatus %v = %v, want %v", tt.status, string(tt.status), tt.want)
		}
	}
}

// TestSeverityConstants verifies Severity constants.
func TestSeverityConstants(t *testing.T) {
	if SeverityWarning != "warning" {
		t.Errorf("SeverityWarning = %v, want warning", SeverityWarning)
	}
	if SeverityError != "error" {
		t.Errorf("SeverityError = %v, want error", SeverityError)
	}
	if SeverityFatal != "fatal" {
		t.Errorf("SeverityFatal = %v, want fatal", SeverityFatal)
	}
}

// TestEventTypeConstants verifies EventType constants have expected values.
func TestEventTypeConstants(t *testing.T) {
	tests := []struct {
		eventType EventType
		want      string
	}{
		{EventSessionStart, "session.start"},
		{EventSessionEnd, "session.end"},
		{EventClaudeText, "claude.text"},
		{EventClaudeToolUse, "claude.tool_use"},
		{EventClaudeToolResult, "claude.tool_result"},
		{EventDrainStart, "drain.start"},
		{EventDrainStop, "drain.stop"},
		{EventIterationStart, "iteration.start"},
		{EventIterationEnd, "iteration.end"},
		{EventBeadAbandoned, "bead.abandoned"},
		{EventBeadCreated, "bead.created"},
		{EventBeadStatus, "bead.status"},
		{EventBeadUpdated, "bead.updated"},
		{EventBeadComment, "bead.comment"},
		{EventBeadClosed, "bead.closed"},
		{EventError, "error"},
	}

	for _, tt := range tests {
		if string(tt.eventType) != tt.want {
			t.Errorf("EventType %v = %v, want %v", tt.eventType, string(tt.eventType), tt.want)
		}
	}
}

// TestSourceConstants verifies Source constants have expected values.
func TestSourceConstants(t *testing.T) {
	if SourceClaude != "claude" {
		t.Errorf("SourceClaude = %v, want claude", SourceClaude)
	}
	if SourceBD != "bd" {
		t.Errorf("SourceBD = %v, want bd", SourceBD)
	}
	if SourceInternal != "atari" {
		t.Errorf("SourceInternal = %v, want atari", SourceInternal)
	}
}
