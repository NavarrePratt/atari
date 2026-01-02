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
		tea.EnterAltScreen,
	)
}

// waitForEvent creates a command that waits for the next event.
func waitForEvent(ch <-chan events.Event) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-ch
		if !ok {
			return nil
		}
		return eventMsg(event)
	}
}

// Update implements tea.Model.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case eventMsg:
		m.handleEvent(events.Event(msg))
		return m, waitForEvent(m.eventChan)

	default:
		return m, nil
	}
}

// View is implemented in view.go

// handleKey processes keyboard input.
func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		if m.onQuit != nil {
			m.onQuit()
		}
		return m, tea.Quit

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
			ID:       e.BeadID,
			Title:    e.Title,
			Priority: e.Priority,
		}

	case *events.IterationEndEvent:
		m.currentBead = nil
		m.status = "idle"
		if e.Success {
			m.stats.Completed++
		} else {
			m.stats.Failed++
		}
		m.stats.TotalCost += e.TotalCostUSD
		m.stats.TotalTurns += e.NumTurns

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

		// Auto-scroll to bottom if enabled
		if m.autoScroll {
			maxScroll := len(m.eventLines) - m.visibleLines()
			if maxScroll > 0 {
				m.scrollPos = maxScroll
			}
		}
	}
}

// visibleLines returns the number of event lines that fit in the viewport.
func (m model) visibleLines() int {
	// Height minus header (3 lines), dividers (2), footer (1)
	return max(1, m.height-6)
}
