// Package testutil provides test infrastructure for unit and integration testing.
package testutil

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/npratt/atari/internal/events"
)

// ObserverLogFixture represents a set of log events for testing the observer.
type ObserverLogFixture struct {
	Events []events.Event
}

// SampleObserverLogEvents returns a sample set of events for observer testing.
// It includes a mix of session events, tool use, and bead events.
func SampleObserverLogEvents() []events.Event {
	baseTime := time.Now().Add(-10 * time.Minute)

	return []events.Event{
		// Drain start
		&events.DrainStartEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventDrainStart,
				Time:      baseTime,
				Src:       "atari",
			},
			WorkDir: "/workspace",
		},

		// First bead - completed successfully
		&events.IterationStartEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventIterationStart,
				Time:      baseTime.Add(1 * time.Second),
				Src:       "atari",
			},
			BeadID: "bd-001",
			Title:  "Fix auth bug",
		},
		&events.SessionStartEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventSessionStart,
				Time:      baseTime.Add(2 * time.Second),
				Src:       "atari",
			},
			BeadID: "bd-001",
			Title:  "Fix auth bug",
		},
		&events.ClaudeTextEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventClaudeText,
				Time:      baseTime.Add(3 * time.Second),
				Src:       "claude",
			},
			Text: "I'll investigate the auth bug by examining the login handler.",
		},
		&events.ClaudeToolUseEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventClaudeToolUse,
				Time:      baseTime.Add(4 * time.Second),
				Src:       "claude",
			},
			ToolID:   "toolu_01ABC123",
			ToolName: "Read",
			Input:    map[string]any{"file_path": "/workspace/auth/handler.go"},
		},
		&events.ClaudeToolResultEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventClaudeToolResult,
				Time:      baseTime.Add(5 * time.Second),
				Src:       "claude",
			},
			ToolID:  "toolu_01ABC123",
			Content: "package auth\n\nfunc Login(w http.ResponseWriter, r *http.Request) {\n    ...",
			IsError: false,
		},
		&events.SessionEndEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventSessionEnd,
				Time:      baseTime.Add(60 * time.Second),
				Src:       "atari",
			},
			SessionID:    "session-001",
			NumTurns:     5,
			TotalCostUSD: 0.25,
		},
		&events.IterationEndEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventIterationEnd,
				Time:      baseTime.Add(61 * time.Second),
				Src:       "atari",
			},
			BeadID:       "bd-001",
			Success:      true,
			NumTurns:     5,
			TotalCostUSD: 0.25,
			DurationMs:   60000,
		},

		// Second bead - currently active
		&events.IterationStartEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventIterationStart,
				Time:      baseTime.Add(2 * time.Minute),
				Src:       "atari",
			},
			BeadID: "bd-002",
			Title:  "Add input validation",
		},
		&events.SessionStartEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventSessionStart,
				Time:      baseTime.Add(2*time.Minute + 1*time.Second),
				Src:       "atari",
			},
			BeadID: "bd-002",
			Title:  "Add input validation",
		},
		&events.ClaudeTextEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventClaudeText,
				Time:      baseTime.Add(2*time.Minute + 2*time.Second),
				Src:       "claude",
			},
			Text: "I'll add validation to the user input form.",
		},
		&events.ClaudeToolUseEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventClaudeToolUse,
				Time:      baseTime.Add(2*time.Minute + 3*time.Second),
				Src:       "claude",
			},
			ToolID:   "toolu_01DEF456",
			ToolName: "Bash",
			Input: map[string]any{
				"command":     "go test ./...",
				"description": "Run test suite",
			},
		},
		&events.ClaudeToolResultEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventClaudeToolResult,
				Time:      baseTime.Add(2*time.Minute + 8*time.Second),
				Src:       "claude",
			},
			ToolID:  "toolu_01DEF456",
			Content: "PASS\nok  \tgithub.com/npratt/atari\t1.234s",
			IsError: false,
		},
		&events.ClaudeTextEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventClaudeText,
				Time:      baseTime.Add(2*time.Minute + 9*time.Second),
				Src:       "claude",
			},
			Text: "Tests pass. Now I'll add the validation logic to the form handler.",
		},
	}
}

// WriteLogFixture writes the given events to a log file in JSON lines format.
func WriteLogFixture(path string, evts []events.Event) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	for _, evt := range evts {
		data, err := json.Marshal(evt)
		if err != nil {
			return fmt.Errorf("failed to marshal event: %w", err)
		}
		if _, err := file.Write(append(data, '\n')); err != nil {
			return fmt.Errorf("failed to write event: %w", err)
		}
	}

	return nil
}

