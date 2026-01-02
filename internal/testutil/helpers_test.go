package testutil

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestTempDir(t *testing.T) {
	dir, cleanup := TempDir(t)
	defer cleanup()

	// Verify directory exists
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("directory should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("should be a directory")
	}

	// Verify directory starts with atari-test-
	if !filepath.IsAbs(dir) {
		t.Error("should return absolute path")
	}
}

func TestTempDir_Cleanup(t *testing.T) {
	dir, cleanup := TempDir(t)

	// Create a file in the directory
	testFile := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	cleanup()

	// Verify directory and file are removed
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("directory should be removed after cleanup")
	}
}

func TestWriteFile(t *testing.T) {
	dir, cleanup := TempDir(t)
	defer cleanup()

	path := WriteFile(t, dir, "test.txt", "hello world")

	// Verify file exists
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file should exist: %v", err)
	}
	if string(content) != "hello world" {
		t.Errorf("content = %q, want %q", content, "hello world")
	}
}

func TestWriteFile_CreatesSubdirectories(t *testing.T) {
	dir, cleanup := TempDir(t)
	defer cleanup()

	path := WriteFile(t, dir, "sub/dir/test.txt", "content")

	// Verify file exists
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file should exist: %v", err)
	}
	if string(content) != "content" {
		t.Errorf("content = %q, want %q", content, "content")
	}
}

func TestReadFile(t *testing.T) {
	dir, cleanup := TempDir(t)
	defer cleanup()

	path := WriteFile(t, dir, "test.txt", "test content")
	content := ReadFile(t, path)

	if content != "test content" {
		t.Errorf("content = %q, want %q", content, "test content")
	}
}

func TestFileExists(t *testing.T) {
	dir, cleanup := TempDir(t)
	defer cleanup()

	path := WriteFile(t, dir, "exists.txt", "content")

	if !FileExists(t, path) {
		t.Error("FileExists should return true for existing file")
	}

	if FileExists(t, filepath.Join(dir, "nonexistent.txt")) {
		t.Error("FileExists should return false for nonexistent file")
	}
}

func TestAssertCalled(t *testing.T) {
	mock := NewMockRunner()
	mock.Responses["bd ready --json"] = []byte("[]")

	_, _ = mock.Run(context.Background(), "bd", "ready", "--json")

	// This should not fail
	AssertCalled(t, mock, "bd", "ready", "--json")
}

func TestAssertNotCalled(t *testing.T) {
	mock := NewMockRunner()
	mock.Responses["bd ready --json"] = []byte("[]")

	_, _ = mock.Run(context.Background(), "bd", "ready", "--json")

	// This should not fail - "git" was not called
	AssertNotCalled(t, mock, "git")
}

func TestAssertCallCount(t *testing.T) {
	mock := NewMockRunner()
	mock.Responses["test"] = []byte("ok")

	_, _ = mock.Run(context.Background(), "test")
	_, _ = mock.Run(context.Background(), "test")
	_, _ = mock.Run(context.Background(), "other")

	AssertCallCount(t, mock, "test", 2)
	AssertCallCount(t, mock, "other", 1)
	AssertCallCount(t, mock, "never", 0)
}

func TestSlicesEqual(t *testing.T) {
	tests := []struct {
		a, b     []string
		expected bool
	}{
		{nil, nil, true},
		{[]string{}, []string{}, true},
		{[]string{"a"}, []string{"a"}, true},
		{[]string{"a", "b"}, []string{"a", "b"}, true},
		{[]string{"a"}, []string{"b"}, false},
		{[]string{"a"}, []string{"a", "b"}, false},
		{[]string{"a", "b"}, []string{"a"}, false},
	}

	for _, tt := range tests {
		result := slicesEqual(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("slicesEqual(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
		}
	}
}

func TestSetupTestDir(t *testing.T) {
	dir, cleanup := SetupTestDir(t)
	defer cleanup()

	// Verify .atari directory exists
	atariDir := filepath.Join(dir, ".atari")
	info, err := os.Stat(atariDir)
	if err != nil {
		t.Fatalf(".atari directory should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error(".atari should be a directory")
	}
}

func TestSetupTestDirWithState(t *testing.T) {
	dir, cleanup := SetupTestDirWithState(t, SampleStateJSON)
	defer cleanup()

	// Verify state file exists and has correct content
	statePath := filepath.Join(dir, ".atari/state.json")
	content := ReadFile(t, statePath)

	if content != SampleStateJSON {
		t.Errorf("state content mismatch\ngot: %s\nwant: %s", content, SampleStateJSON)
	}
}

func TestSetupMockBDReady(t *testing.T) {
	mock := NewMockRunner()
	SetupMockBDReady(mock, SampleBeadReadyJSON)

	result, err := mock.Run(context.Background(), "bd", "ready", "--json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != SampleBeadReadyJSON {
		t.Error("response mismatch")
	}
}

func TestSetupMockBDAgentState(t *testing.T) {
	mock := NewMockRunner()
	SetupMockBDAgentState(mock, "test-agent")

	result, err := mock.Run(context.Background(), "bd", "agent", "state", "test-agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != BDAgentStateSuccess {
		t.Error("response mismatch")
	}
}

func TestSetupMockBDClose(t *testing.T) {
	mock := NewMockRunner()
	SetupMockBDClose(mock, "bd-001")

	result, err := mock.Run(context.Background(), "bd", "close", "bd-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != BDCloseSuccess {
		t.Error("response mismatch")
	}
}
