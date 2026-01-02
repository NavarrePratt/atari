package events

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Sink consumes events from the router.
type Sink interface {
	Start(ctx context.Context, events <-chan Event) error
	Stop() error
}

// LogSink writes events to a JSON lines file for debugging and analysis.
type LogSink struct {
	path    string
	file    *os.File
	encoder *json.Encoder
	mu      sync.Mutex
	done    chan struct{}
}

// NewLogSink creates a new LogSink that writes to the specified path.
func NewLogSink(path string) *LogSink {
	return &LogSink{
		path: path,
		done: make(chan struct{}),
	}
}

// Start opens the log file and begins processing events.
// It runs until the context is canceled or the events channel is closed.
func (s *LogSink) Start(ctx context.Context, events <-chan Event) error {
	if err := s.openFile(); err != nil {
		return err
	}

	go s.run(ctx, events)
	return nil
}

// largeLogThreshold is the size above which we warn about large log files.
const largeLogThreshold = 100 * 1024 * 1024 // 100MB

func (s *LogSink) openFile() error {
	// Ensure directory exists
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}

	// Rotate existing log file on startup if it exists and has content
	if err := s.rotateExistingLog(); err != nil {
		return err
	}

	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	s.mu.Lock()
	s.file = file
	s.encoder = json.NewEncoder(file)
	s.mu.Unlock()

	return nil
}

// rotateExistingLog renames an existing log file with a timestamp suffix.
// This preserves tail -f compatibility by creating a fresh file.
func (s *LogSink) rotateExistingLog() error {
	info, err := os.Stat(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No existing log to rotate
		}
		return fmt.Errorf("stat log file: %w", err)
	}

	if info.Size() == 0 {
		return nil // Empty file, no need to rotate
	}

	// Warn if log is large
	if info.Size() > largeLogThreshold {
		fmt.Fprintf(os.Stderr, "log sink: warning: large log file (%d MB), consider cleaning up old .bak files in %s\n",
			info.Size()/(1024*1024), filepath.Dir(s.path))
	}

	// Generate timestamped backup filename
	timestamp := time.Now().Format("2006-01-02T15-04-05")
	bakPath := fmt.Sprintf("%s.%s.bak", s.path, timestamp)

	if err := os.Rename(s.path, bakPath); err != nil {
		return fmt.Errorf("rotate log file: %w", err)
	}

	return nil
}

func (s *LogSink) run(ctx context.Context, events <-chan Event) {
	defer close(s.done)

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			s.write(event)
		}
	}
}

func (s *LogSink) write(event Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.encoder == nil {
		return
	}

	if err := s.encoder.Encode(event); err != nil {
		// Log to stderr but don't crash
		fmt.Fprintf(os.Stderr, "log sink: failed to write event: %v\n", err)
	}
}

// Stop closes the log file.
func (s *LogSink) Stop() error {
	// Wait for the run goroutine to finish
	<-s.done

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.file != nil {
		err := s.file.Close()
		s.file = nil
		s.encoder = nil
		return err
	}
	return nil
}

// Path returns the log file path.
func (s *LogSink) Path() string {
	return s.path
}
