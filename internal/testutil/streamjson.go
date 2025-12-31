package testutil

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// MockClaudeOutput generates mock Claude stream-json output for testing.
type MockClaudeOutput struct {
	Events []string
}

// Reader returns an io.Reader that yields the events as newline-delimited JSON.
func (m *MockClaudeOutput) Reader() io.Reader {
	return strings.NewReader(strings.Join(m.Events, "\n") + "\n")
}

// String returns the events as a single string with newlines.
func (m *MockClaudeOutput) String() string {
	return strings.Join(m.Events, "\n") + "\n"
}

// AddEvent appends a raw JSON event string to the output.
func (m *MockClaudeOutput) AddEvent(event string) {
	m.Events = append(m.Events, event)
}

// NewSuccessfulSession creates events for a successful Claude session.
// It includes init, assistant response, and success result.
func NewSuccessfulSession(sessionID string) *MockClaudeOutput {
	return &MockClaudeOutput{
		Events: []string{
			fmt.Sprintf(`{"type":"system","subtype":"init","session_id":"%s","cwd":"/workspace"}`, sessionID),
			`{"type":"assistant","message":{"content":[{"type":"text","text":"I'll work on this task."}]}}`,
			`{"type":"assistant","message":{"content":[{"type":"text","text":"Task completed successfully."}]}}`,
			fmt.Sprintf(`{"type":"result","subtype":"success","total_cost_usd":0.05,"duration_ms":10000,"num_turns":3,"session_id":"%s"}`, sessionID),
		},
	}
}

// NewSuccessfulSessionWithBDClose creates events for a session that closes a bead.
func NewSuccessfulSessionWithBDClose(sessionID, beadID string) *MockClaudeOutput {
	return &MockClaudeOutput{
		Events: []string{
			fmt.Sprintf(`{"type":"system","subtype":"init","session_id":"%s","cwd":"/workspace"}`, sessionID),
			`{"type":"assistant","message":{"content":[{"type":"text","text":"Working on the task..."}]}}`,
			fmt.Sprintf(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tool_001","name":"Bash","input":{"command":"bd close %s --reason done"}}]}}`, beadID),
			`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tool_001","content":"Issue closed"}]}}`,
			fmt.Sprintf(`{"type":"result","subtype":"success","total_cost_usd":0.05,"duration_ms":15000,"num_turns":4,"session_id":"%s"}`, sessionID),
		},
	}
}

// NewFailedSession creates events for a failed Claude session.
func NewFailedSession(sessionID, errorMsg string) *MockClaudeOutput {
	return &MockClaudeOutput{
		Events: []string{
			fmt.Sprintf(`{"type":"system","subtype":"init","session_id":"%s","cwd":"/workspace"}`, sessionID),
			`{"type":"assistant","message":{"content":[{"type":"text","text":"I'll try to complete this task."}]}}`,
			fmt.Sprintf(`{"type":"result","subtype":"error_tool_use","error":"%s"}`, errorMsg),
		},
	}
}

// NewMaxTurnsSession creates events for a session that hit the max turns limit.
func NewMaxTurnsSession(sessionID string, numTurns int) *MockClaudeOutput {
	events := []string{
		fmt.Sprintf(`{"type":"system","subtype":"init","session_id":"%s","cwd":"/workspace"}`, sessionID),
	}

	// Add some assistant/tool interactions
	for i := 0; i < numTurns; i++ {
		events = append(events,
			fmt.Sprintf(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tool_%03d","name":"Bash","input":{"command":"echo turn %d"}}]}}`, i, i),
			fmt.Sprintf(`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tool_%03d","content":"turn %d\n"}]}}`, i, i),
		)
	}

	events = append(events,
		fmt.Sprintf(`{"type":"result","subtype":"error_max_turns","total_cost_usd":0.15,"duration_ms":60000,"num_turns":%d,"session_id":"%s"}`, numTurns, sessionID),
	)

	return &MockClaudeOutput{Events: events}
}

// NewTimeoutSession creates events for a session that times out mid-stream.
// The output is truncated (no result event).
func NewTimeoutSession(sessionID string) *MockClaudeOutput {
	return &MockClaudeOutput{
		Events: []string{
			fmt.Sprintf(`{"type":"system","subtype":"init","session_id":"%s","cwd":"/workspace"}`, sessionID),
			`{"type":"assistant","message":{"content":[{"type":"text","text":"Starting work..."}]}}`,
			// No result event - simulates timeout/crash
		},
	}
}

// NewSessionWithToolUse creates a session with specific tool calls.
func NewSessionWithToolUse(sessionID string, tools []ToolCall) *MockClaudeOutput {
	events := []string{
		fmt.Sprintf(`{"type":"system","subtype":"init","session_id":"%s","cwd":"/workspace"}`, sessionID),
	}

	for i, tool := range tools {
		inputJSON, _ := json.Marshal(tool.Input)
		events = append(events,
			fmt.Sprintf(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"tool_%03d","name":"%s","input":%s}]}}`, i, tool.Name, string(inputJSON)),
			fmt.Sprintf(`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tool_%03d","content":"%s"}]}}`, i, escapeJSON(tool.Result)),
		)
	}

	events = append(events,
		fmt.Sprintf(`{"type":"result","subtype":"success","total_cost_usd":0.08,"duration_ms":20000,"num_turns":%d,"session_id":"%s"}`, len(tools)+1, sessionID),
	)

	return &MockClaudeOutput{Events: events}
}

// ToolCall represents a tool invocation for test scenarios.
type ToolCall struct {
	Name   string
	Input  map[string]any
	Result string
}

// escapeJSON escapes a string for use in JSON.
func escapeJSON(s string) string {
	b, _ := json.Marshal(s)
	// Remove surrounding quotes
	return string(b[1 : len(b)-1])
}
