package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/npratt/atari/internal/observer"
)

const (
	// observerInputHeight is the height of the question input textarea.
	observerInputHeight = 3
	// observerTickInterval is the interval for updating elapsed time during queries.
	observerTickInterval = 100 * time.Millisecond
)

// ObserverPane is a TUI component for interactive observer queries.
type ObserverPane struct {
	observer *observer.Observer
	input    textarea.Model
	spinner  spinner.Model
	response string
	loading  bool
	startedAt time.Time
	errorMsg string
	width    int
	height   int
	focused  bool
}

// observerTickMsg signals a tick for updating elapsed time.
type observerTickMsg time.Time

// observerResultMsg carries the result of an observer query.
type observerResultMsg struct {
	response string
	err      error
}

// NewObserverPane creates a new ObserverPane with the given observer.
func NewObserverPane(obs *observer.Observer) ObserverPane {
	// Initialize textarea
	ta := textarea.New()
	ta.Placeholder = "Ask a question about the current session..."
	ta.SetHeight(observerInputHeight)
	ta.CharLimit = 500
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(false) // Enter submits, Shift+Enter for newline

	// Initialize spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return ObserverPane{
		observer: obs,
		input:    ta,
		spinner:  sp,
	}
}

// Init returns initial commands for the observer pane.
func (p ObserverPane) Init() tea.Cmd {
	return textarea.Blink
}

// Update handles messages and returns the updated pane and any commands.
func (p ObserverPane) Update(msg tea.Msg) (ObserverPane, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return p.handleKey(msg)

	case observerTickMsg:
		if p.loading {
			// Update spinner
			var cmd tea.Cmd
			p.spinner, cmd = p.spinner.Update(msg)
			cmds = append(cmds, cmd)
			// Schedule next tick
			cmds = append(cmds, p.tickCmd())
		}
		return p, tea.Batch(cmds...)

	case observerResultMsg:
		p.loading = false
		if msg.err != nil {
			p.errorMsg = msg.err.Error()
			p.response = ""
		} else {
			p.errorMsg = ""
			p.response = msg.response
		}
		return p, nil

	case spinner.TickMsg:
		if p.loading {
			var cmd tea.Cmd
			p.spinner, cmd = p.spinner.Update(msg)
			return p, cmd
		}
		return p, nil

	default:
		// Pass other messages to textarea if focused
		if p.focused && !p.loading {
			var cmd tea.Cmd
			p.input, cmd = p.input.Update(msg)
			cmds = append(cmds, cmd)
		}
		return p, tea.Batch(cmds...)
	}
}

// handleKey processes keyboard input.
func (p ObserverPane) handleKey(msg tea.KeyMsg) (ObserverPane, tea.Cmd) {
	key := msg.String()

	switch key {
	case "enter":
		// Submit question if we have content and not already loading
		if !p.loading && strings.TrimSpace(p.input.Value()) != "" {
			return p.submitQuestion()
		}
		return p, nil

	case "ctrl+c":
		// Cancel in-progress query
		if p.loading && p.observer != nil {
			p.observer.Cancel()
			p.loading = false
			p.errorMsg = "Query cancelled"
		}
		return p, nil

	case "esc":
		// Clear input or error
		if p.errorMsg != "" {
			p.errorMsg = ""
			return p, nil
		}
		if p.input.Value() != "" {
			p.input.Reset()
			return p, nil
		}
		// If nothing to clear, let parent handle (switch focus)
		return p, nil

	default:
		// Pass to textarea if not loading
		if !p.loading {
			var cmd tea.Cmd
			p.input, cmd = p.input.Update(msg)
			return p, cmd
		}
		return p, nil
	}
}

// submitQuestion starts an observer query.
func (p ObserverPane) submitQuestion() (ObserverPane, tea.Cmd) {
	question := strings.TrimSpace(p.input.Value())
	if question == "" {
		return p, nil
	}

	p.loading = true
	p.startedAt = time.Now()
	p.errorMsg = ""
	p.response = ""

	// Start spinner
	cmd := p.spinner.Tick

	// Start the query in background
	queryCmd := p.queryCmd(question)

	// Start tick for elapsed time
	tickCmd := p.tickCmd()

	return p, tea.Batch(cmd, queryCmd, tickCmd)
}

