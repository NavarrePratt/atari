package testutil

import (
	"bufio"
	"encoding/json"
	"strings"
	"testing"
)

func TestMockClaudeOutput_Reader(t *testing.T) {
	output := &MockClaudeOutput{
		Events: []string{
			`{"type":"system"}`,
			`{"type":"assistant"}`,
		},
	}

	reader := output.Reader()
	scanner := bufio.NewScanner(reader)

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != `{"type":"system"}` {
		t.Errorf("unexpected first line: %s", lines[0])
	}
	if lines[1] != `{"type":"assistant"}` {
		t.Errorf("unexpected second line: %s", lines[1])
	}
}

func TestMockClaudeOutput_String(t *testing.T) {
	output := &MockClaudeOutput{
		Events: []string{
			`{"type":"system"}`,
			`{"type":"result"}`,
		},
	}

	str := output.String()
	expected := `{"type":"system"}` + "\n" + `{"type":"result"}` + "\n"

	if str != expected {
		t.Errorf("expected %q, got %q", expected, str)
	}
}

func TestMockClaudeOutput_AddEvent(t *testing.T) {
	output := &MockClaudeOutput{}

	output.AddEvent(`{"type":"system"}`)
	output.AddEvent(`{"type":"result"}`)

	if len(output.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(output.Events))
	}
}

func TestNewSuccessfulSession(t *testing.T) {
	output := NewSuccessfulSession("test-session-123")

	if len(output.Events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(output.Events))
	}

	// First event should be system init
	var firstEvent map[string]any
	if err := json.Unmarshal([]byte(output.Events[0]), &firstEvent); err != nil {
		t.Fatalf("failed to parse first event: %v", err)
	}
	if firstEvent["type"] != "system" {
		t.Errorf("first event type should be 'system', got %v", firstEvent["type"])
	}
	if firstEvent["session_id"] != "test-session-123" {
		t.Errorf("session_id should be 'test-session-123', got %v", firstEvent["session_id"])
	}

	// Last event should be result success
	var lastEvent map[string]any
	if err := json.Unmarshal([]byte(output.Events[len(output.Events)-1]), &lastEvent); err != nil {
		t.Fatalf("failed to parse last event: %v", err)
	}
	if lastEvent["type"] != "result" {
		t.Errorf("last event type should be 'result', got %v", lastEvent["type"])
	}
	if lastEvent["subtype"] != "success" {
		t.Errorf("last event subtype should be 'success', got %v", lastEvent["subtype"])
	}
}

func TestNewSuccessfulSessionWithBRClose(t *testing.T) {
	output := NewSuccessfulSessionWithBRClose("test-session", "bd-001")

	// Should contain a tool_use for br close
	found := false
	for _, event := range output.Events {
		if strings.Contains(event, "br close bd-001") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected br close command in tool_use")
	}
}

func TestNewFailedSession(t *testing.T) {
	output := NewFailedSession("test-session", "command failed")

	// Last event should be result with error
	var lastEvent map[string]any
	if err := json.Unmarshal([]byte(output.Events[len(output.Events)-1]), &lastEvent); err != nil {
		t.Fatalf("failed to parse last event: %v", err)
	}
	if lastEvent["type"] != "result" {
		t.Errorf("last event type should be 'result', got %v", lastEvent["type"])
	}
	if lastEvent["subtype"] != "error_tool_use" {
		t.Errorf("last event subtype should be 'error_tool_use', got %v", lastEvent["subtype"])
	}
	if lastEvent["error"] != "command failed" {
		t.Errorf("error should be 'command failed', got %v", lastEvent["error"])
	}
}

func TestNewMaxTurnsSession(t *testing.T) {
	output := NewMaxTurnsSession("test-session", 5)

	// Last event should be result with max_turns error
	var lastEvent map[string]any
	if err := json.Unmarshal([]byte(output.Events[len(output.Events)-1]), &lastEvent); err != nil {
		t.Fatalf("failed to parse last event: %v", err)
	}
	if lastEvent["type"] != "result" {
		t.Errorf("last event type should be 'result', got %v", lastEvent["type"])
	}
	if lastEvent["subtype"] != "error_max_turns" {
		t.Errorf("last event subtype should be 'error_max_turns', got %v", lastEvent["subtype"])
	}
	if int(lastEvent["num_turns"].(float64)) != 5 {
		t.Errorf("num_turns should be 5, got %v", lastEvent["num_turns"])
	}
}

func TestNewTimeoutSession(t *testing.T) {
	output := NewTimeoutSession("test-session")

	// Should have init and assistant but no result
	hasInit := false
	hasResult := false
	for _, event := range output.Events {
		if strings.Contains(event, `"type":"system"`) {
			hasInit = true
		}
		if strings.Contains(event, `"type":"result"`) {
			hasResult = true
		}
	}

	if !hasInit {
		t.Error("expected system init event")
	}
	if hasResult {
		t.Error("timeout session should not have result event")
	}
}

func TestNewSessionWithToolUse(t *testing.T) {
	tools := []ToolCall{
		{
			Name:   "Bash",
			Input:  map[string]any{"command": "ls"},
			Result: "file1\nfile2",
		},
		{
			Name:   "Read",
			Input:  map[string]any{"file_path": "/tmp/test"},
			Result: "content",
		},
	}

	output := NewSessionWithToolUse("test-session", tools)

	// Should have tool_use and tool_result for each tool
	bashFound := false
	readFound := false
	for _, event := range output.Events {
		if strings.Contains(event, `"name":"Bash"`) {
			bashFound = true
		}
		if strings.Contains(event, `"name":"Read"`) {
			readFound = true
		}
	}

	if !bashFound {
		t.Error("expected Bash tool_use event")
	}
	if !readFound {
		t.Error("expected Read tool_use event")
	}
}

func TestAllEventsAreValidJSON(t *testing.T) {
	outputs := []*MockClaudeOutput{
		NewSuccessfulSession("s1"),
		NewSuccessfulSessionWithBRClose("s2", "bd-001"),
		NewFailedSession("s3", "error"),
		NewMaxTurnsSession("s4", 3),
		NewTimeoutSession("s5"),
		NewSessionWithToolUse("s6", []ToolCall{{Name: "Bash", Input: map[string]any{"command": "ls"}, Result: "ok"}}),
	}

	for i, output := range outputs {
		for j, event := range output.Events {
			var parsed map[string]any
			if err := json.Unmarshal([]byte(event), &parsed); err != nil {
				t.Errorf("output %d, event %d is invalid JSON: %v\nevent: %s", i, j, err, event)
			}
		}
	}
}

func TestEscapeJSON(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"hello\nworld", `hello\nworld`},
		{`hello "world"`, `hello \"world\"`},
		{"tab\there", `tab\there`},
	}

	for _, tt := range tests {
		result := escapeJSON(tt.input)
		if result != tt.expected {
			t.Errorf("escapeJSON(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
