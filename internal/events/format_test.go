package events

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestFormat_AllEventTypes(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		event    Event
		contains []string
	}{
		{
			name:     "nil event",
			event:    nil,
			contains: nil,
		},
		{
			name: "ClaudeTextEvent",
			event: &ClaudeTextEvent{
				BaseEvent: BaseEvent{EventType: EventClaudeText, Time: now, Src: SourceClaude},
				Text:      "Hello world",
			},
			contains: []string{"Hello world"},
		},
		{
			name: "ClaudeToolUseEvent with Bash",
			event: &ClaudeToolUseEvent{
				BaseEvent: BaseEvent{EventType: EventClaudeToolUse, Time: now, Src: SourceClaude},
				ToolName:  "Bash",
				Input:     map[string]any{"command": "ls -la"},
			},
			contains: []string{"tool:", "Bash", "ls -la"},
		},
		{
			name: "ClaudeToolUseEvent with Read",
			event: &ClaudeToolUseEvent{
				BaseEvent: BaseEvent{EventType: EventClaudeToolUse, Time: now, Src: SourceClaude},
				ToolName:  "Read",
				Input:     map[string]any{"file_path": "/path/to/file.go"},
			},
			contains: []string{"tool:", "Read", "file.go"},
		},
		{
			name: "ClaudeToolResultEvent success",
			event: &ClaudeToolResultEvent{
				BaseEvent: BaseEvent{EventType: EventClaudeToolResult, Time: now, Src: SourceClaude},
				IsError:   false,
			},
			contains: []string{"tool result:", "ok"},
		},
		{
			name: "ClaudeToolResultEvent error",
			event: &ClaudeToolResultEvent{
				BaseEvent: BaseEvent{EventType: EventClaudeToolResult, Time: now, Src: SourceClaude},
				IsError:   true,
			},
			contains: []string{"tool result:", "ERROR"},
		},
		{
			name: "SessionStartEvent with title",
			event: &SessionStartEvent{
				BaseEvent: BaseEvent{EventType: EventSessionStart, Time: now, Src: SourceInternal},
				BeadID:    "bd-123",
				Title:     "Fix the bug",
			},
			contains: []string{"session started:", "bd-123", "Fix the bug"},
		},
		{
			name: "SessionEndEvent with cost",
			event: &SessionEndEvent{
				BaseEvent:    BaseEvent{EventType: EventSessionEnd, Time: now, Src: SourceInternal},
				NumTurns:     5,
				TotalCostUSD: 0.05,
			},
			contains: []string{"session ended:", "5 turns", "$0.05"},
		},
		{
			name: "SessionTimeoutEvent",
			event: &SessionTimeoutEvent{
				BaseEvent: BaseEvent{EventType: EventSessionTimeout, Time: now, Src: SourceInternal},
				Duration:  5 * time.Minute,
			},
			contains: []string{"session timeout", "5m"},
		},
		{
			name: "DrainStartEvent",
			event: &DrainStartEvent{
				BaseEvent: BaseEvent{EventType: EventDrainStart, Time: now, Src: SourceInternal},
				WorkDir:   "/project",
			},
			contains: []string{"drain started:", "/project"},
		},
		{
			name: "DrainStopEvent",
			event: &DrainStopEvent{
				BaseEvent: BaseEvent{EventType: EventDrainStop, Time: now, Src: SourceInternal},
				Reason:    "user requested",
			},
			contains: []string{"drain stopped:", "user requested"},
		},
		{
			name: "DrainStateChangedEvent",
			event: &DrainStateChangedEvent{
				BaseEvent: BaseEvent{EventType: EventDrainStateChanged, Time: now, Src: SourceInternal},
				From:      "idle",
				To:        "working",
			},
			contains: []string{"state:", "idle", "->", "working"},
		},
		{
			name: "IterationStartEvent",
			event: &IterationStartEvent{
				BaseEvent: BaseEvent{EventType: EventIterationStart, Time: now, Src: SourceInternal},
				BeadID:    "bd-456",
				Title:     "Implement feature",
				Priority:  2,
			},
			contains: []string{"iteration:", "bd-456", "P2", "Implement feature"},
		},
		{
			name: "IterationEndEvent success",
			event: &IterationEndEvent{
				BaseEvent:    BaseEvent{EventType: EventIterationEnd, Time: now, Src: SourceInternal},
				BeadID:       "bd-456",
				Success:      true,
				NumTurns:     10,
				TotalCostUSD: 0.12,
			},
			contains: []string{"[+]", "bd-456", "completed", "10 turns", "$0.12"},
		},
		{
			name: "IterationEndEvent failure",
			event: &IterationEndEvent{
				BaseEvent: BaseEvent{EventType: EventIterationEnd, Time: now, Src: SourceInternal},
				BeadID:    "bd-456",
				Success:   false,
				NumTurns:  3,
			},
			contains: []string{"[x]", "bd-456", "failed", "3 turns"},
		},
		{
			name: "TurnCompleteEvent",
			event: &TurnCompleteEvent{
				BaseEvent:     BaseEvent{EventType: EventTurnComplete, Time: now, Src: SourceInternal},
				TurnNumber:    3,
				ToolCount:     5,
				ToolElapsedMs: 1500,
			},
			contains: []string{"turn 3", "5 tools", "1500ms"},
		},
		{
			name: "BeadAbandonedEvent",
			event: &BeadAbandonedEvent{
				BaseEvent:   BaseEvent{EventType: EventBeadAbandoned, Time: now, Src: SourceInternal},
				BeadID:      "bd-789",
				Attempts:    3,
				MaxFailures: 3,
			},
			contains: []string{"[!]", "bd-789", "abandoned", "3/3"},
		},
		{
			name: "BeadCreatedEvent",
			event: &BeadCreatedEvent{
				BaseEvent: BaseEvent{EventType: EventBeadCreated, Time: now, Src: SourceBD},
				BeadID:    "bd-new",
				Title:     "New feature",
				Actor:     "claude",
			},
			contains: []string{"bead created:", "bd-new", "New feature", "claude"},
		},
		{
			name: "BeadStatusEvent",
			event: &BeadStatusEvent{
				BaseEvent: BaseEvent{EventType: EventBeadStatus, Time: now, Src: SourceBD},
				BeadID:    "bd-status",
				OldStatus: "open",
				NewStatus: "in_progress",
			},
			contains: []string{"[~]", "bd-status", "open", "->", "in_progress"},
		},
		{
			name: "BeadUpdatedEvent",
			event: &BeadUpdatedEvent{
				BaseEvent: BaseEvent{EventType: EventBeadUpdated, Time: now, Src: SourceBD},
				BeadID:    "bd-update",
				Actor:     "claude",
			},
			contains: []string{"bead updated:", "bd-update", "claude"},
		},
		{
			name: "BeadCommentEvent",
			event: &BeadCommentEvent{
				BaseEvent: BaseEvent{EventType: EventBeadComment, Time: now, Src: SourceBD},
				BeadID:    "bd-comment",
				Actor:     "user",
			},
			contains: []string{"comment on", "bd-comment", "user"},
		},
		{
			name: "BeadClosedEvent",
			event: &BeadClosedEvent{
				BaseEvent: BaseEvent{EventType: EventBeadClosed, Time: now, Src: SourceBD},
				BeadID:    "bd-closed",
				Actor:     "claude",
			},
			contains: []string{"[+]", "bd-closed", "closed", "claude"},
		},
		{
			name: "BeadChangedEvent status transition",
			event: &BeadChangedEvent{
				BaseEvent: BaseEvent{EventType: EventBeadChanged, Time: now, Src: SourceBD},
				BeadID:    "bd-changed",
				OldState:  &BeadState{ID: "bd-changed", Status: "open"},
				NewState:  &BeadState{ID: "bd-changed", Status: "in_progress"},
			},
			contains: []string{"[~]", "bd-changed", "open", "->", "in_progress"},
		},
		{
			name: "BeadChangedEvent created",
			event: &BeadChangedEvent{
				BaseEvent: BaseEvent{EventType: EventBeadChanged, Time: now, Src: SourceBD},
				BeadID:    "bd-new",
				OldState:  nil,
				NewState:  &BeadState{ID: "bd-new", Title: "New bead", Status: "open"},
			},
			contains: []string{"bead created:", "bd-new", "New bead"},
		},
		{
			name: "BeadChangedEvent deleted",
			event: &BeadChangedEvent{
				BaseEvent: BaseEvent{EventType: EventBeadChanged, Time: now, Src: SourceBD},
				BeadID:    "bd-deleted",
				OldState:  &BeadState{ID: "bd-deleted", Status: "open"},
				NewState:  nil,
			},
			contains: []string{"bead deleted:", "bd-deleted"},
		},
		{
			name: "EpicClosedEvent",
			event: &EpicClosedEvent{
				BaseEvent:     BaseEvent{EventType: EventEpicClosed, Time: now, Src: SourceInternal},
				EpicID:        "bd-epic-1",
				Title:         "Feature epic",
				TotalChildren: 5,
			},
			contains: []string{"[+]", "epic", "bd-epic-1", "closed", "Feature epic", "5 children"},
		},
		{
			name: "ErrorEvent with bead",
			event: &ErrorEvent{
				BaseEvent: BaseEvent{EventType: EventError, Time: now, Src: SourceInternal},
				Message:   "Something went wrong",
				Severity:  SeverityError,
				BeadID:    "bd-err",
			},
			contains: []string{"ERROR:", "bd-err", "Something went wrong"},
		},
		{
			name: "ParseErrorEvent",
			event: &ParseErrorEvent{
				BaseEvent: BaseEvent{EventType: EventParseError, Time: now, Src: SourceInternal},
				Error:     "unexpected token",
			},
			contains: []string{"PARSE ERROR:", "unexpected token"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Format(tt.event)

			if tt.event == nil {
				if result != "" {
					t.Errorf("Format(nil) = %q, want empty", result)
				}
				return
			}

			for _, s := range tt.contains {
				if !strings.Contains(result, s) {
					t.Errorf("Format() = %q, want to contain %q", result, s)
				}
			}
		})
	}
}

