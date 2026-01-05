package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/npratt/atari/internal/config"
)

const (
	// graphTickInterval is the interval for updating elapsed time during refresh.
	graphTickInterval = 100 * time.Millisecond
	// minAutoRefreshInterval is the minimum allowed auto-refresh interval.
	minAutoRefreshInterval = 1 * time.Second
)

// GraphPane is a TUI component for bead graph visualization.
type GraphPane struct {
	graph     *Graph
	fetcher   BeadFetcher
	cfg       *config.GraphConfig
	layout    string // "horizontal" or "vertical"
	spinner   spinner.Model
	loading   bool
	startedAt time.Time
	errorMsg  string
	width     int
	height    int
	focused   bool
	visible   bool // Whether the pane is visible (for auto-refresh)
	requestID int  // For staleness detection

	// Inline detail view state (shown before full-screen modal)
	showingDetail   bool        // Whether inline detail view is active
	detailNode      *GraphNode  // Node summary being displayed
	detailBead      *GraphBead  // Full bead data (loaded async)
	detailLoading   bool        // Whether detail fetch is in progress
	detailError     string      // Error from detail fetch
	detailRequestID int         // For staleness detection of detail fetches
	detailScrollPos int         // Scroll position within detail view
}

// graphTickMsg signals a tick for updating elapsed time during refresh.
type graphTickMsg time.Time

// graphResultMsg carries the result of a bead fetch operation.
type graphResultMsg struct {
	beads     []GraphBead
	err       error
	requestID int
}

// graphAutoRefreshMsg signals that auto-refresh interval has elapsed.
type graphAutoRefreshMsg struct{}

// graphDetailResultMsg carries the result of a bead detail fetch operation.
type graphDetailResultMsg struct {
	bead      *GraphBead
	err       error
	requestID int
}

// NewGraphPane creates a new GraphPane with the given configuration.
func NewGraphPane(cfg *config.GraphConfig, fetcher BeadFetcher, layout string) GraphPane {
	// Initialize spinner
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// Create the Graph data structure
	graph := NewGraph(cfg, fetcher, layout)

	return GraphPane{
		graph:   graph,
		fetcher: fetcher,
		cfg:     cfg,
		layout:  layout,
		spinner: sp,
	}
}

// Init returns initial commands for the graph pane.
func (p GraphPane) Init() tea.Cmd {
	// Start an initial refresh to load data
	cmds := []tea.Cmd{p.refreshCmd()}

	// Start auto-refresh ticker if configured
	if cmd := p.autoRefreshCmd(); cmd != nil {
		cmds = append(cmds, cmd)
	}

	return tea.Batch(cmds...)
}

