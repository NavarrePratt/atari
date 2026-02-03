package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

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

	// Get base content
	var baseContent string

	// Check for fullscreen focus mode
	if m.focusMode != FocusModeNone {
		baseContent = m.renderFullscreenPane()
	} else if m.allPanesClosed() {
		// All panels closed: show header-only monitoring view
		baseContent = m.renderHeaderOnlyView()
	} else if m.anyPaneOpen() {
		// If any secondary pane is open, render split layout
		baseContent = m.renderSplitView()
	} else {
		// Single pane view (events only)
		baseContent = m.renderEventsOnlyView()
	}

	// Overlay quit confirmation if open
	if m.quitConfirmOpen {
		return m.renderQuitConfirmDialog()
	}

	// Overlay modal if open
	if m.detailModal != nil && m.detailModal.IsOpen() {
		return m.renderWithModalOverlay(baseContent)
	}

	return baseContent
}

// renderWithModalOverlay renders the modal centered over the base content.
func (m model) renderWithModalOverlay(baseContent string) string {
	// Get modal content
	modalContent := m.detailModal.View(m.width, m.height)
	if modalContent == "" {
		return baseContent
	}

	// Center the modal on screen
	centeredModal := lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modalContent)

	return centeredModal
}

// renderQuitConfirmDialog renders the quit confirmation dialog centered on screen.
func (m model) renderQuitConfirmDialog() string {
	// Build confirmation dialog
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205"))

	messageStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Italic(true)

	var content strings.Builder
	content.WriteString(titleStyle.Render("Quit Atari?"))
	content.WriteString("\n\n")
	content.WriteString(messageStyle.Render("A session is currently in progress."))
	content.WriteString("\n")
	content.WriteString(messageStyle.Render("Quitting will interrupt the active work."))
	content.WriteString("\n\n")
	content.WriteString(hintStyle.Render("[y/Enter] Quit  [n/Esc/q] Cancel"))

	// Create modal box
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(1, 3).
		Width(44)

	modalContent := modalStyle.Render(content.String())

	// Center the modal on screen
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modalContent)
}

// renderFullscreenPane renders a single pane in fullscreen mode.
func (m model) renderFullscreenPane() string {
	switch m.focusMode {
	case FocusEvents:
		return m.renderEventsOnlyView()
	case FocusObserver:
		return m.renderFullscreenObserverView()
	case FocusGraph:
		return m.renderFullscreenGraphView()
	default:
		return m.renderEventsOnlyView()
	}
}

// renderFullscreenGraphView renders the graph pane in fullscreen mode with header and footer.
func (m model) renderFullscreenGraphView() string {
	w := safeWidth(m.width - 4) // Account for container borders

	// Header: status, bead info, stats (same as events view)
	var sections []string
	sections = append(sections, m.renderHeaderForWidth(w))
	sections = append(sections, m.renderDividerForWidth(w))

	// Graph content takes remaining space
	// Calculate available height: total height - header (3) - dividers (2) - footer (1) - borders (2)
	graphHeight := m.height - 8
	if graphHeight < minGraphRows {
		graphHeight = minGraphRows
	}

	// Render graph pane content at the calculated size
	m.graphPane.SetSize(w, graphHeight)
	graphContent := m.graphPane.View()
	sections = append(sections, graphContent)

	sections = append(sections, m.renderDividerForWidth(w))

	// Footer with fullscreen-specific help
	footer := m.renderFullscreenGraphFooter()
	sections = append(sections, footer)

	content := strings.Join(sections, "\n")

	// Get focus-aware container style
	containerStyle := m.containerStyleForFocus(FocusGraph)

	rendered := containerStyle.
		Width(safeWidth(m.width - 2)).
		Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, rendered)
}

