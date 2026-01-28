// Package bdactivity provides JSONL file watching for bead state changes.
package bdactivity

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/events"
)

const (
	// defaultJSONLPath is the default path to the beads JSONL file.
	defaultJSONLPath = ".beads/issues.jsonl"

	// debounceInterval is the time to wait for rapid file changes to settle.
	debounceInterval = 100 * time.Millisecond

	// warningInterval is the minimum time between warning events.
	warningInterval = 5 * time.Second
)

// Watcher monitors the .beads/issues.jsonl file and emits BeadChangedEvents.
type Watcher struct {
	config    *config.BDActivityConfig
	router    *events.Router
	logger    *slog.Logger
	jsonlPath string

	running     atomic.Bool
	done        chan struct{}
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.Mutex
	lastWarning time.Time

	// beadState tracks the last known state of each bead for diff detection.
	beadState map[string]*events.BeadState

	// initialized tracks whether baseline state has been loaded.
	initialized atomic.Bool
	// fileExistedAtStart caches whether the JSONL file existed when Start() was called.
	fileExistedAtStart bool
}

// New creates a new BD Activity Watcher that monitors the JSONL file.
func New(cfg *config.BDActivityConfig, router *events.Router, _ interface{}, logger *slog.Logger) *Watcher {
	if logger == nil {
		logger = slog.Default()
	}
	return &Watcher{
		config:    cfg,
		router:    router,
		logger:    logger.With("component", "bdactivity"),
		jsonlPath: defaultJSONLPath,
		beadState: make(map[string]*events.BeadState),
	}
}

// NewWithPath creates a Watcher with a custom JSONL path (for testing).
func NewWithPath(cfg *config.BDActivityConfig, router *events.Router, logger *slog.Logger, jsonlPath string) *Watcher {
	w := New(cfg, router, nil, logger)
	w.jsonlPath = jsonlPath
	return w
}

// Start begins watching the JSONL file in a background goroutine.
// Returns immediately. Use Stop() to terminate.
func (w *Watcher) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running.Load() {
		return fmt.Errorf("watcher already running")
	}

	// Check if file exists before starting - used for silent initialization
	_, err := os.Stat(w.jsonlPath)
	w.fileExistedAtStart = err == nil

	w.ctx, w.cancel = context.WithCancel(ctx)
	w.done = make(chan struct{})
	w.running.Store(true)
	w.initialized.Store(false)
	w.beadState = make(map[string]*events.BeadState)

	go w.runLoop()

	return nil
}

// Stop terminates the watcher gracefully.
func (w *Watcher) Stop() error {
	w.mu.Lock()
	if !w.running.Load() {
		w.mu.Unlock()
		return nil
	}
	w.mu.Unlock()

	w.cancel()
	<-w.done

	return nil
}

// Running returns whether the watcher is currently active.
func (w *Watcher) Running() bool {
	return w.running.Load()
}

// runLoop is the main file watching loop.
func (w *Watcher) runLoop() {
	defer func() {
		w.running.Store(false)
		close(w.done)
	}()

	// Create fsnotify watcher
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		w.emitWarning(fmt.Sprintf("failed to create file watcher: %v", err))
		return
	}
	defer func() { _ = fsWatcher.Close() }()

	// Watch the parent directory since the file may not exist yet
	dir := filepath.Dir(w.jsonlPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		w.emitWarning(fmt.Sprintf("failed to create directory %s: %v", dir, err))
	}
	if err := fsWatcher.Add(dir); err != nil {
		w.emitWarning(fmt.Sprintf("failed to watch directory %s: %v", dir, err))
		return
	}

	w.logger.Info("started watching JSONL file", "path", w.jsonlPath)

	// Do initial load if file exists
	if err := w.loadAndDiff(); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			w.emitWarning(fmt.Sprintf("initial load failed: %v", err))
		}
	}

	// Debounce timer for rapid changes
	var debounceTimer *time.Timer
	var debounceMu sync.Mutex

	triggerReload := func() {
		debounceMu.Lock()
		if debounceTimer != nil {
			debounceTimer.Stop()
		}
		debounceTimer = time.AfterFunc(debounceInterval, func() {
			if err := w.loadAndDiff(); err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					w.emitWarning(fmt.Sprintf("reload failed: %v", err))
				}
			}
		})
		debounceMu.Unlock()
	}

	targetFile := filepath.Base(w.jsonlPath)

	for {
		select {
		case <-w.ctx.Done():
			debounceMu.Lock()
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceMu.Unlock()
			return

		case event, ok := <-fsWatcher.Events:
			if !ok {
				return
			}

			// Only react to changes to our target file
			if filepath.Base(event.Name) != targetFile {
				continue
			}

			// Trigger reload on write or create
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
				triggerReload()
			}

		case err, ok := <-fsWatcher.Errors:
			if !ok {
				return
			}
			w.emitWarning(fmt.Sprintf("file watcher error: %v", err))
		}
	}
}

