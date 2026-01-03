package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/npratt/atari/internal/events"
)

func TestFormat_NilEvent(t *testing.T) {
	result := Format(nil)
	if result != "" {
		t.Errorf("Format(nil) = %q, want empty string", result)
	}
}

func TestFormat_ClaudeTextEvent(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		wantSub  string
		maxLen   bool
	}{
		{
			name:    "short text",
			text:    "Hello world",
			wantSub: "Hello world",
		},
		{
			name:    "long text is truncated",
			text:    strings.Repeat("a", 300),
			maxLen:  true,
		},
		{
			name:    "text with newlines",
			text:    "line1\nline2\nline3",
			wantSub: "line1 line2 line3",
		},
		{
			name:    "text with ANSI codes",
			text:    "\x1b[32mgreen\x1b[0m text",
			wantSub: "green text",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := &events.ClaudeTextEvent{
				BaseEvent: events.NewClaudeEvent(events.EventClaudeText),
				Text:      tc.text,
			}
			result := Format(e)
			if tc.maxLen {
				if len(result) > maxTextLength {
					t.Errorf("result length %d > maxTextLength %d", len(result), maxTextLength)
				}
				if !strings.HasSuffix(result, truncateIndicator) {
					t.Errorf("truncated result should end with %q", truncateIndicator)
				}
			} else if !strings.Contains(result, tc.wantSub) {
				t.Errorf("result %q does not contain %q", result, tc.wantSub)
			}
		})
	}
}

func TestFormat_ClaudeToolUseEvent(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		input    map[string]any
		wantSub  string
	}{
		{
			name:     "Bash with command",
			toolName: "Bash",
			input:    map[string]any{"command": "ls -la"},
			wantSub:  "tool: Bash ls -la",
		},
		{
			name:     "Read with file path",
			toolName: "Read",
			input:    map[string]any{"file_path": "/path/to/file.go"},
			wantSub:  "tool: Read /path/to/file.go",
		},
		{
			name:     "Write with file path",
			toolName: "Write",
			input:    map[string]any{"file_path": "/path/to/file.go"},
			wantSub:  "tool: Write /path/to/file.go",
		},
		{
			name:     "Edit with file path",
			toolName: "Edit",
			input:    map[string]any{"file_path": "/path/to/file.go"},
			wantSub:  "tool: Edit /path/to/file.go",
		},
		{
			name:     "Glob with pattern",
			toolName: "Glob",
			input:    map[string]any{"pattern": "**/*.go"},
			wantSub:  "tool: Glob **/*.go",
		},
		{
			name:     "Grep with pattern",
			toolName: "Grep",
			input:    map[string]any{"pattern": "func Test"},
			wantSub:  "tool: Grep func Test",
		},
		{
			name:     "Task with description",
			toolName: "Task",
			input:    map[string]any{"description": "Search for files"},
			wantSub:  "tool: Task Search for files",
		},
		{
			name:     "WebFetch with URL",
			toolName: "WebFetch",
			input:    map[string]any{"url": "https://example.com"},
			wantSub:  "tool: WebFetch https://example.com",
		},
		{
			name:     "TodoWrite",
			toolName: "TodoWrite",
			input:    map[string]any{"todos": []any{}},
			wantSub:  "tool: TodoWrite (updating todos)",
		},
		{
			name:     "unknown tool",
			toolName: "Unknown",
			input:    map[string]any{},
			wantSub:  "tool: Unknown",
		},
		{
			name:     "empty tool name",
			toolName: "",
			input:    nil,
			wantSub:  "tool: (unknown)",
		},
		{
			name:     "nil input",
			toolName: "Bash",
			input:    nil,
			wantSub:  "tool: Bash",
		},
		{
			name:     "wrong type in input",
			toolName: "Bash",
			input:    map[string]any{"command": 123}, // int, not string
			wantSub:  "tool: Bash",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := &events.ClaudeToolUseEvent{
				BaseEvent: events.NewClaudeEvent(events.EventClaudeToolUse),
				ToolName:  tc.toolName,
				Input:     tc.input,
			}
			result := Format(e)
			if !strings.Contains(result, tc.wantSub) {
				t.Errorf("result %q does not contain %q", result, tc.wantSub)
			}
		})
	}
}

