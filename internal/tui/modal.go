package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DetailModal displays full bead information in a modal overlay.
type DetailModal struct {
	node      *GraphNode  // Summary data from graph
	fullBead  *GraphBead  // Full data from bd show
	fetcher   BeadFetcher // For fetching full details
	loading   bool
	errorMsg  string
	width     int
	height    int
	scrollPos int
	open      bool
	requestID int // For staleness detection
}

// modalFetchResultMsg carries the result of a bead fetch operation.
type modalFetchResultMsg struct {
	bead      *GraphBead
	err       error
	requestID int
}

// NewDetailModal creates a new DetailModal.
func NewDetailModal(fetcher BeadFetcher) *DetailModal {
	return &DetailModal{
		fetcher: fetcher,
	}
}

// Open opens the modal with the given node and starts fetching full details.
func (m *DetailModal) Open(node *GraphNode) tea.Cmd {
	m.open = true
	m.node = node
	m.fullBead = nil
	m.loading = true
	m.errorMsg = ""
	m.scrollPos = 0
	m.requestID++

	if m.fetcher == nil || node == nil {
		m.loading = false
		return nil
	}

	reqID := m.requestID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		bead, err := m.fetcher.FetchBead(ctx, node.ID)
		return modalFetchResultMsg{bead: bead, err: err, requestID: reqID}
	}
}

// Close closes the modal.
func (m *DetailModal) Close() {
	m.open = false
	m.node = nil
	m.fullBead = nil
	m.loading = false
	m.errorMsg = ""
	m.scrollPos = 0
}

// IsOpen returns true if the modal is open.
func (m *DetailModal) IsOpen() bool {
	return m.open
}

// SetSize updates the modal dimensions.
func (m *DetailModal) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Update handles messages for the modal.
func (m *DetailModal) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case modalFetchResultMsg:
		// Drop stale results
		if msg.requestID != m.requestID {
			return nil
		}
		m.loading = false
		if msg.err != nil {
			m.errorMsg = msg.err.Error()
		} else {
			m.errorMsg = ""
			m.fullBead = msg.bead
		}
		return nil

	case spinner.TickMsg:
		// Ignore spinner ticks - we don't use a spinner in modal
		return nil
	}

	return nil
}

// handleKey processes keyboard input for the modal.
func (m *DetailModal) handleKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	switch key {
	case "esc", "enter", "q":
		m.Close()
		return nil

	case "up", "k":
		if m.scrollPos > 0 {
			m.scrollPos--
		}
		return nil

	case "down", "j":
		// Scroll down (capped in View based on content height)
		m.scrollPos++
		return nil

	case "home", "g":
		m.scrollPos = 0
		return nil

	case "end", "G":
		// Set to large number, will be capped in View
		m.scrollPos = 9999
		return nil
	}

	return nil
}

// View renders the modal.
func (m *DetailModal) View(parentWidth, parentHeight int) string {
	if !m.open || m.node == nil {
		return ""
	}

	// Calculate modal size (~90% of parent)
	modalWidth := parentWidth * 90 / 100
	modalHeight := parentHeight * 90 / 100
	if modalWidth < 40 {
		modalWidth = 40
	}
	if modalHeight < 10 {
		modalHeight = 10
	}

	// Build content
	var content strings.Builder

	// Title bar
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Width(modalWidth - 4)

	content.WriteString(titleStyle.Render(m.node.ID))
	content.WriteString("\n")

	// Status | Priority | Type row
	metaStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))

	statusText := fmt.Sprintf("Status: %s | Priority: %d | Type: %s",
		m.node.Status, m.node.Priority, m.node.Type)
	content.WriteString(metaStyle.Render(statusText))
	content.WriteString("\n\n")

	// Loading or error state
	if m.loading {
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Italic(true)
		content.WriteString(loadingStyle.Render("Loading full details..."))
		content.WriteString("\n")
	} else if m.errorMsg != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))
		content.WriteString(errorStyle.Render("Error: " + m.errorMsg))
		content.WriteString("\n")
	}

	// Full bead details if loaded
	if m.fullBead != nil {
		content.WriteString(m.renderFullDetails(modalWidth - 4))
	} else if !m.loading && m.errorMsg == "" {
		// Show summary from node if no full bead
		content.WriteString(m.renderSummary(modalWidth - 4))
	}

	// Build the full content
	fullContent := content.String()
	lines := strings.Split(fullContent, "\n")

	// Calculate visible area (reserve 3 lines for border/footer)
	visibleHeight := modalHeight - 4
	if visibleHeight < 3 {
		visibleHeight = 3
	}

	// Cap scroll position
	maxScroll := len(lines) - visibleHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.scrollPos > maxScroll {
		m.scrollPos = maxScroll
	}

	// Extract visible lines
	startLine := m.scrollPos
	endLine := startLine + visibleHeight
	if endLine > len(lines) {
		endLine = len(lines)
	}

	visibleContent := strings.Join(lines[startLine:endLine], "\n")

	// Footer with key hints
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Italic(true)

	scrollInfo := ""
	if maxScroll > 0 {
		scrollInfo = fmt.Sprintf(" | Line %d/%d", m.scrollPos+1, len(lines))
	}
	footer := footerStyle.Render("[Enter/Esc] close | [j/k] scroll" + scrollInfo)

	// Build modal box
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(1, 2).
		Width(modalWidth).
		Height(modalHeight)

	modalContent := visibleContent + "\n\n" + footer

	return modalStyle.Render(modalContent)
}

