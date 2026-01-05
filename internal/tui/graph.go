package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/npratt/atari/internal/config"
)

// Layout holds the computed positions of nodes in the graph.
type Layout struct {
	Direction LayoutDirection
	Layers    [][]string           // Node IDs organized by layer (root at layer 0)
	Positions map[string]Position  // Computed positions for each node
}

// Graph manages the bead graph visualization state.
type Graph struct {
	config  *config.GraphConfig
	fetcher BeadFetcher
	layout  string // "horizontal" or "vertical" from TUI config

	mu          sync.RWMutex
	nodes       map[string]*GraphNode
	edges       []GraphEdge
	computed    *Layout
	selected    string           // Selected node ID
	viewport    Viewport
	collapsed   map[string]bool  // Collapsed epic IDs
	view        GraphView        // Active or Backlog
	currentBead string           // Currently processing bead (highlighted)
}

// NewGraph creates a new Graph with the given configuration.
func NewGraph(cfg *config.GraphConfig, fetcher BeadFetcher, layout string) *Graph {
	return &Graph{
		config:    cfg,
		fetcher:   fetcher,
		layout:    layout,
		nodes:     make(map[string]*GraphNode),
		edges:     nil,
		computed:  nil,
		collapsed: make(map[string]bool),
		view:      ViewActive,
	}
}

// Refresh fetches bead data and rebuilds the graph.
func (g *Graph) Refresh(ctx context.Context) error {
	var beads []GraphBead
	var err error

	g.mu.RLock()
	view := g.view
	g.mu.RUnlock()

	if view == ViewActive {
		beads, err = g.fetcher.FetchActive(ctx)
	} else {
		beads, err = g.fetcher.FetchBacklog(ctx)
	}
	if err != nil {
		return err
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	g.buildFromBeads(beads)
	return nil
}

// RebuildFromBeads rebuilds the graph from bead data with proper locking.
// This is the public API for external callers.
func (g *Graph) RebuildFromBeads(beads []GraphBead) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.buildFromBeads(beads)
}

// buildFromBeads builds the graph from bead data.
// Must be called with mu held.
func (g *Graph) buildFromBeads(beads []GraphBead) {
	g.nodes = make(map[string]*GraphNode)
	g.edges = nil

	// Track which node IDs exist in the current dataset
	existingIDs := make(map[string]bool)

	// Build nodes
	for i := range beads {
		node := beads[i].ToNode()
		g.nodes[node.ID] = &node
		existingIDs[node.ID] = true
	}

	// Build edges
	for i := range beads {
		edges := beads[i].ExtractEdges()
		g.edges = append(g.edges, edges...)
	}

	// Track missing dependencies for pseudo-node
	missingDeps := make(map[string]bool)
	for _, edge := range g.edges {
		if !existingIDs[edge.From] {
			missingDeps[edge.From] = true
		}
	}

	// Create pseudo-node for missing dependencies if any
	if len(missingDeps) > 0 {
		pseudoID := "_hidden_deps"
		g.nodes[pseudoID] = &GraphNode{
			ID:     pseudoID,
			Title:  pluralize(len(missingDeps), "dep hidden", "deps hidden"),
			Status: "closed",
			Type:   "pseudo",
		}
	}

	// Compute layout
	g.computeLayout()

	// Validate selected node still exists
	if g.selected != "" && g.nodes[g.selected] == nil {
		g.selected = ""
	}

	// Auto-select first node if none selected
	if g.selected == "" && len(g.computed.Layers) > 0 && len(g.computed.Layers[0]) > 0 {
		g.selected = g.computed.Layers[0][0]
	}
}