// Update handles messages and returns the updated pane and any commands.
func (p GraphPane) Update(msg tea.Msg) (GraphPane, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if p.focused {
			return p.handleKey(msg)
		}
		return p, nil

	case graphTickMsg:
		if p.loading {
			// Update spinner
			var cmd tea.Cmd
			p.spinner, cmd = p.spinner.Update(msg)
			cmds = append(cmds, cmd)
			// Schedule next tick
			cmds = append(cmds, p.tickCmd())
		}
		return p, tea.Batch(cmds...)

	case graphStartLoadingMsg:
		// Track this request and start loading
		p.requestID = msg.requestID
		p.loading = true
		p.startedAt = time.Now()
		// If startFetch is set, start the actual fetch now that requestID is set.
		// This ensures the staleness check in graphResultMsg works correctly.
		if msg.startFetch {
			return p, tea.Batch(p.fetchCmd(msg.requestID), p.tickCmd())
		}
		return p, nil

	case graphResultMsg:
		// Drop stale results
		if msg.requestID != p.requestID {
			return p, nil
		}
		p.loading = false
		if msg.err != nil {
			p.errorMsg = msg.err.Error()
		} else {
			p.errorMsg = ""
			// Rebuild the graph with new data
			p.rebuildGraph(msg.beads)
		}
		return p, nil

	case spinner.TickMsg:
		if p.loading {
			var cmd tea.Cmd
			p.spinner, cmd = p.spinner.Update(msg)
			return p, cmd
		}
		return p, nil

	case graphAutoRefreshMsg:
		// Auto-refresh: trigger refresh and schedule next tick
		var cmds []tea.Cmd
		// Only refresh if visible and not already loading
		if p.visible {
			if cmd := p.refreshCmd(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		// Always schedule next auto-refresh tick
		if cmd := p.autoRefreshCmd(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return p, tea.Batch(cmds...)

	case graphDetailResultMsg:
		// Drop stale results
		if msg.requestID != p.detailRequestID {
			return p, nil
		}
		p.detailLoading = false
		if msg.err != nil {
			p.detailError = msg.err.Error()
		} else {
			p.detailError = ""
			p.detailBead = msg.bead
		}
		return p, nil

	default:
		return p, nil
	}
}

// handleKey processes keyboard input when focused.
func (p GraphPane) handleKey(msg tea.KeyMsg) (GraphPane, tea.Cmd) {
	key := msg.String()

	// When showing detail view, handle scrolling separately
	if p.showingDetail {
		switch key {
		case "up", "k":
			if p.detailScrollPos > 0 {
				p.detailScrollPos--
			}
			return p, nil
		case "down", "j":
			// Scroll down - cap is handled in renderDetailView
			p.detailScrollPos++
			return p, nil
		case "home", "g":
			p.detailScrollPos = 0
			return p, nil
		case "end", "G":
			p.detailScrollPos = 9999 // Will be capped in renderDetailView
			return p, nil
		}
		// Fall through to enter/esc handling below
	}

	switch key {
	case "up", "k":
		// Navigate to parent (in hierarchy) or previous sibling (in layer)
		if p.graph != nil {
			p.graph.SelectParent()
		}
		return p, nil

	case "down", "j":
		// Navigate to first child (in hierarchy) or next sibling (in layer)
		if p.graph != nil {
			p.graph.SelectChild()
		}
		return p, nil

	case "left", "h":
		// Navigate to previous sibling in current layer
		if p.graph != nil {
			p.graph.SelectPrev()
		}
		return p, nil

	case "right", "l":
		// Navigate to next sibling in current layer
		if p.graph != nil {
			p.graph.SelectNext()
		}
		return p, nil

	case "a":
		// Cycle through Active/Backlog/Closed views
		if p.graph != nil {
			switch p.graph.GetView() {
			case ViewActive:
				p.graph.SetView(ViewBacklog)
			case ViewBacklog:
				p.graph.SetView(ViewClosed)
			case ViewClosed:
				p.graph.SetView(ViewActive)
			default:
				p.graph.SetView(ViewActive)
			}
			return p, p.refreshCmd()
		}
		return p, nil

	case "c":
		// Collapse/expand selected epic
		if p.graph != nil {
			if selected := p.graph.GetSelectedID(); selected != "" {
				p.graph.ToggleCollapse(selected)
			}
		}
		return p, nil

	case "d":
		// Cycle density level
		if p.graph != nil {
			p.graph.CycleDensity()
		}
		return p, nil

	case "R":
		// Manual refresh
		return p, p.refreshCmd()

	case "enter":
		// Two-step selection:
		// 1. First Enter: show inline detail view
		// 2. Second Enter (while showing detail): open full-screen modal
		if p.showingDetail {
			// Already showing detail, open full-screen modal
			return p, func() tea.Msg {
				return GraphOpenModalMsg{NodeID: p.detailNode.ID}
			}
		}
		// Not showing detail yet, open inline detail view
		if p.graph != nil {
			node := p.graph.GetSelected()
			if node != nil {
				return p.openDetailView(node)
			}
		}
		return p, nil

	case "esc":
		// If showing detail, close it and return to graph
		if p.showingDetail {
			p.closeDetailView()
			return p, nil
		}
		// If there's an error, clear it
		if p.errorMsg != "" {
			p.errorMsg = ""
			return p, nil
		}
		// Otherwise unfocus to signal parent should close/unfocus
		p.focused = false
		return p, nil

	default:
		return p, nil
	}
}

// GraphOpenModalMsg is emitted when Enter is pressed to open detail modal.
type GraphOpenModalMsg struct {
	NodeID string
}

// refreshCmd returns a command that fetches bead data.
func (p GraphPane) refreshCmd() tea.Cmd {
	// If already loading, don't start another request
	if p.loading {
		return nil
	}

	// Generate new requestID (will be tracked when startLoadingMsg is handled)
	reqID := p.requestID + 1

	// Return only graphStartLoadingMsg first. The fetch will be started
	// after this message is processed, ensuring requestID is set before
	// any results arrive. This fixes a race condition where fast fetches
	// would complete before requestID was updated, causing results to be
	// dropped as "stale".
	return tea.Batch(
		p.spinner.Tick,
		func() tea.Msg {
			return graphStartLoadingMsg{requestID: reqID, startFetch: true}
		},
	)
}

// graphStartLoadingMsg signals the start of a loading operation.
type graphStartLoadingMsg struct {
	requestID  int
	startFetch bool // when true, start the fetch after processing this message
}

// fetchCmd returns a command that fetches bead data in the background.
func (p GraphPane) fetchCmd(requestID int) tea.Cmd {
	return func() tea.Msg {
		if p.fetcher == nil {
			return graphResultMsg{err: nil, requestID: requestID}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var beads []GraphBead
		var err error

		view := ViewActive
		if p.graph != nil {
			view = p.graph.GetView()
		}

		switch view {
		case ViewActive:
			beads, err = p.fetcher.FetchActive(ctx)
		case ViewBacklog:
			beads, err = p.fetcher.FetchBacklog(ctx)
		case ViewClosed:
			beads, err = p.fetcher.FetchClosed(ctx)
		default:
			beads, err = p.fetcher.FetchActive(ctx)
		}

		return graphResultMsg{beads: beads, err: err, requestID: requestID}
	}
}

// tickCmd returns a command that sends a tick message.
func (p GraphPane) tickCmd() tea.Cmd {
	return tea.Tick(graphTickInterval, func(t time.Time) tea.Msg {
		return graphTickMsg(t)
	})
}

// autoRefreshCmd returns a command that schedules an auto-refresh.
// Returns nil if auto-refresh is disabled.
func (p GraphPane) autoRefreshCmd() tea.Cmd {
	if p.cfg == nil || p.cfg.AutoRefreshInterval <= 0 {
		return nil
	}

	interval := p.cfg.AutoRefreshInterval
	// Enforce minimum interval
	if interval < minAutoRefreshInterval {
		interval = minAutoRefreshInterval
	}

	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return graphAutoRefreshMsg{}
	})
}

// rebuildGraph rebuilds the graph with new bead data.
func (p *GraphPane) rebuildGraph(beads []GraphBead) {
	if p.graph == nil {
		p.graph = NewGraph(p.cfg, p.fetcher, p.layout)
	}
	p.graph.RebuildFromBeads(beads)
}

// View renders the graph pane.
func (p GraphPane) View() string {
	if p.width == 0 || p.height == 0 {
		return ""
	}

	contentWidth := safeWidth(p.width - 4)    // Account for padding
	contentHeight := safeHeight(p.height - 3) // Account for status bar and padding

	// If showing inline detail view, render that instead of the graph
	if p.showingDetail {
		return p.renderDetailView(contentWidth, contentHeight)
	}

	var sections []string

	// Section 1: Status bar
	statusBar := p.renderStatusBar(contentWidth)
	sections = append(sections, statusBar)

	// Section 2: Graph area (takes remaining space)
	graphSection := p.renderGraph(contentWidth, contentHeight)
	sections = append(sections, graphSection)

	return strings.Join(sections, "\n")
}

// renderStatusBar renders the status bar with view, density, and loading state.
func (p GraphPane) renderStatusBar(width int) string {
	if p.errorMsg != "" {
		return styles.Error.Width(width).Render("Error: " + truncateString(p.errorMsg, width-7))
	}

	// Show view and density info
	var viewStr string
	if p.graph != nil {
		viewStr = p.graph.GetView().String()
	} else {
		viewStr = "active"
	}

	var densityStr string
	if p.graph != nil {
		densityStr = p.graph.GetDensity().String()
	} else {
		densityStr = "standard"
	}

	nodeCount := 0
	if p.graph != nil {
		nodeCount = p.graph.NodeCount()
	}

	info := viewStr + " | " + densityStr + " | " + pluralize(nodeCount, "node", "nodes")
	hint := " | a:view d:density R:refresh c:collapse"

	// Add loading indicator at the very end if refreshing
	loadingIndicator := ""
	if p.loading {
		loadingIndicator = " | " + p.spinner.View() + " refreshing"
	}

	if len(info)+len(hint)+len(loadingIndicator) <= width {
		info += hint + loadingIndicator
	} else if len(info)+len(loadingIndicator) <= width {
		info += loadingIndicator
	}

	return styles.Footer.Width(width).Render(info)
}

// renderGraph renders the graph visualization area.
func (p GraphPane) renderGraph(width, height int) string {
	if height < 1 {
		height = 1
	}

	if p.graph == nil || p.graph.NodeCount() == 0 {
		placeholder := "No beads to display. Press R to refresh."
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Width(width).
			Height(height).
			Render(placeholder)
	}

	// Update graph viewport and render
	p.graph.SetViewport(width, height)
	return p.graph.Render(width, height)
}

// SetSize updates the pane dimensions.
func (p *GraphPane) SetSize(width, height int) {
	p.width = width
	p.height = height
	if p.graph != nil {
		p.graph.SetViewport(safeWidth(width-4), safeHeight(height-3))
	}
}

// SetFocused updates the focus state.
func (p *GraphPane) SetFocused(focused bool) {
	p.focused = focused
}

// IsFocused returns true if the pane is focused.
func (p GraphPane) IsFocused() bool {
	return p.focused
}

// SetVisible updates the visibility state (used for auto-refresh).
func (p *GraphPane) SetVisible(visible bool) {
	p.visible = visible
}

// IsLoading returns true if a fetch is in progress.
func (p GraphPane) IsLoading() bool {
	return p.loading
}

// SetCurrentBead sets the currently processing bead for highlighting.
func (p *GraphPane) SetCurrentBead(beadID string) {
	if p.graph != nil {
		p.graph.SetCurrentBead(beadID)
	}
}

// GetSelectedNode returns the currently selected node, or nil if none.
func (p GraphPane) GetSelectedNode() *GraphNode {
	if p.graph != nil {
		return p.graph.GetSelected()
	}
	return nil
}

// Refresh triggers an async refresh of the graph data.
func (p *GraphPane) Refresh() tea.Cmd {
	return p.refreshCmd()
}

// IsShowingDetail returns true if inline detail view is active.
func (p GraphPane) IsShowingDetail() bool {
	return p.showingDetail
}

// openDetailView opens the inline detail view for a node and starts fetching full details.
func (p GraphPane) openDetailView(node *GraphNode) (GraphPane, tea.Cmd) {
	p.showingDetail = true
	p.detailNode = node
	p.detailBead = nil
	p.detailLoading = true
	p.detailError = ""
	p.detailScrollPos = 0
	p.detailRequestID++

	if p.fetcher == nil || node == nil {
		p.detailLoading = false
		return p, nil
	}

	reqID := p.detailRequestID
	return p, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		bead, err := p.fetcher.FetchBead(ctx, node.ID)
		return graphDetailResultMsg{bead: bead, err: err, requestID: reqID}
	}
}

