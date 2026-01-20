package observer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/events"
)

func TestNewContextBuilder(t *testing.T) {
	logReader := NewLogReader("/tmp/test.log")
	cfg := &config.ObserverConfig{
		RecentEvents: 10,
	}

	builder := NewContextBuilder(logReader, cfg)

	if builder == nil {
		t.Fatal("expected non-nil builder")
	}
	if builder.logReader != logReader {
		t.Error("logReader not set correctly")
	}
	if builder.config != cfg {
		t.Error("config not set correctly")
	}
}

func TestContextBuilder_BuildWithEmptyLog(t *testing.T) {
	// Create empty log file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	if err := os.WriteFile(logPath, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	logReader := NewLogReader(logPath)
	cfg := &config.ObserverConfig{
		RecentEvents: 10,
	}
	builder := NewContextBuilder(logReader, cfg)

	state := DrainState{
		Status:    "idle",
		Uptime:    5 * time.Minute,
		TotalCost: 0.0,
	}

	ctx, err := builder.Build(state, nil)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Verify sections present
	if !strings.Contains(ctx, "## Drain Status") {
		t.Error("missing drain status section")
	}
	if !strings.Contains(ctx, "State: idle") {
		t.Error("missing state in drain status")
	}
	if !strings.Contains(ctx, "## Retrieving Full Event Details") {
		t.Error("missing tips section")
	}
}

func TestContextBuilder_BuildWithCurrentBead(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Write some events
	evts := []any{
		map[string]any{
			"type":      "iteration.start",
			"timestamp": time.Now().Add(-2 * time.Minute).Format(time.RFC3339Nano),
			"source":    "atari",
			"bead_id":   "bd-test-001",
			"title":     "Test bead",
			"priority":  2,
			"attempt":   1,
		},
		map[string]any{
			"type":      "claude.text",
			"timestamp": time.Now().Add(-1 * time.Minute).Format(time.RFC3339Nano),
			"source":    "claude",
			"text":      "Working on the test task...",
		},
	}

	file, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	enc := json.NewEncoder(file)
	for _, e := range evts {
		if err := enc.Encode(e); err != nil {
			t.Fatal(err)
		}
	}
	_ = file.Close()

	logReader := NewLogReader(logPath)
	cfg := &config.ObserverConfig{
		RecentEvents: 10,
	}
	builder := NewContextBuilder(logReader, cfg)

	state := DrainState{
		Status:    "working",
		Uptime:    10 * time.Minute,
		TotalCost: 0.25,
		CurrentBead: &CurrentBeadInfo{
			ID:        "bd-test-001",
			Title:     "Test bead",
			StartedAt: time.Now().Add(-2 * time.Minute),
		},
	}

	ctx, err := builder.Build(state, nil)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Verify current bead section
	if !strings.Contains(ctx, "## Current Bead: bd-test-001") {
		t.Error("missing current bead section")
	}
	if !strings.Contains(ctx, "Title: Test bead") {
		t.Error("missing bead title")
	}
	if !strings.Contains(ctx, "### Recent Activity") {
		t.Error("missing recent activity section")
	}
}

func TestContextBuilder_SessionHistory(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Write iteration start/end pairs
	evts := []any{
		map[string]any{
			"type":      "iteration.start",
			"timestamp": time.Now().Add(-30 * time.Minute).Format(time.RFC3339Nano),
			"source":    "atari",
			"bead_id":   "bd-001",
			"title":     "First bead",
			"priority":  2,
			"attempt":   1,
		},
		map[string]any{
			"type":           "iteration.end",
			"timestamp":      time.Now().Add(-25 * time.Minute).Format(time.RFC3339Nano),
			"source":         "atari",
			"bead_id":        "bd-001",
			"success":        true,
			"num_turns":      8,
			"duration_ms":    300000,
			"total_cost_usd": 0.36,
		},
		map[string]any{
			"type":      "iteration.start",
			"timestamp": time.Now().Add(-20 * time.Minute).Format(time.RFC3339Nano),
			"source":    "atari",
			"bead_id":   "bd-002",
			"title":     "Second bead",
			"priority":  2,
			"attempt":   1,
		},
		map[string]any{
			"type":           "iteration.end",
			"timestamp":      time.Now().Add(-10 * time.Minute).Format(time.RFC3339Nano),
			"source":         "atari",
			"bead_id":        "bd-002",
			"success":        false,
			"num_turns":      12,
			"duration_ms":    600000,
			"total_cost_usd": 0.52,
		},
	}

	file, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	enc := json.NewEncoder(file)
	for _, e := range evts {
		if err := enc.Encode(e); err != nil {
			t.Fatal(err)
		}
	}
	_ = file.Close()

	logReader := NewLogReader(logPath)
	builder := NewContextBuilder(logReader, nil)

	history, err := builder.loadSessionHistory()
	if err != nil {
		t.Fatalf("loadSessionHistory failed: %v", err)
	}

	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history))
	}

	// Check first entry
	if history[0].BeadID != "bd-001" {
		t.Errorf("expected bd-001, got %s", history[0].BeadID)
	}
	if history[0].Outcome != "completed" {
		t.Errorf("expected completed, got %s", history[0].Outcome)
	}
	if history[0].Turns != 8 {
		t.Errorf("expected 8 turns, got %d", history[0].Turns)
	}

	// Check second entry
	if history[1].BeadID != "bd-002" {
		t.Errorf("expected bd-002, got %s", history[1].BeadID)
	}
	if history[1].Outcome != "failed" {
		t.Errorf("expected failed, got %s", history[1].Outcome)
	}
}

