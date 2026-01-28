package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// StateBufferSize is the recommended buffer size for state sink subscriptions.
const StateBufferSize = 1000

// CurrentStateVersion is the current state file format version.
// Increment this when making incompatible changes to the State struct.
const CurrentStateVersion = 1

// State represents the persistent drain state.
type State struct {
	Version            int                     `json:"version"`
	Status             string                  `json:"status"`
	Iteration          int                     `json:"iteration"`
	CurrentBead        string                  `json:"current_bead,omitempty"`
	History            map[string]*BeadHistory `json:"history"`
	TotalCost          float64                 `json:"total_cost"`
	TotalTurns         int                     `json:"total_turns"`
	UpdatedAt          time.Time               `json:"updated_at"`
	ActiveTopLevel     string                  `json:"active_top_level,omitempty"`
	ActiveTopLevelTitle string                 `json:"active_top_level_title,omitempty"`
}

// DefaultMinSaveDelay is the minimum time between saves.
const DefaultMinSaveDelay = 5 * time.Second

// StateSink persists state to a JSON file for crash recovery.
type StateSink struct {
	path            string
	state           *State
	dirty           bool
	mu              sync.Mutex
	done            chan struct{}
	lastSave        time.Time
	minDelay        time.Duration
	countedSessions map[string]bool // tracks sessions whose cost has been counted
}

// NewStateSink creates a new StateSink that writes to the specified path.
func NewStateSink(path string) *StateSink {
	return &StateSink{
		path: path,
		state: &State{
			Version: CurrentStateVersion,
			History: make(map[string]*BeadHistory),
		},
		done:            make(chan struct{}),
		minDelay:        DefaultMinSaveDelay,
		countedSessions: make(map[string]bool),
	}
}

// Start ensures the directory exists, loads existing state, and begins processing events.
func (s *StateSink) Start(ctx context.Context, events <-chan Event) error {
	// Ensure directory exists
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}

	// Load existing state if present
	if err := s.Load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load state: %w", err)
	}

	go s.run(ctx, events)
	return nil
}

func (s *StateSink) run(ctx context.Context, events <-chan Event) {
	defer close(s.done)

	for {
		select {
		case <-ctx.Done():
			s.flushIfDirty()
			return
		case event, ok := <-events:
			if !ok {
				s.flushIfDirty()
				return
			}
			s.handleEvent(event)
		}
	}
}

func (s *StateSink) handleEvent(event Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch e := event.(type) {
	case *DrainStartEvent:
		s.state.Status = "running"
		s.dirty = true

	case *DrainStateChangedEvent:
		s.state.Status = e.To
		s.dirty = true

	case *DrainStopEvent:
		s.state.Status = "stopped"
		s.dirty = true
		// Always save immediately on stop
		s.saveUnlocked()
		return

	case *IterationStartEvent:
		s.state.Iteration++
		s.state.CurrentBead = e.BeadID
		// Initialize or update history
		if s.state.History[e.BeadID] == nil {
			s.state.History[e.BeadID] = &BeadHistory{
				ID:     e.BeadID,
				Status: HistoryWorking,
			}
		}
		h := s.state.History[e.BeadID]
		h.Status = HistoryWorking
		h.Attempts = e.Attempt
		h.LastAttempt = event.Timestamp()
		s.dirty = true

	case *IterationEndEvent:
		s.state.CurrentBead = ""
		// Only add cost if this session hasn't been counted (by SessionEndEvent)
		if !s.countedSessions[e.BeadID] {
			s.state.TotalCost += e.TotalCostUSD
			s.state.TotalTurns += e.NumTurns
			s.countedSessions[e.BeadID] = true
		}
		// Update history
		if h := s.state.History[e.BeadID]; h != nil {
			if e.Success {
				h.Status = HistoryCompleted
			} else {
				h.Status = HistoryFailed
				h.LastError = e.Error
			}
			// Store session ID for resume capability (especially on graceful pause)
			if e.SessionID != "" {
				h.LastSessionID = e.SessionID
			}
		}
		s.dirty = true

	case *BeadAbandonedEvent:
		if h := s.state.History[e.BeadID]; h != nil {
			h.Status = HistoryAbandoned
			h.LastError = e.LastError
		}
		s.dirty = true

	case *SessionEndEvent:
		// Only add cost if this session hasn't been counted via IterationEndEvent.
		// Use CurrentBead to identify the session (set by IterationStartEvent).
		// This is a fallback for when IterationEndEvent is missed.
		beadID := s.state.CurrentBead
		if beadID != "" && !s.countedSessions[beadID] {
			s.state.TotalCost += e.TotalCostUSD
			s.state.TotalTurns += e.NumTurns
			s.countedSessions[beadID] = true
			s.dirty = true
		}
	}

	// Debounced save
	if s.dirty && time.Since(s.lastSave) >= s.minDelay {
		s.saveUnlocked()
	}
}

