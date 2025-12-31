# Terminal UI

Rich terminal interface for monitoring atari using bubbletea.

## Purpose

The TUI component is responsible for:
- Displaying current status and statistics
- Showing live event feed
- Providing keyboard controls for pause/resume/quit
- Adapting to terminal size changes

## Interface

```go
type TUI struct {
    events   <-chan events.Event
    onPause  func()
    onResume func()
    onQuit   func()
}

// Public API
func New(events <-chan events.Event, opts ...Option) *TUI
func (t *TUI) Run() error
```

## Dependencies

| Component | Usage |
|-----------|-------|
| events.Router | Subscribe to event stream |

External:
- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/lipgloss` - Styling

## Layout

### Standard Layout

```
┌─ ATARI ─────────────────────────────────────────────────────────┐
│ Status: WORKING                            Cost: $2.35          │
│ Current: bd-042 "Fix auth bug"             Turns: 42            │
│ Progress: 4 completed, 1 failed, 3 remaining                    │
├─ Events ────────────────────────────────────────────────────────┤
│ 14:23:45 $ go test ./...                                        │
│ 14:23:50 ✓ bd-042 closed                                        │
│ 14:23:51 BEAD bd-043 "Add rate limiting" (priority 2)           │
│ 14:23:52 SESSION started opus                                   │
│ 14:23:54 Read: src/ratelimit.go                                 │
│ 14:23:56 Edit: src/ratelimit.go                                 │
│ 14:23:58 $ go build ./...                                       │
│ ...                                                             │
├─────────────────────────────────────────────────────────────────┤
│ [o] observer  [p] pause  [r] resume  [q] quit  [↑↓] scroll      │
└─────────────────────────────────────────────────────────────────┘
```

### Layout with Observer Pane

When observer mode is active, the TUI splits to show Q&A:

```
┌─ ATARI ─────────────────────────────────────────────────────────┐
│ Status: WORKING                            Cost: $2.35          │
│ Current: bd-042 "Fix auth bug"             Turns: 42            │
├─ Events ────────────────────────────────────────────────────────┤
│ 14:23:45 $ go test ./...                                        │
│ 14:23:50 ✓ bd-042 closed                                        │
│ 14:23:51 BEAD bd-043 "Add rate limiting"                        │
│ ...                                                             │
├─ Observer (haiku) ──────────────────────── Cost: $0.03 ─────────┤
│ > Why did Claude run the tests twice?                           │
│                                                                 │
│ The tests were run twice because the first run had a flaky      │
│ failure in test_auth.go. Claude retried to confirm whether      │
│ it was a genuine failure or transient issue.                    │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│ [o] close  [Tab] focus  [Enter] send  [p] pause  [q] quit       │
└─────────────────────────────────────────────────────────────────┘
```

See [observer.md](observer.md) for Observer Mode details.

## Implementation

### Bubbletea Model

```go
type model struct {
    // State
    status      string
    currentBead *BeadInfo
    stats       Stats
    events      []EventLine
    eventChan   <-chan events.Event

    // UI state
    width       int
    height      int
    scrollPos   int
    autoScroll  bool

    // Observer state (future feature)
    observerOpen    bool
    observerFocused bool
    observerInput   string
    observerOutput  string
    observerCost    float64
    observer        *observer.Observer

    // Callbacks
    onPause  func()
    onResume func()
    onQuit   func()
}

type BeadInfo struct {
    ID       string
    Title    string
    Priority int
}

type EventLine struct {
    Time    time.Time
    Text    string
    Style   lipgloss.Style
}

type Stats struct {
    Completed int
    Failed    int
    Remaining int
    TotalCost float64
    TotalTurns int
}
```

### Initialization

```go
func New(eventChan <-chan events.Event, opts ...Option) *TUI {
    t := &TUI{
        events: eventChan,
    }

    for _, opt := range opts {
        opt(t)
    }

    return t
}

func (t *TUI) Run() error {
    m := model{
        eventChan:  t.events,
        autoScroll: true,
        onPause:    t.onPause,
        onResume:   t.onResume,
        onQuit:     t.onQuit,
    }

    p := tea.NewProgram(m, tea.WithAltScreen())
    _, err := p.Run()
    return err
}

func (m model) Init() tea.Cmd {
    return tea.Batch(
        waitForEvent(m.eventChan),
        tea.EnterAltScreen,
    )
}
```

### Event Handling

```go
// Custom message types
type eventMsg events.Event
type tickMsg time.Time