// computeLayout computes the graph layout using BFS layer assignment.
// Must be called with mu held.
func (g *Graph) computeLayout() {
	direction := LayoutTopDown
	if g.layout == "vertical" {
		direction = LayoutLeftRight
	}

	g.computed = &Layout{
		Direction: direction,
		Layers:    nil,
		Positions: make(map[string]Position),
	}

	// Find root nodes (nodes with no incoming hierarchy edges)
	hasParent := make(map[string]bool)
	for _, edge := range g.edges {
		if edge.Type == EdgeHierarchy {
			hasParent[edge.To] = true
		}
	}

	var roots []string
	for id := range g.nodes {
		if !hasParent[id] && id != "_hidden_deps" {
			roots = append(roots, id)
		}
	}

	// Sort roots for deterministic ordering (epics first, then by ID)
	sort.Slice(roots, func(i, j int) bool {
		ni, nj := g.nodes[roots[i]], g.nodes[roots[j]]
		if ni.IsEpic != nj.IsEpic {
			return ni.IsEpic // Epics first
		}
		return roots[i] < roots[j]
	})

	// BFS layer assignment
	if len(roots) == 0 {
		// No roots means no hierarchy - put all nodes in layer 0
		var allNodes []string
		for id := range g.nodes {
			allNodes = append(allNodes, id)
		}
		sort.Strings(allNodes)
		if len(allNodes) > 0 {
			g.computed.Layers = [][]string{allNodes}
		}
	} else {
		g.computed.Layers = g.assignLayers(roots)
	}

	// Add pseudo-node to last layer if it exists
	if _, ok := g.nodes["_hidden_deps"]; ok {
		if len(g.computed.Layers) == 0 {
			g.computed.Layers = [][]string{{"_hidden_deps"}}
		} else {
			lastIdx := len(g.computed.Layers) - 1
			g.computed.Layers[lastIdx] = append(g.computed.Layers[lastIdx], "_hidden_deps")
		}
	}

	// Position nodes within layers
	g.positionNodes()
}

// assignLayers assigns nodes to layers using BFS from roots.
// Must be called with mu held.
func (g *Graph) assignLayers(roots []string) [][]string {
	layers := [][]string{roots}
	visited := make(map[string]bool)
	for _, r := range roots {
		visited[r] = true
	}

	// Build adjacency list for children (hierarchy edges)
	children := make(map[string][]string)
	for _, edge := range g.edges {
		if edge.Type == EdgeHierarchy {
			children[edge.From] = append(children[edge.From], edge.To)
		}
	}

	for {
		var nextLayer []string
		currentLayer := layers[len(layers)-1]

		for _, nodeID := range currentLayer {
			for _, childID := range children[nodeID] {
				if !visited[childID] {
					visited[childID] = true
					nextLayer = append(nextLayer, childID)
				}
			}
		}

		if len(nextLayer) == 0 {
			break
		}

		// Sort for deterministic ordering
		sort.Strings(nextLayer)
		layers = append(layers, nextLayer)
	}

	// Add any orphan nodes (not reachable from roots) to appropriate layer
	for id := range g.nodes {
		if !visited[id] && id != "_hidden_deps" {
			visited[id] = true
			// Put orphans in first layer
			layers[0] = append(layers[0], id)
		}
	}

	return layers
}

// positionNodes computes positions for all nodes.
// Must be called with mu held.
func (g *Graph) positionNodes() {
	// Node dimensions based on density
	nodeW, nodeH := g.nodeDimensions()
	spacing := 2 // Space between nodes

	for layerIdx, layer := range g.computed.Layers {
		for nodeIdx, nodeID := range layer {
			var pos Position
			if g.computed.Direction == LayoutTopDown {
				// Top-down: X varies by position in layer, Y varies by layer
				pos = Position{
					X: nodeIdx * (nodeW + spacing),
					Y: layerIdx * (nodeH + spacing),
					W: nodeW,
					H: nodeH,
				}
			} else {
				// Left-right: X varies by layer, Y varies by position in layer
				pos = Position{
					X: layerIdx * (nodeW + spacing),
					Y: nodeIdx * (nodeH + spacing),
					W: nodeW,
					H: nodeH,
				}
			}
			g.computed.Positions[nodeID] = pos
		}
	}
}

// nodeDimensions returns node width and height based on density.
func (g *Graph) nodeDimensions() (int, int) {
	density := ParseDensity(g.config.Density)
	switch density {
	case DensityCompact:
		return 16, 1
	case DensityDetailed:
		return 26, 3
	default: // DensityStandard
		return 26, 2
	}
}

// SetView sets the graph view (Active or Backlog).
func (g *Graph) SetView(view GraphView) {
	g.mu.Lock()
	g.view = view
	g.mu.Unlock()
}

