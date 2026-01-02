// Package tui provides a terminal UI for monitoring atari using bubbletea.
package tui

import (
	"github.com/charmbracelet/bubbletea"
	"github.com/npratt/atari/internal/events"
)

// StatsGetter provides access to controller statistics.
type StatsGetter interface {
	Iteration() int
	Completed() int
	Failed() int
	Abandoned() int
	CurrentBead() string
}

// TUI is the terminal UI for monitoring atari.
type TUI struct {
	eventChan   <-chan events.Event
	onPause     func()
	onResume    func()
	onQuit      func()
	statsGetter StatsGetter
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

// Run starts the TUI and blocks until it exits.
func (t *TUI) Run() error {
	m := newModel(t.eventChan, t.onPause, t.onResume, t.onQuit, t.statsGetter)

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