func waitForEvent(ch <-chan events.Event) tea.Cmd {
    return func() tea.Msg {
        event, ok := <-ch
        if !ok {
            return nil
        }
        return eventMsg(event)
    }
}

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
```

### Keyboard Handling

```go
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

    case "o":
        // Toggle observer pane (future feature)
        m.observerOpen = !m.observerOpen
        if m.observerOpen {
            return m, m.startObserver()
        }
        return m, m.stopObserver()

    case "tab":
        // Switch focus between events and observer (when observer open)
        if m.observerOpen {
            m.observerFocused = !m.observerFocused
        }
        return m, nil

    case "enter":
        // Send observer query (when observer focused)
        if m.observerOpen && m.observerFocused && m.observerInput != "" {
            return m, m.sendObserverQuery()
        }
        return m, nil

    case "escape":
        // Close observer pane
        if m.observerOpen {
            m.observerOpen = false
            return m, m.stopObserver()
        }
        return m, nil

    case "up", "k":
        m.autoScroll = false
        if m.scrollPos > 0 {
            m.scrollPos--
        }
        return m, nil

    case "down", "j":
        maxScroll := len(m.events) - m.visibleLines()
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
        m.scrollPos = len(m.events) - m.visibleLines()
        return m, nil

    default:
        return m, nil
    }
}
```

### Event Processing

```go
func (m *model) handleEvent(event events.Event) {
    // Update state based on event
    switch e := event.(type) {
    case *events.IterationStart:
        m.status = "working"
        m.currentBead = &BeadInfo{
            ID:       e.BeadID,
            Title:    e.Title,
            Priority: e.Priority,
        }

    case *events.IterationEnd:
        m.currentBead = nil
        m.status = "idle"
        if e.Success {
            m.stats.Completed++
        } else {
            m.stats.Failed++
        }
        m.stats.TotalCost += e.TotalCostUSD
        m.stats.TotalTurns += e.NumTurns

    case *events.DrainPause:
        m.status = "paused"

    case *events.DrainResume:
        m.status = "idle"

    case *events.DrainStop:
        m.status = "stopped"
    }

    // Add to event log
    line := m.formatEvent(event)
    if line.Text != "" {
        m.events = append(m.events, line)

        // Auto-scroll to bottom
        if m.autoScroll {
            maxScroll := len(m.events) - m.visibleLines()
            if maxScroll > 0 {
                m.scrollPos = maxScroll
            }
        }

        // Limit event buffer
        if len(m.events) > 1000 {
            m.events = m.events[100:]
            m.scrollPos = max(0, m.scrollPos-100)
        }
    }
}

func (m model) formatEvent(event events.Event) EventLine {
    ts := event.Timestamp()

    switch e := event.(type) {
    case *events.ToolUse:
        return EventLine{
            Time:  ts,
            Text:  e.Format(),
            Style: styles.Tool,
        }

    case *events.BeadStatus:
        return EventLine{
            Time:  ts,
            Text:  e.Format(),
            Style: styles.BeadStatus,
        }

    case *events.SessionStart:
        return EventLine{
            Time:  ts,
            Text:  fmt.Sprintf("SESSION started %s", e.Model),
            Style: styles.Session,
        }

    case *events.SessionEnd:
        return EventLine{
            Time:  ts,
            Text:  fmt.Sprintf("SESSION ended | turns: %d | cost: $%.2f", e.NumTurns, e.TotalCostUSD),
            Style: styles.Session,
        }

    case *events.Error:
        return EventLine{
            Time:  ts,
            Text:  fmt.Sprintf("ERROR: %s", e.Message),
            Style: styles.Error,
        }

    default:
        return EventLine{}
    }
}
```

### View Rendering

```go
func (m model) View() string {
    if m.width == 0 {
        return "Loading..."
    }

    var b strings.Builder

    // Header
    b.WriteString(m.renderHeader())
    b.WriteString("\n")

    // Divider
    b.WriteString(styles.Divider.Render(strings.Repeat("─", m.width-2)))
    b.WriteString("\n")

    // Event feed
    b.WriteString(m.renderEvents())
    b.WriteString("\n")

    // Footer
    b.WriteString(styles.Divider.Render(strings.Repeat("─", m.width-2)))
    b.WriteString("\n")
    b.WriteString(m.renderFooter())

    return styles.Container.Width(m.width).Height(m.height).Render(b.String())
}

func (m model) renderHeader() string {
    // Status line
    statusText := fmt.Sprintf("Status: %s", strings.ToUpper(m.status))
    costText := fmt.Sprintf("Cost: $%.2f", m.stats.TotalCost)

    statusLine := lipgloss.JoinHorizontal(
        lipgloss.Top,
        styles.Status.Render(statusText),
        styles.Spacer.Width(m.width-len(statusText)-len(costText)-10).Render(""),
        styles.Cost.Render(costText),
    )

    // Current bead
    var beadLine string
    if m.currentBead != nil {
        beadLine = fmt.Sprintf("Current: %s %q", m.currentBead.ID, truncate(m.currentBead.Title, 40))
    } else {
        beadLine = "Current: (none)"
    }

    turnsText := fmt.Sprintf("Turns: %d", m.stats.TotalTurns)

    beadRow := lipgloss.JoinHorizontal(
        lipgloss.Top,
        styles.Bead.Render(beadLine),
        styles.Spacer.Width(m.width-len(beadLine)-len(turnsText)-10).Render(""),
        styles.Turns.Render(turnsText),
    )

    // Progress line
    progressText := fmt.Sprintf("Progress: %d completed, %d failed",
        m.stats.Completed, m.stats.Failed)

    return lipgloss.JoinVertical(lipgloss.Left, statusLine, beadRow, progressText)
}