// GetView returns the current graph view.
func (g *Graph) GetView() GraphView {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.view
}

// Select sets the selected node by ID.
func (g *Graph) Select(nodeID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.nodes[nodeID] != nil {
		g.selected = nodeID
		g.adjustViewport()
	}
}

// GetSelected returns the currently selected node, or nil if none.
func (g *Graph) GetSelected() *GraphNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.selected == "" {
		return nil
	}
	if node := g.nodes[g.selected]; node != nil {
		// Return a copy
		copy := *node
		return &copy
	}
	return nil
}

// SelectNext moves selection to the next sibling in the current layer.
func (g *Graph) SelectNext() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.selected == "" || g.computed == nil {
		return
	}

	// Find current layer and position
	for _, layer := range g.computed.Layers {
		for i, nodeID := range layer {
			if nodeID == g.selected {
				// Move to next in layer (wrap around)
				nextIdx := (i + 1) % len(layer)
				g.selected = layer[nextIdx]
				g.adjustViewport()
				return
			}
		}
	}
}

// SelectPrev moves selection to the previous sibling in the current layer.
func (g *Graph) SelectPrev() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.selected == "" || g.computed == nil {
		return
	}

	// Find current layer and position
	for _, layer := range g.computed.Layers {
		for i, nodeID := range layer {
			if nodeID == g.selected {
				// Move to previous in layer (wrap around)
				prevIdx := i - 1
				if prevIdx < 0 {
					prevIdx = len(layer) - 1
				}
				g.selected = layer[prevIdx]
				g.adjustViewport()
				return
			}
		}
	}
}

// SelectParent moves selection to the parent node (if any).
func (g *Graph) SelectParent() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.selected == "" {
		return
	}

	// Find parent via hierarchy edges
	for _, edge := range g.edges {
		if edge.Type == EdgeHierarchy && edge.To == g.selected {
			if g.nodes[edge.From] != nil {
				g.selected = edge.From
				g.adjustViewport()
				return
			}
		}
	}
}

// SelectChild moves selection to the first child node (if any).
func (g *Graph) SelectChild() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.selected == "" {
		return
	}

	// Find first child via hierarchy edges
	for _, edge := range g.edges {
		if edge.Type == EdgeHierarchy && edge.From == g.selected {
			if g.nodes[edge.To] != nil && !g.collapsed[g.selected] {
				g.selected = edge.To
				g.adjustViewport()
				return
			}
		}
	}
}

// ToggleCollapse toggles the collapsed state of an epic.
func (g *Graph) ToggleCollapse(nodeID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	node := g.nodes[nodeID]
	if node == nil || !node.IsEpic {
		return // Can only collapse epics
	}

	g.collapsed[nodeID] = !g.collapsed[nodeID]
}

// IsCollapsed returns whether a node is collapsed.
func (g *Graph) IsCollapsed(nodeID string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.collapsed[nodeID]
}

// SetCurrentBead sets the currently processing bead for highlighting.
func (g *Graph) SetCurrentBead(beadID string) {
	g.mu.Lock()
	g.currentBead = beadID
	g.mu.Unlock()
}

// GetCurrentBead returns the currently processing bead ID.
func (g *Graph) GetCurrentBead() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.currentBead
}

// SetViewport sets the viewport dimensions.
func (g *Graph) SetViewport(width, height int) {
	g.mu.Lock()
	g.viewport.Width = width
	g.viewport.Height = height
	g.mu.Unlock()
}

// adjustViewport adjusts the viewport to keep the selected node visible.
// Must be called with mu held.
func (g *Graph) adjustViewport() {
	if g.selected == "" || g.computed == nil {
		return
	}

	pos, ok := g.computed.Positions[g.selected]
	if !ok {
		return
	}

	// Adjust X offset to keep node visible
	if pos.X < g.viewport.OffsetX {
		g.viewport.OffsetX = pos.X
	} else if pos.X+pos.W > g.viewport.OffsetX+g.viewport.Width {
		g.viewport.OffsetX = pos.X + pos.W - g.viewport.Width
	}

	// Adjust Y offset to keep node visible
	if pos.Y < g.viewport.OffsetY {
		g.viewport.OffsetY = pos.Y
	} else if pos.Y+pos.H > g.viewport.OffsetY+g.viewport.Height {
		g.viewport.OffsetY = pos.Y + pos.H - g.viewport.Height
	}
}

