package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
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

// chatRole represents the role of a chat message.
type chatRole int

const (
	roleUser chatRole = iota
	roleAssistant
)

// chatMessage represents a single message in the chat history.
type chatMessage struct {
	role    chatRole
	content string
	time    time.Time
}

// ObserverPane is a TUI component for interactive observer queries.
type ObserverPane struct {
	observer   *observer.Observer
	input      textarea.Model
	viewport   viewport.Model
	spinner    spinner.Model
	history    []chatMessage
	loading    bool
	startedAt  time.Time
	errorMsg   string
	width      int
	height     int
	focused    bool
	insertMode bool // vim-style insert mode for text input
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

	// Initialize viewport for chat history
	vp := viewport.New(40, 10) // Will be resized later
	vp.Style = lipgloss.NewStyle()

	return ObserverPane{
		observer: obs,
		input:    ta,
		viewport: vp,
		spinner:  sp,
		history:  make([]chatMessage, 0),
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
		} else {
			p.errorMsg = ""
			// Calculate where the new response will start
			startLine := p.calculateHistoryLines(len(p.history))
			// Add assistant response to history
			p.history = append(p.history, chatMessage{
				role:    roleAssistant,
				content: msg.response,
				time:    time.Now(),
			})
			// Update viewport content and scroll to show response start
			p.updateViewportContent()
			p.scrollToLine(startLine)
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
		// Pass other messages to textarea if focused and not loading
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

	// Handle insert mode: most keys go to textarea
	if p.insertMode {
		return p.handleInsertModeKey(msg, key)
	}

	// Normal mode: navigation and commands
	return p.handleNormalModeKey(msg, key)
}

// handleInsertModeKey processes keys when in insert mode.
func (p ObserverPane) handleInsertModeKey(msg tea.KeyMsg, key string) (ObserverPane, tea.Cmd) {
	switch key {
	case "esc":
		// Exit insert mode
		p.insertMode = false
		p.input.Blur()
		return p, nil

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

	default:
		// Pass all other keys to textarea for typing
		if !p.loading {
			var cmd tea.Cmd
			p.input, cmd = p.input.Update(msg)
			return p, cmd
		}
		return p, nil
	}
}

// handleNormalModeKey processes keys when in normal mode.
func (p ObserverPane) handleNormalModeKey(msg tea.KeyMsg, key string) (ObserverPane, tea.Cmd) {
	switch key {
	case "q":
		// Quit application
		return p, tea.Quit

	case "i":
		// Enter insert mode
		p.insertMode = true
		p.input.Focus()
		return p, nil

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
		// Clear error first
		if p.errorMsg != "" {
			p.errorMsg = ""
			return p, nil
		}
		// Then clear input
		if p.input.Value() != "" {
			p.input.Reset()
			return p, nil
		}
		// If nothing to clear, unfocus to signal parent should close
		p.focused = false
		return p, nil

	case "up", "k":
		// Scroll up in history
		p.viewport.ScrollUp(1)
		return p, nil

	case "down", "j":
		// Scroll down in history
		p.viewport.ScrollDown(1)
		return p, nil

	case "pgup":
		p.viewport.HalfPageUp()
		return p, nil

	case "pgdown":
		p.viewport.HalfPageDown()
		return p, nil

	case "home", "g":
		p.viewport.GotoTop()
		return p, nil

	case "end", "G":
		p.viewport.GotoBottom()
		return p, nil

	default:
		// In normal mode, don't pass letter keys to textarea
		return p, nil
	}
}

// submitQuestion starts an observer query.
func (p ObserverPane) submitQuestion() (ObserverPane, tea.Cmd) {
	question := strings.TrimSpace(p.input.Value())
	if question == "" {
		return p, nil
	}

	// Calculate where the new question will start
	startLine := p.calculateHistoryLines(len(p.history))

	// Add user question to history
	p.history = append(p.history, chatMessage{
		role:    roleUser,
		content: question,
		time:    time.Now(),
	})

	// Clear input
	p.input.Reset()

	// Update viewport and scroll to show the submitted question
	p.updateViewportContent()
	p.scrollToLine(startLine)

	p.loading = true
	p.startedAt = time.Now()
	p.errorMsg = ""

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

// updateViewportContent rebuilds the viewport content from history.
func (p *ObserverPane) updateViewportContent() {
	contentWidth := safeWidth(p.width - 6) // Account for padding and role prefix
	if contentWidth < 10 {
		contentWidth = 10
	}

	var lines []string

	for _, msg := range p.history {
		// Add role prefix with styling
		var prefix string
		var style lipgloss.Style

		if msg.role == roleUser {
			prefix = "You: "
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
		} else {
			prefix = "Claude: "
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
		}

		// Wrap content to fit width
		wrapped := wordWrap(msg.content, contentWidth-len(prefix))
		wrappedLines := strings.Split(wrapped, "\n")

		// First line gets the prefix
		if len(wrappedLines) > 0 {
			lines = append(lines, style.Render(prefix)+wrappedLines[0])
		}

		// Subsequent lines are indented
		indent := strings.Repeat(" ", len(prefix))
		for i := 1; i < len(wrappedLines); i++ {
			lines = append(lines, indent+wrappedLines[i])
		}

		// Add blank line between messages
		lines = append(lines, "")
	}

	p.viewport.SetContent(strings.Join(lines, "\n"))
}

// calculateHistoryLines returns the number of rendered lines for the first n messages.
// This is used to determine where new content will start in the viewport.
func (p *ObserverPane) calculateHistoryLines(n int) int {
	if n <= 0 || n > len(p.history) {
		return 0
	}

	contentWidth := safeWidth(p.width - 6)
	if contentWidth < 10 {
		contentWidth = 10
	}

	lineCount := 0
	for i := 0; i < n; i++ {
		msg := p.history[i]
		var prefix string
		if msg.role == roleUser {
			prefix = "You: "
		} else {
			prefix = "Claude: "
		}

		wrapped := wordWrap(msg.content, contentWidth-len(prefix))
		wrappedLines := strings.Split(wrapped, "\n")
		lineCount += len(wrappedLines)
		lineCount++ // Blank line between messages
	}

	return lineCount
}

// scrollToLine scrolls the viewport to show the given line at the top,
// but tries to show some context (the triggering question) if space permits.
func (p *ObserverPane) scrollToLine(targetLine int) {
	totalLines := p.viewport.TotalLineCount()
	viewportHeight := p.viewport.Height

	// If all content fits in viewport, just go to top
	if totalLines <= viewportHeight {
		p.viewport.GotoTop()
		return
	}

	// Try to show one message of context above the target if it fits
	// Find where the previous message starts (if any)
	contextLine := targetLine
	if len(p.history) > 1 && targetLine > 0 {
		// Try to include the previous message for context
		prevMsgLines := p.calculateHistoryLines(len(p.history) - 1)
		if targetLine-prevMsgLines >= 0 {
			// Check if we can fit the previous message + current content in viewport
			contentFromPrev := totalLines - prevMsgLines
			if contentFromPrev <= viewportHeight {
				contextLine = prevMsgLines
			}
		}
	}

	// Calculate offset - we want targetLine (or contextLine) at top of viewport
	offset := contextLine
	maxOffset := totalLines - viewportHeight
	if offset > maxOffset {
		offset = maxOffset
	}
	if offset < 0 {
		offset = 0
	}

	p.viewport.SetYOffset(offset)
}

// View renders the observer pane.
func (p ObserverPane) View() string {
	if p.width == 0 || p.height == 0 {
		return ""
	}

	contentWidth := safeWidth(p.width - 4) // Account for padding

	var sections []string

	// Section 1: Chat history (takes most of the space)
	historyHeight := p.height - observerInputHeight - 3 // input + status + padding
	if historyHeight < 1 {
		historyHeight = 1
	}

	if len(p.history) == 0 {
		placeholder := "Ask questions about the current drain session.\nConversation history will appear here."
		historySection := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Width(contentWidth).
			Height(historyHeight).
			Render(placeholder)
		sections = append(sections, historySection)
	} else {
		sections = append(sections, p.viewport.View())
	}

	// Section 2: Status bar
	statusBar := p.renderStatusBar(contentWidth)
	sections = append(sections, statusBar)

	// Section 3: Input area (at the bottom)
	p.input.SetWidth(contentWidth)
	inputSection := p.input.View()
	sections = append(sections, inputSection)

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

	// Show mode indicator and hints
	msgCount := len(p.history)
	var hint string
	if p.insertMode {
		modeIndicator := lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true).
			Render("[INSERT]")
		hint = fmt.Sprintf("%s %d messages | Enter: send | Esc: normal mode", modeIndicator, msgCount)
	} else {
		modeIndicator := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render("[NORMAL]")
		hint = fmt.Sprintf("%s %d messages | i: insert | j/k: scroll | Esc: close", modeIndicator, msgCount)
	}
	return styles.Footer.Width(width).Render(hint)
}

// SetSize updates the pane dimensions.
func (p *ObserverPane) SetSize(width, height int) {
	p.width = width
	p.height = height
	p.input.SetWidth(safeWidth(width - 4))

	// Update viewport size
	historyHeight := height - observerInputHeight - 3
	if historyHeight < 1 {
		historyHeight = 1
	}
	p.viewport.Width = safeWidth(width - 4)
	p.viewport.Height = historyHeight

	// Rebuild content with new width
	p.updateViewportContent()
}

// SetFocused updates the focus state.
func (p *ObserverPane) SetFocused(focused bool) {
	p.focused = focused
	if !focused {
		// Exit insert mode when unfocused
		p.insertMode = false
		p.input.Blur()
	}
	// Note: when focused, user must press 'i' to enter insert mode
}

// IsFocused returns true if the pane is focused.
func (p ObserverPane) IsFocused() bool {
	return p.focused
}

// IsInsertMode returns true if the pane is in insert mode.
func (p ObserverPane) IsInsertMode() bool {
	return p.insertMode
}

// IsLoading returns true if a query is in progress.
func (p ObserverPane) IsLoading() bool {
	return p.loading
}

// ClearResponse clears the current response and error.
func (p *ObserverPane) ClearResponse() {
	p.history = nil
	p.errorMsg = ""
	p.updateViewportContent()
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
