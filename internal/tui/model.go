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

// PaneRect represents a rectangular region on screen for mouse hit testing.
type PaneRect struct {
	X, Y, Width, Height int
}

// Contains returns true if the point (x, y) is within the rectangle.
func (r PaneRect) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height
}

// IsEmpty returns true if the rectangle has zero area (pane not visible).
func (r PaneRect) IsEmpty() bool {
	return r.Width <= 0 || r.Height <= 0
}

const (
	// LayoutHorizontal shows events left, observer right.
	LayoutHorizontal LayoutMode = "horizontal"
	// LayoutVertical shows events top, observer bottom.
	LayoutVertical LayoutMode = "vertical"
)

// Layout size constants.
const (
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
	activeTopLevelID    string                     // active top-level item ID (when selection_mode=top-level)
	activeTopLevelTitle string                     // active top-level item title

	// Event log
	eventLines []eventLine

	// UI state
	width       int
	height      int
	scrollPos   int
	autoScroll  bool
	focusedPane FocusedPane

	// Events state
	eventsOpen             bool
	lastEventsVisibleLines int // cached from last render for scroll bounds

	// Observer state
	observerPane ObserverPane
	observerOpen bool
	layout       LayoutMode

	// Graph state
	graphPane GraphPane
	graphOpen bool
	focusMode FocusedPane // FocusModeNone for normal, or pane index for fullscreen

	// Pane rectangles for mouse hit testing (computed in updatePaneSizes)
	eventsRect   PaneRect
	observerRect PaneRect
	graphRect    PaneRect

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
	if epicID != "" {
		graphPane.SetEpicFilter(epicID)
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
	// Exit fullscreen mode on any toggle operation
	m.focusMode = FocusModeNone

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
	m.ensureFocusModeValid()
}

// toggleObserver toggles the observer pane visibility.
func (m *model) toggleObserver() {
	// Exit fullscreen mode on any toggle operation
	m.focusMode = FocusModeNone

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
	m.ensureFocusModeValid()
}

// toggleGraph toggles the graph pane visibility.
func (m *model) toggleGraph() {
	// Exit fullscreen mode on any toggle operation
	m.focusMode = FocusModeNone

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
	m.ensureFocusModeValid()
}

// updatePaneSizes recalculates pane dimensions based on current layout.
func (m *model) updatePaneSizes() {
	// Reset all rectangles
	m.eventsRect = PaneRect{}
	m.observerRect = PaneRect{}
	m.graphRect = PaneRect{}

	// Count visible panes (at least one pane is always open)
	numPanes := 0
	if m.eventsOpen {
		numPanes++
	}
	if m.observerOpen {
		numPanes++
	}
	if m.graphOpen {
		numPanes++
	}

	// Handle fullscreen mode
	if m.focusMode != FocusModeNone {
		m.updateFullscreenRects()
		m.updateEventsVisibleLines(1) // Fullscreen is effectively single pane
		return
	}

	// Update cached events visible lines based on layout
	m.updateEventsVisibleLines(numPanes)

	if numPanes == 0 {
		// All panes closed (header-only view)
		return
	}

	if numPanes == 1 {
		// Single pane gets full size
		m.updateSinglePaneRects()
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
	// In horizontal split layout:
	// - Shared header at top (4 lines content + 2 borders = 6 rows)
	// - Panes side by side below header
	// - Shared footer at bottom (1 row)
	headerRows := 6
	footerRows := 1
	paneY := headerRows
	paneHeight := m.height - headerRows - footerRows

	// Calculate equal widths for all panes
	paneWidth := m.width / numPanes
	currentX := 0
	panesRendered := 0

	// Events pane rectangle
	if m.eventsOpen {
		panesRendered++
		eventsW := paneWidth
		if panesRendered == numPanes {
			eventsW = m.width - currentX
		}
		m.eventsRect = PaneRect{X: currentX, Y: paneY, Width: eventsW, Height: paneHeight}
		currentX += eventsW
	}

	// Observer pane
	if m.observerOpen {
		panesRendered++
		obsW := paneWidth
		if panesRendered == numPanes {
			obsW = m.width - currentX
		}
		m.observerRect = PaneRect{X: currentX, Y: paneY, Width: obsW, Height: paneHeight}
		m.observerPane.SetSize(obsW-2, paneHeight-2)
		currentX += obsW
	}

	// Graph pane (gets remaining width)
	if m.graphOpen {
		graphW := m.width - currentX
		m.graphRect = PaneRect{X: currentX, Y: paneY, Width: graphW, Height: paneHeight}
		m.graphPane.SetSize(graphW-2, paneHeight-2)
	}
}

// updateVerticalPaneSizes calculates pane sizes for vertical layout.
func (m *model) updateVerticalPaneSizes(numPanes int) {
	// In vertical split layout:
	// - Events pane at top (eventsHeightPercent of height)
	// - Secondary panes stacked below
	eventsHeight := m.height * eventsHeightPercent / 100
	if eventsHeight < minEventsRows {
		eventsHeight = minEventsRows
	}
	remainingHeight := m.height - eventsHeight
	currentY := 0

	// Events pane rectangle
	if m.eventsOpen {
		m.eventsRect = PaneRect{X: 0, Y: currentY, Width: m.width, Height: eventsHeight}
		currentY += eventsHeight
	}

	if numPanes == 2 {
		// Two panes: events + (observer OR graph)
		paneHeight := remainingHeight
		if m.observerOpen {
			if paneHeight < minObserverRows {
				paneHeight = minObserverRows
			}
			m.observerRect = PaneRect{X: 0, Y: currentY, Width: m.width, Height: paneHeight}
			m.observerPane.SetSize(m.width-2, paneHeight-2)
		} else if m.graphOpen {
			if paneHeight < minGraphRows {
				paneHeight = minGraphRows
			}
			m.graphRect = PaneRect{X: 0, Y: currentY, Width: m.width, Height: paneHeight}
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

		if m.observerOpen {
			m.observerRect = PaneRect{X: 0, Y: currentY, Width: m.width, Height: observerHeight}
			m.observerPane.SetSize(m.width-2, observerHeight-2)
			currentY += observerHeight
		}
		if m.graphOpen {
			m.graphRect = PaneRect{X: 0, Y: currentY, Width: m.width, Height: graphHeight}
			m.graphPane.SetSize(m.width-2, graphHeight-2)
		}
	}
}

// updateFullscreenRects sets the rectangle for the fullscreen pane.
func (m *model) updateFullscreenRects() {
	fullRect := PaneRect{X: 0, Y: 0, Width: m.width, Height: m.height}
	switch m.focusMode {
	case FocusEvents:
		m.eventsRect = fullRect
	case FocusObserver:
		m.observerRect = fullRect
	case FocusGraph:
		m.graphRect = fullRect
	}
}

// updateSinglePaneRects sets the rectangle for the only open pane.
func (m *model) updateSinglePaneRects() {
	fullRect := PaneRect{X: 0, Y: 0, Width: m.width, Height: m.height}
	if m.eventsOpen {
		m.eventsRect = fullRect
	} else if m.observerOpen {
		m.observerRect = fullRect
	} else if m.graphOpen {
		m.graphRect = fullRect
	}
}

// updateEventsVisibleLines computes and caches the number of visible event lines
// based on the current layout. This is used for scroll bounds calculations.
func (m *model) updateEventsVisibleLines(numPanes int) {
	if !m.eventsOpen {
		// Keep last cached value when events not visible (minimum 1)
		if m.lastEventsVisibleLines < 1 {
			m.lastEventsVisibleLines = 1
		}
		return
	}

	if numPanes == 1 {
		// Events-only view: height - border(2) - header(3) - dividers(2) - footer(1) = 8
		m.lastEventsVisibleLines = max(1, m.height-8)
		return
	}

	// Split layout
	if m.layout == LayoutHorizontal {
		// Horizontal split: shared header/footer, events pane gets full height minus chrome
		// headerHeight(4) + footerHeight(1) + padding(2) = 7, then border(2)
		paneHeight := m.height - 4 - 1 - 2
		innerHeight := paneHeight - 2
		m.lastEventsVisibleLines = max(1, innerHeight)
	} else {
		// Vertical split: events pane gets percentage of height
		eventsHeight := max(m.height*eventsHeightPercent/100, minEventsRows)
		// innerHeight - header(3) - dividers(2) - footer(1) = 6
		innerHeight := eventsHeight - 2
		m.lastEventsVisibleLines = max(1, innerHeight-6)
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
// Uses the cached value from updatePaneSizes() when available.
func (m model) visibleLines() int {
	if m.lastEventsVisibleLines > 0 {
		return m.lastEventsVisibleLines
	}
	// Fallback for initial state before first layout update
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

// paneAt returns which pane contains the given screen coordinates.
// Returns FocusModeNone if no pane contains the point (e.g., header/footer area).
func (m model) paneAt(x, y int) FocusedPane {
	// Check in focus order: events first, then observer, then graph
	if m.eventsOpen && m.eventsRect.Contains(x, y) {
		return FocusEvents
	}
	if m.observerOpen && m.observerRect.Contains(x, y) {
		return FocusObserver
	}
	if m.graphOpen && m.graphRect.Contains(x, y) {
		return FocusGraph
	}
	return FocusModeNone
}

// ensureFocusModeValid validates and corrects focus state consistency.
// Invariant: if focusMode != FocusModeNone, the corresponding pane must be open
// and focusedPane must match focusMode.
func (m *model) ensureFocusModeValid() {
	if m.focusMode == FocusModeNone {
		return
	}

	// Check if the fullscreen pane is still open
	paneOpen := false
	switch m.focusMode {
	case FocusEvents:
		paneOpen = m.eventsOpen
	case FocusObserver:
		paneOpen = m.observerOpen
	case FocusGraph:
		paneOpen = m.graphOpen
	}

	if !paneOpen {
		// Fullscreen pane was closed, exit fullscreen mode
		m.focusMode = FocusModeNone
		return
	}

	// Ensure focusedPane matches focusMode when in fullscreen
	if m.focusedPane != m.focusMode {
		m.focusedPane = m.focusMode
	}
}