func TestContextBuilder_SessionHistoryLimit(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Write more than maxSessionHistory entries
	file, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	enc := json.NewEncoder(file)

	for i := 0; i < 10; i++ {
		beadID := "bd-" + string(rune('a'+i))
		start := map[string]any{
			"type":      "iteration.start",
			"timestamp": time.Now().Add(-time.Duration(20-i) * time.Minute).Format(time.RFC3339Nano),
			"source":    "atari",
			"bead_id":   beadID,
			"title":     "Bead " + string(rune('A'+i)),
			"priority":  2,
			"attempt":   1,
		}
		end := map[string]any{
			"type":           "iteration.end",
			"timestamp":      time.Now().Add(-time.Duration(19-i) * time.Minute).Format(time.RFC3339Nano),
			"source":         "atari",
			"bead_id":        beadID,
			"success":        true,
			"num_turns":      5,
			"duration_ms":    60000,
			"total_cost_usd": 0.10,
		}
		if err := enc.Encode(start); err != nil {
			t.Fatal(err)
		}
		if err := enc.Encode(end); err != nil {
			t.Fatal(err)
		}
	}
	_ = file.Close()

	logReader := NewLogReader(logPath)
	builder := NewContextBuilder(logReader, nil)

	history, err := builder.loadSessionHistory()
	if err != nil {
		t.Fatalf("loadSessionHistory failed: %v", err)
	}

	// Should be limited to maxSessionHistory (5)
	if len(history) != maxSessionHistory {
		t.Errorf("expected %d history entries, got %d", maxSessionHistory, len(history))
	}

	// Should be the most recent entries
	if history[0].BeadID != "bd-f" {
		t.Errorf("expected bd-f (6th bead), got %s", history[0].BeadID)
	}
}

func TestFormatEvent_ClaudeText(t *testing.T) {
	ev := &events.ClaudeTextEvent{
		BaseEvent: events.BaseEvent{
			EventType: events.EventClaudeText,
			Time:      time.Date(2024, 1, 15, 14, 2, 13, 0, time.UTC),
			Src:       "claude",
		},
		Text: "Working on the implementation...",
	}

	result := FormatEvent(ev)

	if !strings.Contains(result, "[14:02:13]") {
		t.Error("missing timestamp")
	}
	if !strings.Contains(result, "Claude:") {
		t.Error("missing Claude prefix")
	}
	if !strings.Contains(result, "Working on the implementation") {
		t.Error("missing text content")
	}
}

func TestFormatEvent_ClaudeToolUse(t *testing.T) {
	ev := &events.ClaudeToolUseEvent{
		BaseEvent: events.BaseEvent{
			EventType: events.EventClaudeToolUse,
			Time:      time.Date(2024, 1, 15, 14, 2, 15, 0, time.UTC),
			Src:       "claude",
		},
		ToolID:   "toolu_01ABCdefghijklmnop",
		ToolName: "Bash",
		Input: map[string]any{
			"command":     "go test ./...",
			"description": "Run all tests",
		},
	}

	result := FormatEvent(ev)

	if !strings.Contains(result, "[14:02:15]") {
		t.Error("missing timestamp")
	}
	if !strings.Contains(result, "Tool: Bash") {
		t.Error("missing tool name")
	}
	if !strings.Contains(result, "Run all tests") {
		t.Error("missing description")
	}
	if !strings.Contains(result, "toolu_01ABCdefg...") {
		t.Error("missing truncated tool ID")
	}
}