func TestFormat_ClaudeToolResultEvent(t *testing.T) {
	tests := []struct {
		name    string
		isError bool
		wantSub string
	}{
		{
			name:    "success",
			isError: false,
			wantSub: "ok",
		},
		{
			name:    "error",
			isError: true,
			wantSub: "ERROR",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := &events.ClaudeToolResultEvent{
				BaseEvent: events.NewClaudeEvent(events.EventClaudeToolResult),
				IsError:   tc.isError,
			}
			result := Format(e)
			if !strings.Contains(result, tc.wantSub) {
				t.Errorf("result %q does not contain %q", result, tc.wantSub)
			}
		})
	}
}

func TestFormat_SessionStartEvent(t *testing.T) {
	tests := []struct {
		name    string
		beadID  string
		title   string
		wantSub string
	}{
		{
			name:    "with title",
			beadID:  "bd-123",
			title:   "Fix the bug",
			wantSub: "session started: bd-123 - Fix the bug",
		},
		{
			name:    "without title",
			beadID:  "bd-456",
			title:   "",
			wantSub: "session started: bd-456",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := &events.SessionStartEvent{
				BaseEvent: events.NewClaudeEvent(events.EventSessionStart),
				BeadID:    tc.beadID,
				Title:     tc.title,
			}
			result := Format(e)
			if !strings.Contains(result, tc.wantSub) {
				t.Errorf("result %q does not contain %q", result, tc.wantSub)
			}
		})
	}
}

func TestFormat_SessionEndEvent(t *testing.T) {
	tests := []struct {
		name    string
		turns   int
		cost    float64
		wantSub string
	}{
		{
			name:    "with cost",
			turns:   15,
			cost:    0.0123,
			wantSub: "session ended: 15 turns, $0.0123",
		},
		{
			name:    "without cost",
			turns:   10,
			cost:    0,
			wantSub: "session ended: 10 turns",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := &events.SessionEndEvent{
				BaseEvent:    events.NewClaudeEvent(events.EventSessionEnd),
				NumTurns:     tc.turns,
				TotalCostUSD: tc.cost,
			}
			result := Format(e)
			if !strings.Contains(result, tc.wantSub) {
				t.Errorf("result %q does not contain %q", result, tc.wantSub)
			}
		})
	}
}

func TestFormat_SessionTimeoutEvent(t *testing.T) {
	e := &events.SessionTimeoutEvent{
		BaseEvent: events.NewInternalEvent(events.EventSessionTimeout),
		Duration:  5 * time.Minute,
	}
	result := Format(e)
	if !strings.Contains(result, "session timeout") {
		t.Errorf("result %q does not contain 'session timeout'", result)
	}
	if !strings.Contains(result, "5m") {
		t.Errorf("result %q does not contain duration", result)
	}
}

func TestFormat_DrainStartEvent(t *testing.T) {
	e := &events.DrainStartEvent{
		BaseEvent: events.NewInternalEvent(events.EventDrainStart),
		WorkDir:   "/path/to/work",
	}
	result := Format(e)
	if !strings.Contains(result, "drain started") {
		t.Errorf("result %q does not contain 'drain started'", result)
	}
	if !strings.Contains(result, "/path/to/work") {
		t.Errorf("result %q does not contain work dir", result)
	}
}

func TestFormat_DrainStopEvent(t *testing.T) {
	tests := []struct {
		name    string
		reason  string
		wantSub string
	}{
		{
			name:    "with reason",
			reason:  "user requested",
			wantSub: "drain stopped: user requested",
		},
		{
			name:    "without reason",
			reason:  "",
			wantSub: "drain stopped",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := &events.DrainStopEvent{
				BaseEvent: events.NewInternalEvent(events.EventDrainStop),
				Reason:    tc.reason,
			}
			result := Format(e)
			if !strings.Contains(result, tc.wantSub) {
				t.Errorf("result %q does not contain %q", result, tc.wantSub)
			}
		})
	}
}

func TestFormat_DrainStateChangedEvent(t *testing.T) {
	e := &events.DrainStateChangedEvent{
		BaseEvent: events.NewInternalEvent(events.EventDrainStateChanged),
		From:      "idle",
		To:        "working",
	}
	result := Format(e)
	if !strings.Contains(result, "state: idle -> working") {
		t.Errorf("result %q does not contain state transition", result)
	}
}

