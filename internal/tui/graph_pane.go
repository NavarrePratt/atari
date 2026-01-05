package tui

import (
	"context"
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
	requestID int // For staleness detection
}

// graphTickMsg signals a tick for updating elapsed time during refresh.
type graphTickMsg time.Time

// graphResultMsg carries the result of a bead fetch operation.
type graphResultMsg struct {
	beads     []GraphBead
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
	return p.refreshCmd()
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

	default:
		return p, nil
	}
}

// handleKey processes keyboard input when focused.
func (p GraphPane) handleKey(msg tea.KeyMsg) (GraphPane, tea.Cmd) {
	key := msg.String()

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
		// Toggle Active/Backlog view
		if p.graph != nil {
			if p.graph.GetView() == ViewActive {
				p.graph.SetView(ViewBacklog)
			} else {
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
		// Return a command to signal modal should open
		// The parent model handles this
		return p, func() tea.Msg {
			return GraphOpenModalMsg{NodeID: p.graph.GetSelectedID()}
		}

	case "esc":
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

		if view == ViewActive {
			beads, err = p.fetcher.FetchActive(ctx)
		} else {
			beads, err = p.fetcher.FetchBacklog(ctx)
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

// rebuildGraph rebuilds the graph with new bead data.
func (p *GraphPane) rebuildGraph(beads []GraphBead) {
	if p.graph == nil {
		p.graph = NewGraph(p.cfg, p.fetcher, p.layout)
	}
	p.graph.buildFromBeads(beads)
}

// View renders the graph pane.
func (p GraphPane) View() string {
	if p.width == 0 || p.height == 0 {
		return ""
	}

	contentWidth := safeWidth(p.width - 4)   // Account for padding
	contentHeight := safeHeight(p.height - 3) // Account for status bar and padding

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
	if p.loading {
		elapsed := time.Since(p.startedAt).Round(100 * time.Millisecond)
		status := p.spinner.View() + " Loading beads... (" + elapsed.String() + " elapsed)"
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Width(width).
			Render(status)
	}

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
	if len(info)+len(hint) <= width {
		info += hint
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

// safeHeight ensures height is at least 1.
func safeHeight(h int) int {
	if h < 1 {
		return 1
	}
	return h
}