// CreateSampleLogFile creates a log file with sample events for testing.
func CreateSampleLogFile(path string) error {
	return WriteLogFixture(path, SampleObserverLogEvents())
}

// EmptyLogEvents returns an empty slice of events.
func EmptyLogEvents() []events.Event {
	return []events.Event{}
}

// SingleBeadLogEvents returns events for a single in-progress bead.
func SingleBeadLogEvents(beadID, title string) []events.Event {
	now := time.Now()
	return []events.Event{
		&events.DrainStartEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventDrainStart,
				Time:      now.Add(-5 * time.Minute),
				Src:       "atari",
			},
			WorkDir: "/workspace",
		},
		&events.IterationStartEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventIterationStart,
				Time:      now.Add(-1 * time.Minute),
				Src:       "atari",
			},
			BeadID: beadID,
			Title:  title,
		},
		&events.SessionStartEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventSessionStart,
				Time:      now.Add(-55 * time.Second),
				Src:       "atari",
			},
			BeadID: beadID,
			Title:  title,
		},
		&events.ClaudeTextEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventClaudeText,
				Time:      now.Add(-50 * time.Second),
				Src:       "claude",
			},
			Text: "Working on this task...",
		},
	}
}

// CompletedBeadsLogEvents returns events for multiple completed beads.
func CompletedBeadsLogEvents(count int) []events.Event {
	baseTime := time.Now().Add(-time.Duration(count*2) * time.Minute)
	var evts []events.Event

	evts = append(evts, &events.DrainStartEvent{
		BaseEvent: events.BaseEvent{
			EventType: events.EventDrainStart,
			Time:      baseTime,
			Src:       "atari",
		},
		WorkDir: "/workspace",
	})

	for i := 0; i < count; i++ {
		offset := time.Duration(i*2) * time.Minute
		beadID := fmt.Sprintf("bd-%03d", i+1)
		title := fmt.Sprintf("Test bead %d", i+1)

		evts = append(evts,
			&events.IterationStartEvent{
				BaseEvent: events.BaseEvent{
					EventType: events.EventIterationStart,
					Time:      baseTime.Add(offset),
					Src:       "atari",
				},
				BeadID: beadID,
				Title:  title,
			},
			&events.SessionStartEvent{
				BaseEvent: events.BaseEvent{
					EventType: events.EventSessionStart,
					Time:      baseTime.Add(offset + 1*time.Second),
					Src:       "atari",
				},
				BeadID: beadID,
				Title:  title,
			},
			&events.SessionEndEvent{
				BaseEvent: events.BaseEvent{
					EventType: events.EventSessionEnd,
					Time:      baseTime.Add(offset + 30*time.Second),
					Src:       "atari",
				},
				SessionID:    fmt.Sprintf("session-%03d", i+1),
				NumTurns:     5 + i,
				TotalCostUSD: 0.10 + float64(i)*0.05,
			},
			&events.IterationEndEvent{
				BaseEvent: events.BaseEvent{
					EventType: events.EventIterationEnd,
					Time:      baseTime.Add(offset + 31*time.Second),
					Src:       "atari",
				},
				BeadID:       beadID,
				Success:      true,
				NumTurns:     5 + i,
				TotalCostUSD: 0.10 + float64(i)*0.05,
				DurationMs:   30000,
			},
		)
	}

	return evts
}

// FailedBeadLogEvents returns events for a failed bead.
func FailedBeadLogEvents(beadID, title string) []events.Event {
	now := time.Now()
	return []events.Event{
		&events.DrainStartEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventDrainStart,
				Time:      now.Add(-5 * time.Minute),
				Src:       "atari",
			},
			WorkDir: "/workspace",
		},
		&events.IterationStartEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventIterationStart,
				Time:      now.Add(-2 * time.Minute),
				Src:       "atari",
			},
			BeadID: beadID,
			Title:  title,
		},
		&events.SessionStartEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventSessionStart,
				Time:      now.Add(-115 * time.Second),
				Src:       "atari",
			},
			BeadID: beadID,
			Title:  title,
		},
		&events.ErrorEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventError,
				Time:      now.Add(-60 * time.Second),
				Src:       "atari",
			},
			BeadID:  beadID,
			Message: "session timeout exceeded",
		},
		&events.IterationEndEvent{
			BaseEvent: events.BaseEvent{
				EventType: events.EventIterationEnd,
				Time:      now.Add(-59 * time.Second),
				Src:       "atari",
			},
			BeadID:       beadID,
			Success:      false,
			NumTurns:     3,
			TotalCostUSD: 0.15,
			DurationMs:   61000,
		},
	}
}