func (m model) renderEvents() string {
    visible := m.visibleLines()
    start := m.scrollPos
    end := min(start+visible, len(m.events))

    var lines []string
    for i := start; i < end; i++ {
        e := m.events[i]
        ts := e.Time.Format("15:04:05")
        line := fmt.Sprintf("[%s] %s", ts, e.Text)
        lines = append(lines, e.Style.Render(truncate(line, m.width-4)))
    }

    // Pad with empty lines if needed
    for len(lines) < visible {
        lines = append(lines, "")
    }

    return strings.Join(lines, "\n")
}

func (m model) renderFooter() string {
    keys := []string{
        "[p] pause",
        "[r] resume",
        "[q] quit",
        "[↑↓] scroll",
    }

    return styles.Footer.Render(strings.Join(keys, "  "))
}

func (m model) visibleLines() int {
    // Height minus header (3 lines), dividers (2), footer (1)
    return m.height - 6
}
```

### Styles

```go
package tui

import "github.com/charmbracelet/lipgloss"

var styles = struct {
    Container  lipgloss.Style
    Status     lipgloss.Style
    Cost       lipgloss.Style
    Bead       lipgloss.Style
    Turns      lipgloss.Style
    Divider    lipgloss.Style
    Footer     lipgloss.Style
    Tool       lipgloss.Style
    BeadStatus lipgloss.Style
    Session    lipgloss.Style
    Error      lipgloss.Style
    Spacer     lipgloss.Style
}{
    Container: lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color("240")),

    Status: lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("212")),

    Cost: lipgloss.NewStyle().
        Foreground(lipgloss.Color("220")),

    Bead: lipgloss.NewStyle().
        Foreground(lipgloss.Color("39")),

    Turns: lipgloss.NewStyle().
        Foreground(lipgloss.Color("245")),

    Divider: lipgloss.NewStyle().
        Foreground(lipgloss.Color("240")),

    Footer: lipgloss.NewStyle().
        Foreground(lipgloss.Color("245")),

    Tool: lipgloss.NewStyle().
        Foreground(lipgloss.Color("250")),

    BeadStatus: lipgloss.NewStyle().
        Foreground(lipgloss.Color("114")),

    Session: lipgloss.NewStyle().
        Foreground(lipgloss.Color("177")),

    Error: lipgloss.NewStyle().
        Foreground(lipgloss.Color("196")),

    Spacer: lipgloss.NewStyle(),
}
```

## Options

```go
type Option func(*TUI)

func WithOnPause(fn func()) Option {
    return func(t *TUI) {
        t.onPause = fn
    }
}

func WithOnResume(fn func()) Option {
    return func(t *TUI) {
        t.onResume = fn
    }
}

func WithOnQuit(fn func()) Option {
    return func(t *TUI) {
        t.onQuit = fn
    }
}
```

## Graceful Degradation

If terminal doesn't support TUI (no TTY, too small, etc.):

```go
func (t *TUI) Run() error {
    if !isTerminal() || terminalTooSmall() {
        return t.runSimple()
    }
    return t.runTUI()
}

func isTerminal() bool {
    return term.IsTerminal(int(os.Stdout.Fd()))
}

func terminalTooSmall() bool {
    w, h, err := term.GetSize(int(os.Stdout.Fd()))
    if err != nil {
        return true
    }
    return w < 60 || h < 15
}

func (t *TUI) runSimple() error {
    // Fall back to simple line output
    for event := range t.events {
        line := formatSimple(event)
        if line != "" {
            fmt.Println(line)
        }
    }
    return nil
}
```

## Testing

### Unit Tests

- Model updates: verify state changes
- Event formatting: test all event types
- Scrolling: test bounds checking

### Visual Testing

TUI testing is primarily visual - use golden files or manual testing:

```go
func TestRenderHeader(t *testing.T) {
    m := model{
        width:  80,
        height: 24,
        status: "working",
        currentBead: &BeadInfo{
            ID:    "bd-042",
            Title: "Fix auth bug",
        },
        stats: Stats{
            Completed: 4,
            Failed:    1,
            TotalCost: 2.35,
        },
    }

    output := m.renderHeader()

    if !strings.Contains(output, "WORKING") {
        t.Error("expected WORKING in header")
    }
    if !strings.Contains(output, "$2.35") {
        t.Error("expected cost in header")
    }
}
```

## Error Handling

| Error | Action |
|-------|--------|
| No TTY | Fall back to simple output |
| Terminal too small | Show warning, use simple output |
| Event channel closed | Exit gracefully |
| Render panic | Recover, log error, continue |

## Future Considerations

- **Observer Mode**: Interactive Q&A pane - see [observer.md](observer.md) for design
- **Themes**: User-configurable color schemes
- **Mouse support**: Click to scroll, select events
- **Split panes**: Show logs and events separately
- **Search**: Filter events by pattern
- **Export**: Save visible events to file