// GetLayout returns the computed layout (for rendering).
func (g *Graph) GetLayout() *Layout {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.computed
}

// GetNodes returns a copy of the nodes map.
func (g *Graph) GetNodes() map[string]*GraphNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make(map[string]*GraphNode, len(g.nodes))
	for k, v := range g.nodes {
		copy := *v
		result[k] = &copy
	}
	return result
}

// GetEdges returns a copy of the edges slice.
func (g *Graph) GetEdges() []GraphEdge {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]GraphEdge, len(g.edges))
	copy(result, g.edges)
	return result
}

// GetViewport returns the current viewport.
func (g *Graph) GetViewport() Viewport {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.viewport
}

// NodeCount returns the number of nodes in the graph.
func (g *Graph) NodeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.nodes)
}

// EdgeCount returns the number of edges in the graph.
func (g *Graph) EdgeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.edges)
}

// GetSelectedID returns the ID of the selected node.
func (g *Graph) GetSelectedID() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.selected
}

// ChildCount returns the number of children for a node.
func (g *Graph) ChildCount(nodeID string) int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	count := 0
	for _, edge := range g.edges {
		if edge.Type == EdgeHierarchy && edge.From == nodeID {
			count++
		}
	}
	return count
}

// pluralize returns singular or plural form based on count.
func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", count, plural)
}

// statusIcon returns a single-character icon for the given status.
func statusIcon(status string) string {
	switch status {
	case "open":
		return "o"
	case "in_progress":
		return "*"
	case "blocked":
		return "x"
	case "deferred":
		return "-"
	case "closed":
		return "."
	default:
		return "?"
	}
}

// priorityLabel returns a short priority label (P0-P4).
func priorityLabel(priority int) string {
	if priority < 0 || priority > 4 {
		return "P?"
	}
	return fmt.Sprintf("P%d", priority)
}

// CycleDensity cycles through density levels and updates config.
func (g *Graph) CycleDensity() {
	g.mu.Lock()
	defer g.mu.Unlock()

	current := ParseDensity(g.config.Density)
	var next NodeDensity
	switch current {
	case DensityCompact:
		next = DensityStandard
	case DensityStandard:
		next = DensityDetailed
	case DensityDetailed:
		next = DensityCompact
	}
	g.config.Density = next.String()

	// Recompute layout with new dimensions
	if len(g.nodes) > 0 {
		g.positionNodes()
	}
}

// GetDensity returns the current density level.
func (g *Graph) GetDensity() NodeDensity {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return ParseDensity(g.config.Density)
}

// Render renders the graph to a string for the given viewport dimensions.
func (g *Graph) Render(width, height int) string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.computed == nil || len(g.nodes) == 0 {
		return g.renderEmpty(width, height)
	}

	// Create a 2D character grid
	grid := newGrid(width, height)

	// Collect visible nodes (not children of collapsed epics)
	visibleNodes := g.getVisibleNodes()

	// Render edges first (so nodes draw on top)
	g.renderEdges(grid, visibleNodes)

	// Render nodes
	for _, nodeID := range visibleNodes {
		g.renderNodeToGrid(grid, nodeID)
	}

	return grid.String()
}

// renderEmpty renders a placeholder for an empty graph.
func (g *Graph) renderEmpty(width, height int) string {
	msg := "No beads to display"
	if width < len(msg) {
		msg = "Empty"
	}
	// Center the message
	padLeft := (width - len(msg)) / 2
	if padLeft < 0 {
		padLeft = 0
	}
	line := strings.Repeat(" ", padLeft) + msg
	if len(line) < width {
		line += strings.Repeat(" ", width-len(line))
	}
	// Put in middle vertically
	var lines []string
	midY := height / 2
	for y := 0; y < height; y++ {
		if y == midY {
			lines = append(lines, line)
		} else {
			lines = append(lines, strings.Repeat(" ", width))
		}
	}
	return strings.Join(lines, "\n")
}

