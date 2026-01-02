package testutil

import (
	"context"
	"errors"
	"io"
	"testing"
)

func TestMockProcessRunner_BasicFlow(t *testing.T) {
	mock := NewMockProcessRunner()
	mock.SetOutput("hello\nworld\n")

	stdout, stderr, err := mock.Start(context.Background(), "test", "arg1", "arg2")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Read stdout
	out, err := io.ReadAll(stdout)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	if string(out) != "hello\nworld\n" {
		t.Errorf("stdout = %q, want %q", string(out), "hello\nworld\n")
	}

	// Stderr should be empty
	errOut, err := io.ReadAll(stderr)
	if err != nil {
		t.Fatalf("ReadAll stderr failed: %v", err)
	}
	if len(errOut) != 0 {
		t.Errorf("stderr = %q, want empty", string(errOut))
	}

	// Wait should succeed
	if err := mock.Wait(); err != nil {
		t.Errorf("Wait failed: %v", err)
	}
}

func TestMockProcessRunner_Stderr(t *testing.T) {
	mock := NewMockProcessRunner()
	mock.SetOutput("out")
	mock.SetStderr("error message\n")

	stdout, stderr, err := mock.Start(context.Background(), "test")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Drain stdout
	_, _ = io.ReadAll(stdout)

	errOut, err := io.ReadAll(stderr)
	if err != nil {
		t.Fatalf("ReadAll stderr failed: %v", err)
	}
	if string(errOut) != "error message\n" {
		t.Errorf("stderr = %q, want %q", string(errOut), "error message\n")
	}

	_ = mock.Wait()
}

func TestMockProcessRunner_StartError(t *testing.T) {
	mock := NewMockProcessRunner()
	testErr := errors.New("start failed")
	mock.SetStartError(testErr)

	_, _, err := mock.Start(context.Background(), "test")
	if !errors.Is(err, testErr) {
		t.Errorf("Start error = %v, want %v", err, testErr)
	}

	// Should be able to try again after error (reset not required for start error)
	mock.SetStartError(nil)
	mock.SetOutput("success")
	stdout, _, err := mock.Start(context.Background(), "test")
	if err != nil {
		t.Fatalf("Second Start failed: %v", err)
	}
	_, _ = io.ReadAll(stdout)
}

func TestMockProcessRunner_WaitError(t *testing.T) {
	mock := NewMockProcessRunner()
	testErr := errors.New("process exited with code 1")
	mock.SetWaitError(testErr)
	mock.SetOutput("output")

	stdout, _, err := mock.Start(context.Background(), "test")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	_, _ = io.ReadAll(stdout)

	err = mock.Wait()
	if !errors.Is(err, testErr) {
		t.Errorf("Wait error = %v, want %v", err, testErr)
	}
}

func TestMockProcessRunner_Kill(t *testing.T) {
	mock := NewMockProcessRunner()
	mock.SetOutput("output")

	stdout, _, err := mock.Start(context.Background(), "test")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if err := mock.Kill(); err != nil {
		t.Errorf("Kill failed: %v", err)
	}

	if !mock.Killed() {
		t.Error("expected Killed() to be true")
	}

	// Pipes should be closed
	_, err = io.ReadAll(stdout)
	if err != nil && err != io.EOF {
		t.Errorf("expected EOF or nil after kill, got %v", err)
	}

	// Wait should return killed error
	err = mock.Wait()
	if !errors.Is(err, ErrProcessKilled) {
		t.Errorf("Wait after Kill = %v, want %v", err, ErrProcessKilled)
	}
}

func TestMockProcessRunner_KillNotStarted(t *testing.T) {
	mock := NewMockProcessRunner()

	// Kill before start should be safe
	if err := mock.Kill(); err != nil {
		t.Errorf("Kill before Start should not error: %v", err)
	}
}

func TestMockProcessRunner_WaitNotStarted(t *testing.T) {
	mock := NewMockProcessRunner()

	err := mock.Wait()
	if !errors.Is(err, ErrProcessNotStarted) {
		t.Errorf("Wait without Start = %v, want %v", err, ErrProcessNotStarted)
	}
}

func TestMockProcessRunner_DoubleStart(t *testing.T) {
	mock := NewMockProcessRunner()
	mock.SetOutput("output")

	_, _, err := mock.Start(context.Background(), "test")
	if err != nil {
		t.Fatalf("First Start failed: %v", err)
	}

	_, _, err = mock.Start(context.Background(), "test")
	if !errors.Is(err, ErrProcessAlreadyStarted) {
		t.Errorf("Second Start = %v, want %v", err, ErrProcessAlreadyStarted)
	}
}

