package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// TUILoggerResult contains the results of setting up logging for TUI mode.
type TUILoggerResult struct {
	Logger   *slog.Logger
	LogFile  *os.File
	FilePath string
}

// Close closes the log file if it was opened.
func (r *TUILoggerResult) Close() error {
	if r.LogFile != nil {
		return r.LogFile.Close()
	}
	return nil
}

// SetupTUILogger creates a logger that writes to a file instead of stderr.
// This prevents log output from corrupting the TUI display.
// Returns the logger, the open file (caller must close), and the file path.
func SetupTUILogger(logDir string, level slog.Leveler) (*TUILoggerResult, error) {
	debugLogPath := filepath.Join(logDir, "atari-debug.log")
	debugLogFile, err := os.OpenFile(debugLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("open debug log file: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(debugLogFile, &slog.HandlerOptions{Level: level}))

	return &TUILoggerResult{
		Logger:   logger,
		LogFile:  debugLogFile,
		FilePath: debugLogPath,
	}, nil
}

// SetupTUILoggerWithWriter creates a logger that writes to the given writer.
// This is useful for testing where we want to capture the output.
func SetupTUILoggerWithWriter(w io.Writer, level slog.Leveler) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
}
