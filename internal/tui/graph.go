package tui

import (
	"context"
	"fmt"
	"sort"
	"sync"

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
