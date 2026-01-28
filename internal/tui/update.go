package tui

import (
	"log/slog"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/npratt/atari/internal/events"
)

const (
	// maxEventLines is the maximum number of event lines to keep in the buffer.
	maxEventLines = 1000
	// trimEventLines is the number of lines to remove when buffer exceeds max.
	trimEventLines = 100
	// tickInterval is the interval for periodic stats sync.
	tickInterval = 2 * time.Second
)

// channelClosedMsg signals that the event channel was closed.
type channelClosedMsg struct{}

// tickMsg signals a periodic tick for stats synchronization.
type tickMsg time.Time

// waitForEvent creates a command that waits for the next event from the channel.
// Returns channelClosedMsg if the channel is closed.
func waitForEvent(ch <-chan events.Event) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-ch
		if !ok {
			return channelClosedMsg{}
		}
		return eventMsg(event)
	}
}

// doTick creates a command that waits for the tick interval and sends a tickMsg.
func doTick() tea.Cmd {
	return tea.Tick(tickInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update implements tea.Model. It handles all message types and updates the model.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleMouse(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updatePaneSizes()
		return m, nil

	case eventMsg:
		m.handleEvent(events.Event(msg))
		return m, waitForEvent(m.eventChan)

	case channelClosedMsg:
		// Event channel closed - clean exit
		slog.Info("event channel closed, exiting TUI")
		return m, tea.Quit

	case tickMsg:
		m.handleTick()
		return m, doTick()

	case observerTickMsg, observerResultMsg:
		// Forward observer messages to observer pane
		if m.observerOpen {
			var cmd tea.Cmd
			m.observerPane, cmd = m.observerPane.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)

	case graphTickMsg, graphResultMsg, graphStartLoadingMsg, graphDetailResultMsg, graphAutoRefreshMsg:
		// Forward graph messages to graph pane
		if m.graphOpen {
			var cmd tea.Cmd
			m.graphPane, cmd = m.graphPane.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)

	case spinner.TickMsg:
		// Forward spinner ticks to panes that are loading, regardless of focus.
		// This ensures spinner animation continues when pane is visible but not focused.
		if m.observerOpen && m.observerPane.IsLoading() {
			var cmd tea.Cmd
			m.observerPane, cmd = m.observerPane.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		if m.graphOpen && m.graphPane.IsLoading() {
			var cmd tea.Cmd
			m.graphPane, cmd = m.graphPane.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)

	case GraphOpenModalMsg:
		// Graph pane requested to open a modal
		if m.detailModal != nil && msg.NodeID != "" {
			node := m.graphPane.GetSelectedNode()
			if node != nil {
				cmd := m.detailModal.Open(node)
				return m, cmd
			}
		}
		return m, nil

	case modalFetchResultMsg:
		// Forward modal fetch result to modal
		if m.detailModal != nil {
			cmd := m.detailModal.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)

	default:
		// Forward unknown messages to focused secondary pane
		if m.observerOpen && m.isObserverFocused() {
			var cmd tea.Cmd
			m.observerPane, cmd = m.observerPane.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		} else if m.graphOpen && m.isGraphFocused() {
			var cmd tea.Cmd
			m.graphPane, cmd = m.graphPane.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)
	}
}

// handleKey processes keyboard input and returns the updated model and command.
func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If quit confirmation is open, handle confirmation keys
	if m.quitConfirmOpen {
		return m.handleQuitConfirm(msg)
	}

	// If modal is open, forward all keys to modal
	if m.detailModal != nil && m.detailModal.IsOpen() {
		cmd := m.detailModal.Update(msg)
		return m, cmd
	}

	key := msg.String()

	// Global keys: always work regardless of focus
	switch key {
	case "ctrl+c":
		// If observer is focused, forward ctrl+c to it (for query cancellation)
		if m.observerOpen && m.isObserverFocused() {
			if m.observerPane.IsLoading() {
				var cmd tea.Cmd
				m.observerPane, cmd = m.observerPane.Update(msg)
				return m, cmd
			}
			// When focused but not loading, ignore ctrl+c to prevent accidental quit
			return m, nil
		}
		if m.onQuit != nil {
			m.onQuit()
		}
		return m, tea.Quit

	case "tab":
		// Cycle focus if any secondary pane is open
		if m.anyPaneOpen() {
			m.cycleFocus()
		}
		return m, nil

	case "q":
		// Exit fullscreen mode if active (like esc)
		if m.focusMode != FocusModeNone {
			m.focusMode = FocusModeNone
			return m, nil
		}
		// Otherwise fall through to quit handling below

	case "esc":
		// Exit fullscreen mode if active
		if m.focusMode != FocusModeNone {
			m.focusMode = FocusModeNone
			return m, nil
		}
		// When observer is focused, first try to clear input/error
		if m.isObserverFocused() && m.observerOpen {
			var cmd tea.Cmd
			m.observerPane, cmd = m.observerPane.Update(msg)
			// If observer unfocused itself (nothing to clear), close the pane
			// Otherwise it cleared something and stays open
			if !m.observerPane.IsFocused() {
				m.toggleObserver()
			}
			return m, cmd
		}
		// When graph is focused, close it
		if m.isGraphFocused() && m.graphOpen {
			var cmd tea.Cmd
			m.graphPane, cmd = m.graphPane.Update(msg)
			// If graph unfocused itself, close the pane
			if !m.graphPane.IsFocused() {
				m.toggleGraph()
			}
			return m, cmd
		}
		return m, nil
	}

	// When observer is in INSERT mode, forward all other keys to it for typing.
	// This must come BEFORE panel toggle keys so e/o/b type instead of toggling.
	// Global keys (ctrl+c, tab, esc) are handled above and still work.
	if m.observerOpen && m.isObserverFocused() && m.observerPane.IsInsertMode() {
		var cmd tea.Cmd
		m.observerPane, cmd = m.observerPane.Update(msg)
		return m, cmd
	}

	// Panel toggle keys - always global so you can switch panels from anywhere
	switch key {
	case "e":
		m.toggleEvents()
		return m, nil

	case "o":
		m.toggleObserver()
		return m, nil

	case "b":
		m.toggleGraph()
		return m, m.graphPane.Init()

	case "E":
		if m.eventsOpen {
			if m.focusMode == FocusEvents {
				m.focusMode = FocusModeNone
			} else {
				m.focusMode = FocusEvents
				m.focusedPane = FocusEvents
				m.observerPane.SetFocused(false)
				m.graphPane.SetFocused(false)
			}
		}
		return m, nil

	case "B":
		if m.graphOpen {
			if m.focusMode == FocusGraph {
				m.focusMode = FocusModeNone
			} else {
				m.focusMode = FocusGraph
				m.focusedPane = FocusGraph
				m.graphPane.SetFocused(true)
				m.observerPane.SetFocused(false)
			}
		}
		return m, nil

	case "O":
		if m.observerOpen {
			if m.focusMode == FocusObserver {
				m.focusMode = FocusModeNone
			} else {
				m.focusMode = FocusObserver
				m.focusedPane = FocusObserver
				m.observerPane.SetFocused(true)
				m.graphPane.SetFocused(false)
			}
		}
		return m, nil
	}

	// Global control keys - work when observer is in normal mode (not typing)
	// In insert mode, these keys go to the textarea instead
	if !m.observerPane.IsInsertMode() {
		switch key {
		case "q":
			return m.tryQuit()

		case "p":
			if m.onPause != nil {
				m.onPause()
			}
			m.status = "pausing..."
			return m, nil

		case "r":
			if m.onResume != nil {
				m.onResume()
			}
			m.status = "resuming..."
			return m, nil
		}
	}

	// When observer is focused, forward remaining keys to observer pane
	if m.observerOpen && m.isObserverFocused() {
		var cmd tea.Cmd
		m.observerPane, cmd = m.observerPane.Update(msg)
		return m, cmd
	}

	// Global control keys for non-observer focus (graph pane, events pane)
	switch key {
	case "q":
		return m.tryQuit()

	case "p":
		if m.onPause != nil {
			m.onPause()
		}
		m.status = "pausing..."
		return m, nil

	case "r":
		if m.onResume != nil {
			m.onResume()
		}
		m.status = "resuming..."
		return m, nil
	}

	// When graph is focused, forward remaining keys to graph pane
	if m.graphOpen && m.isGraphFocused() {
		var cmd tea.Cmd
		m.graphPane, cmd = m.graphPane.Update(msg)
		return m, cmd
	}

	// Events pane focused: scrolling keys
	switch key {
	case "up", "k":
		m.autoScroll = false
		if m.scrollPos > 0 {
			m.scrollPos--
		}
		return m, nil

	case "down", "j":
		maxScroll := len(m.eventLines) - m.visibleLines()
		if m.scrollPos < maxScroll {
			m.scrollPos++
		}
		if m.scrollPos >= maxScroll {
			m.autoScroll = true
		}
		return m, nil

	case "home", "g":
		m.autoScroll = false
		m.scrollPos = 0
		return m, nil

	case "end", "G":
		m.autoScroll = true
		m.scrollPos = max(0, len(m.eventLines)-m.visibleLines())
		return m, nil

	default:
		return m, nil
	}
}

// handleEvent processes an event and updates model state.
func (m *model) handleEvent(event events.Event) {
	switch e := event.(type) {
	case *events.DrainStateChangedEvent:
		m.status = e.To

	case *events.IterationStartEvent:
		m.status = "working"
		m.currentBead = &beadInfo{
			ID:        e.BeadID,
			Title:     e.Title,
			Priority:  e.Priority,
			StartTime: event.Timestamp(),
		}
		m.stats.CurrentDurationMs = 0
		// Update graph pane with current bead for highlighting
		m.graphPane.SetCurrentBead(e.BeadID)
		// Update active top-level tracking
		m.activeTopLevelID = e.TopLevelID
		m.activeTopLevelTitle = e.TopLevelTitle
		// Update graph pane with active top-level for subtree highlighting
		m.graphPane.SetActiveTopLevel(e.TopLevelID)

	case *events.IterationEndEvent:
		m.currentBead = nil
		m.status = "idle"
		m.currentSessionTurns = 0 // Reset turn count for next session
		if e.Success {
			m.stats.Completed++
		} else {
			m.stats.Failed++
		}
		m.stats.TotalCost += e.TotalCostUSD
		m.stats.TotalTurns += e.NumTurns
		m.stats.TotalDurationMs += e.DurationMs
		m.stats.CurrentDurationMs = 0
		// Clear current bead highlighting in graph pane
		m.graphPane.SetCurrentBead("")
		// Note: We don't clear activeTopLevelID/Title here because the top-level
		// context persists across iterations within the same top-level item.
		// It will be updated when a new iteration starts with different top-level.

	case *events.TurnCompleteEvent:
		m.currentSessionTurns = e.TurnNumber

	case *events.BeadAbandonedEvent:
		m.stats.Abandoned++

	case *events.DrainStopEvent:
		m.status = "stopped"
	}

	// Add to event log with formatting
	text := Format(event)
	if text != "" {
		el := eventLine{
			Time:  event.Timestamp(),
			Text:  text,
			Style: StyleForEvent(event),
		}
		m.eventLines = append(m.eventLines, el)

		// Trim buffer if over max lines
		if len(m.eventLines) > maxEventLines {
			m.eventLines = m.eventLines[trimEventLines:]
			// Adjust scroll position after trimming
			m.scrollPos = max(0, m.scrollPos-trimEventLines)
		}

		// Auto-scroll to bottom if enabled
		if m.autoScroll {
			maxScroll := len(m.eventLines) - m.visibleLines()
			if maxScroll > 0 {
				m.scrollPos = maxScroll
			}
		}
	}
}

// handleTick processes periodic tick for stats synchronization.
func (m *model) handleTick() {
	// Update current bead elapsed time
	if m.currentBead != nil && !m.currentBead.StartTime.IsZero() {
		m.stats.CurrentDurationMs = time.Since(m.currentBead.StartTime).Milliseconds()
	}

	if m.statsGetter == nil {
		return
	}

	// Sync stats from controller using single GetStats() call
	stats := m.statsGetter.GetStats()

	// Check for drift and log warning if stats differ significantly
	if stats.Completed != m.stats.Completed {
		slog.Warn("stats drift detected",
			"field", "completed",
			"tui", m.stats.Completed,
			"controller", stats.Completed)
		m.stats.Completed = stats.Completed
	}
	if stats.Failed != m.stats.Failed {
		slog.Warn("stats drift detected",
			"field", "failed",
			"tui", m.stats.Failed,
			"controller", stats.Failed)
		m.stats.Failed = stats.Failed
	}
	if stats.Abandoned != m.stats.Abandoned {
		slog.Warn("stats drift detected",
			"field", "abandoned",
			"tui", m.stats.Abandoned,
			"controller", stats.Abandoned)
		m.stats.Abandoned = stats.Abandoned
	}

	// Update current bead from controller
	if stats.CurrentBead == "" && m.currentBead != nil {
		// Controller says no current bead but TUI thinks there is one
		slog.Warn("current bead drift detected",
			"tui", m.currentBead.ID,
			"controller", "none")
		m.currentBead = nil
	}

	// Update backoff stats for header display
	m.inBackoff = stats.InBackoff
	m.topBlockedBead = stats.TopBlockedBead

	// Note: Turn count is tracked via TurnCompleteEvent, not synced from controller.
	// The TUI is authoritative for session turn tracking.
}

// tryQuit attempts to quit, showing confirmation if atari is actively working.
// Only shows confirmation when status indicates active work (not idle/paused/stopped).
func (m model) tryQuit() (tea.Model, tea.Cmd) {
	// Check if we need confirmation - only when actively working
	needsConfirm := m.status != "idle" && m.status != "paused" && m.status != "stopped"

	if needsConfirm {
		m.quitConfirmOpen = true
		return m, nil
	}

	// Safe to quit immediately
	if m.onQuit != nil {
		m.onQuit()
	}
	return m, tea.Quit
}

// handleQuitConfirm handles keys when the quit confirmation dialog is open.
func (m model) handleQuitConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "y", "Y", "enter":
		// Confirmed - quit
		m.quitConfirmOpen = false
		if m.onQuit != nil {
			m.onQuit()
		}
		return m, tea.Quit

	case "n", "N", "esc", "q":
		// Cancelled - close dialog
		m.quitConfirmOpen = false
		return m, nil
	}

	// Ignore other keys while dialog is open
	return m, nil
}