func (s *StateSink) saveUnlocked() {
	s.state.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "state sink: marshal error: %v\n", err)
		return
	}

	// Atomic write: temp file + rename
	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "state sink: write error: %v\n", err)
		return
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		fmt.Fprintf(os.Stderr, "state sink: rename error: %v\n", err)
		return
	}

	s.dirty = false
	s.lastSave = time.Now()
}

func (s *StateSink) flushIfDirty() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dirty {
		s.saveUnlocked()
	}
}

// Stop waits for the run goroutine to finish and performs a final save if needed.
func (s *StateSink) Stop() error {
	<-s.done
	return nil
}

// Load reads the state file from disk.
// If the version is missing or incompatible, the old state is backed up and a fresh state is used.
func (s *StateSink) Load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		// Corrupted JSON: backup and start fresh
		if backupErr := s.backupStateFile(); backupErr != nil {
			slog.Warn("state file corrupted, failed to backup",
				"path", s.path,
				"error", err,
				"backup_error", backupErr)
		} else {
			slog.Warn("state file corrupted, backed up and starting fresh",
				"path", s.path,
				"error", err)
		}
		s.resetState()
		return nil
	}

	// Check version compatibility
	if state.Version == 0 || state.Version != CurrentStateVersion {
		if backupErr := s.backupStateFile(); backupErr != nil {
			slog.Warn("incompatible state version, failed to backup",
				"path", s.path,
				"file_version", state.Version,
				"current_version", CurrentStateVersion,
				"backup_error", backupErr)
		} else {
			slog.Warn("incompatible state version, backed up and starting fresh",
				"path", s.path,
				"file_version", state.Version,
				"current_version", CurrentStateVersion)
		}
		s.resetState()
		return nil
	}

	// Initialize history map if nil
	if state.History == nil {
		state.History = make(map[string]*BeadHistory)
	}

	s.state = &state
	return nil
}

// backupStateFile moves the current state file to a .backup file.
// Must be called with s.mu held.
func (s *StateSink) backupStateFile() error {
	backupPath := s.path + ".backup"
	return os.Rename(s.path, backupPath)
}

// resetState initializes a fresh state.
// Must be called with s.mu held.
func (s *StateSink) resetState() {
	s.state = &State{
		Version: CurrentStateVersion,
		History: make(map[string]*BeadHistory),
	}
}

// State returns a copy of the current state.
func (s *StateSink) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return *s.state
}

// Path returns the state file path.
func (s *StateSink) Path() string {
	return s.path
}

// SetMinDelay sets the minimum delay between saves (for testing).
func (s *StateSink) SetMinDelay(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.minDelay = d
}

// SetActiveTopLevel updates the active top-level item in the state.
// Pass empty strings to clear the active top-level.
func (s *StateSink) SetActiveTopLevel(id, title string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state.ActiveTopLevel = id
	s.state.ActiveTopLevelTitle = title
	s.dirty = true

	// Debounced save
	if time.Since(s.lastSave) >= s.minDelay {
		s.saveUnlocked()
	}
}
