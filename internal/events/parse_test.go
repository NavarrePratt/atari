package events

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseEvent_AllTypes(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)

	tests := []struct {
		name      string
		event     Event
		wantType  EventType
		wantBeadID string
	}{
		{
			name: "SessionStartEvent",
			event: &SessionStartEvent{
				BaseEvent: BaseEvent{EventType: EventSessionStart, Time: now, Src: SourceInternal},
				BeadID:    "bd-123",
				Title:     "Test bead",
			},
			wantType:   EventSessionStart,
			wantBeadID: "bd-123",
		},
		{
			name: "SessionEndEvent",
			event: &SessionEndEvent{
				BaseEvent:    BaseEvent{EventType: EventSessionEnd, Time: now, Src: SourceInternal},
				SessionID:    "sess-1",
				NumTurns:     5,
				DurationMs:   30000,
				TotalCostUSD: 0.05,
			},
			wantType:   EventSessionEnd,
			wantBeadID: "",
		},
		{
			name: "SessionTimeoutEvent",
			event: &SessionTimeoutEvent{
				BaseEvent: BaseEvent{EventType: EventSessionTimeout, Time: now, Src: SourceInternal},
				Duration:  5 * time.Minute,
			},
			wantType:   EventSessionTimeout,
			wantBeadID: "",
		},
		{
			name: "ClaudeTextEvent",
			event: &ClaudeTextEvent{
				BaseEvent: BaseEvent{EventType: EventClaudeText, Time: now, Src: SourceClaude},
				Text:      "Hello world",
			},
			wantType:   EventClaudeText,
			wantBeadID: "",
		},
		{
			name: "ClaudeToolUseEvent",
			event: &ClaudeToolUseEvent{
				BaseEvent: BaseEvent{EventType: EventClaudeToolUse, Time: now, Src: SourceClaude},
				ToolID:    "tool-1",
				ToolName:  "Bash",
				Input:     map[string]any{"command": "ls -la"},
			},
			wantType:   EventClaudeToolUse,
			wantBeadID: "",
		},
		{
			name: "ClaudeToolResultEvent",
			event: &ClaudeToolResultEvent{
				BaseEvent: BaseEvent{EventType: EventClaudeToolResult, Time: now, Src: SourceClaude},
				ToolID:    "tool-1",
				Content:   "success",
				IsError:   false,
			},
			wantType:   EventClaudeToolResult,
			wantBeadID: "",
		},
		{
			name: "DrainStartEvent",
			event: &DrainStartEvent{
				BaseEvent: BaseEvent{EventType: EventDrainStart, Time: now, Src: SourceInternal},
				WorkDir:   "/home/user/project",
			},
			wantType:   EventDrainStart,
			wantBeadID: "",
		},
		{
			name: "DrainStopEvent",
			event: &DrainStopEvent{
				BaseEvent: BaseEvent{EventType: EventDrainStop, Time: now, Src: SourceInternal},
				Reason:    "user requested",
			},
			wantType:   EventDrainStop,
			wantBeadID: "",
		},
		{
			name: "DrainStateChangedEvent",
			event: &DrainStateChangedEvent{
				BaseEvent: BaseEvent{EventType: EventDrainStateChanged, Time: now, Src: SourceInternal},
				From:      "idle",
				To:        "working",
			},
			wantType:   EventDrainStateChanged,
			wantBeadID: "",
		},
		{
			name: "IterationStartEvent",
			event: &IterationStartEvent{
				BaseEvent: BaseEvent{EventType: EventIterationStart, Time: now, Src: SourceInternal},
				BeadID:    "bd-456",
				Title:     "Fix bug",
				Priority:  2,
				Attempt:   1,
			},
			wantType:   EventIterationStart,
			wantBeadID: "bd-456",
		},
		{
			name: "IterationEndEvent",
			event: &IterationEndEvent{
				BaseEvent:    BaseEvent{EventType: EventIterationEnd, Time: now, Src: SourceInternal},
				BeadID:       "bd-456",
				Success:      true,
				NumTurns:     10,
				DurationMs:   60000,
				TotalCostUSD: 0.12,
			},
			wantType:   EventIterationEnd,
			wantBeadID: "bd-456",
		},
		{
			name: "TurnCompleteEvent",
			event: &TurnCompleteEvent{
				BaseEvent:     BaseEvent{EventType: EventTurnComplete, Time: now, Src: SourceInternal},
				TurnNumber:    3,
				ToolCount:     5,
				ToolElapsedMs: 1500,
			},
			wantType:   EventTurnComplete,
			wantBeadID: "",
		},
		{
			name: "BeadAbandonedEvent",
			event: &BeadAbandonedEvent{
				BaseEvent:   BaseEvent{EventType: EventBeadAbandoned, Time: now, Src: SourceInternal},
				BeadID:      "bd-789",
				Attempts:    3,
				MaxFailures: 3,
				LastError:   "tests failed",
			},
			wantType:   EventBeadAbandoned,
			wantBeadID: "bd-789",
		},
		{
			name: "EpicClosedEvent",
			event: &EpicClosedEvent{
				BaseEvent:        BaseEvent{EventType: EventEpicClosed, Time: now, Src: SourceInternal},
				EpicID:           "bd-epic-1",
				Title:            "Feature epic",
				TotalChildren:    5,
				TriggeringBeadID: "bd-last",
				CloseReason:      "all children completed",
			},
			wantType:   EventEpicClosed,
			wantBeadID: "bd-epic-1",
		},
		{
			name: "BeadCreatedEvent",
			event: &BeadCreatedEvent{
				BaseEvent: BaseEvent{EventType: EventBeadCreated, Time: now, Src: SourceBD},
				BeadID:    "bd-new",
				Title:     "New feature",
				Actor:     "claude",
			},
			wantType:   EventBeadCreated,
			wantBeadID: "bd-new",
		},
		{
			name: "BeadStatusEvent",
			event: &BeadStatusEvent{
				BaseEvent: BaseEvent{EventType: EventBeadStatus, Time: now, Src: SourceBD},
				BeadID:    "bd-status",
				OldStatus: "open",
				NewStatus: "in_progress",
				Actor:     "claude",
			},
			wantType:   EventBeadStatus,
			wantBeadID: "bd-status",
		},
		{
			name: "BeadUpdatedEvent",
			event: &BeadUpdatedEvent{
				BaseEvent: BaseEvent{EventType: EventBeadUpdated, Time: now, Src: SourceBD},
				BeadID:    "bd-update",
				Actor:     "claude",
			},
			wantType:   EventBeadUpdated,
			wantBeadID: "bd-update",
		},
		{
			name: "BeadCommentEvent",
			event: &BeadCommentEvent{
				BaseEvent: BaseEvent{EventType: EventBeadComment, Time: now, Src: SourceBD},
				BeadID:    "bd-comment",
				Actor:     "user",
			},
			wantType:   EventBeadComment,
			wantBeadID: "bd-comment",
		},
		{
			name: "BeadClosedEvent",
			event: &BeadClosedEvent{
				BaseEvent: BaseEvent{EventType: EventBeadClosed, Time: now, Src: SourceBD},
				BeadID:    "bd-closed",
				Actor:     "claude",
			},
			wantType:   EventBeadClosed,
			wantBeadID: "bd-closed",
		},
		{
			name: "BeadChangedEvent",
			event: &BeadChangedEvent{
				BaseEvent: BaseEvent{EventType: EventBeadChanged, Time: now, Src: SourceBD},
				BeadID:    "bd-changed",
				OldState:  &BeadState{ID: "bd-changed", Status: "open"},
				NewState:  &BeadState{ID: "bd-changed", Status: "in_progress"},
			},
			wantType:   EventBeadChanged,
			wantBeadID: "bd-changed",
		},
		{
			name: "ErrorEvent",
			event: &ErrorEvent{
				BaseEvent: BaseEvent{EventType: EventError, Time: now, Src: SourceInternal},
				Message:   "Something went wrong",
				Severity:  SeverityError,
				BeadID:    "bd-err",
			},
			wantType:   EventError,
			wantBeadID: "bd-err",
		},
		{
			name: "ParseErrorEvent",
			event: &ParseErrorEvent{
				BaseEvent: BaseEvent{EventType: EventParseError, Time: now, Src: SourceInternal},
				Line:      "invalid json",
				Error:     "unexpected token",
			},
			wantType:   EventParseError,
			wantBeadID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to JSON
			data, err := json.Marshal(tt.event)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			// Parse back
			parsed, err := ParseEvent(data)
			if err != nil {
				t.Fatalf("ParseEvent() error = %v", err)
			}

			if parsed == nil {
				t.Fatal("ParseEvent() returned nil")
			}

			// Verify type
			if parsed.Type() != tt.wantType {
				t.Errorf("Type() = %v, want %v", parsed.Type(), tt.wantType)
			}

			// Verify bead ID extraction
			gotBeadID := GetBeadID(parsed)
			if gotBeadID != tt.wantBeadID {
				t.Errorf("GetBeadID() = %v, want %v", gotBeadID, tt.wantBeadID)
			}
		})
	}
}