// closeDetailView closes the inline detail view and returns to graph.
func (p *GraphPane) closeDetailView() {
	p.showingDetail = false
	p.detailNode = nil
	p.detailBead = nil
	p.detailLoading = false
	p.detailError = ""
	p.detailScrollPos = 0
}

// renderDetailView renders the inline bead detail view.
func (p GraphPane) renderDetailView(width, height int) string {
	if p.detailNode == nil {
		return ""
	}

	var content strings.Builder

	// Header with bead ID
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205"))
	content.WriteString(titleStyle.Render(p.detailNode.ID))
	content.WriteString("\n")

	// Status | Priority | Type row
	metaStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245"))
	statusText := fmt.Sprintf("Status: %s | Priority: %d | Type: %s",
		p.detailNode.Status, p.detailNode.Priority, p.detailNode.Type)
	content.WriteString(metaStyle.Render(statusText))
	content.WriteString("\n\n")

	// Loading or error state
	if p.detailLoading {
		loadingStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Italic(true)
		content.WriteString(loadingStyle.Render("Loading details..."))
		content.WriteString("\n")
	} else if p.detailError != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))
		content.WriteString(errorStyle.Render("Error: " + p.detailError))
		content.WriteString("\n")
	}

	// Full bead details if loaded
	if p.detailBead != nil {
		content.WriteString(p.renderDetailContent(width))
	} else if !p.detailLoading && p.detailError == "" {
		// Show summary from node if no full bead
		sectionStyle := lipgloss.NewStyle().Bold(true)
		content.WriteString(sectionStyle.Render("Title:"))
		content.WriteString("\n")
		content.WriteString(wordWrap(p.detailNode.Title, width))
		content.WriteString("\n\n")
		content.WriteString(metaStyle.Render("(Full details not available)"))
		content.WriteString("\n")
	}

	// Build the full content
	fullContent := content.String()
	lines := strings.Split(fullContent, "\n")

	// Calculate visible area (reserve 2 lines for footer)
	visibleHeight := height - 2
	if visibleHeight < 3 {
		visibleHeight = 3
	}

	// Cap scroll position
	maxScroll := len(lines) - visibleHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if p.detailScrollPos > maxScroll {
		p.detailScrollPos = maxScroll
	}

	// Extract visible lines
	startLine := p.detailScrollPos
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
		scrollInfo = fmt.Sprintf(" | Line %d/%d", p.detailScrollPos+1, len(lines))
	}
	footer := footerStyle.Render("[Enter] fullscreen | [Esc] back | [j/k] scroll" + scrollInfo)

	return visibleContent + "\n\n" + footer
}