// getVisibleNodes returns IDs of nodes that should be rendered.
// Excludes children of collapsed epics.
func (g *Graph) getVisibleNodes() []string {
	var visible []string
	for _, nodeID := range g.allNodeIDs() {
		if g.isNodeVisible(nodeID) {
			visible = append(visible, nodeID)
		}
	}
	return visible
}

// allNodeIDs returns all node IDs in layer order.
func (g *Graph) allNodeIDs() []string {
	var ids []string
	if g.computed != nil {
		for _, layer := range g.computed.Layers {
			ids = append(ids, layer...)
		}
	}
	return ids
}

// isNodeVisible returns true if the node should be rendered.
// Returns false if any ancestor is collapsed.
func (g *Graph) isNodeVisible(nodeID string) bool {
	// Check if any parent in hierarchy is collapsed
	for _, edge := range g.edges {
		if edge.Type == EdgeHierarchy && edge.To == nodeID {
			if g.collapsed[edge.From] {
				return false
			}
			// Recursively check grandparents
			if !g.isNodeVisible(edge.From) {
				return false
			}
		}
	}
	return true
}

// renderNodeToGrid renders a single node to the grid.
func (g *Graph) renderNodeToGrid(grid *charGrid, nodeID string) {
	node := g.nodes[nodeID]
	if node == nil {
		return
	}

	pos, ok := g.computed.Positions[nodeID]
	if !ok {
		return
	}

	// Adjust for viewport offset
	x := pos.X - g.viewport.OffsetX
	y := pos.Y - g.viewport.OffsetY

	// Skip if completely outside viewport
	if x+pos.W < 0 || x >= grid.width || y+pos.H < 0 || y >= grid.height {
		return
	}

	// Determine style
	isCurrent := nodeID == g.currentBead
	isSelected := nodeID == g.selected
	isCollapsedEpic := node.IsEpic && g.collapsed[nodeID]
	childCount := 0
	if isCollapsedEpic {
		childCount = g.countChildren(nodeID)
	}

	// Render the node content
	content := g.formatNode(node, pos.W, isCurrent, isSelected, isCollapsedEpic, childCount)

	// Write to grid
	lines := strings.Split(content, "\n")
	for dy, line := range lines {
		if y+dy >= 0 && y+dy < grid.height {
			grid.writeString(x, y+dy, line)
		}
	}
}

// countChildren counts hierarchy children of a node.
func (g *Graph) countChildren(nodeID string) int {
	count := 0
	for _, edge := range g.edges {
		if edge.Type == EdgeHierarchy && edge.From == nodeID {
			count++
		}
	}
	return count
}

// formatNode formats a node for display based on density.
func (g *Graph) formatNode(node *GraphNode, width int, isCurrent, isSelected, isCollapsed bool, childCount int) string {
	density := ParseDensity(g.config.Density)

	// Build node text based on density
	var text string
	switch density {
	case DensityCompact:
		text = g.formatNodeCompact(node, isCollapsed, childCount)
	case DensityDetailed:
		text = g.formatNodeDetailed(node, isCollapsed, childCount)
	default: // DensityStandard
		text = g.formatNodeStandard(node, isCollapsed, childCount)
	}

	// Apply styling
	style := graphStyles.Node
	if isCurrent {
		style = graphStyles.NodeCurrent
	} else if isSelected {
		style = graphStyles.NodeSelected
	}

	return style.Render(text)
}

// formatNodeCompact formats a node in compact density: "bd-xxx o"
func (g *Graph) formatNodeCompact(node *GraphNode, isCollapsed bool, childCount int) string {
	icon := statusIcon(node.Status)
	text := fmt.Sprintf("%s %s", node.ID, icon)
	if isCollapsed && childCount > 0 {
		text += fmt.Sprintf(" +%d", childCount)
	}
	return text
}

// formatNodeStandard formats a node in standard density: "bd-xxx o Title..."
func (g *Graph) formatNodeStandard(node *GraphNode, isCollapsed bool, childCount int) string {
	icon := statusIcon(node.Status)
	title := truncate(node.Title, 12)
	text := fmt.Sprintf("%s %s %s", node.ID, icon, title)
	if isCollapsed && childCount > 0 {
		text += fmt.Sprintf(" +%d", childCount)
	}
	return text
}

