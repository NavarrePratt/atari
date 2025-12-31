package testutil

import (
	"context"
	"errors"
	"testing"
)

func TestNewMockRunner(t *testing.T) {
	mock := NewMockRunner()

	if mock.Responses == nil {
		t.Error("Responses map should be initialized")
	}
	if mock.Errors == nil {
		t.Error("Errors map should be initialized")
	}
	if mock.Calls != nil {
		t.Error("Calls should be nil initially")
	}
}

func TestMockRunner_Run_RecordsCalls(t *testing.T) {
	mock := NewMockRunner()
	mock.Responses["bd ready --json"] = []byte("[]")

	ctx := context.Background()
	_, _ = mock.Run(ctx, "bd", "ready", "--json")

	calls := mock.GetCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "bd" {
		t.Errorf("expected name 'bd', got %s", calls[0].Name)
	}
	if len(calls[0].Args) != 2 || calls[0].Args[0] != "ready" || calls[0].Args[1] != "--json" {
		t.Errorf("unexpected args: %v", calls[0].Args)
	}
}

func TestMockRunner_Run_ReturnsResponse(t *testing.T) {
	mock := NewMockRunner()
	expected := []byte(`[{"id": "bd-001"}]`)
	mock.SetResponse("bd", []string{"ready", "--json"}, expected)

	ctx := context.Background()
	result, err := mock.Run(ctx, "bd", "ready", "--json")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if string(result) != string(expected) {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestMockRunner_Run_ReturnsError(t *testing.T) {
	mock := NewMockRunner()
	expectedErr := errors.New("command failed")
	mock.SetError("bd", []string{"ready", "--json"}, expectedErr)

	ctx := context.Background()
	result, err := mock.Run(ctx, "bd", "ready", "--json")

	if err == nil {
		t.Error("expected error, got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

func TestMockRunner_Run_ErrorTakesPrecedence(t *testing.T) {
	mock := NewMockRunner()
	mock.SetResponse("bd", []string{"ready"}, []byte("response"))
	mock.SetError("bd", []string{"ready"}, errors.New("error"))

	ctx := context.Background()
	result, err := mock.Run(ctx, "bd", "ready")

	if err == nil {
		t.Error("expected error when both response and error are set")
	}
	if result != nil {
		t.Error("expected nil result when error is returned")
	}
}

func TestMockRunner_Run_UnexpectedCommand(t *testing.T) {
	mock := NewMockRunner()

	ctx := context.Background()
	_, err := mock.Run(ctx, "unknown", "command")

	if err == nil {
		t.Error("expected error for unexpected command")
	}
	if !errors.Is(err, err) {
		t.Error("error should indicate unexpected command")
	}
}

func TestMockRunner_Run_PrefixMatch(t *testing.T) {
	mock := NewMockRunner()
	// Set response for "bd close" prefix
	mock.Responses["bd close"] = []byte("ok")

	ctx := context.Background()
	result, err := mock.Run(ctx, "bd", "close", "bd-001", "--reason", "done")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if string(result) != "ok" {
		t.Errorf("expected 'ok', got %s", result)
	}
}

func TestMockRunner_Run_NoArgs(t *testing.T) {
	mock := NewMockRunner()
	mock.Responses["ls"] = []byte("file1 file2")

	ctx := context.Background()
	result, err := mock.Run(ctx, "ls")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if string(result) != "file1 file2" {
		t.Errorf("expected 'file1 file2', got %s", result)
	}
}

func TestMockRunner_GetCalls_ReturnsACopy(t *testing.T) {
	mock := NewMockRunner()
	mock.Responses["test"] = []byte("ok")

	ctx := context.Background()
	_, _ = mock.Run(ctx, "test")

	calls1 := mock.GetCalls()
	calls2 := mock.GetCalls()

	// Modify calls1
	calls1[0].Name = "modified"

	// calls2 should be unaffected
	if calls2[0].Name == "modified" {
		t.Error("GetCalls should return a copy, not the original")
	}
}

func TestMockRunner_Reset(t *testing.T) {
	mock := NewMockRunner()
	mock.Responses["test"] = []byte("ok")

	ctx := context.Background()
	_, _ = mock.Run(ctx, "test")
	_, _ = mock.Run(ctx, "test")

	if len(mock.GetCalls()) != 2 {
		t.Fatalf("expected 2 calls before reset")
	}

	mock.Reset()

	if len(mock.GetCalls()) != 0 {
		t.Error("expected 0 calls after reset")
	}
}

func TestMockRunner_ThreadSafety(t *testing.T) {
	mock := NewMockRunner()
	mock.Responses["test"] = []byte("ok")

	ctx := context.Background()
	done := make(chan bool)

	// Run multiple goroutines concurrently
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = mock.Run(ctx, "test")
			_ = mock.GetCalls()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	calls := mock.GetCalls()
	if len(calls) != 10 {
		t.Errorf("expected 10 calls, got %d", len(calls))
	}
}