// renderDetailContent renders the full bead details for inline view.
func (p GraphPane) renderDetailContent(width int) string {
	if p.detailBead == nil {
		return ""
	}

	var sb strings.Builder

	sectionStyle := lipgloss.NewStyle().Bold(true)

	// Title section
	sb.WriteString(sectionStyle.Render("Title:"))
	sb.WriteString("\n")
	sb.WriteString(wordWrap(p.detailBead.Title, width))
	sb.WriteString("\n\n")

	// Description section
	if p.detailBead.Description != "" {
		sb.WriteString(sectionStyle.Render("Description:"))
		sb.WriteString("\n")
		// Clean ANSI codes from description
		cleanDesc := stripANSI(p.detailBead.Description)
		sb.WriteString(wordWrap(cleanDesc, width))
		sb.WriteString("\n\n")
	}

	// Dependencies section
	if len(p.detailBead.Dependencies) > 0 {
		sb.WriteString(sectionStyle.Render("Dependencies:"))
		sb.WriteString("\n")
		for _, dep := range p.detailBead.Dependencies {
			depLine := "  - " + dep.ID + ": " + truncateString(dep.Title, 30) + " (" + dep.Status + ")"
			sb.WriteString(depLine)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Dependents section
	if len(p.detailBead.Dependents) > 0 {
		sb.WriteString(sectionStyle.Render("Dependents:"))
		sb.WriteString("\n")
		for _, dep := range p.detailBead.Dependents {
			depLine := "  - " + dep.ID + ": " + truncateString(dep.Title, 30) + " (" + dep.Status + ")"
			sb.WriteString(depLine)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Notes section
	if p.detailBead.Notes != "" {
		sb.WriteString(sectionStyle.Render("Notes:"))
		sb.WriteString("\n")
		cleanNotes := stripANSI(p.detailBead.Notes)
		sb.WriteString(wordWrap(cleanNotes, width))
		sb.WriteString("\n\n")
	}

	// Metadata
	metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	sb.WriteString(metaStyle.Render("Created: " + p.detailBead.CreatedAt + " by " + p.detailBead.CreatedBy))
	sb.WriteString("\n")
	if p.detailBead.UpdatedAt != "" {
		sb.WriteString(metaStyle.Render("Updated: " + p.detailBead.UpdatedAt))
		sb.WriteString("\n")
	}
	if p.detailBead.ClosedAt != "" {
		sb.WriteString(metaStyle.Render("Closed: " + p.detailBead.ClosedAt))
		sb.WriteString("\n")
	}

	return sb.String()
}

// safeHeight ensures height is at least 1.
func safeHeight(h int) int {
	if h < 1 {
		return 1
	}
	return h
}
