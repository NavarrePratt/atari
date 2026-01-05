// Package testutil provides test infrastructure for unit and integration testing.
package testutil

import (
	"os"
	"path/filepath"
)

// MockClaudeScript generates a mock claude script for observer testing.
// It outputs stream-json format to match actual Claude CLI behavior.
type MockClaudeScript struct {
	// Response is the text response to return.
	Response string

	// Delay is an optional sleep duration (e.g., "0.1" for 100ms).
	Delay string

	// FailWithError causes the script to exit with an error.
	FailWithError string

	// VerifyResume if true, checks that --resume flag is present.
	VerifyResume bool

	// SessionID to check when VerifyResume is true.
	ExpectedSessionID string

	// OutputSessionID is the session ID to include in the result event.
	OutputSessionID string
}

// Write creates the mock claude script at the given path.
// Returns an error if the script cannot be created.
func (m *MockClaudeScript) Write(path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	var script string

	if m.FailWithError != "" {
		script = m.buildFailingScript()
	} else if m.VerifyResume {
		script = m.buildResumeVerifyScript()
	} else {
		script = m.buildSuccessScript()
	}

	return os.WriteFile(path, []byte(script), 0755)
}

func (m *MockClaudeScript) buildSuccessScript() string {
	delay := m.Delay
	if delay == "" {
		delay = "0"
	}

	sessionID := m.OutputSessionID
	if sessionID == "" {
		sessionID = "mock-session-" + delay
	}

	// Output stream-json format to match actual Claude CLI behavior
	return `#!/bin/bash
# Mock claude for observer testing - returns stream-json format

# Optional delay
sleep ` + delay + `

# Output stream-json events
echo '{"type":"system","subtype":"init","model":"haiku","tools":[]}'
echo '{"type":"assistant","message":{"content":[{"type":"text","text":"` + m.Response + `"}]}}'
echo '{"type":"result","result":"` + m.Response + `","session_id":"` + sessionID + `","num_turns":1}'
`
}

func (m *MockClaudeScript) buildFailingScript() string {
	return `#!/bin/bash
# Mock claude that fails
echo "Error: ` + m.FailWithError + `" >&2
exit 1
`
}

func (m *MockClaudeScript) buildResumeVerifyScript() string {
	delay := m.Delay
	if delay == "" {
		delay = "0"
	}

	expectedID := m.ExpectedSessionID
	if expectedID == "" {
		expectedID = "test-session-123"
	}

	sessionID := m.OutputSessionID
	if sessionID == "" {
		sessionID = expectedID
	}

	return `#!/bin/bash
# Mock claude that verifies --resume flag and outputs stream-json

# Check for --resume flag
RESUME_FLAG=""
for arg in "$@"; do
    if [ "$arg" = "--resume" ]; then
        RESUME_FLAG="found"
    fi
    if [ "$RESUME_FLAG" = "found" ] && [ "$arg" != "--resume" ]; then
        if [ "$arg" = "` + expectedID + `" ]; then
            sleep ` + delay + `
            echo '{"type":"system","subtype":"init","model":"haiku","tools":[]}'
            echo '{"type":"assistant","message":{"content":[{"type":"text","text":"Follow-up response with session context preserved."}]}}'
            echo '{"type":"result","result":"Follow-up response with session context preserved.","session_id":"` + sessionID + `","num_turns":1}'
            exit 0
        else
            echo "Error: unexpected session ID: $arg" >&2
            exit 1
        fi
    fi
done

# No --resume flag - this is the first question
sleep ` + delay + `
echo '{"type":"system","subtype":"init","model":"haiku","tools":[]}'
echo '{"type":"assistant","message":{"content":[{"type":"text","text":"` + m.Response + `"}]}}'
echo '{"type":"result","result":"` + m.Response + `","session_id":"` + sessionID + `","num_turns":1}'
`
}

// CreateMockClaudeForObserver creates a simple mock claude script for observer tests.
// It returns stream-json format output.
func CreateMockClaudeForObserver(path, response string) error {
	script := &MockClaudeScript{Response: response}
	return script.Write(path)
}

// CreateSlowMockClaude creates a mock that delays before responding.
func CreateSlowMockClaude(path, response string, delaySeconds string) error {
	script := &MockClaudeScript{
		Response: response,
		Delay:    delaySeconds,
	}
	return script.Write(path)
}

// CreateFailingMockClaudeForObserver creates a mock that fails with an error.
func CreateFailingMockClaudeForObserver(path, errorMsg string) error {
	script := &MockClaudeScript{FailWithError: errorMsg}
	return script.Write(path)
}
