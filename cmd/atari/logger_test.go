package main

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupTUILogger_WritesToFile(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Setup TUI logger
	result, err := SetupTUILogger(tmpDir, slog.LevelInfo)
	if err != nil {
		t.Fatalf("SetupTUILogger failed: %v", err)
	}
	defer func() { _ = result.Close() }()

	// Verify file path is correct
	expectedPath := filepath.Join(tmpDir, "atari-debug.log")
	if result.FilePath != expectedPath {
		t.Errorf("FilePath = %q, want %q", result.FilePath, expectedPath)
	}

	// Write a log message
	result.Logger.Info("test message", "key", "value")

	// Sync the file to ensure writes are flushed
	_ = result.LogFile.Sync()

	// Read back the file and verify content
	content, err := os.ReadFile(result.FilePath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if !strings.Contains(string(content), "test message") {
		t.Errorf("log file should contain 'test message', got: %s", content)
	}
	if !strings.Contains(string(content), `"key":"value"`) {
		t.Errorf("log file should contain key=value, got: %s", content)
	}
}

func TestSetupTUILogger_DoesNotWriteToStderr(t *testing.T) {
	// This test verifies that the TUI logger writes to a file,
	// not to stderr. This is critical because stderr output would
	// corrupt the TUI display.

	tmpDir := t.TempDir()

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Setup TUI logger
	result, err := SetupTUILogger(tmpDir, slog.LevelInfo)
	if err != nil {
		os.Stderr = oldStderr
		t.Fatalf("SetupTUILogger failed: %v", err)
	}
	defer func() { _ = result.Close() }()

	// Write a log message
	result.Logger.Info("this should not appear on stderr")

	// Restore stderr and close pipe
	_ = w.Close()
	os.Stderr = oldStderr

	// Read captured stderr
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	// Verify nothing was written to stderr
	if buf.Len() > 0 {
		t.Errorf("TUI logger wrote to stderr: %s", buf.String())
	}
}

func TestSetupTUILoggerWithWriter_WritesToWriter(t *testing.T) {
	// Test that we can create a logger that writes to any writer
	var buf bytes.Buffer

	logger := SetupTUILoggerWithWriter(&buf, slog.LevelInfo)
	logger.Info("test message", "foo", "bar")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Errorf("output should contain 'test message', got: %s", output)
	}
	if !strings.Contains(output, `"foo":"bar"`) {
		t.Errorf("output should contain foo=bar, got: %s", output)
	}
}

func TestSetupTUILogger_AppendsToExistingFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Write initial content
	logPath := filepath.Join(tmpDir, "atari-debug.log")
	if err := os.WriteFile(logPath, []byte("existing content\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Setup logger and write
	result, err := SetupTUILogger(tmpDir, slog.LevelInfo)
	if err != nil {
		t.Fatalf("SetupTUILogger failed: %v", err)
	}
	result.Logger.Info("new message")
	_ = result.LogFile.Sync()
	_ = result.Close()

	// Verify both contents exist
	content, _ := os.ReadFile(logPath)
	if !strings.Contains(string(content), "existing content") {
		t.Error("should preserve existing content")
	}
	if !strings.Contains(string(content), "new message") {
		t.Error("should append new message")
	}
}

func TestSetupTUILogger_FailsOnInvalidDirectory(t *testing.T) {
	// Try to create logger in non-existent directory
	result, err := SetupTUILogger("/nonexistent/path/that/does/not/exist", slog.LevelInfo)
	if err == nil {
		_ = result.Close()
		t.Error("expected error for invalid directory")
	}
}

func TestSetupTUILogger_RespectsLogLevel(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup logger with WARN level
	result, err := SetupTUILogger(tmpDir, slog.LevelWarn)
	if err != nil {
		t.Fatalf("SetupTUILogger failed: %v", err)
	}
	defer func() { _ = result.Close() }()

	// Write INFO message (should be filtered)
	result.Logger.Info("info message")
	// Write WARN message (should appear)
	result.Logger.Warn("warn message")

	_ = result.LogFile.Sync()

	content, _ := os.ReadFile(result.FilePath)
	contentStr := string(content)

	if strings.Contains(contentStr, "info message") {
		t.Error("INFO message should be filtered out at WARN level")
	}
	if !strings.Contains(contentStr, "warn message") {
		t.Error("WARN message should appear")
	}
}