func TestParseEvent_RoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)

	tests := []struct {
		name  string
		event Event
	}{
		{
			name: "SessionStartEvent with all fields",
			event: &SessionStartEvent{
				BaseEvent: BaseEvent{EventType: EventSessionStart, Time: now, Src: SourceInternal},
				BeadID:    "bd-123",
				Title:     "Test bead with special chars: <>&\"'",
			},
		},
		{
			name: "IterationStartEvent with top-level fields",
			event: &IterationStartEvent{
				BaseEvent:     BaseEvent{EventType: EventIterationStart, Time: now, Src: SourceInternal},
				BeadID:        "bd-456",
				Title:         "Fix bug",
				Priority:      2,
				Attempt:       1,
				TopLevelID:    "bd-epic-1",
				TopLevelTitle: "Feature epic",
			},
		},
		{
			name: "BeadChangedEvent with state transition",
			event: &BeadChangedEvent{
				BaseEvent: BaseEvent{EventType: EventBeadChanged, Time: now, Src: SourceBD},
				BeadID:    "bd-changed",
				OldState:  &BeadState{ID: "bd-changed", Title: "Old title", Status: "open", Priority: 2, IssueType: "task"},
				NewState:  &BeadState{ID: "bd-changed", Title: "New title", Status: "in_progress", Priority: 1, IssueType: "task"},
			},
		},
		{
			name: "ClaudeToolUseEvent with complex input",
			event: &ClaudeToolUseEvent{
				BaseEvent: BaseEvent{EventType: EventClaudeToolUse, Time: now, Src: SourceClaude},
				ToolID:    "tool-1",
				ToolName:  "Edit",
				Input: map[string]any{
					"file_path":  "/path/to/file.go",
					"old_string": "func old() {}",
					"new_string": "func new() {}",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal to JSON
			data1, err := json.Marshal(tt.event)
			if err != nil {
				t.Fatalf("first marshal failed: %v", err)
			}

			// Parse
			parsed, err := ParseEvent(data1)
			if err != nil {
				t.Fatalf("ParseEvent() error = %v", err)
			}

			// Marshal again
			data2, err := json.Marshal(parsed)
			if err != nil {
				t.Fatalf("second marshal failed: %v", err)
			}

			// Verify JSON is equivalent
			if string(data1) != string(data2) {
				t.Errorf("round-trip mismatch:\noriginal: %s\nparsed:   %s", data1, data2)
			}
		})
	}
}

