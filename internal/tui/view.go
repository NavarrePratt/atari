package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/npratt/atari/internal/events"
)

const (
	minWidth  = 60
	minHeight = 15
)

// View implements tea.Model. This renders the full TUI display.
func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	// Handle too small terminal
	if m.width < minWidth || m.height < minHeight {
		return m.renderTooSmall()
	}

	// If observer is open, render split layout
	if m.observerOpen {
		return m.renderSplitView()
	}

	// Single pane view (events only)
	return m.renderEventsOnlyView()
}

// renderEventsOnlyView renders the full-width events view when observer is closed.
func (m model) renderEventsOnlyView() string {
	// Build the view
	var sections []string
	sections = append(sections, m.renderHeader())
	sections = append(sections, m.renderDivider())
	sections = append(sections, m.renderEvents())
	sections = append(sections, m.renderDivider())
	sections = append(sections, m.renderFooter())

	content := strings.Join(sections, "\n")

	// Get focus-aware container style
	containerStyle := m.containerStyleForFocus(FocusEvents)

	// Render content in container without setting Height
	// Height() can cause clipping issues; let content determine size
	rendered := containerStyle.
		Width(safeWidth(m.width - 2)).
		Render(content)

	// Place container at top-left of terminal
	return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, rendered)
}

// renderSplitView renders the split layout with events and observer panes.
func (m model) renderSplitView() string {
	// Check if we can fit split layout
	if !m.canShowSplitLayout() {
		// Fall back to single pane with warning
		return m.renderEventsOnlyView()
	}

	if m.layout == LayoutHorizontal {
		return m.renderHorizontalSplit()
	}
	return m.renderVerticalSplit()
}

// renderHorizontalSplit renders events left, observer right.
func (m model) renderHorizontalSplit() string {
	// Calculate pane widths
	eventsWidth := m.width * eventsWidthPercent / 100
	observerWidth := m.width - eventsWidth

	// Enforce minimums
	if eventsWidth < minEventsCols {
		eventsWidth = minEventsCols
		observerWidth = m.width - eventsWidth
	}
	if observerWidth < minObserverCols {
		observerWidth = minObserverCols
		eventsWidth = m.width - observerWidth
	}

	// Render events pane
	eventsPane := m.renderEventsPane(eventsWidth, m.height)

	// Render observer pane
	observerPane := m.renderObserverPane(observerWidth, m.height)

	// Join horizontally
	return lipgloss.JoinHorizontal(lipgloss.Top, eventsPane, observerPane)
}

// renderVerticalSplit renders events top, observer bottom.
func (m model) renderVerticalSplit() string {
	// Calculate pane heights
	eventsHeight := m.height * eventsHeightPercent / 100
	observerHeight := m.height - eventsHeight

	// Enforce minimums
	if eventsHeight < minEventsRows {
		eventsHeight = minEventsRows
		observerHeight = m.height - eventsHeight
	}
	if observerHeight < minObserverRows {
		observerHeight = minObserverRows
		eventsHeight = m.height - observerHeight
	}

	// Render events pane
	eventsPane := m.renderEventsPane(m.width, eventsHeight)

	// Render observer pane
	observerPane := m.renderObserverPane(m.width, observerHeight)

	// Join vertically
	return lipgloss.JoinVertical(lipgloss.Left, eventsPane, observerPane)
}

// renderEventsPane renders the events pane within the given dimensions.
func (m model) renderEventsPane(width, height int) string {
	// Account for borders
	innerWidth := safeWidth(width - 2)
	innerHeight := height - 2

	// Calculate visible lines for this pane size
	// Height minus: header (3), dividers (2), footer (1) = 6
	visibleLines := max(1, innerHeight-6)

	// Build the view
	var sections []string
	sections = append(sections, m.renderHeaderForWidth(innerWidth))
	sections = append(sections, m.renderDividerForWidth(innerWidth))
	sections = append(sections, m.renderEventsForSize(innerWidth, visibleLines))
	sections = append(sections, m.renderDividerForWidth(innerWidth))
	sections = append(sections, m.renderFooter())

	content := strings.Join(sections, "\n")

	// Get focus-aware container style
	containerStyle := m.containerStyleForFocus(FocusEvents)

	return containerStyle.
		Width(innerWidth).
		Height(height - 2).
		Render(content)
}

// renderObserverPane renders the observer pane within the given dimensions.
func (m model) renderObserverPane(width, height int) string {
	// Account for borders
	innerWidth := safeWidth(width - 2)
	innerHeight := height - 2

	// Update observer pane size
	m.observerPane.SetSize(innerWidth, innerHeight)

	// Get observer pane content
	content := m.observerPane.View()

	// Get focus-aware container style
	containerStyle := m.containerStyleForFocus(FocusObserver)

	return containerStyle.
		Width(innerWidth).
		Height(innerHeight).
		Render(content)
}