// queryCmd returns a command that executes the observer query.
func (p ObserverPane) queryCmd(question string) tea.Cmd {
	return func() tea.Msg {
		if p.observer == nil {
			return observerResultMsg{err: fmt.Errorf("observer not initialized")}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		response, err := p.observer.Ask(ctx, question)
		return observerResultMsg{response: response, err: err}
	}
}

// tickCmd returns a command that sends a tick message.
func (p ObserverPane) tickCmd() tea.Cmd {
	return tea.Tick(observerTickInterval, func(t time.Time) tea.Msg {
		return observerTickMsg(t)
	})
}

// View renders the observer pane.
func (p ObserverPane) View() string {
	if p.width == 0 || p.height == 0 {
		return ""
	}

	contentWidth := safeWidth(p.width - 4) // Account for padding

	var sections []string

	// Section 1: Input area
	p.input.SetWidth(contentWidth)
	inputSection := p.input.View()
	sections = append(sections, inputSection)

	// Section 2: Status bar
	statusBar := p.renderStatusBar(contentWidth)
	sections = append(sections, statusBar)

	// Section 3: Response area (takes remaining space)
	responseHeight := p.height - observerInputHeight - 3 // input + status + padding
	responseSection := p.renderResponse(contentWidth, responseHeight)
	sections = append(sections, responseSection)

	return strings.Join(sections, "\n")
}

// renderStatusBar renders the status bar with loading state or error.
func (p ObserverPane) renderStatusBar(width int) string {
	if p.loading {
		elapsed := time.Since(p.startedAt).Round(100 * time.Millisecond)
		status := fmt.Sprintf("%s Asking Claude... (%s elapsed)", p.spinner.View(), elapsed)
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Width(width).
			Render(status)
	}

	if p.errorMsg != "" {
		return styles.Error.Width(width).Render("Error: " + truncateString(p.errorMsg, width-7))
	}

	// Show hint when idle
	hint := "Enter: submit | Ctrl+C: cancel | Esc: clear"
	return styles.Footer.Width(width).Render(hint)
}

// renderResponse renders the response area.
func (p ObserverPane) renderResponse(width, height int) string {
	if height < 1 {
		height = 1
	}

	if p.response == "" {
		placeholder := "Response will appear here..."
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Width(width).
			Height(height).
			Render(placeholder)
	}

	// Word wrap and truncate response to fit
	wrapped := wordWrap(p.response, width)
	lines := strings.Split(wrapped, "\n")

	// Take only what fits in the available height
	if len(lines) > height {
		lines = lines[:height]
		lines[height-1] = lines[height-1][:max(0, len(lines[height-1])-3)] + "..."
	}

	// Pad with empty lines if needed
	for len(lines) < height {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// SetSize updates the pane dimensions.
func (p *ObserverPane) SetSize(width, height int) {
	p.width = width
	p.height = height
	p.input.SetWidth(safeWidth(width - 4))
}

// SetFocused updates the focus state.
func (p *ObserverPane) SetFocused(focused bool) {
	p.focused = focused
	if focused {
		p.input.Focus()
	} else {
		p.input.Blur()
	}
}

// IsFocused returns true if the pane is focused.
func (p ObserverPane) IsFocused() bool {
	return p.focused
}

// IsLoading returns true if a query is in progress.
func (p ObserverPane) IsLoading() bool {
	return p.loading
}

// ClearResponse clears the current response and error.
func (p *ObserverPane) ClearResponse() {
	p.response = ""
	p.errorMsg = ""
}

// truncateString truncates a string to maxLen with ellipsis.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// wordWrap wraps text to fit within the given width.
func wordWrap(text string, width int) string {
	if width < 1 {
		width = 1
	}

	var result strings.Builder
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		if i > 0 {
			result.WriteString("\n")
		}

		// Process each line
		for len(line) > width {
			// Find a good break point
			breakAt := width
			for j := width; j > 0; j-- {
				if line[j-1] == ' ' {
					breakAt = j
					break
				}
			}

			result.WriteString(strings.TrimRight(line[:breakAt], " "))
			result.WriteString("\n")
			line = strings.TrimLeft(line[breakAt:], " ")
		}
		result.WriteString(line)
	}

	return result.String()
}