// loadAndDiff reads the JSONL file and emits events for changed beads.
func (w *Watcher) loadAndDiff() error {
	newState, err := w.parseJSONLFile()
	if err != nil {
		return err
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Silent initialization: if file existed at start, seed state without emitting events
	if !w.initialized.Load() && w.fileExistedAtStart {
		w.beadState = newState
		w.initialized.Store(true)
		return nil
	}

	oldState := w.beadState
	w.beadState = newState
	w.initialized.Store(true)

	// Find changes
	now := time.Now()

	// Check for new and modified beads
	for id, newBead := range newState {
		oldBead := oldState[id]
		if oldBead == nil {
			// New bead
			w.router.Emit(&events.BeadChangedEvent{
				BaseEvent: events.BaseEvent{
					EventType: events.EventBeadChanged,
					Time:      now,
					Src:       events.SourceBD,
				},
				BeadID:   id,
				OldState: nil,
				NewState: newBead,
			})
		} else if !beadStateEqual(oldBead, newBead) {
			// Modified bead
			w.router.Emit(&events.BeadChangedEvent{
				BaseEvent: events.BaseEvent{
					EventType: events.EventBeadChanged,
					Time:      now,
					Src:       events.SourceBD,
				},
				BeadID:   id,
				OldState: oldBead,
				NewState: newBead,
			})
		}
	}

	// Check for deleted beads
	for id, oldBead := range oldState {
		if newState[id] == nil {
			w.router.Emit(&events.BeadChangedEvent{
				BaseEvent: events.BaseEvent{
					EventType: events.EventBeadChanged,
					Time:      now,
					Src:       events.SourceBD,
				},
				BeadID:   id,
				OldState: oldBead,
				NewState: nil,
			})
		}
	}

	return nil
}

// parseJSONLFile reads the entire JSONL file and returns bead states.
func (w *Watcher) parseJSONLFile() (map[string]*events.BeadState, error) {
	f, err := os.Open(w.jsonlPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	state := make(map[string]*events.BeadState)
	reader := bufio.NewReader(f)
	lineNum := 0

	for {
		line, err := reader.ReadBytes('\n')
		lineNum++

		if len(line) > 0 {
			// Remove trailing newline
			if line[len(line)-1] == '\n' {
				line = line[:len(line)-1]
			}
			if len(line) > 0 {
				bead, parseErr := ParseJSONLLine(line)
				if parseErr != nil {
					w.logger.Debug("parse error", "line", lineNum, "error", parseErr)
				} else if bead != nil {
					state[bead.ID] = bead
				}
			}
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
	}

	return state, nil
}

// beadStateEqual compares two bead states for equality.
func beadStateEqual(a, b *events.BeadState) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.ID == b.ID &&
		a.Title == b.Title &&
		a.Status == b.Status &&
		a.Priority == b.Priority &&
		a.IssueType == b.IssueType
}

// emitWarning emits a warning event to the router.
func (w *Watcher) emitWarning(msg string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()
	if now.Sub(w.lastWarning) < warningInterval {
		return
	}
	w.lastWarning = now

	w.logger.Warn(msg)
	w.router.Emit(&events.ErrorEvent{
		BaseEvent: events.BaseEvent{
			EventType: events.EventError,
			Time:      now,
			Src:       events.SourceInternal,
		},
		Message:  msg,
		Severity: "warning",
	})
}

// jsonlBead represents a bead record from the JSONL file.
type jsonlBead struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Priority  int    `json:"priority"`
	IssueType string `json:"issue_type"`
}

// ParseJSONLLine parses a single line from the JSONL file.
// Returns nil, nil for empty lines.
// Returns nil, error for invalid JSON.
func ParseJSONLLine(line []byte) (*events.BeadState, error) {
	if len(line) == 0 {
		return nil, nil
	}

	var bead jsonlBead
	if err := json.Unmarshal(line, &bead); err != nil {
		return nil, err
	}

	if bead.ID == "" {
		return nil, nil
	}

	return &events.BeadState{
		ID:        bead.ID,
		Title:     bead.Title,
		Status:    bead.Status,
		Priority:  bead.Priority,
		IssueType: bead.IssueType,
	}, nil
}