// renderHeaderForWidth renders the header for a specific width.
func (m model) renderHeaderForWidth(w int) string {
	// Line 1: Status and cost
	status := m.renderStatus()
	cost := styles.Cost.Render(fmt.Sprintf("$%.4f", m.stats.TotalCost))

	statusLine := lipgloss.JoinHorizontal(
		lipgloss.Top,
		status,
		strings.Repeat(" ", max(1, w-lipgloss.Width(status)-lipgloss.Width(cost))),
		cost,
	)

	// Line 2: Current bead (or idle message)
	var beadLine string
	if m.currentBead != nil {
		beadText := fmt.Sprintf("bead: %s - %s", m.currentBead.ID, m.currentBead.Title)
		if len(beadText) > w {
			beadText = beadText[:w-3] + "..."
		}
		beadLine = styles.Bead.Render(beadText)
	} else {
		beadLine = styles.Bead.Render("no active bead")
	}

	// Line 3: Turns and progress stats
	turnsText := fmt.Sprintf("turns: %d", m.stats.TotalTurns)
	statsText := fmt.Sprintf("completed: %d  failed: %d  abandoned: %d",
		m.stats.Completed, m.stats.Failed, m.stats.Abandoned)

	// Style first, then calculate spacing based on visual width
	styledTurns := styles.Turns.Render(turnsText)
	styledStats := styles.Turns.Render(statsText)
	statsLine := lipgloss.JoinHorizontal(
		lipgloss.Top,
		styledTurns,
		strings.Repeat(" ", max(1, w-lipgloss.Width(styledTurns)-lipgloss.Width(styledStats))),
		styledStats,
	)

	return strings.Join([]string{statusLine, beadLine, statsLine}, "\n")
}

// renderDividerForWidth renders a divider for a specific width.
func (m model) renderDividerForWidth(w int) string {
	return styles.Divider.Render(strings.Repeat("─", w))
}

