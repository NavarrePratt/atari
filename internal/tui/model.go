package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/npratt/atari/internal/events"
	"github.com/npratt/atari/internal/observer"
)

// FocusedPane represents which pane currently has keyboard focus.
type FocusedPane int

const (
	// FocusEvents means the events pane has focus (default).
	FocusEvents FocusedPane = iota
	// FocusObserver means the observer pane has focus.
	FocusObserver
)

// beadInfo holds information about the current bead being processed.
type beadInfo struct {
	ID        string
	Title     string
	Priority  int
	StartTime time.Time
}

// modelStats holds display statistics.
type modelStats struct {
	Completed         int
	Failed            int
	Abandoned         int
	TotalCost         float64
	TotalTurns        int
	TotalDurationMs   int64
	CurrentDurationMs int64
}

// eventLine represents a formatted event for display.
type eventLine struct {
	Time  time.Time
	Text  string
	Style lipgloss.Style
}

// LayoutMode represents the split layout orientation.
type LayoutMode string

const (
	// LayoutHorizontal shows events left, observer right.
	LayoutHorizontal LayoutMode = "horizontal"
	// LayoutVertical shows events top, observer bottom.
	LayoutVertical LayoutMode = "vertical"
)

// Layout size constants.
const (
	// eventsWidthPercent is the percentage of width for events pane in horizontal layout.
	eventsWidthPercent = 60
	// eventsHeightPercent is the percentage of height for events pane in vertical layout.
	eventsHeightPercent = 60
	// minEventsCols is the minimum width for events pane.
	minEventsCols = 40
	// minEventsRows is the minimum height for events pane.
	minEventsRows = 10
	// minObserverCols is the minimum width for observer pane.
	minObserverCols = 30
	// minObserverRows is the minimum height for observer pane.
	minObserverRows = 8
)

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
	width       int
	height      int
	scrollPos   int
	autoScroll  bool
	focusedPane FocusedPane

	// Observer state
	observerPane ObserverPane
	observerOpen bool
	layout       LayoutMode

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
	obs *observer.Observer,
) model {
	return model{
		eventChan:    eventChan,
		status:       "idle",
		autoScroll:   true,
		onPause:      onPause,
		onResume:     onResume,
		onQuit:       onQuit,
		statsGetter:  statsGetter,
		observerPane: NewObserverPane(obs),
		layout:       LayoutHorizontal,
	}
}

// toggleObserver toggles the observer pane visibility.
func (m *model) toggleObserver() {
	m.observerOpen = !m.observerOpen
	if m.observerOpen {
		m.focusedPane = FocusObserver
		m.observerPane.SetFocused(true)
	} else {
		m.focusedPane = FocusEvents
		m.observerPane.SetFocused(false)
	}
	m.updatePaneSizes()
}

// updatePaneSizes recalculates pane dimensions based on current layout.
func (m *model) updatePaneSizes() {
	if !m.observerOpen {
		// Observer closed - events pane gets full width/height
		return
	}

	// Calculate observer pane size based on layout
	if m.layout == LayoutHorizontal {
		observerWidth := m.width * (100 - eventsWidthPercent) / 100
		if observerWidth < minObserverCols {
			observerWidth = minObserverCols
		}
		m.observerPane.SetSize(observerWidth-2, m.height-2) // Account for borders
	} else {
		observerHeight := m.height * (100 - eventsHeightPercent) / 100
		if observerHeight < minObserverRows {
			observerHeight = minObserverRows
		}
		m.observerPane.SetSize(m.width-2, observerHeight-2) // Account for borders
	}
}

// canShowSplitLayout returns true if terminal is large enough for split view.
func (m model) canShowSplitLayout() bool {
	if m.layout == LayoutHorizontal {
		return m.width >= minEventsCols+minObserverCols+4 // +4 for borders
	}
	return m.height >= minEventsRows+minObserverRows+4
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

// cycleFocus advances focus to the next pane.
func (m *model) cycleFocus() {
	switch m.focusedPane {
	case FocusEvents:
		m.focusedPane = FocusObserver
	case FocusObserver:
		m.focusedPane = FocusEvents
	}
}

// isObserverFocused returns true if the observer pane has focus.
func (m model) isObserverFocused() bool {
	return m.focusedPane == FocusObserver
}