// formatNodeDetailed formats a node in detailed density with cost/attempts.
func (g *Graph) formatNodeDetailed(node *GraphNode, isCollapsed bool, childCount int) string {
	icon := statusIcon(node.Status)
	priority := priorityLabel(node.Priority)
	title := truncate(node.Title, 10)
	details := ""
	if node.Attempts > 0 || node.Cost > 0 {
		details = fmt.Sprintf(" [%d $%.2f]", node.Attempts, node.Cost)
	}
	text := fmt.Sprintf("%s %s %s %s%s", node.ID, icon, priority, title, details)
	if isCollapsed && childCount > 0 {
		text += fmt.Sprintf(" +%d", childCount)
	}
	return text
}

// renderEdges renders all edges between visible nodes.
func (g *Graph) renderEdges(grid *charGrid, visibleNodes []string) {
	visibleSet := make(map[string]bool)
	for _, id := range visibleNodes {
		visibleSet[id] = true
	}

	for _, edge := range g.edges {
		// Only render if both endpoints are visible
		if !visibleSet[edge.From] || !visibleSet[edge.To] {
			continue
		}

		fromPos, fromOK := g.computed.Positions[edge.From]
		toPos, toOK := g.computed.Positions[edge.To]
		if !fromOK || !toOK {
			continue
		}

		g.renderEdge(grid, fromPos, toPos, edge.Type)
	}
}

// renderEdge renders a single edge between two positions.
func (g *Graph) renderEdge(grid *charGrid, from, to Position, edgeType EdgeType) {
	// Adjust for viewport
	fromX := from.X + from.W/2 - g.viewport.OffsetX
	fromY := from.Y + from.H - g.viewport.OffsetY
	toX := to.X + to.W/2 - g.viewport.OffsetX
	toY := to.Y - g.viewport.OffsetY

	// Choose characters based on edge type
	var hChar, vChar, cornerChar rune
	if edgeType == EdgeHierarchy {
		hChar = '─'
		vChar = '│'
		cornerChar = '└'
	} else {
		// Dependency edges use dashed characters
		hChar = '╌'
		vChar = '╎'
		cornerChar = '└'
	}

	// Simple L-shaped edge: down from source, then across to target
	// Draw vertical segment
	minY, maxY := fromY, toY
	if minY > maxY {
		minY, maxY = maxY, minY
	}
	for y := minY + 1; y < maxY; y++ {
		grid.writeRune(fromX, y, vChar)
	}

	// Draw horizontal segment and corner
	if fromX != toX {
		minX, maxX := fromX, toX
		if minX > maxX {
			minX, maxX = maxX, minX
		}
		for x := minX; x <= maxX; x++ {
			if x == fromX && toY > fromY {
				grid.writeRune(x, toY-1, cornerChar)
			} else if x != toX {
				grid.writeRune(x, toY-1, hChar)
			}
		}
	}
}

// charGrid is a 2D character grid for rendering.
type charGrid struct {
	width  int
	height int
	cells  [][]rune
	styles [][]lipgloss.Style
}

// newGrid creates a new character grid filled with spaces.
func newGrid(width, height int) *charGrid {
	cells := make([][]rune, height)
	styles := make([][]lipgloss.Style, height)
	for y := 0; y < height; y++ {
		cells[y] = make([]rune, width)
		styles[y] = make([]lipgloss.Style, width)
		for x := 0; x < width; x++ {
			cells[y][x] = ' '
		}
	}
	return &charGrid{
		width:  width,
		height: height,
		cells:  cells,
		styles: styles,
	}
}

// writeRune writes a single rune at the given position.
func (g *charGrid) writeRune(x, y int, r rune) {
	if x >= 0 && x < g.width && y >= 0 && y < g.height {
		g.cells[y][x] = r
	}
}

// writeString writes a string starting at the given position.
func (g *charGrid) writeString(x, y int, s string) {
	for i, r := range s {
		g.writeRune(x+i, y, r)
	}
}

// String converts the grid to a string.
func (g *charGrid) String() string {
	var lines []string
	for _, row := range g.cells {
		lines = append(lines, string(row))
	}
	return strings.Join(lines, "\n")
}
