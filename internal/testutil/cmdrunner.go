// Package testutil provides test infrastructure for unit and integration testing.
// It includes mocks, fixtures, and helpers that other packages use for testing.
package testutil

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/npratt/atari/internal/exec"
)

// CommandRunner is an alias to exec.CommandRunner for backward compatibility.
type CommandRunner = exec.CommandRunner

// CommandCall records a command invocation for assertion purposes.
type CommandCall struct {
	Name string
	Args []string
}

// DynamicResponseFunc is called to generate dynamic responses for commands.
// If it returns (nil, nil), the normal response lookup is used.
type DynamicResponseFunc func(ctx context.Context, name string, args []string) ([]byte, error, bool)

// MockRunner returns canned responses based on command patterns.
// It records all calls for later assertion.
type MockRunner struct {
	mu              sync.Mutex
	Responses       map[string][]byte
	Errors          map[string]error
	Calls           []CommandCall
	DynamicResponse DynamicResponseFunc
}

// NewMockRunner creates a MockRunner with initialized maps.
func NewMockRunner() *MockRunner {
	return &MockRunner{
		Responses: make(map[string][]byte),
		Errors:    make(map[string]error),
		Calls:     nil,
	}
}

// Run executes a mock command, recording the call and returning canned responses.
// The key format is "name arg1 arg2 ...".
func (m *MockRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Calls = append(m.Calls, CommandCall{Name: name, Args: args})

	// Check dynamic response first
	if m.DynamicResponse != nil {
		if resp, err, handled := m.DynamicResponse(ctx, name, args); handled {
			return resp, err
		}
	}

	key := makeKey(name, args)

	// Check for exact match first
	if err, ok := m.Errors[key]; ok {
		return nil, err
	}
	if resp, ok := m.Responses[key]; ok {
		return resp, nil
	}

	// Check for prefix matches (for commands with variable args)
	for k, err := range m.Errors {
		if strings.HasPrefix(key, k) {
			return nil, err
		}
	}
	for k, resp := range m.Responses {
		if strings.HasPrefix(key, k) {
			return resp, nil
		}
	}

	return nil, fmt.Errorf("unexpected command: %s", key)
}

// SetResponse configures a canned response for a command.
func (m *MockRunner) SetResponse(name string, args []string, response []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Responses[makeKey(name, args)] = response
}

// SetError configures an error response for a command.
func (m *MockRunner) SetError(name string, args []string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Errors[makeKey(name, args)] = err
}

// GetCalls returns a copy of all recorded calls.
func (m *MockRunner) GetCalls() []CommandCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]CommandCall, len(m.Calls))
	copy(result, m.Calls)
	return result
}

// Reset clears all recorded calls.
func (m *MockRunner) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = nil
}

// makeKey constructs the lookup key from command name and args.
func makeKey(name string, args []string) string {
	if len(args) == 0 {
		return name
	}
	return name + " " + strings.Join(args, " ")
}

// NewExecRunner creates a new ExecRunner for production use.
// This is a convenience wrapper that delegates to exec.NewExecRunner.
func NewExecRunner() *exec.ExecRunner {
	return exec.NewExecRunner()
}

// SetupMockEnrichment configures the MockRunner to return bead-specific responses
// for "br show <id> --json" calls. This simulates the enrichment flow where
// br list returns basic data and br show returns full dependency details.
//
// Usage:
//
//	runner := testutil.NewMockRunner()
//	runner.SetResponse("br", []string{"list", "--json"}, []byte(testutil.GraphListActiveJSON))
//	testutil.SetupMockEnrichment(runner, map[string]string{
//	    "bd-epic-001": testutil.GraphShowBeadEpic001JSON,
//	    "bd-task-001": testutil.GraphShowBeadTask001JSON,
//	    "bd-task-002": testutil.GraphShowBeadTask002JSON,
//	})
func SetupMockEnrichment(runner *MockRunner, fixtures map[string]string) {
	runner.DynamicResponse = func(ctx context.Context, name string, args []string) ([]byte, error, bool) {
		if name == "br" && len(args) >= 3 && args[0] == "show" && args[2] == "--json" {
			beadID := args[1]
			if fixture, ok := fixtures[beadID]; ok {
				return []byte(fixture), nil, true
			}
			return nil, fmt.Errorf("bead not found: %s", beadID), true
		}
		return nil, nil, false
	}
}

