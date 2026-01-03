package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/npratt/atari/internal/events"
)

// beadInfo holds information about the current bead being processed.
type beadInfo struct {
	ID       string
	Title    string
	Priority int
}

// modelStats holds display statistics.
type modelStats struct {
	Completed  int
	Failed     int
	Abandoned  int
	TotalCost  float64
	TotalTurns int
}

// eventLine represents a formatted event for display.
type eventLine struct {
	Time  time.Time
	Text  string
	Style lipgloss.Style
}

// model is the bubbletea model for the TUI.
type model struct {
	// Event source
	eventChan <-chan events.Event

	// State
	status      string
	currentBead *beadInfo
	stats       modelStats

	// Event log
	eventLines []eventLine

	// UI state
	width      int
	height     int
	scrollPos  int
	autoScroll bool

	// Callbacks
	onPause  func()
	onResume func()
	onQuit   func()

	// Stats provider
	statsGetter StatsGetter
}

// eventMsg wraps an event for the bubbletea message system.
type eventMsg events.Event

// newModel creates a new model with the given configuration.
func newModel(
	eventChan <-chan events.Event,
	onPause, onResume, onQuit func(),
	statsGetter StatsGetter,
) model {
	return model{
		eventChan:   eventChan,
		status:      "idle",
		autoScroll:  true,
		onPause:     onPause,
		onResume:    onResume,
		onQuit:      onQuit,
		statsGetter: statsGetter,
	}
}

// Init implements tea.Model.
func (m model) Init() tea.Cmd {
	return tea.Batch(
		waitForEvent(m.eventChan),
		doTick(),
		tea.EnterAltScreen,
	)
}

// Update, handleKey, handleEvent, handleTick are implemented in update.go
// View is implemented in view.go

// visibleLines returns the number of event lines that fit in the viewport.
func (m model) visibleLines() int {
	// Height minus: border (2), header (3), dividers (2), footer (1) = 8
	return max(1, m.height-8)
}
