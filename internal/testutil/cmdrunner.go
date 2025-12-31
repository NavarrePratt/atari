// Package testutil provides test infrastructure for unit and integration testing.
// It includes mocks, fixtures, and helpers that other packages use for testing.
package testutil

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// CommandRunner abstracts command execution for testing.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// CommandCall records a command invocation for assertion purposes.
type CommandCall struct {
	Name string
	Args []string
}

// MockRunner returns canned responses based on command patterns.
// It records all calls for later assertion.
type MockRunner struct {
	mu        sync.Mutex
	Responses map[string][]byte
	Errors    map[string]error
	Calls     []CommandCall
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