func TestFormatWithTimestamp(t *testing.T) {
	ts := time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC)
	event := &SessionStartEvent{
		BaseEvent: BaseEvent{EventType: EventSessionStart, Time: ts, Src: SourceInternal},
		BeadID:    "bd-123",
	}

	result := FormatWithTimestamp(event)

	// Should have timestamp prefix
	if !strings.HasPrefix(result, "[10:30:45]") {
		t.Errorf("FormatWithTimestamp() = %q, want timestamp prefix [10:30:45]", result)
	}

	// Should contain the formatted event
	if !strings.Contains(result, "bd-123") {
		t.Errorf("FormatWithTimestamp() = %q, want to contain bd-123", result)
	}
}

func TestFormatWithTimestamp_NilEvent(t *testing.T) {
	result := FormatWithTimestamp(nil)
	if result != "" {
		t.Errorf("FormatWithTimestamp(nil) = %q, want empty", result)
	}
}

func TestFormat_RoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Millisecond)

	// Test that all event types can be parsed and formatted without panic
	events := []Event{
		&SessionStartEvent{BaseEvent: BaseEvent{EventType: EventSessionStart, Time: now, Src: SourceInternal}, BeadID: "bd-1", Title: "Test"},
		&SessionEndEvent{BaseEvent: BaseEvent{EventType: EventSessionEnd, Time: now, Src: SourceInternal}, NumTurns: 5},
		&SessionTimeoutEvent{BaseEvent: BaseEvent{EventType: EventSessionTimeout, Time: now, Src: SourceInternal}, Duration: time.Minute},
		&ClaudeTextEvent{BaseEvent: BaseEvent{EventType: EventClaudeText, Time: now, Src: SourceClaude}, Text: "Hello"},
		&ClaudeToolUseEvent{BaseEvent: BaseEvent{EventType: EventClaudeToolUse, Time: now, Src: SourceClaude}, ToolName: "Bash"},
		&ClaudeToolResultEvent{BaseEvent: BaseEvent{EventType: EventClaudeToolResult, Time: now, Src: SourceClaude}},
		&DrainStartEvent{BaseEvent: BaseEvent{EventType: EventDrainStart, Time: now, Src: SourceInternal}, WorkDir: "/test"},
		&DrainStopEvent{BaseEvent: BaseEvent{EventType: EventDrainStop, Time: now, Src: SourceInternal}},
		&DrainStateChangedEvent{BaseEvent: BaseEvent{EventType: EventDrainStateChanged, Time: now, Src: SourceInternal}, From: "a", To: "b"},
		&IterationStartEvent{BaseEvent: BaseEvent{EventType: EventIterationStart, Time: now, Src: SourceInternal}, BeadID: "bd-1"},
		&IterationEndEvent{BaseEvent: BaseEvent{EventType: EventIterationEnd, Time: now, Src: SourceInternal}, BeadID: "bd-1"},
		&TurnCompleteEvent{BaseEvent: BaseEvent{EventType: EventTurnComplete, Time: now, Src: SourceInternal}, TurnNumber: 1},
		&BeadAbandonedEvent{BaseEvent: BaseEvent{EventType: EventBeadAbandoned, Time: now, Src: SourceInternal}, BeadID: "bd-1"},
		&EpicClosedEvent{BaseEvent: BaseEvent{EventType: EventEpicClosed, Time: now, Src: SourceInternal}, EpicID: "bd-1"},
		&BeadCreatedEvent{BaseEvent: BaseEvent{EventType: EventBeadCreated, Time: now, Src: SourceBD}, BeadID: "bd-1"},
		&BeadStatusEvent{BaseEvent: BaseEvent{EventType: EventBeadStatus, Time: now, Src: SourceBD}, BeadID: "bd-1"},
		&BeadUpdatedEvent{BaseEvent: BaseEvent{EventType: EventBeadUpdated, Time: now, Src: SourceBD}, BeadID: "bd-1"},
		&BeadCommentEvent{BaseEvent: BaseEvent{EventType: EventBeadComment, Time: now, Src: SourceBD}, BeadID: "bd-1"},
		&BeadClosedEvent{BaseEvent: BaseEvent{EventType: EventBeadClosed, Time: now, Src: SourceBD}, BeadID: "bd-1"},
		&BeadChangedEvent{BaseEvent: BaseEvent{EventType: EventBeadChanged, Time: now, Src: SourceBD}, BeadID: "bd-1"},
		&ErrorEvent{BaseEvent: BaseEvent{EventType: EventError, Time: now, Src: SourceInternal}, Message: "err"},
		&ParseErrorEvent{BaseEvent: BaseEvent{EventType: EventParseError, Time: now, Src: SourceInternal}, Error: "err"},
	}

	for _, event := range events {
		t.Run(string(event.Type()), func(t *testing.T) {
			// Marshal to JSON
			data, err := json.Marshal(event)
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

			// Format should not panic and should produce non-empty output
			result := Format(parsed)
			if result == "" {
				t.Errorf("Format() returned empty string for %s", event.Type())
			}

			// FormatWithTimestamp should also work
			tsResult := FormatWithTimestamp(parsed)
			if tsResult == "" {
				t.Errorf("FormatWithTimestamp() returned empty string for %s", event.Type())
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello world", 8, "hello..."},
		{"hello", 5, "hello"},
		{"hello", 3, "..."},
		{"hello", 2, "..."},
		{"", 10, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestSafeString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"hello\nworld", "hello world"},
		{"hello\r\nworld", "hello world"},
		{"hello\x1b[31mred\x1b[0m", "hellored"},
		{"multiple   spaces", "multiple spaces"},
		{"  leading and trailing  ", "leading and trailing"},
		{"control\x00char", "controlchar"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SafeString(tt.input)
			if got != tt.want {
				t.Errorf("SafeString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStatusSymbol(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"ready", ">"},
		{"in_progress", "~"},
		{"blocked", "!"},
		{"closed", "+"},
		{"unknown", "-"},
		{"", "-"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := StatusSymbol(tt.status)
			if got != tt.want {
				t.Errorf("StatusSymbol(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestExtractToolDetail(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		input    map[string]any
		want     string
	}{
		{
			name:     "Bash with command",
			toolName: "Bash",
			input:    map[string]any{"command": "ls -la"},
			want:     "ls -la",
		},
		{
			name:     "Read with file_path",
			toolName: "Read",
			input:    map[string]any{"file_path": "/path/to/file.go"},
			want:     "file.go",
		},
		{
			name:     "Glob with pattern",
			toolName: "Glob",
			input:    map[string]any{"pattern": "**/*.go"},
			want:     "**/*.go",
		},
		{
			name:     "TodoWrite",
			toolName: "TodoWrite",
			input:    map[string]any{},
			want:     "(updating todos)",
		},
		{
			name:     "Unknown tool",
			toolName: "Unknown",
			input:    map[string]any{"foo": "bar"},
			want:     "",
		},
		{
			name:     "Nil input",
			toolName: "Bash",
			input:    nil,
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractToolDetail(tt.toolName, tt.input)
			if got != tt.want {
				t.Errorf("ExtractToolDetail(%q, %v) = %q, want %q", tt.toolName, tt.input, got, tt.want)
			}
		})
	}
}