func TestFormat_IterationStartEvent(t *testing.T) {
	tests := []struct {
		name     string
		beadID   string
		title    string
		priority int
		wantSub  string
	}{
		{
			name:     "with title",
			beadID:   "bd-123",
			title:    "Implement feature",
			priority: 2,
			wantSub:  "iteration: bd-123 (P2) - Implement feature",
		},
		{
			name:     "without title",
			beadID:   "bd-456",
			title:    "",
			priority: 1,
			wantSub:  "iteration: bd-456 (P1)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := &events.IterationStartEvent{
				BaseEvent: events.NewInternalEvent(events.EventIterationStart),
				BeadID:    tc.beadID,
				Title:     tc.title,
				Priority:  tc.priority,
			}
			result := Format(e)
			if !strings.Contains(result, tc.wantSub) {
				t.Errorf("result %q does not contain %q", result, tc.wantSub)
			}
		})
	}
}

func TestFormat_IterationEndEvent(t *testing.T) {
	tests := []struct {
		name    string
		beadID  string
		success bool
		turns   int
		cost    float64
		wantSub []string
	}{
		{
			name:    "success with cost",
			beadID:  "bd-123",
			success: true,
			turns:   10,
			cost:    0.05,
			wantSub: []string{"[+]", "bd-123", "completed", "10 turns", "$0.05"},
		},
		{
			name:    "failure",
			beadID:  "bd-456",
			success: false,
			turns:   5,
			cost:    0,
			wantSub: []string{"[x]", "bd-456", "failed", "5 turns"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := &events.IterationEndEvent{
				BaseEvent:    events.NewInternalEvent(events.EventIterationEnd),
				BeadID:       tc.beadID,
				Success:      tc.success,
				NumTurns:     tc.turns,
				TotalCostUSD: tc.cost,
			}
			result := Format(e)
			for _, sub := range tc.wantSub {
				if !strings.Contains(result, sub) {
					t.Errorf("result %q does not contain %q", result, sub)
				}
			}
		})
	}
}

func TestFormat_BeadAbandonedEvent(t *testing.T) {
	e := &events.BeadAbandonedEvent{
		BaseEvent:   events.NewInternalEvent(events.EventBeadAbandoned),
		BeadID:      "bd-123",
		Attempts:    3,
		MaxFailures: 3,
	}
	result := Format(e)
	if !strings.Contains(result, "[!]") {
		t.Errorf("result %q does not contain '[!]'", result)
	}
	if !strings.Contains(result, "bd-123") {
		t.Errorf("result %q does not contain bead ID", result)
	}
	if !strings.Contains(result, "abandoned") {
		t.Errorf("result %q does not contain 'abandoned'", result)
	}
	if !strings.Contains(result, "3/3") {
		t.Errorf("result %q does not contain attempt count", result)
	}
}

func TestFormat_BeadCreatedEvent(t *testing.T) {
	tests := []struct {
		name    string
		beadID  string
		title   string
		actor   string
		wantSub string
	}{
		{
			name:    "with title",
			beadID:  "bd-123",
			title:   "New feature",
			actor:   "user",
			wantSub: "bead created: bd-123 - New feature (by user)",
		},
		{
			name:    "without title",
			beadID:  "bd-456",
			title:   "",
			actor:   "system",
			wantSub: "bead created: bd-456 (by system)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := &events.BeadCreatedEvent{
				BaseEvent: events.NewBDEvent(events.EventBeadCreated),
				BeadID:    tc.beadID,
				Title:     tc.title,
				Actor:     tc.actor,
			}
			result := Format(e)
			if !strings.Contains(result, tc.wantSub) {
				t.Errorf("result %q does not contain %q", result, tc.wantSub)
			}
		})
	}
}

