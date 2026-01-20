package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/events"
	"github.com/npratt/atari/internal/observer"
	"github.com/npratt/atari/internal/viewmodel"
)

// FocusedPane represents which pane currently has keyboard focus.
type FocusedPane int

const (
	// FocusEvents means the events pane has focus (default).
	FocusEvents FocusedPane = iota
	// FocusObserver means the observer pane has focus.
	FocusObserver
	// FocusGraph means the graph pane has focus.
	FocusGraph
)

// FocusModeNone indicates no pane is in fullscreen focus mode.
const FocusModeNone FocusedPane = -1

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
	// minGraphCols is the minimum width for graph pane.
	minGraphCols = 30
	// minGraphRows is the minimum height for graph pane.
	minGraphRows = 10
)

// model is the bubbletea model for the TUI.
type model struct {
	// Event source
	eventChan <-chan events.Event

	// State
	status              string
	currentBead         *beadInfo
	stats               modelStats
	currentSessionTurns int                        // turns in current session (reset on iteration end)
	inBackoff           int                        // number of beads currently in backoff period
	topBlockedBead      *viewmodel.BlockedBeadInfo // bead with shortest remaining backoff
	epicID              string                     // active epic filter, if any

	// Event log
	eventLines []eventLine

	// UI state
	width       int
	height      int
	scrollPos   int
	autoScroll  bool
	focusedPane FocusedPane

	// Events state
	eventsOpen bool

	// Observer state
	observerPane ObserverPane
	observerOpen bool
	layout       LayoutMode

	// Graph state
	graphPane GraphPane
	graphOpen bool
	focusMode FocusedPane // FocusModeNone for normal, or pane index for fullscreen

	// Modal state
	detailModal      *DetailModal
	quitConfirmOpen  bool // Quit confirmation dialog is open

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
	graphFetcher BeadFetcher,
	beadStateGetter BeadStateGetter,
	epicID string,
) model {
	// Create default graph config for the graph pane
	graphCfg := &config.GraphConfig{
		Enabled:             true,
		Density:             "standard",
		AutoRefreshInterval: 5 * time.Second,
	}

	graphPane := NewGraphPane(graphCfg, graphFetcher, "horizontal")
	if beadStateGetter != nil {
		graphPane.SetStateGetter(beadStateGetter)
	}

	return model{
		eventChan:    eventChan,
		status:       "idle",
		autoScroll:   true,
		onPause:      onPause,
		onResume:     onResume,
		onQuit:       onQuit,
		statsGetter:  statsGetter,
		eventsOpen:   true, // Events panel visible by default
		observerPane: NewObserverPane(obs),
		graphPane:    graphPane,
		layout:       LayoutHorizontal,
		focusMode:    FocusModeNone,
		detailModal:  NewDetailModal(graphFetcher),
		epicID:       epicID,
	}
}

// toggleEvents toggles the events pane visibility.
func (m *model) toggleEvents() {
	m.eventsOpen = !m.eventsOpen
	if !m.eventsOpen && m.focusedPane == FocusEvents {
		// Move focus to another open pane, or clear focus if none open
		if m.observerOpen {
			m.focusedPane = FocusObserver
			m.observerPane.SetFocused(true)
		} else if m.graphOpen {
			m.focusedPane = FocusGraph
			m.graphPane.SetFocused(true)
		}
		// If no panes open, focusedPane stays as FocusEvents (will be used when reopening)
	}
	m.updatePaneSizes()
}

// toggleObserver toggles the observer pane visibility.
func (m *model) toggleObserver() {
	m.observerOpen = !m.observerOpen
	if m.observerOpen {
		m.focusedPane = FocusObserver
		m.observerPane.SetFocused(true)
		m.graphPane.SetFocused(false)
	} else {
		m.observerPane.SetFocused(false)
		m.observerPane.ClearResponse()
		// Move focus to an open pane
		if m.eventsOpen {
			m.focusedPane = FocusEvents
		} else if m.graphOpen {
			m.focusedPane = FocusGraph
			m.graphPane.SetFocused(true)
		}
	}
	m.updatePaneSizes()
}

// toggleGraph toggles the graph pane visibility.
func (m *model) toggleGraph() {
	m.graphOpen = !m.graphOpen
	m.graphPane.SetVisible(m.graphOpen)
	if m.graphOpen {
		m.focusedPane = FocusGraph
		m.graphPane.SetFocused(true)
		m.observerPane.SetFocused(false)
	} else {
		m.graphPane.SetFocused(false)
		// Move focus to an open pane
		if m.eventsOpen {
			m.focusedPane = FocusEvents
		} else if m.observerOpen {
			m.focusedPane = FocusObserver
			m.observerPane.SetFocused(true)
		}
	}
	m.updatePaneSizes()
}

// updatePaneSizes recalculates pane dimensions based on current layout.
func (m *model) updatePaneSizes() {
	// Count visible panes (at least one pane is always open)
	numPanes := 1
	if m.observerOpen {
		numPanes++
	}
	if m.graphOpen {
		numPanes++
	}

	if numPanes == 1 {
		// Only events pane - gets full size
		return
	}

	// Calculate sizes based on layout mode
	if m.layout == LayoutHorizontal {
		m.updateHorizontalPaneSizes(numPanes)
	} else {
		m.updateVerticalPaneSizes(numPanes)
	}
}

