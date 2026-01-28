package main

import (
	"io"
	"log/slog"
	"path/filepath"

	"github.com/npratt/atari/internal/config"
	"gopkg.in/natefinch/lumberjack.v2"
)

// TUILoggerResult contains the results of setting up logging for TUI mode.
type TUILoggerResult struct {
	Logger   *slog.Logger
	LogFile  io.WriteCloser
	FilePath string
}

// Close closes the log file if it was opened.
func (r *TUILoggerResult) Close() error {
	if r.LogFile != nil {
		return r.LogFile.Close()
	}
	return nil
}

// SetupTUILogger creates a logger that writes to a rotating file instead of stderr.
// This prevents log output from corrupting the TUI display.
// Uses lumberjack for automatic log rotation based on the provided config.
// Returns the logger, the writer (caller must close), and the file path.
func SetupTUILogger(logDir string, level slog.Leveler, rotationCfg config.LogRotationConfig) (*TUILoggerResult, error) {
	debugLogPath := filepath.Join(logDir, "atari-debug.log")

	// Use lumberjack for automatic rotation
	debugLogWriter := &lumberjack.Logger{
		Filename:   debugLogPath,
		MaxSize:    rotationCfg.MaxSizeMB,
		MaxBackups: rotationCfg.MaxBackups,
		MaxAge:     rotationCfg.MaxAgeDays,
		Compress:   rotationCfg.Compress,
	}

	logger := slog.New(slog.NewJSONHandler(debugLogWriter, &slog.HandlerOptions{Level: level}))

	return &TUILoggerResult{
		Logger:   logger,
		LogFile:  debugLogWriter,
		FilePath: debugLogPath,
	}, nil
}

// SetupTUILoggerWithWriter creates a logger that writes to the given writer.
// This is useful for testing where we want to capture the output.
func SetupTUILoggerWithWriter(w io.Writer, level slog.Leveler) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))
}