// renderFullscreenObserverView renders the observer pane in fullscreen mode with header and footer.
func (m model) renderFullscreenObserverView() string {
	w := safeWidth(m.width - 4) // Account for container borders

	// Header: status, bead info, stats
	var sections []string
	sections = append(sections, m.renderHeaderForWidth(w))
	sections = append(sections, m.renderDividerForWidth(w))

	// Observer content takes remaining space
	// Calculate available height: total height - header (3) - dividers (2) - footer (1) - borders (2)
	observerHeight := m.height - 8
	if observerHeight < minObserverRows {
		observerHeight = minObserverRows
	}

	// Render observer pane content at the calculated size
	m.observerPane.SetSize(w, observerHeight)
	observerContent := m.observerPane.View()
	sections = append(sections, observerContent)

	sections = append(sections, m.renderDividerForWidth(w))

	// Footer with fullscreen-specific help
	footer := m.renderFullscreenObserverFooter()
	sections = append(sections, footer)

	content := strings.Join(sections, "\n")

	// Get focus-aware container style
	containerStyle := m.containerStyleForFocus(FocusObserver)

	rendered := containerStyle.
		Width(safeWidth(m.width - 2)).
		Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, rendered)
}

// renderFullscreenGraphFooter returns footer help when graph is in fullscreen mode.
func (m model) renderFullscreenGraphFooter() string {
	var help string
	if m.graphPane.IsShowingDetail() {
		help = "enter: fullscreen  esc: back  j/k: scroll  B: exit fullscreen  q: quit"
	} else {
		help = "↑/↓/←/→: nav  d: density  a: view  R: refresh  B/esc: exit fullscreen  q: quit"
	}
	return styles.Footer.Render(help)
}

// renderFullscreenObserverFooter returns footer help when observer is in fullscreen mode.
func (m model) renderFullscreenObserverFooter() string {
	help := "enter: ask  O/esc: exit fullscreen  ctrl+c: cancel  q: quit"
	return styles.Footer.Render(help)
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

// renderHeaderOnlyView renders a minimal view with just header, status info, and footer
// when all panes are closed. This allows users to monitor status and cost without panels.
func (m model) renderHeaderOnlyView() string {
	w := safeWidth(m.width - 4) // Account for container borders

	// Build the view with header, centered message, and footer
	// No divider after header - the box border provides separation
	var sections []string
	sections = append(sections, m.renderHeaderForWidth(w))

	// Calculate space for centered content
	// Total height minus header (3), divider (1), footer (1), borders (2) = 7
	contentHeight := m.height - 7
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Create centered monitoring message
	msg := "All panels closed - monitoring only"
	hint := "Press e/o/b to open panels"
	centeredMsg := lipgloss.PlaceHorizontal(w, lipgloss.Center, styles.Divider.Render(msg))
	centeredHint := lipgloss.PlaceHorizontal(w, lipgloss.Center, styles.Footer.Render(hint))

	// Pad to center vertically
	topPadding := (contentHeight - 2) / 2
	bottomPadding := contentHeight - 2 - topPadding
	if topPadding < 0 {
		topPadding = 0
	}
	if bottomPadding < 0 {
		bottomPadding = 0
	}

	var contentLines []string
	for i := 0; i < topPadding; i++ {
		contentLines = append(contentLines, "")
	}
	contentLines = append(contentLines, centeredMsg)
	contentLines = append(contentLines, centeredHint)
	for i := 0; i < bottomPadding; i++ {
		contentLines = append(contentLines, "")
	}
	sections = append(sections, strings.Join(contentLines, "\n"))

	sections = append(sections, m.renderDividerForWidth(w))
	sections = append(sections, m.renderHeaderOnlyFooter())

	content := strings.Join(sections, "\n")

	// Use unfocused border style since no pane is focused
	rendered := styles.UnfocusedBorder.
		Width(safeWidth(m.width - 2)).
		Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Left, lipgloss.Top, rendered)
}

// renderHeaderOnlyFooter returns footer help when all panels are closed.
func (m model) renderHeaderOnlyFooter() string {
	var parts []string

	// Pause/resume based on status
	switch m.status {
	case "paused", "pausing...", "pausing":
		parts = append(parts, "r: resume")
	case "stopped":
		// No pause/resume for stopped state
	default:
		parts = append(parts, "p: pause")
	}

	// Panel open hints
	parts = append(parts, "e: events", "o: observer", "b: beads")

	// Quit
	parts = append(parts, "q: quit")

	return styles.Footer.Render(strings.Join(parts, "  "))
}

