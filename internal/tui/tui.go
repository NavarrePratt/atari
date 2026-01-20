// Package tui provides a terminal UI for monitoring atari using bubbletea.
package tui

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/npratt/atari/internal/events"
	"github.com/npratt/atari/internal/observer"
	"github.com/npratt/atari/internal/viewmodel"
)

// StatsGetter provides access to controller statistics.
type StatsGetter interface {
	Iteration() int
	Completed() int
	Failed() int
	Abandoned() int
	CurrentBead() string
	CurrentTurns() int
	GetStats() viewmodel.TUIStats
}

// TUI is the terminal UI for monitoring atari.
type TUI struct {
	eventChan       <-chan events.Event
	onPause         func()
	onResume        func()
	onQuit          func()
	statsGetter     StatsGetter
	observer        *observer.Observer
	graphFetcher    BeadFetcher
	beadStateGetter BeadStateGetter
	epicID          string
}

// Option configures the TUI.
type Option func(*TUI)

// New creates a new TUI with the given event channel and options.
func New(eventChan <-chan events.Event, opts ...Option) *TUI {
	t := &TUI{
		eventChan: eventChan,
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

// WithOnPause sets the callback invoked when the user presses 'p'.
func WithOnPause(fn func()) Option {
	return func(t *TUI) {
		t.onPause = fn
	}
}

// WithOnResume sets the callback invoked when the user presses 'r'.
func WithOnResume(fn func()) Option {
	return func(t *TUI) {
		t.onResume = fn
	}
}

// WithOnQuit sets the callback invoked when the user presses 'q'.
func WithOnQuit(fn func()) Option {
	return func(t *TUI) {
		t.onQuit = fn
	}
}

// WithStatsGetter sets the stats provider for header display.
func WithStatsGetter(sg StatsGetter) Option {
	return func(t *TUI) {
		t.statsGetter = sg
	}
}

// WithObserver sets the observer for interactive Q&A in the observer pane.
func WithObserver(obs *observer.Observer) Option {
	return func(t *TUI) {
		t.observer = obs
	}
}

// WithGraphFetcher sets the bead fetcher for graph visualization.
func WithGraphFetcher(fetcher BeadFetcher) Option {
	return func(t *TUI) {
		t.graphFetcher = fetcher
	}
}

// WithBeadStateGetter sets the workqueue state getter for graph node styling.
func WithBeadStateGetter(sg BeadStateGetter) Option {
	return func(t *TUI) {
		t.beadStateGetter = sg
	}
}

// WithEpicID sets the epic filter ID to display in the status.
func WithEpicID(epicID string) Option {
	return func(t *TUI) {
		t.epicID = epicID
	}
}

// Run starts the TUI and blocks until it exits.
// If the environment is non-interactive (no TTY) or the terminal is too small,
// it falls back to simple line-by-line output.
func (t *TUI) Run() error {
	// Fall back to simple mode for non-TTY environments
	if !isTerminal() {
		return t.runSimple()
	}

	// Fall back to simple mode if terminal is too small at startup
	if terminalTooSmall() {
		return t.runSimple()
	}

	// Run the full bubbletea TUI
	m := newModel(t.eventChan, t.onPause, t.onResume, t.onQuit, t.statsGetter, t.observer, t.graphFetcher, t.beadStateGetter, t.epicID)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