func TestFormat_BeadStatusEvent(t *testing.T) {
	tests := []struct {
		name      string
		beadID    string
		oldStatus string
		newStatus string
		wantSub   []string
	}{
		{
			name:      "to ready",
			beadID:    "bd-123",
			oldStatus: "pending",
			newStatus: "ready",
			wantSub:   []string{"[>]", "bd-123", "pending -> ready"},
		},
		{
			name:      "to in_progress",
			beadID:    "bd-456",
			oldStatus: "ready",
			newStatus: "in_progress",
			wantSub:   []string{"[~]", "bd-456", "ready -> in_progress"},
		},
		{
			name:      "to blocked",
			beadID:    "bd-789",
			oldStatus: "in_progress",
			newStatus: "blocked",
			wantSub:   []string{"[!]", "bd-789", "in_progress -> blocked"},
		},
		{
			name:      "to closed",
			beadID:    "bd-abc",
			oldStatus: "in_progress",
			newStatus: "closed",
			wantSub:   []string{"[+]", "bd-abc", "in_progress -> closed"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := &events.BeadStatusEvent{
				BaseEvent: events.NewBDEvent(events.EventBeadStatus),
				BeadID:    tc.beadID,
				OldStatus: tc.oldStatus,
				NewStatus: tc.newStatus,
			}
			result := Format(e)
			for _, sub := range tc.wantSub {
				if !strings.Contains(result, sub) {
					t.Errorf("result %q does not contain %q", result, sub)
				}
			}
		})
	}
}

func TestFormat_BeadUpdatedEvent(t *testing.T) {
	e := &events.BeadUpdatedEvent{
		BaseEvent: events.NewBDEvent(events.EventBeadUpdated),
		BeadID:    "bd-123",
		Actor:     "user",
	}
	result := Format(e)
	if !strings.Contains(result, "bead updated: bd-123 (by user)") {
		t.Errorf("result %q unexpected", result)
	}
}

func TestFormat_BeadCommentEvent(t *testing.T) {
	e := &events.BeadCommentEvent{
		BaseEvent: events.NewBDEvent(events.EventBeadComment),
		BeadID:    "bd-123",
		Actor:     "user",
	}
	result := Format(e)
	if !strings.Contains(result, "comment on bd-123 (by user)") {
		t.Errorf("result %q unexpected", result)
	}
}

func TestFormat_BeadClosedEvent(t *testing.T) {
	e := &events.BeadClosedEvent{
		BaseEvent: events.NewBDEvent(events.EventBeadClosed),
		BeadID:    "bd-123",
		Actor:     "user",
	}
	result := Format(e)
	if !strings.Contains(result, "[+]") {
		t.Errorf("result %q does not contain '[+]'", result)
	}
	if !strings.Contains(result, "bd-123 closed (by user)") {
		t.Errorf("result %q unexpected", result)
	}
}

func TestFormat_ErrorEvent(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		severity string
		beadID   string
		wantSub  []string
	}{
		{
			name:     "error with bead",
			message:  "Something went wrong",
			severity: events.SeverityError,
			beadID:   "bd-123",
			wantSub:  []string{"ERROR:", "bd-123", "Something went wrong"},
		},
		{
			name:     "warning without bead",
			message:  "Minor issue",
			severity: events.SeverityWarning,
			beadID:   "",
			wantSub:  []string{"WARNING:", "Minor issue"},
		},
		{
			name:     "fatal error",
			message:  "Critical failure",
			severity: events.SeverityFatal,
			beadID:   "",
			wantSub:  []string{"FATAL:", "Critical failure"},
		},
		{
			name:     "empty severity defaults to error",
			message:  "Unknown error",
			severity: "",
			beadID:   "",
			wantSub:  []string{"ERROR:", "Unknown error"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := &events.ErrorEvent{
				BaseEvent: events.NewInternalEvent(events.EventError),
				Message:   tc.message,
				Severity:  tc.severity,
				BeadID:    tc.beadID,
			}
			result := Format(e)
			for _, sub := range tc.wantSub {
				if !strings.Contains(result, sub) {
					t.Errorf("result %q does not contain %q", result, sub)
				}
			}
		})
	}
}