// renderSplitView renders the split layout with events and secondary panes.
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

// renderHorizontalSplit renders events left, secondary panes right with shared header and footer.
func (m model) renderHorizontalSplit() string {
	// Count visible panes
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

	// Must have at least one pane
	if numPanes == 0 {
		return m.renderEventsOnlyView()
	}

	// Header spans full width (4 lines: 3 header + 1 divider)
	headerHeight := 4
	headerContent := m.renderSharedHeader(m.width - 2)
	header := styles.UnfocusedBorder.
		Width(m.width - 2).
		Render(headerContent)

	// Footer spans full width (1 line)
	footerHeight := 1
	footer := m.renderGlobalFooter(m.width - 2)

	// Remaining height for panes (subtract header, footer, and some padding)
	paneHeight := m.height - headerHeight - footerHeight - 2

	// Calculate equal widths for all panes
	paneWidth := m.width / numPanes
	remainingWidth := m.width

	// Render panes with equal widths
	var panes []string
	panesRendered := 0

	// Events pane (no header or footer, just events content)
	if m.eventsOpen {
		panesRendered++
		eventsW := paneWidth
		if panesRendered == numPanes {
			eventsW = remainingWidth
		}
		remainingWidth -= eventsW
		panes = append(panes, m.renderEventsPaneContent(eventsW, paneHeight))
	}

	// Observer pane (if open)
	if m.observerOpen {
		panesRendered++
		obsW := paneWidth
		if panesRendered == numPanes {
			obsW = remainingWidth
		}
		remainingWidth -= obsW
		panes = append(panes, m.renderObserverPane(obsW, paneHeight))
	}

	// Graph pane (if open) - gets remaining width
	if m.graphOpen {
		panes = append(panes, m.renderGraphPane(remainingWidth, paneHeight))
	}

	// Join panes horizontally
	panesRow := lipgloss.JoinHorizontal(lipgloss.Top, panes...)

	// Stack header, panes, and footer vertically
	return lipgloss.JoinVertical(lipgloss.Left, header, panesRow, footer)
}

// renderVerticalSplit renders events top, secondary panes bottom.
func (m model) renderVerticalSplit() string {
	// Calculate pane heights
	eventsHeight := m.height * eventsHeightPercent / 100
	remainingHeight := m.height - eventsHeight

	// Enforce minimums for events
	if eventsHeight < minEventsRows {
		eventsHeight = minEventsRows
		remainingHeight = m.height - eventsHeight
	}

	// Render events pane
	eventsPane := m.renderEventsPane(m.width, eventsHeight)

	// Determine secondary panes based on what's open
	var secondaryPanes []string

	if m.observerOpen && m.graphOpen {
		// Both open - split remaining height
		halfHeight := remainingHeight / 2
		observerHeight := halfHeight
		graphHeight := remainingHeight - halfHeight

		if observerHeight < minObserverRows {
			observerHeight = minObserverRows
		}
		if graphHeight < minGraphRows {
			graphHeight = minGraphRows
		}

		secondaryPanes = append(secondaryPanes, m.renderObserverPane(m.width, observerHeight))
		secondaryPanes = append(secondaryPanes, m.renderGraphPane(m.width, graphHeight))
	} else if m.observerOpen {
		// Only observer
		observerHeight := remainingHeight
		if observerHeight < minObserverRows {
			observerHeight = minObserverRows
		}
		secondaryPanes = append(secondaryPanes, m.renderObserverPane(m.width, observerHeight))
	} else if m.graphOpen {
		// Only graph
		graphHeight := remainingHeight
		if graphHeight < minGraphRows {
			graphHeight = minGraphRows
		}
		secondaryPanes = append(secondaryPanes, m.renderGraphPane(m.width, graphHeight))
	}

	// Join all panes vertically
	allPanes := []string{eventsPane}
	allPanes = append(allPanes, secondaryPanes...)
	return lipgloss.JoinVertical(lipgloss.Left, allPanes...)
}