// renderEventsForSize renders events for a specific size.
func (m model) renderEventsForSize(w, visibleLines int) string {
	if len(m.eventLines) == 0 {
		// Center a placeholder message
		placeholder := "Waiting for events..."
		padding := strings.Repeat("\n", visibleLines/2)
		return padding + lipgloss.PlaceHorizontal(w, lipgloss.Center, placeholder)
	}

	// Calculate scroll bounds
	scrollPos := safeScroll(m.scrollPos, len(m.eventLines), visibleLines)

	// Get visible slice of events
	endPos := min(scrollPos+visibleLines, len(m.eventLines))
	visibleEvents := m.eventLines[scrollPos:endPos]

	// Render each event line
	var lines []string
	for _, el := range visibleEvents {
		line := m.renderEventLine(el, w)
		lines = append(lines, line)
	}

	// Pad with empty lines if needed
	for len(lines) < visibleLines {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// containerStyleForFocus returns the appropriate container style based on
// whether the specified pane is currently focused.
func (m model) containerStyleForFocus(pane FocusedPane) lipgloss.Style {
	if m.focusedPane == pane {
		return styles.FocusedBorder
	}
	return styles.UnfocusedBorder
}

// renderTooSmall renders a minimal message for terminals that are too small.
func (m model) renderTooSmall() string {
	msg := fmt.Sprintf("Terminal too small (%dx%d). Need %dx%d minimum.",
		m.width, m.height, minWidth, minHeight)
	return msg
}

// renderHeader renders the status header with state, cost, bead info, and stats.
func (m model) renderHeader() string {
	w := safeWidth(m.width - 4) // Account for container borders

	// Line 1: Status and cost
	status := m.renderStatus()
	cost := styles.Cost.Render(fmt.Sprintf("$%.4f", m.stats.TotalCost))

	statusLine := lipgloss.JoinHorizontal(
		lipgloss.Top,
		status,
		strings.Repeat(" ", max(1, w-lipgloss.Width(status)-lipgloss.Width(cost))),
		cost,
	)

	// Line 2: Current bead (or idle message)
	var beadLine string
	if m.currentBead != nil {
		beadText := fmt.Sprintf("bead: %s - %s", m.currentBead.ID, m.currentBead.Title)
		if len(beadText) > w {
			beadText = beadText[:w-3] + "..."
		}
		beadLine = styles.Bead.Render(beadText)
	} else {
		beadLine = styles.Bead.Render("no active bead")
	}

	// Line 3: Turns and progress stats
	turnsText := fmt.Sprintf("turns: %d", m.stats.TotalTurns)
	statsText := fmt.Sprintf("completed: %d  failed: %d  abandoned: %d",
		m.stats.Completed, m.stats.Failed, m.stats.Abandoned)

	// Style first, then calculate spacing based on visual width
	styledTurns := styles.Turns.Render(turnsText)
	styledStats := styles.Turns.Render(statsText)
	statsLine := lipgloss.JoinHorizontal(
		lipgloss.Top,
		styledTurns,
		strings.Repeat(" ", max(1, w-lipgloss.Width(styledTurns)-lipgloss.Width(styledStats))),
		styledStats,
	)

	return strings.Join([]string{statusLine, beadLine, statsLine}, "\n")
}

// renderStatus renders the status indicator with appropriate styling.
func (m model) renderStatus() string {
	status := strings.ToUpper(m.status)
	var style lipgloss.Style

	switch m.status {
	case "idle":
		style = styles.StatusIdle
	case "working":
		style = styles.StatusWorking
	case "paused", "pausing...", "pausing":
		style = styles.StatusPaused
	case "stopped":
		style = styles.StatusStopped
	default:
		style = styles.StatusIdle
	}

	return style.Render(status)
}

// renderDivider renders a horizontal divider line.
func (m model) renderDivider() string {
	w := safeWidth(m.width - 4) // Account for container borders
	return styles.Divider.Render(strings.Repeat("─", w))
}

// renderEvents renders the scrollable event feed.
func (m model) renderEvents() string {
	visible := m.visibleLines()
	w := safeWidth(m.width - 4) // Account for container borders

	if len(m.eventLines) == 0 {
		// Center a placeholder message
		placeholder := "Waiting for events..."
		padding := strings.Repeat("\n", visible/2)
		return padding + lipgloss.PlaceHorizontal(w, lipgloss.Center, placeholder)
	}

	// Calculate scroll bounds
	scrollPos := safeScroll(m.scrollPos, len(m.eventLines), visible)

	// Get visible slice of events
	endPos := min(scrollPos+visible, len(m.eventLines))
	visibleEvents := m.eventLines[scrollPos:endPos]

	// Render each event line
	var lines []string
	for _, el := range visibleEvents {
		line := m.renderEventLine(el, w)
		lines = append(lines, line)
	}

	// Pad with empty lines if needed
	for len(lines) < visible {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// renderEventLine renders a single event with timestamp and styling.
func (m model) renderEventLine(el eventLine, maxWidth int) string {
	// Format timestamp as HH:MM:SS
	timestamp := el.Time.Format("15:04:05")
	prefix := timestamp + " "

	// Calculate available width for text
	textWidth := maxWidth - len(prefix)
	if textWidth < 10 {
		textWidth = 10
	}

	// Truncate text if needed
	text := el.Text
	if len(text) > textWidth {
		text = text[:textWidth-3] + "..."
	}

	// Apply style and combine
	styledText := el.Style.Render(text)
	return styles.Turns.Render(prefix) + styledText
}

// renderFooter renders keyboard shortcuts help text.
func (m model) renderFooter() string {
	var help string

	// Show different help based on focus and observer state
	if m.observerOpen && m.isObserverFocused() {
		help = "enter: ask  tab: switch  esc: close  ctrl+c: quit"
	} else if m.observerOpen {
		// Events pane focused, observer open
		switch m.status {
		case "paused", "pausing...", "pausing":
			help = "r: resume  tab: switch  q: quit  ↑/↓: scroll"
		case "stopped":
			help = "tab: switch  q: quit  ↑/↓: scroll  g/G: top/bottom"
		default:
			help = "p: pause  tab: switch  q: quit  ↑/↓: scroll"
		}
	} else {
		// Observer closed - show 'o' to open
		switch m.status {
		case "paused", "pausing...", "pausing":
			help = "r: resume  o: observer  q: quit  ↑/↓: scroll  g/G: top/bottom"
		case "stopped":
			help = "o: observer  q: quit  ↑/↓: scroll  g/G: top/bottom"
		default:
			help = "p: pause  o: observer  q: quit  ↑/↓: scroll  g/G: top/bottom"
		}
	}
	return styles.Footer.Render(help)
}

// safeWidth returns a width that is at least 1 to prevent negative values.
func safeWidth(w int) int {
	if w < 1 {
		return 1
	}
	return w
}

// safeScroll clamps scroll position to valid bounds.
func safeScroll(pos, totalLines, visibleLines int) int {
	if pos < 0 {
		return 0
	}
	maxScroll := totalLines - visibleLines
	if maxScroll < 0 {
		return 0
	}
	if pos > maxScroll {
		return maxScroll
	}
	return pos
}

// StyleForEvent returns the appropriate style for an event type.
func StyleForEvent(event events.Event) lipgloss.Style {
	if event == nil {
		return styles.Tool
	}

	switch event.(type) {
	case *events.ClaudeToolUseEvent, *events.ClaudeToolResultEvent:
		return styles.Tool
	case *events.ClaudeTextEvent:
		return styles.Session
	case *events.SessionStartEvent, *events.SessionEndEvent, *events.SessionTimeoutEvent:
		return styles.Session
	case *events.IterationStartEvent, *events.IterationEndEvent:
		return styles.BeadStatus
	case *events.BeadCreatedEvent, *events.BeadStatusEvent, *events.BeadUpdatedEvent,
		*events.BeadCommentEvent, *events.BeadClosedEvent, *events.BeadAbandonedEvent:
		return styles.BeadStatus
	case *events.DrainStartEvent, *events.DrainStopEvent, *events.DrainStateChangedEvent:
		return styles.Session
	case *events.ErrorEvent, *events.ParseErrorEvent:
		return styles.Error
	default:
		return styles.Tool
	}
}