func TestParseEvent_UnknownType(t *testing.T) {
	data := []byte(`{"type":"unknown.type","timestamp":"2024-01-01T00:00:00Z","source":"test"}`)

	event, err := ParseEvent(data)
	if err != nil {
		t.Fatalf("ParseEvent() error = %v, want nil for unknown type", err)
	}

	if event != nil {
		t.Errorf("ParseEvent() = %v, want nil for unknown type", event)
	}
}

func TestParseEvent_InvalidJSON(t *testing.T) {
	data := []byte(`{invalid json`)

	event, err := ParseEvent(data)
	if err == nil {
		t.Fatalf("ParseEvent() error = nil, want error for invalid JSON")
	}

	if event != nil {
		t.Errorf("ParseEvent() = %v, want nil for invalid JSON", event)
	}
}

func TestParseEvent_BackwardCompatibility(t *testing.T) {
	// Test parsing older log formats that might be missing optional fields
	tests := []struct {
		name string
		json string
		want EventType
	}{
		{
			name: "SessionEnd without cost",
			json: `{"type":"session.end","timestamp":"2024-01-01T00:00:00Z","source":"atari","session_id":"s1","num_turns":5,"duration_ms":30000}`,
			want: EventSessionEnd,
		},
		{
			name: "IterationEnd without session_id",
			json: `{"type":"iteration.end","timestamp":"2024-01-01T00:00:00Z","source":"atari","bead_id":"bd-1","success":true,"num_turns":10,"duration_ms":60000,"total_cost_usd":0.12}`,
			want: EventIterationEnd,
		},
		{
			name: "IterationStart without top-level fields",
			json: `{"type":"iteration.start","timestamp":"2024-01-01T00:00:00Z","source":"atari","bead_id":"bd-1","title":"Test","priority":2,"attempt":1}`,
			want: EventIterationStart,
		},
		{
			name: "ErrorEvent without context",
			json: `{"type":"error","timestamp":"2024-01-01T00:00:00Z","source":"atari","message":"error","severity":"error"}`,
			want: EventError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := ParseEvent([]byte(tt.json))
			if err != nil {
				t.Fatalf("ParseEvent() error = %v", err)
			}

			if event == nil {
				t.Fatal("ParseEvent() returned nil")
			}

			if event.Type() != tt.want {
				t.Errorf("Type() = %v, want %v", event.Type(), tt.want)
			}
		})
	}
}

func TestGetBeadID(t *testing.T) {
	tests := []struct {
		name  string
		event Event
		want  string
	}{
		{
			name:  "nil event",
			event: nil,
			want:  "",
		},
		{
			name:  "ClaudeTextEvent has no bead ID",
			event: &ClaudeTextEvent{Text: "hello"},
			want:  "",
		},
		{
			name:  "DrainStartEvent has no bead ID",
			event: &DrainStartEvent{WorkDir: "/test"},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetBeadID(tt.event)
			if got != tt.want {
				t.Errorf("GetBeadID() = %v, want %v", got, tt.want)
			}
		})
	}
}
