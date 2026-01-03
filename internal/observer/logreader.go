package observer

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"syscall"
	"time"

	"github.com/npratt/atari/internal/events"
)

const (
	// maxLineSize is the maximum size for a single JSON line (1MB).
	maxLineSize = 1 << 20

	// truncationMarker is appended to truncated lines.
	truncationMarker = "...[TRUNCATED]"
)

var (
	// ErrFileNotFound is returned when the log file does not exist.
	ErrFileNotFound = errors.New("log file not found")

	// ErrEmptyFile is returned when the log file is empty.
	ErrEmptyFile = errors.New("log file is empty")
)

// LogReader reads events from the atari log file with rotation detection.
type LogReader struct {
	path      string
	lastInode uint64
	lastSize  int64
}

// NewLogReader creates a new LogReader for the given log file path.
func NewLogReader(path string) *LogReader {
	return &LogReader{
		path: path,
	}
}

// ReadRecent returns the last n events from the log file.
// If the file has fewer than n events, all events are returned.
func (r *LogReader) ReadRecent(n int) ([]events.Event, error) {
	if n <= 0 {
		return nil, nil
	}

	allEvents, err := r.readAllEvents()
	if err != nil {
		return nil, err
	}

	if len(allEvents) <= n {
		return allEvents, nil
	}

	return allEvents[len(allEvents)-n:], nil
}

// ReadByBeadID returns all events associated with the given bead ID.
func (r *LogReader) ReadByBeadID(beadID string) ([]events.Event, error) {
	if beadID == "" {
		return nil, nil
	}

	allEvents, err := r.readAllEvents()
	if err != nil {
		return nil, err
	}

	var filtered []events.Event
	for _, ev := range allEvents {
		if getBeadID(ev) == beadID {
			filtered = append(filtered, ev)
		}
	}

	return filtered, nil
}

// ReadAfterTimestamp returns all events after the given timestamp.
func (r *LogReader) ReadAfterTimestamp(t time.Time) ([]events.Event, error) {
	allEvents, err := r.readAllEvents()
	if err != nil {
		return nil, err
	}

	var filtered []events.Event
	for _, ev := range allEvents {
		if ev.Timestamp().After(t) {
			filtered = append(filtered, ev)
		}
	}

	return filtered, nil
}

// readAllEvents reads and parses all events from the log file.
func (r *LogReader) readAllEvents() ([]events.Event, error) {
	file, err := os.Open(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrFileNotFound
		}
		return nil, err
	}
	defer func() { _ = file.Close() }()

	// Check for rotation
	r.checkRotation(file)

	// Get file info
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if info.Size() == 0 {
		return nil, ErrEmptyFile
	}

	// Update tracking state
	r.lastSize = info.Size()
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		r.lastInode = stat.Ino
	}

	// Read all lines
	reader := bufio.NewReaderSize(file, maxLineSize)
	var result []events.Event

	for {
		line, err := r.readLine(reader)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		if len(line) == 0 {
			continue
		}

		ev, err := parseEvent(line)
		if err != nil {
			slog.Warn("failed to parse event line",
				"error", err,
				"line_preview", truncateForLog(string(line), 100))
			continue
		}

		if ev != nil {
			result = append(result, ev)
		}
	}

	return result, nil
}

// readLine reads a single line, handling lines that exceed maxLineSize.
func (r *LogReader) readLine(reader *bufio.Reader) ([]byte, error) {
	var line []byte
	var isPrefix bool

	for {
		chunk, prefix, err := reader.ReadLine()
		if err != nil {
			if err == io.EOF && len(line) > 0 {
				return line, nil
			}
			return line, err
		}

		line = append(line, chunk...)
		isPrefix = prefix

		// Check if line exceeds max size
		if len(line) > maxLineSize {
			// Discard the rest of the line
			for isPrefix {
				_, prefix, err = reader.ReadLine()
				if err != nil {
					break
				}
				isPrefix = prefix
			}

			// Truncate and mark
			truncated := make([]byte, maxLineSize)
			copy(truncated, line[:maxLineSize-len(truncationMarker)])
			copy(truncated[maxLineSize-len(truncationMarker):], truncationMarker)
			return truncated, nil
		}

		if !isPrefix {
			break
		}
	}

	return line, nil
}

// checkRotation detects if the log file has been rotated.
func (r *LogReader) checkRotation(file *os.File) {
	info, err := file.Stat()
	if err != nil {
		return
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return
	}

	// Detect rotation: inode changed or file size decreased
	if r.lastInode != 0 && (stat.Ino != r.lastInode || info.Size() < r.lastSize) {
		slog.Debug("log rotation detected",
			"old_inode", r.lastInode,
			"new_inode", stat.Ino,
			"old_size", r.lastSize,
			"new_size", info.Size())

		// Reset tracking state
		r.lastInode = stat.Ino
		r.lastSize = 0
	}
}

// eventEnvelope is used for initial JSON parsing to determine event type.
type eventEnvelope struct {
	Type      events.EventType `json:"type"`
	Timestamp time.Time        `json:"timestamp"`
	Source    string           `json:"source"`
}

// parseEvent parses a JSON line into a typed Event.
func parseEvent(line []byte) (events.Event, error) {
	// First pass: determine event type
	var envelope eventEnvelope
	if err := json.Unmarshal(line, &envelope); err != nil {
		return nil, err
	}

	// Second pass: unmarshal into the correct type
	var ev events.Event
	var err error

	switch envelope.Type {
	case events.EventSessionStart:
		var e events.SessionStartEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case events.EventSessionEnd:
		var e events.SessionEndEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case events.EventSessionTimeout:
		var e events.SessionTimeoutEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case events.EventClaudeText:
		var e events.ClaudeTextEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case events.EventClaudeToolUse:
		var e events.ClaudeToolUseEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case events.EventClaudeToolResult:
		var e events.ClaudeToolResultEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case events.EventDrainStart:
		var e events.DrainStartEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case events.EventDrainStop:
		var e events.DrainStopEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case events.EventDrainStateChanged:
		var e events.DrainStateChangedEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case events.EventIterationStart:
		var e events.IterationStartEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case events.EventIterationEnd:
		var e events.IterationEndEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case events.EventBeadAbandoned:
		var e events.BeadAbandonedEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case events.EventBeadCreated:
		var e events.BeadCreatedEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case events.EventBeadStatus:
		var e events.BeadStatusEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case events.EventBeadUpdated:
		var e events.BeadUpdatedEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case events.EventBeadComment:
		var e events.BeadCommentEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case events.EventBeadClosed:
		var e events.BeadClosedEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case events.EventError:
		var e events.ErrorEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case events.EventParseError:
		var e events.ParseErrorEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	default:
		// Unknown event type - skip it
		slog.Debug("unknown event type", "type", envelope.Type)
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return ev, nil
}

// getBeadID extracts the bead ID from an event, if present.
func getBeadID(ev events.Event) string {
	switch e := ev.(type) {
	case *events.SessionStartEvent:
		return e.BeadID
	case *events.IterationStartEvent:
		return e.BeadID
	case *events.IterationEndEvent:
		return e.BeadID
	case *events.BeadAbandonedEvent:
		return e.BeadID
	case *events.BeadCreatedEvent:
		return e.BeadID
	case *events.BeadStatusEvent:
		return e.BeadID
	case *events.BeadUpdatedEvent:
		return e.BeadID
	case *events.BeadCommentEvent:
		return e.BeadID
	case *events.BeadClosedEvent:
		return e.BeadID
	case *events.ErrorEvent:
		return e.BeadID
	default:
		return ""
	}
}

// truncateForLog truncates a string for logging purposes.
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