func TestFormat_ParseErrorEvent(t *testing.T) {
	e := &events.ParseErrorEvent{
		BaseEvent: events.NewInternalEvent(events.EventParseError),
		Line:      "invalid json",
		Error:     "unexpected token",
	}
	result := Format(e)
	if !strings.Contains(result, "PARSE ERROR:") {
		t.Errorf("result %q does not contain 'PARSE ERROR:'", result)
	}
	if !strings.Contains(result, "unexpected token") {
		t.Errorf("result %q does not contain error message", result)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "short string unchanged",
			input:  "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "exact length unchanged",
			input:  "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "long string truncated",
			input:  "hello world",
			maxLen: 8,
			want:   "hello...",
		},
		{
			name:   "very short max returns indicator only",
			input:  "hello",
			maxLen: 2,
			want:   "...",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := truncate(tc.input, tc.maxLen)
			if result != tc.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tc.input, tc.maxLen, result, tc.want)
			}
		})
	}
}

func TestSafeString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain text unchanged",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "newlines replaced",
			input: "line1\nline2\nline3",
			want:  "line1 line2 line3",
		},
		{
			name:  "ANSI codes stripped",
			input: "\x1b[32mgreen\x1b[0m",
			want:  "green",
		},
		{
			name:  "control chars removed",
			input: "hello\x00\x01\x02world",
			want:  "helloworld",
		},
		{
			name:  "multiple spaces collapsed",
			input: "hello    world",
			want:  "hello world",
		},
		{
			name:  "leading/trailing space trimmed",
			input: "  hello  ",
			want:  "hello",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := safeString(tc.input)
			if result != tc.want {
				t.Errorf("safeString(%q) = %q, want %q", tc.input, result, tc.want)
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

	for _, tc := range tests {
		t.Run(tc.status, func(t *testing.T) {
			result := statusSymbol(tc.status)
			if result != tc.want {
				t.Errorf("statusSymbol(%q) = %q, want %q", tc.status, result, tc.want)
			}
		})
	}
}

func TestGetStringValue(t *testing.T) {
	tests := []struct {
		name    string
		m       map[string]any
		key     string
		wantVal string
		wantOk  bool
	}{
		{
			name:    "string value found",
			m:       map[string]any{"key": "value"},
			key:     "key",
			wantVal: "value",
			wantOk:  true,
		},
		{
			name:    "key not found",
			m:       map[string]any{"other": "value"},
			key:     "key",
			wantVal: "",
			wantOk:  false,
		},
		{
			name:    "nil map",
			m:       nil,
			key:     "key",
			wantVal: "",
			wantOk:  false,
		},
		{
			name:    "wrong type",
			m:       map[string]any{"key": 123},
			key:     "key",
			wantVal: "",
			wantOk:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			val, ok := getStringValue(tc.m, tc.key)
			if val != tc.wantVal || ok != tc.wantOk {
				t.Errorf("getStringValue(%v, %q) = (%q, %v), want (%q, %v)",
					tc.m, tc.key, val, ok, tc.wantVal, tc.wantOk)
			}
		})
	}
}

func TestFormatDurationHuman(t *testing.T) {
	tests := []struct {
		name string
		ms   int64
		want string
	}{
		{
			name: "zero returns <60s",
			ms:   0,
			want: "<60s",
		},
		{
			name: "negative returns <60s",
			ms:   -1000,
			want: "<60s",
		},
		{
			name: "under 60s returns <60s",
			ms:   30000,
			want: "<60s",
		},
		{
			name: "exactly 60s returns 1m",
			ms:   60000,
			want: "1m",
		},
		{
			name: "minutes only",
			ms:   300000, // 5 minutes
			want: "5m",
		},
		{
			name: "59 minutes",
			ms:   3540000, // 59 minutes
			want: "59m",
		},
		{
			name: "exactly 1 hour",
			ms:   3600000, // 60 minutes
			want: "1h",
		},
		{
			name: "hours only",
			ms:   7200000, // 2 hours
			want: "2h",
		},
		{
			name: "hours and minutes",
			ms:   5400000, // 1h 30m
			want: "1h 30m",
		},
		{
			name: "large duration",
			ms:   36000000, // 10 hours
			want: "10h",
		},
		{
			name: "large duration with minutes",
			ms:   36900000, // 10h 15m
			want: "10h 15m",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := formatDurationHuman(tc.ms)
			if result != tc.want {
				t.Errorf("formatDurationHuman(%d) = %q, want %q", tc.ms, result, tc.want)
			}
		})
	}
}