// handleMouse processes mouse input for scrolling and focus changes.
func (m model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Ignore mouse when modal or quit confirmation is open
	if m.quitConfirmOpen {
		return m, nil
	}
	if m.detailModal != nil && m.detailModal.IsOpen() {
		return m, nil
	}

	switch msg.Button {
	case tea.MouseButtonWheelUp:
		return m.handleMouseScroll(msg, -3)

	case tea.MouseButtonWheelDown:
		return m.handleMouseScroll(msg, 3)

	case tea.MouseButtonLeft:
		if msg.Action == tea.MouseActionPress {
			return m.handleMouseClick(msg.X, msg.Y)
		}
	}

	return m, nil
}

// handleMouseScroll handles mouse wheel scrolling in the pane at the given position.
func (m model) handleMouseScroll(msg tea.MouseMsg, delta int) (tea.Model, tea.Cmd) {
	pane := m.paneAt(msg.X, msg.Y)

	switch pane {
	case FocusEvents:
		if !m.eventsOpen {
			return m, nil
		}
		oldPos := m.scrollPos
		if delta < 0 {
			// Scroll up
			m.scrollPos = max(0, m.scrollPos+delta)
		} else {
			// Scroll down
			maxScroll := len(m.eventLines) - m.visibleLines()
			if maxScroll < 0 {
				maxScroll = 0
			}
			m.scrollPos = min(maxScroll, m.scrollPos+delta)
			// Re-enable autoScroll if scrolled to bottom
			if m.scrollPos >= maxScroll {
				m.autoScroll = true
			}
		}
		// Only disable autoScroll if position actually changed
		if m.scrollPos != oldPos && delta < 0 {
			m.autoScroll = false
		}

	case FocusObserver:
		// Forward mouse scroll to observer pane
		if m.observerOpen {
			var cmd tea.Cmd
			m.observerPane, cmd = m.observerPane.Update(msg)
			return m, cmd
		}

	case FocusGraph:
		// Forward mouse scroll to graph pane
		if m.graphOpen {
			var cmd tea.Cmd
			m.graphPane, cmd = m.graphPane.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

// handleMouseClick handles mouse clicks for focus changes.
func (m model) handleMouseClick(x, y int) (tea.Model, tea.Cmd) {
	pane := m.paneAt(x, y)

	// Ignore clicks outside panes (header/footer areas)
	if pane == FocusModeNone {
		return m, nil
	}

	// Update focus if clicking a different pane
	if pane != m.focusedPane {
		m.observerPane.SetFocused(false)
		m.graphPane.SetFocused(false)
		m.focusedPane = pane

		switch pane {
		case FocusObserver:
			m.observerPane.SetFocused(true)
		case FocusGraph:
			m.graphPane.SetFocused(true)
		}
	}

	return m, nil
}
