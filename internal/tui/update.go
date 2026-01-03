package tui

import (
	"log/slog"
	"time"

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

	default:
		// Forward unknown messages to observer pane if focused
		if m.observerOpen && m.isObserverFocused() {
			var cmd tea.Cmd
			m.observerPane, cmd = m.observerPane.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return m, tea.Batch(cmds...)
	}
}

// handleKey processes keyboard input and returns the updated model and command.
func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Global keys: always work regardless of focus
	switch key {
	case "ctrl+c":
		// If observer has a query in progress, cancel it
		if m.observerOpen && m.observerPane.IsLoading() {
			var cmd tea.Cmd
			m.observerPane, cmd = m.observerPane.Update(msg)
			return m, cmd
		}
		if m.onQuit != nil {
			m.onQuit()
		}
		return m, tea.Quit

	case "tab":
		// Only cycle focus if observer is open
		if m.observerOpen {
			m.cycleFocus()
			m.observerPane.SetFocused(m.isObserverFocused())
		}
		return m, nil

	case "esc":
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
		return m, nil
	}

	// When observer is focused, forward keys to observer pane
	if m.observerOpen && m.isObserverFocused() {
		var cmd tea.Cmd
		m.observerPane, cmd = m.observerPane.Update(msg)
		return m, cmd
	}

	// Events pane focused: normal key handling
	switch key {
	case "q":
		if m.onQuit != nil {
			m.onQuit()
		}
		return m, tea.Quit

	case "o":
		// Toggle observer pane
		m.toggleObserver()
		return m, nil

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

	// Sync stats from controller
	completed := m.statsGetter.Completed()
	failed := m.statsGetter.Failed()
	abandoned := m.statsGetter.Abandoned()

	// Check for drift and log warning if stats differ significantly
	if completed != m.stats.Completed {
		slog.Warn("stats drift detected",
			"field", "completed",
			"tui", m.stats.Completed,
			"controller", completed)
		m.stats.Completed = completed
	}
	if failed != m.stats.Failed {
		slog.Warn("stats drift detected",
			"field", "failed",
			"tui", m.stats.Failed,
			"controller", failed)
		m.stats.Failed = failed
	}
	if abandoned != m.stats.Abandoned {
		slog.Warn("stats drift detected",
			"field", "abandoned",
			"tui", m.stats.Abandoned,
			"controller", abandoned)
		m.stats.Abandoned = abandoned
	}

	// Update current bead from controller
	currentBead := m.statsGetter.CurrentBead()
	if currentBead == "" && m.currentBead != nil {
		// Controller says no current bead but TUI thinks there is one
		slog.Warn("current bead drift detected",
			"tui", m.currentBead.ID,
			"controller", "none")
		m.currentBead = nil
	}

	// Note: Turn count is tracked via TurnCompleteEvent, not synced from controller.
	// The TUI is authoritative for session turn tracking.
}