// renderSharedHeader renders the header that spans all panes.
func (m model) renderSharedHeader(w int) string {
	// Line 1: Status, working directory, and cost
	cost := styles.Cost.Render(fmt.Sprintf("$%.4f", m.stats.TotalCost))
	costWidth := lipgloss.Width(cost)
	status := m.renderStatusWithWorkDir(w, costWidth)

	statusLine := lipgloss.JoinHorizontal(
		lipgloss.Top,
		status,
		strings.Repeat(" ", max(1, w-lipgloss.Width(status)-costWidth)),
		cost,
	)

	// Line 2: Current bead (or idle message with backoff info) with elapsed time and turn count
	var beadLine string
	if m.currentBead != nil {
		elapsed := formatDurationHuman(m.stats.CurrentDurationMs)
		var beadText string
		if m.currentSessionTurns > 0 {
			beadText = fmt.Sprintf("bead: %s - %s [%s, turn %d]",
				m.currentBead.ID, m.currentBead.Title, elapsed, m.currentSessionTurns)
		} else {
			beadText = fmt.Sprintf("bead: %s - %s [%s]",
				m.currentBead.ID, m.currentBead.Title, elapsed)
		}
		if len(beadText) > w {
			beadText = beadText[:w-3] + "..."
		}
		beadLine = styles.Bead.Render(beadText)
	} else {
		beadLine = m.renderBlockedInfo(w)
	}

	// Line 3: Turns, total duration, and progress stats
	turnsText := fmt.Sprintf("turns: %d", m.stats.TotalTurns)
	totalDur := formatDurationHuman(m.stats.TotalDurationMs)
	statsText := fmt.Sprintf("total: %s  completed: %d  failed: %d  abandoned: %d",
		totalDur, m.stats.Completed, m.stats.Failed, m.stats.Abandoned)

	// Style first, then calculate spacing based on visual width
	styledTurns := styles.Turns.Render(turnsText)
	styledStats := styles.Turns.Render(statsText)
	statsLine := lipgloss.JoinHorizontal(
		lipgloss.Top,
		styledTurns,
		strings.Repeat(" ", max(1, w-lipgloss.Width(styledTurns)-lipgloss.Width(styledStats))),
		styledStats,
	)

	// Add dividers
	divider := styles.Divider.Render(strings.Repeat("─", w))

	return strings.Join([]string{statusLine, beadLine, statsLine, divider}, "\n")
}

