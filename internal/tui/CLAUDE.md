# TUI Package

Terminal UI for monitoring atari using bubbletea and lipgloss.

## Purpose

The TUI component provides:
- Real-time status and statistics display
- Live event feed with scrolling
- Keyboard controls for pause/resume/quit
- Terminal size adaptation

## Key Types

- `TUI`: Main entry point with event channel and callbacks
- `StatsGetter`: Interface for fetching controller stats
- `model`: Bubbletea model with state, events, and UI state
- `beadInfo`: Current bead information (ID, title, priority)
- `modelStats`: Display statistics (completed, failed, cost, turns)
- `eventLine`: Formatted event for display

## Public API

```go
func New(events <-chan events.Event, opts ...Option) *TUI
func (t *TUI) Run() error
```

## Options

```go
WithOnPause(fn func())     // Callback when user presses 'p'
WithOnResume(fn func())    // Callback when user presses 'r'
WithOnQuit(fn func())      // Callback when user presses 'q'
WithStatsGetter(sg StatsGetter)  // Stats provider for header display
```

## Files

| File | Contents |
|------|----------|
| tui.go | TUI struct, New(), Run(), Option functions |
| model.go | Bubbletea model, Init(), Update(), event handling |
| view.go | View() rendering, header, events, footer, tooSmall mode |
| styles.go | Lipgloss style definitions |
| format.go | Event formatting for display |

## Dependencies

- `events.Router`: Subscribe to unified event stream
- `github.com/charmbracelet/bubbletea`: TUI framework
- `github.com/charmbracelet/lipgloss`: Styling

## Keyboard Controls

| Key | Action |
|-----|--------|
| q, Ctrl+C | Quit |
| p | Pause drain |
| r | Resume drain |
| Up, k | Scroll up |
| Down, j | Scroll down |
| Home, g | Scroll to top |
| End, G | Scroll to bottom |

## Testing

Tests use mock event channels and verify:
- Model state transitions
- Event formatting (format_test.go)
- View rendering: header, events, footer (view_test.go)
- Scroll bounds and safe width calculations
- Terminal size handling (tooSmall mode)