func TestFormatEvent_ClaudeToolResult(t *testing.T) {
	ev := &events.ClaudeToolResultEvent{
		BaseEvent: events.BaseEvent{
			EventType: events.EventClaudeToolResult,
			Time:      time.Date(2024, 1, 15, 14, 2, 16, 0, time.UTC),
			Src:       "claude",
		},
		ToolID:  "toolu_01XYZ",
		Content: "PASS ok github.com/example/project 0.5s",
		IsError: false,
	}

	result := FormatEvent(ev)

	if !strings.Contains(result, "Result:") {
		t.Error("missing Result prefix")
	}
	if strings.Contains(result, "ERROR") {
		t.Error("should not contain ERROR for non-error result")
	}

	// Test error result
	ev.IsError = true
	ev.Content = "exit status 1"
	result = FormatEvent(ev)

	if !strings.Contains(result, "Result ERROR:") {
		t.Error("missing ERROR prefix for error result")
	}
}

func TestFormatEvent_IterationEvents(t *testing.T) {
	// Test iteration start
	startEv := &events.IterationStartEvent{
		BaseEvent: events.BaseEvent{
			EventType: events.EventIterationStart,
			Time:      time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC),
			Src:       "atari",
		},
		BeadID:   "bd-test-001",
		Title:    "Fix authentication bug",
		Priority: 1,
		Attempt:  1,
	}

	result := FormatEvent(startEv)
	if !strings.Contains(result, "Started bead bd-test-001") {
		t.Error("missing bead ID in start event")
	}
	if !strings.Contains(result, "Fix authentication bug") {
		t.Error("missing title in start event")
	}

	// Test iteration end
	endEv := &events.IterationEndEvent{
		BaseEvent: events.BaseEvent{
			EventType: events.EventIterationEnd,
			Time:      time.Date(2024, 1, 15, 14, 10, 0, 0, time.UTC),
			Src:       "atari",
		},
		BeadID:       "bd-test-001",
		Success:      true,
		NumTurns:     15,
		DurationMs:   600000,
		TotalCostUSD: 0.45,
	}

	result = FormatEvent(endEv)
	if !strings.Contains(result, "Bead bd-test-001 completed") {
		t.Error("missing completion message")
	}
	if !strings.Contains(result, "$0.45") {
		t.Error("missing cost")
	}

	// Test failed iteration
	endEv.Success = false
	result = FormatEvent(endEv)
	if !strings.Contains(result, "failed") {
		t.Error("should show failed for unsuccessful iteration")
	}
}

func TestFormatEvent_ErrorEvent(t *testing.T) {
	ev := &events.ErrorEvent{
		BaseEvent: events.BaseEvent{
			EventType: events.EventError,
			Time:      time.Date(2024, 1, 15, 14, 5, 0, 0, time.UTC),
			Src:       "atari",
		},
		Message:  "Session timeout after 5m0s",
		Severity: events.SeverityError,
	}

	result := FormatEvent(ev)
	if !strings.Contains(result, "ERROR:") {
		t.Error("missing ERROR prefix")
	}
	if !strings.Contains(result, "Session timeout") {
		t.Error("missing error message")
	}
}