// renderFullDetails renders the full bead details.
func (m *DetailModal) renderFullDetails(width int) string {
	if m.fullBead == nil {
		return ""
	}

	var sb strings.Builder

	// Title section
	titleStyle := lipgloss.NewStyle().Bold(true)
	sb.WriteString(titleStyle.Render("Title:"))
	sb.WriteString("\n")
	sb.WriteString(wordWrap(m.fullBead.Title, width))
	sb.WriteString("\n\n")

	// Description section
	if m.fullBead.Description != "" {
		sb.WriteString(titleStyle.Render("Description:"))
		sb.WriteString("\n")
		// Clean ANSI codes from description
		cleanDesc := stripANSI(m.fullBead.Description)
		sb.WriteString(wordWrap(cleanDesc, width))
		sb.WriteString("\n\n")
	}

	// Dependencies section
	if len(m.fullBead.Dependencies) > 0 {
		sb.WriteString(titleStyle.Render("Dependencies:"))
		sb.WriteString("\n")
		for _, dep := range m.fullBead.Dependencies {
			depLine := fmt.Sprintf("  - %s: %s (%s)", dep.ID, truncateString(dep.Title, 30), dep.Status)
			sb.WriteString(depLine)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Dependents section
	if len(m.fullBead.Dependents) > 0 {
		sb.WriteString(titleStyle.Render("Dependents:"))
		sb.WriteString("\n")
		for _, dep := range m.fullBead.Dependents {
			depLine := fmt.Sprintf("  - %s: %s (%s)", dep.ID, truncateString(dep.Title, 30), dep.Status)
			sb.WriteString(depLine)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Notes section
	if m.fullBead.Notes != "" {
		sb.WriteString(titleStyle.Render("Notes:"))
		sb.WriteString("\n")
		cleanNotes := stripANSI(m.fullBead.Notes)
		sb.WriteString(wordWrap(cleanNotes, width))
		sb.WriteString("\n\n")
	}

	// Metadata
	metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	sb.WriteString(metaStyle.Render(fmt.Sprintf("Created: %s by %s", m.fullBead.CreatedAt, m.fullBead.CreatedBy)))
	sb.WriteString("\n")
	if m.fullBead.UpdatedAt != "" {
		sb.WriteString(metaStyle.Render(fmt.Sprintf("Updated: %s", m.fullBead.UpdatedAt)))
		sb.WriteString("\n")
	}
	if m.fullBead.ClosedAt != "" {
		sb.WriteString(metaStyle.Render(fmt.Sprintf("Closed: %s", m.fullBead.ClosedAt)))
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderSummary renders summary info when full details aren't available.
func (m *DetailModal) renderSummary(width int) string {
	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true)
	sb.WriteString(titleStyle.Render("Title:"))
	sb.WriteString("\n")
	sb.WriteString(wordWrap(m.node.Title, width))
	sb.WriteString("\n\n")

	metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	sb.WriteString(metaStyle.Render("(Full details not available)"))
	sb.WriteString("\n")

	return sb.String()
}