// renderEventsPaneContent renders events pane content only (no header or footer, for split view).
func (m model) renderEventsPaneContent(width, height int) string {
	// Account for borders
	innerWidth := safeWidth(width - 2)
	innerHeight := height - 2

	// All height available for events
	visibleLines := max(1, innerHeight)

	// Get events content
	content := m.renderEventsForSize(innerWidth, visibleLines)

	// Get focus-aware container style
	containerStyle := m.containerStyleForFocus(FocusEvents)

	return containerStyle.
		Width(innerWidth).
		Height(height - 2).
		Render(content)
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

// renderGraphPane renders the graph pane within the given dimensions.
func (m model) renderGraphPane(width, height int) string {
	// Account for borders
	innerWidth := safeWidth(width - 2)
	innerHeight := height - 2

	// Update graph pane size
	m.graphPane.SetSize(innerWidth, innerHeight)

	// Get graph pane content
	content := m.graphPane.View()

	// Get focus-aware container style
	containerStyle := m.containerStyleForFocus(FocusGraph)

	return containerStyle.
		Width(innerWidth).
		Height(innerHeight).
		Render(content)
}

// renderHeaderForWidth renders the header for a specific width.
func (m model) renderHeaderForWidth(w int) string {
	// Line 1: Status, working directory, and cost
	cost := styles.Cost.Render(fmt.Sprintf("$%.4f", m.stats.TotalCost))
	costWidth := lipgloss.Width(cost)
	status := m.renderStatusWithWorkDir(w, costWidth)

	statusLine := lipgloss.JoinHorizontal(
		lipgloss.Top,
		status,
		strings.Repeat(" ", max(1, w-lipgloss.Width(status)-costWidth)),
		cost,
	)

	// Line 2: Current bead (or idle message with backoff info) with elapsed time and turn count
	var beadLine string
	if m.currentBead != nil {
		elapsed := formatDurationHuman(m.stats.CurrentDurationMs)
		var beadText string
		if m.currentSessionTurns > 0 {
			beadText = fmt.Sprintf("bead: %s - %s [%s, turn %d]",
				m.currentBead.ID, m.currentBead.Title, elapsed, m.currentSessionTurns)
		} else {
			beadText = fmt.Sprintf("bead: %s - %s [%s]",
				m.currentBead.ID, m.currentBead.Title, elapsed)
		}
		if len(beadText) > w {
			beadText = beadText[:w-3] + "..."
		}
		beadLine = styles.Bead.Render(beadText)
	} else {
		beadLine = m.renderBlockedInfo(w)
	}

	// Line 3: Turns, total duration, and progress stats
	turnsText := fmt.Sprintf("turns: %d", m.stats.TotalTurns)
	totalDur := formatDurationHuman(m.stats.TotalDurationMs)
	statsText := fmt.Sprintf("total: %s  completed: %d  failed: %d  abandoned: %d",
		totalDur, m.stats.Completed, m.stats.Failed, m.stats.Abandoned)

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
// Observer pane uses distinct cyan-themed borders for visual distinction.
func (m model) containerStyleForFocus(pane FocusedPane) lipgloss.Style {
	// Observer pane uses distinct cyan-themed borders
	if pane == FocusObserver {
		if m.focusedPane == pane {
			return styles.ObserverFocusedBorder
		}
		return styles.ObserverUnfocusedBorder
	}

	// Default focus indicators for other panes
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

	// Line 1: Status, working directory, and cost
	cost := styles.Cost.Render(fmt.Sprintf("$%.4f", m.stats.TotalCost))
	costWidth := lipgloss.Width(cost)
	status := m.renderStatusWithWorkDir(w, costWidth)

	statusLine := lipgloss.JoinHorizontal(
		lipgloss.Top,
		status,
		strings.Repeat(" ", max(1, w-lipgloss.Width(status)-costWidth)),
		cost,
	)

	// Line 2: Current bead (or idle message with backoff info) with elapsed time and turn count
	var beadLine string
	if m.currentBead != nil {
		elapsed := formatDurationHuman(m.stats.CurrentDurationMs)
		var beadText string
		if m.currentSessionTurns > 0 {
			beadText = fmt.Sprintf("bead: %s - %s [%s, turn %d]",
				m.currentBead.ID, m.currentBead.Title, elapsed, m.currentSessionTurns)
		} else {
			beadText = fmt.Sprintf("bead: %s - %s [%s]",
				m.currentBead.ID, m.currentBead.Title, elapsed)
		}
		if len(beadText) > w {
			beadText = beadText[:w-3] + "..."
		}
		beadLine = styles.Bead.Render(beadText)
	} else {
		beadLine = m.renderBlockedInfo(w)
	}

	// Line 3: Turns, total duration, and progress stats
	turnsText := fmt.Sprintf("turns: %d", m.stats.TotalTurns)
	totalDur := formatDurationHuman(m.stats.TotalDurationMs)
	statsText := fmt.Sprintf("total: %s  completed: %d  failed: %d  abandoned: %d",
		totalDur, m.stats.Completed, m.stats.Failed, m.stats.Abandoned)

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
// If an epic filter is active, appends "(epic: bd-xxx)" suffix.
// If an active top-level item exists, appends "(top-level: bd-xxx - Title)" suffix.
// This version does not include working directory (for backwards compatibility).
func (m model) renderStatus() string {
	return m.renderStatusWithWorkDir(0, 0)
}

// renderStatusWithWorkDir renders the status with optional working directory.
// If totalWidth and costWidth are provided (non-zero), attempts to append the
// working directory with width-aware truncation.
func (m model) renderStatusWithWorkDir(totalWidth, costWidth int) string {
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

	result := style.Render(status)
	if m.epicID != "" {
		result += styles.Footer.Render(fmt.Sprintf(" (epic: %s)", m.epicID))
	} else if m.activeTopLevelID != "" {
		topLevelText := m.activeTopLevelID
		if m.activeTopLevelTitle != "" {
			topLevelText = fmt.Sprintf("%s - %s", m.activeTopLevelID, m.activeTopLevelTitle)
		}
		result += styles.Footer.Render(fmt.Sprintf(" (top-level: %s)", topLevelText))
	}

	// Skip working directory if no width info or no working directory
	if totalWidth == 0 || m.workingDirectory == "" {
		return result
	}

	// Compute available space for working directory
	// Leave at least 1 space between status+workdir and cost
	statusWidth := lipgloss.Width(result)
	availableForWorkDir := totalWidth - statusWidth - costWidth - 1

	// Need at least 5 chars for minimal display like " ..." + separator
	if availableForWorkDir < 5 {
		return result
	}

	// Try to fit the full path (with leading space)
	fullPath := " " + m.workingDirectory
	if lipgloss.Width(fullPath) <= availableForWorkDir {
		return result + styles.Footer.Render(fullPath)
	}

	// Truncate with "..." prefix showing as much of the path end as possible
	truncated := truncatePathForWidth(m.workingDirectory, availableForWorkDir-1) // -1 for leading space
	if truncated == "" {
		return result
	}

	return result + styles.Footer.Render(" "+truncated)
}

// truncatePathForWidth truncates a path to fit within maxWidth using "..." prefix.
// Shows as much of the path's trailing components as possible.
// Returns empty string if even minimal display won't fit.
func truncatePathForWidth(path string, maxWidth int) string {
	if maxWidth < 4 { // "..." requires at least 3 chars
		return ""
	}

	// Full path fits without truncation
	if lipgloss.Width(path) <= maxWidth {
		return path
	}

	// Use system path separator for display
	sep := string(filepath.Separator)
	ellipsis := "..."

	// Split path into components
	// Normalize to forward slashes for splitting, then use system separator for display
	normalized := filepath.ToSlash(path)
	parts := strings.Split(normalized, "/")

	// Filter out empty parts (from leading slash or double slashes)
	var nonEmpty []string
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}

	if len(nonEmpty) == 0 {
		return ""
	}

	// Try progressively shorter suffixes: last N components
	for i := 1; i <= len(nonEmpty); i++ {
		suffix := strings.Join(nonEmpty[len(nonEmpty)-i:], sep)
		display := ellipsis + sep + suffix
		if lipgloss.Width(display) <= maxWidth {
			continue // Keep trying to show more
		}
		// This is too long, use the previous length
		if i == 1 {
			// Even the last component with ellipsis doesn't fit
			// Try just showing truncated last component
			lastPart := nonEmpty[len(nonEmpty)-1]
			return truncateStringForWidth(ellipsis+sep+lastPart, maxWidth)
		}
		// Use i-1 components
		suffix = strings.Join(nonEmpty[len(nonEmpty)-i+1:], sep)
		return ellipsis + sep + suffix
	}

	// All components fit with ellipsis (shouldn't reach here if full path didn't fit)
	suffix := strings.Join(nonEmpty, sep)
	return ellipsis + sep + suffix
}

// truncateStringForWidth truncates a string to fit within maxWidth, adding "..." if needed.
func truncateStringForWidth(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	if maxWidth < 4 {
		return ""
	}

	// Truncate from the end, keeping as much as possible
	runes := []rune(s)
	for i := len(runes); i > 0; i-- {
		candidate := string(runes[:i]) + "..."
		if lipgloss.Width(candidate) <= maxWidth {
			return candidate
		}
	}
	return ""
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

	// Show different help based on focus and pane states
	switch {
	case m.isObserverFocused() && m.observerOpen:
		help = "enter: ask  tab: switch  esc: close  ctrl+c: quit"

	case m.isGraphFocused() && m.graphOpen:
		if m.graphPane.IsShowingDetail() {
			help = "enter: fullscreen  esc: back  j/k: scroll  tab: switch"
		} else {
			help = "↑/↓/←/→: nav  d: density  a: view  R: refresh  tab: switch  esc: close"
		}

	case m.focusedPane == FocusEvents:
		help = m.renderEventsFooter()
	}

	return styles.Footer.Render(help)
}

// renderGlobalFooter renders a global footer bar that spans the full width.
func (m model) renderGlobalFooter(width int) string {
	var help string

	// Show different help based on focus
	switch {
	case m.isObserverFocused() && m.observerOpen:
		help = "enter: ask  e/o/b: panels  tab: switch  esc: close  q: quit"

	case m.isGraphFocused() && m.graphOpen:
		if m.graphPane.IsShowingDetail() {
			help = "enter: fullscreen  esc: back  j/k: scroll  e/o/b: panels  tab: switch  q: quit"
		} else {
			help = "↑/↓/←/→: nav  d: density  a: view  R: refresh  e/o/b: panels  tab: switch  q: quit"
		}

	default:
		help = m.renderEventsFooter()
	}

	// Style the footer to span the full width
	return styles.Footer.Width(width).Render(help)
}

// renderEventsFooter returns footer help when events pane is focused.
func (m model) renderEventsFooter() string {
	var parts []string

	// Pause/resume based on status
	switch m.status {
	case "paused", "pausing...", "pausing":
		parts = append(parts, "r: resume")
	case "stopped":
		// No pause/resume for stopped state
	default:
		parts = append(parts, "p: pause")
	}

	// Panel toggles (show letter for each panel)
	parts = append(parts, "e/o/b: panels")

	// Fullscreen hint
	parts = append(parts, "E/O/B: fullscreen")

	// Tab switch if multiple panes open
	numOpen := 0
	if m.eventsOpen {
		numOpen++
	}
	if m.observerOpen {
		numOpen++
	}
	if m.graphOpen {
		numOpen++
	}
	if numOpen > 1 {
		parts = append(parts, "tab: switch")
	}

	// Common controls
	parts = append(parts, "q: quit", "↑/↓: scroll")

	return strings.Join(parts, "  ")
}

// formatDurationShort formats a time.Duration to compact form like "30s", "5m", "1h".
// Returns "now" for negative or near-zero durations.
func formatDurationShort(d time.Duration) string {
	if d < time.Second {
		return "now"
	}

	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}

	if d < time.Hour {
		minutes := int(d.Minutes())
		return fmt.Sprintf("%dm", minutes)
	}

	hours := int(d.Hours())
	return fmt.Sprintf("%dh", hours)
}

// renderBlockedInfo renders the blocked bead information for the header.
// Returns the styled text showing backoff status when idle with blocked beads.
func (m model) renderBlockedInfo(w int) string {
	// Only show blocked info when idle AND no current bead AND beads in backoff
	if m.status != "idle" || m.currentBead != nil || m.inBackoff == 0 {
		return styles.Bead.Render("no active bead")
	}

	var text string
	if m.topBlockedBead != nil {
		// Format: "no active bead - N in backoff (bd-xxx failed Nx, retry in Xm)"
		retryIn := formatDurationShort(m.topBlockedBead.RetryIn)
		text = fmt.Sprintf("no active bead - %d in backoff (%s failed %dx, retry in %s)",
			m.inBackoff, m.topBlockedBead.BeadID, m.topBlockedBead.FailureCount, retryIn)
	} else {
		// No top blocked bead info available
		text = fmt.Sprintf("no active bead - %d in backoff", m.inBackoff)
	}

	// Truncate if exceeds width
	if len(text) > w {
		text = text[:w-3] + "..."
	}

	return styles.StatusPaused.Render(text)
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
	case *events.IterationStartEvent, *events.IterationEndEvent, *events.TurnCompleteEvent:
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