func TestFormatToolSummary(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		input    map[string]any
		want     string
	}{
		{
			name:     "Bash with description",
			toolName: "Bash",
			input:    map[string]any{"description": "Run tests", "command": "go test ./..."},
			want:     `"Run tests"`,
		},
		{
			name:     "Bash with command only",
			toolName: "Bash",
			input:    map[string]any{"command": "ls -la"},
			want:     `"ls -la"`,
		},
		{
			name:     "Read with file_path",
			toolName: "Read",
			input:    map[string]any{"file_path": "/path/to/file.go"},
			want:     "file.go",
		},
		{
			name:     "Grep with pattern",
			toolName: "Grep",
			input:    map[string]any{"pattern": "func TestFoo"},
			want:     `"func TestFoo"`,
		},
		{
			name:     "Task with description",
			toolName: "Task",
			input:    map[string]any{"description": "Search for errors"},
			want:     `"Search for errors"`,
		},
		{
			name:     "Unknown tool",
			toolName: "Unknown",
			input:    map[string]any{"foo": "bar"},
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatToolSummary(tt.toolName, tt.input)
			if got != tt.want {
				t.Errorf("formatToolSummary(%s) = %q, want %q", tt.toolName, got, tt.want)
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
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a long string", 10, "this is..."},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc"},
		{"", 10, ""},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

func TestShortID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"toolu_01ABCdefghijklmnopqrs", "toolu_01ABCdefg..."},
		{"short", "short"},
		{"exactly15chars!", "exactly15chars!"},
	}

	for _, tt := range tests {
		got := shortID(tt.input)
		if got != tt.want {
			t.Errorf("shortID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{5 * time.Minute, "5m"},
		{2 * time.Hour, "2h"},
		{2*time.Hour + 30*time.Minute, "2h 30m"},
		{90 * time.Second, "1m"},
		{0, "0s"},
	}

	for _, tt := range tests {
		got := formatDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestContextBuilder_RecentEventsLimit(t *testing.T) {
	// Test with config
	cfg := &config.ObserverConfig{RecentEvents: 15}
	builder := NewContextBuilder(nil, cfg)
	if builder.recentEventsLimit() != 15 {
		t.Errorf("expected 15, got %d", builder.recentEventsLimit())
	}

	// Test with nil config
	builder = NewContextBuilder(nil, nil)
	if builder.recentEventsLimit() != defaultRecentEvents {
		t.Errorf("expected %d, got %d", defaultRecentEvents, builder.recentEventsLimit())
	}

	// Test with zero config value
	cfg = &config.ObserverConfig{RecentEvents: 0}
	builder = NewContextBuilder(nil, cfg)
	if builder.recentEventsLimit() != defaultRecentEvents {
		t.Errorf("expected %d, got %d", defaultRecentEvents, builder.recentEventsLimit())
	}
}

func TestContextBuilder_SystemPrompt(t *testing.T) {
	builder := NewContextBuilder(nil, nil)
	prompt := builder.buildSystemPrompt()

	// Check for key elements
	if !strings.Contains(prompt, "observer assistant") {
		t.Error("missing role description")
	}
	if !strings.Contains(prompt, "Answer questions") {
		t.Error("missing responsibility")
	}
	if !strings.Contains(prompt, "grep") {
		t.Error("missing tool mention")
	}
}

func TestContextBuilder_TipsSection(t *testing.T) {
	builder := NewContextBuilder(nil, nil)
	tips := builder.buildTipsSection()

	if !strings.Contains(tips, "grep") {
		t.Error("missing grep command")
	}
	if !strings.Contains(tips, "tail") {
		t.Error("missing tail command")
	}
	if !strings.Contains(tips, "br show") {
		t.Error("missing br show command")
	}
}

func TestContextBuilder_ConversationHistorySection(t *testing.T) {
	builder := NewContextBuilder(nil, nil)

	conversation := []Exchange{
		{Question: "What is happening?", Answer: "The drain is idle."},
		{Question: "Any errors?", Answer: "No errors found."},
	}

	section := builder.buildConversationHistorySection(conversation)

	// Check header
	if !strings.Contains(section, "## Conversation History") {
		t.Error("missing section header")
	}

	// Check exchange 1
	if !strings.Contains(section, "### Exchange 1") {
		t.Error("missing exchange 1 header")
	}
	if !strings.Contains(section, "**User:** What is happening?") {
		t.Error("missing question 1")
	}
	if !strings.Contains(section, "**Assistant:** The drain is idle.") {
		t.Error("missing answer 1")
	}

	// Check exchange 2
	if !strings.Contains(section, "### Exchange 2") {
		t.Error("missing exchange 2 header")
	}
	if !strings.Contains(section, "**User:** Any errors?") {
		t.Error("missing question 2")
	}
	if !strings.Contains(section, "**Assistant:** No errors found.") {
		t.Error("missing answer 2")
	}
}

func TestContextBuilder_BuildWithConversationHistory(t *testing.T) {
	// Create temp dir for log file
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "atari.log")

	// Create empty log file
	if err := os.WriteFile(logPath, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create log file: %v", err)
	}

	logReader := NewLogReader(logPath)
	builder := NewContextBuilder(logReader, nil)

	state := DrainState{
		Status: "idle",
	}

	conversation := []Exchange{
		{Question: "What is happening?", Answer: "The drain is idle."},
	}

	ctx, err := builder.Build(state, conversation)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Verify conversation history is included
	if !strings.Contains(ctx, "## Conversation History") {
		t.Error("missing conversation history section")
	}
	if !strings.Contains(ctx, "What is happening?") {
		t.Error("missing question in context")
	}
	if !strings.Contains(ctx, "The drain is idle.") {
		t.Error("missing answer in context")
	}
}

func TestContextBuilder_BuildWithoutConversationHistory(t *testing.T) {
	// Create temp dir for log file
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "atari.log")

	// Create empty log file
	if err := os.WriteFile(logPath, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create log file: %v", err)
	}

	logReader := NewLogReader(logPath)
	builder := NewContextBuilder(logReader, nil)

	state := DrainState{
		Status: "idle",
	}

	// Pass nil or empty conversation
	ctx, err := builder.Build(state, nil)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Verify conversation history section is NOT included
	if strings.Contains(ctx, "## Conversation History") {
		t.Error("conversation history section should not be present with nil history")
	}

	// Also test with empty slice
	ctx, err = builder.Build(state, []Exchange{})
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if strings.Contains(ctx, "## Conversation History") {
		t.Error("conversation history section should not be present with empty history")
	}
}