// updateHorizontalPaneSizes calculates pane sizes for horizontal layout.
func (m *model) updateHorizontalPaneSizes(numPanes int) {
	// Events always takes eventsWidthPercent
	eventsWidth := m.width * eventsWidthPercent / 100
	remainingWidth := m.width - eventsWidth

	if numPanes == 2 {
		// Two panes: events + (observer OR graph)
		paneWidth := remainingWidth
		if m.observerOpen {
			if paneWidth < minObserverCols {
				paneWidth = minObserverCols
			}
			m.observerPane.SetSize(paneWidth-2, m.height-2)
		} else {
			if paneWidth < minGraphCols {
				paneWidth = minGraphCols
			}
			m.graphPane.SetSize(paneWidth-2, m.height-2)
		}
	} else {
		// Three panes: split remaining width between observer and graph
		halfWidth := remainingWidth / 2
		observerWidth := halfWidth
		graphWidth := remainingWidth - halfWidth

		if observerWidth < minObserverCols {
			observerWidth = minObserverCols
		}
		if graphWidth < minGraphCols {
			graphWidth = minGraphCols
		}

		m.observerPane.SetSize(observerWidth-2, m.height-2)
		m.graphPane.SetSize(graphWidth-2, m.height-2)
	}
}

// updateVerticalPaneSizes calculates pane sizes for vertical layout.
func (m *model) updateVerticalPaneSizes(numPanes int) {
	// Events always takes eventsHeightPercent
	eventsHeight := m.height * eventsHeightPercent / 100
	remainingHeight := m.height - eventsHeight

	if numPanes == 2 {
		// Two panes: events + (observer OR graph)
		paneHeight := remainingHeight
		if m.observerOpen {
			if paneHeight < minObserverRows {
				paneHeight = minObserverRows
			}
			m.observerPane.SetSize(m.width-2, paneHeight-2)
		} else {
			if paneHeight < minGraphRows {
				paneHeight = minGraphRows
			}
			m.graphPane.SetSize(m.width-2, paneHeight-2)
		}
	} else {
		// Three panes: split remaining height between observer and graph
		halfHeight := remainingHeight / 2
		observerHeight := halfHeight
		graphHeight := remainingHeight - halfHeight

		if observerHeight < minObserverRows {
			observerHeight = minObserverRows
		}
		if graphHeight < minGraphRows {
			graphHeight = minGraphRows
		}

		m.observerPane.SetSize(m.width-2, observerHeight-2)
		m.graphPane.SetSize(m.width-2, graphHeight-2)
	}
}

// canShowSplitLayout returns true if terminal is large enough for split view.
func (m model) canShowSplitLayout() bool {
	// Calculate minimum required size based on which panes are open
	minWidth := minEventsCols + 4 // events + borders
	minHeight := minEventsRows + 4

	if m.observerOpen {
		if m.layout == LayoutHorizontal {
			minWidth += minObserverCols
		} else {
			minHeight += minObserverRows
		}
	}
	if m.graphOpen {
		if m.layout == LayoutHorizontal {
			minWidth += minGraphCols
		} else {
			minHeight += minGraphRows
		}
	}

	if m.layout == LayoutHorizontal {
		return m.width >= minWidth
	}
	return m.height >= minHeight
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

// cycleFocus advances focus to the next visible pane.
// Order: Events -> Observer (if open) -> Graph (if open) -> Events
func (m *model) cycleFocus() {
	m.observerPane.SetFocused(false)
	m.graphPane.SetFocused(false)

	// Build list of open panes in order: Events, Observer, Graph
	var openPanes []FocusedPane
	if m.eventsOpen {
		openPanes = append(openPanes, FocusEvents)
	}
	if m.observerOpen {
		openPanes = append(openPanes, FocusObserver)
	}
	if m.graphOpen {
		openPanes = append(openPanes, FocusGraph)
	}

	if len(openPanes) <= 1 {
		return // Can't cycle with only one pane
	}

	// Find current position and advance to next
	currentIdx := 0
	for i, pane := range openPanes {
		if pane == m.focusedPane {
			currentIdx = i
			break
		}
	}
	nextIdx := (currentIdx + 1) % len(openPanes)
	m.focusedPane = openPanes[nextIdx]

	// Set focus on the appropriate pane
	switch m.focusedPane {
	case FocusObserver:
		m.observerPane.SetFocused(true)
	case FocusGraph:
		m.graphPane.SetFocused(true)
	}
}

// isObserverFocused returns true if the observer pane has focus.
func (m model) isObserverFocused() bool {
	return m.focusedPane == FocusObserver
}

// isGraphFocused returns true if the graph pane has focus.
func (m model) isGraphFocused() bool {
	return m.focusedPane == FocusGraph
}

// anyPaneOpen returns true if observer or graph pane is open.
func (m model) anyPaneOpen() bool {
	return m.observerOpen || m.graphOpen
}

// allPanesClosed returns true if events, observer, and graph panes are all closed.
func (m model) allPanesClosed() bool {
	return !m.eventsOpen && !m.observerOpen && !m.graphOpen
}