func TestMockProcessRunner_Reset(t *testing.T) {
	mock := NewMockProcessRunner()
	mock.SetOutput("output")

	stdout, _, err := mock.Start(context.Background(), "test")
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	_, _ = io.ReadAll(stdout)
	_ = mock.Wait()

	// Reset should allow another start
	mock.Reset()

	stdout, _, err = mock.Start(context.Background(), "test2")
	if err != nil {
		t.Fatalf("Start after Reset failed: %v", err)
	}
	_, _ = io.ReadAll(stdout)
}

func TestMockProcessRunner_OnStart(t *testing.T) {
	mock := NewMockProcessRunner()

	callCount := 0
	mock.OnStart(func(attempt int, name string, args []string) (string, string, error) {
		callCount++
		if attempt == 1 {
			return "", "", errors.New("first attempt fails")
		}
		return "success on attempt 2", "", nil
	})

	// First attempt fails
	_, _, err := mock.Start(context.Background(), "test")
	if err == nil {
		t.Error("expected first Start to fail")
	}

	// Second attempt succeeds (reset not needed for start failure)
	stdout, _, err := mock.Start(context.Background(), "test")
	if err != nil {
		t.Fatalf("Second Start failed: %v", err)
	}

	out, _ := io.ReadAll(stdout)
	if string(out) != "success on attempt 2" {
		t.Errorf("stdout = %q, want %q", string(out), "success on attempt 2")
	}

	if callCount != 2 {
		t.Errorf("callback called %d times, want 2", callCount)
	}
}

func TestMockProcessRunner_StartCount(t *testing.T) {
	mock := NewMockProcessRunner()
	mock.SetOutput("output")

	if mock.StartCount() != 0 {
		t.Errorf("initial StartCount = %d, want 0", mock.StartCount())
	}

	stdout, _, _ := mock.Start(context.Background(), "test1")
	_, _ = io.ReadAll(stdout)
	_ = mock.Wait()
	mock.Reset()

	if mock.StartCount() != 1 {
		t.Errorf("StartCount after first = %d, want 1", mock.StartCount())
	}

	stdout, _, _ = mock.Start(context.Background(), "test2")
	_, _ = io.ReadAll(stdout)
	_ = mock.Wait()
	mock.Reset()

	if mock.StartCount() != 2 {
		t.Errorf("StartCount after second = %d, want 2", mock.StartCount())
	}
}

func TestMockProcessRunner_Processes(t *testing.T) {
	mock := NewMockProcessRunner()
	mock.SetOutput("output")

	stdout, _, _ := mock.Start(context.Background(), "cmd1", "arg1")
	_, _ = io.ReadAll(stdout)
	_ = mock.Wait()
	mock.Reset()

	stdout, _, _ = mock.Start(context.Background(), "cmd2", "arg2", "arg3")
	_, _ = io.ReadAll(stdout)
	_ = mock.Wait()

	procs := mock.Processes()
	if len(procs) != 2 {
		t.Fatalf("Processes() returned %d, want 2", len(procs))
	}

	if procs[0].Name != "cmd1" || len(procs[0].Args) != 1 || procs[0].Args[0] != "arg1" {
		t.Errorf("first process = %+v, want {cmd1 [arg1]}", procs[0])
	}

	if procs[1].Name != "cmd2" || len(procs[1].Args) != 2 {
		t.Errorf("second process = %+v, want {cmd2 [arg2 arg3]}", procs[1])
	}
}

func TestMockProcessRunner_StateTracking(t *testing.T) {
	mock := NewMockProcessRunner()
	mock.SetOutput("output")

	// Initial state
	if mock.Started() {
		t.Error("should not be started initially")
	}
	if mock.Killed() {
		t.Error("should not be killed initially")
	}
	if mock.WaitCalled() {
		t.Error("wait should not be called initially")
	}

	// After start
	stdout, _, _ := mock.Start(context.Background(), "test")
	if !mock.Started() {
		t.Error("should be started after Start")
	}

	// After wait
	_, _ = io.ReadAll(stdout)
	_ = mock.Wait()
	if !mock.WaitCalled() {
		t.Error("WaitCalled should be true after Wait")
	}

	// After reset
	mock.Reset()
	if mock.Started() {
		t.Error("should not be started after Reset")
	}
	if mock.WaitCalled() {
		t.Error("WaitCalled should be false after Reset")
	}
}
