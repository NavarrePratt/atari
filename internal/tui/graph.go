package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/npratt/atari/internal/config"
)

// Layout holds the computed positions of nodes in the graph.
type Layout struct {
	Direction LayoutDirection
	Layers    [][]string          // Node IDs organized by layer (root at layer 0)
	Positions map[string]Position // Computed positions for each node
	ListOrder []ListNode          // Ordered list of nodes for list mode
}

// Graph manages the bead graph visualization state.
type Graph struct {
	config  *config.GraphConfig
	fetcher BeadFetcher
	layout  string // "horizontal" or "vertical" from TUI config

	mu             sync.RWMutex
	nodes          map[string]*GraphNode
	edges          []GraphEdge
	computed       *Layout
	selected       string          // Selected node ID
	viewport       Viewport
	collapsed      map[string]bool // Collapsed epic IDs
	view           GraphView       // Active or Backlog
	currentBead    string          // Currently processing bead (highlighted)
	listOrder      []ListNode      // Ordered list of nodes for list view
	epicFilter     string          // Epic ID filter (empty = no filter)
	activeTopLevel string          // Active top-level item for subtree highlighting
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

	switch view {
	case ViewActive:
		beads, err = g.fetcher.FetchActive(ctx)
	case ViewBacklog:
		beads, err = g.fetcher.FetchBacklog(ctx)
	case ViewClosed:
		beads, err = g.fetcher.FetchClosed(ctx)
	default:
		beads, err = g.fetcher.FetchActive(ctx)
	}
	if err != nil {
		return err
	}

	// Identify out-of-view dependencies (beads referenced but not in current view).
	// These need to be fetched and marked as out-of-view in the graph.
	beads, outOfViewIDs := g.fetchOutOfViewDeps(ctx, beads)

	g.mu.Lock()
	defer g.mu.Unlock()

	g.buildFromBeads(beads, outOfViewIDs)
	return nil
}

// RebuildFromBeads rebuilds the graph from bead data with proper locking.
// This is the public API for external callers.
// Note: This method does not fetch missing dependencies; use Refresh() for full graph building.
func (g *Graph) RebuildFromBeads(beads []GraphBead) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.buildFromBeads(beads, nil)
}

// fetchOutOfViewDeps fetches dependencies that are not in the current bead set.
// Returns the augmented beads slice and a set of out-of-view IDs for marking.
func (g *Graph) fetchOutOfViewDeps(ctx context.Context, beads []GraphBead) ([]GraphBead, map[string]bool) {
	existingIDs := make(map[string]bool, len(beads))
	for _, b := range beads {
		existingIDs[b.ID] = true
	}

	// Find dependency IDs not in current view
	missingIDs := make(map[string]bool)
	for _, b := range beads {
		for _, dep := range b.Dependencies {
			if !existingIDs[dep.ID] {
				missingIDs[dep.ID] = true
			}
		}
	}

	if len(missingIDs) == 0 {
		return beads, nil
	}

	// Fetch out-of-view beads
	outOfViewIDs := make(map[string]bool, len(missingIDs))
	for id := range missingIDs {
		select {
		case <-ctx.Done():
			return beads, outOfViewIDs
		default:
		}
		bead, err := g.fetcher.FetchBead(ctx, id)
		if err != nil || bead == nil {
			beads = append(beads, GraphBead{ID: id, Title: "?", Status: "?"})
		} else {
			beads = append(beads, *bead)
		}
		outOfViewIDs[id] = true
	}

	return beads, outOfViewIDs
}

// buildFromBeads builds the graph from bead data.
// outOfViewIDs marks nodes that are from a different view (e.g., closed deps when viewing Active).
// Must be called with mu held.
func (g *Graph) buildFromBeads(beads []GraphBead, outOfViewIDs map[string]bool) {
	g.nodes = make(map[string]*GraphNode)
	g.edges = nil

	// Build nodes
	for i := range beads {
		node := beads[i].ToNode()
		if outOfViewIDs != nil && outOfViewIDs[node.ID] {
			node.OutOfView = true
		}
		g.nodes[node.ID] = &node
	}

	// Build edges
	for i := range beads {
		edges := beads[i].ExtractEdges()
		g.edges = append(g.edges, edges...)
	}

	// Mark nodes as OutOfScope if epic filter is active
	if g.epicFilter != "" {
		descendants := g.computeEpicDescendants()
		for id, node := range g.nodes {
			if !descendants[id] {
				node.OutOfScope = true
			}
		}
	}

	// Compute layout (handles both grid and list positioning)
	g.computeLayout()

	// Validate selected node still exists
	if g.selected != "" && g.nodes[g.selected] == nil {
		g.selected = ""
	}

	// Auto-select first node if none selected (skip out-of-view and out-of-scope nodes)
	if g.selected == "" && len(g.computed.Layers) > 0 {
		for _, layer := range g.computed.Layers {
			for _, nodeID := range layer {
				if node := g.nodes[nodeID]; node != nil && !node.OutOfView && !node.OutOfScope {
					g.selected = nodeID
					break
				}
			}
			if g.selected != "" {
				break
			}
		}
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
		if !hasParent[id] {
			roots = append(roots, id)
		}
	}

	// Sort roots for deterministic ordering (epics first, out-of-view last, then by ID)
	sort.Slice(roots, func(i, j int) bool {
		ni, nj := g.nodes[roots[i]], g.nodes[roots[j]]
		// Out-of-view nodes go last
		if ni.OutOfView != nj.OutOfView {
			return !ni.OutOfView
		}
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

	// Compute list order and position nodes linearly
	g.computeListOrder()
	g.positionNodesForList()
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
		if !visited[id] {
			visited[id] = true
			// Put orphans in first layer
			layers[0] = append(layers[0], id)
		}
	}

	return layers
}

// computeListOrder computes the list order using DFS traversal.
// Must be called with mu held.
func (g *Graph) computeListOrder() {
	g.listOrder = nil

	// Build adjacency list for children (hierarchy edges)
	children := g.getChildrenMap()

	// Sort children for deterministic ordering
	for parent := range children {
		sort.Strings(children[parent])
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
		if !hasParent[id] {
			roots = append(roots, id)
		}
	}

	// Sort roots for deterministic ordering (epics first, out-of-view last, then by ID)
	sort.Slice(roots, func(i, j int) bool {
		ni, nj := g.nodes[roots[i]], g.nodes[roots[j]]
		// Out-of-view nodes go last
		if ni.OutOfView != nj.OutOfView {
			return !ni.OutOfView
		}
		if ni.IsEpic != nj.IsEpic {
			return ni.IsEpic // Epics first
		}
		return roots[i] < roots[j]
	})

	// Track visited nodes to prevent cycles
	visited := make(map[string]bool)

	// DFS traversal to build list order
	var dfs func(nodeID string, depth int, parentID string)
	dfs = func(nodeID string, depth int, parentID string) {
		// Cycle protection: skip already visited nodes
		if visited[nodeID] {
			return
		}
		visited[nodeID] = true

		visible := g.isNodeVisible(nodeID)
		g.listOrder = append(g.listOrder, ListNode{
			ID:       nodeID,
			Depth:    depth,
			ParentID: parentID,
			Visible:  visible,
		})

		// Don't recurse into collapsed epics
		if g.collapsed[nodeID] {
			return
		}

		for _, childID := range children[nodeID] {
			dfs(childID, depth+1, nodeID)
		}
	}

	for _, root := range roots {
		dfs(root, 0, "")
	}

	// Add orphan nodes not reached through hierarchy edges
	// (e.g., nodes only connected via dependency edges, or out-of-view deps)
	var orphans []string
	for id := range g.nodes {
		if !visited[id] {
			orphans = append(orphans, id)
		}
	}
	// Sort orphans: out-of-view nodes last, then by ID
	sort.Slice(orphans, func(i, j int) bool {
		ni, nj := g.nodes[orphans[i]], g.nodes[orphans[j]]
		if ni.OutOfView != nj.OutOfView {
			return !ni.OutOfView
		}
		return orphans[i] < orphans[j]
	})
	for _, id := range orphans {
		dfs(id, 0, "")
	}

	// Also store in Layout for external access
	if g.computed != nil {
		g.computed.ListOrder = g.listOrder
	}
}

// getChildrenMap builds an adjacency list for children (hierarchy edges).
// Must be called with mu held.
func (g *Graph) getChildrenMap() map[string][]string {
	children := make(map[string][]string)
	for _, edge := range g.edges {
		if edge.Type == EdgeHierarchy {
			children[edge.From] = append(children[edge.From], edge.To)
		}
	}
	return children
}

// getParent returns the parent node ID for a given node via hierarchy edges.
// Returns empty string if no parent exists. Must be called with mu held.
func (g *Graph) getParent(nodeID string) string {
	for _, edge := range g.edges {
		if edge.Type == EdgeHierarchy && edge.To == nodeID {
			return edge.From
		}
	}
	return ""
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

// nodeHeight returns node height based on density (for list mode).
func (g *Graph) nodeHeight() int {
	_, h := g.nodeDimensions()
	return h
}

// positionNodesForList computes positions for list mode layout.
// Y = cumulative line count (density-aware node height)
// X = depth * 2 (2-space indent per level), clamped to prevent overflow
// Width = viewport.Width - X
// Must be called with mu held.
func (g *Graph) positionNodesForList() {
	if g.computed == nil {
		return
	}

	// Get node height based on density
	nodeH := g.nodeHeight()

	// Track cumulative Y position for visible nodes
	y := 0

	// Maximum X indent: limit to 1/4 of viewport width to prevent overflow
	maxIndent := g.viewport.Width / 4
	if maxIndent < 2 {
		maxIndent = 2
	}

	for _, item := range g.listOrder {
		// Calculate X as depth * 2 (2-space indent per level)
		x := item.Depth * 2

		// Clamp X to prevent overflow
		if x > maxIndent {
			x = maxIndent
		}

		// Calculate width as remaining space after indent
		w := g.viewport.Width - x
		if w < 1 {
			w = 1 // Minimum width of 1
		}

		// Store position for all nodes (visible and hidden)
		// Hidden nodes get positioned but won't be rendered
		g.computed.Positions[item.ID] = Position{
			X: x,
			Y: y,
			W: w,
			H: nodeH,
		}

		// Only increment Y for visible nodes
		if item.Visible {
			y += nodeH
		}
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

// SelectNext moves selection to the next visible node in list order (no wrap).
func (g *Graph) SelectNext() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.selected == "" || g.computed == nil {
		return
	}

	g.selectNextInList()
}

// selectNextInList moves to the next visible node in list order.
// No wrapping: stops at last visible node.
// Must be called with mu held.
func (g *Graph) selectNextInList() {
	// Find current index in list order
	currentIdx := -1
	for i, item := range g.listOrder {
		if item.ID == g.selected {
			currentIdx = i
			break
		}
	}

	if currentIdx < 0 {
		return
	}

	// Find next visible node
	for i := currentIdx + 1; i < len(g.listOrder); i++ {
		if g.listOrder[i].Visible {
			g.selected = g.listOrder[i].ID
			g.adjustViewport()
			return
		}
	}
	// No next visible node: stay at current (no wrap)
}

// SelectPrev moves selection to the previous visible node in list order (no wrap).
func (g *Graph) SelectPrev() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.selected == "" || g.computed == nil {
		return
	}

	g.selectPrevInList()
}

// selectPrevInList moves to the previous visible node in list order.
// No wrapping: stops at first visible node.
// Must be called with mu held.
func (g *Graph) selectPrevInList() {
	// Find current index in list order
	currentIdx := -1
	for i, item := range g.listOrder {
		if item.ID == g.selected {
			currentIdx = i
			break
		}
	}

	if currentIdx < 0 {
		return
	}

	// Find previous visible node
	for i := currentIdx - 1; i >= 0; i-- {
		if g.listOrder[i].Visible {
			g.selected = g.listOrder[i].ID
			g.adjustViewport()
			return
		}
	}
	// No previous visible node: stay at current (no wrap)
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

	wasExpanded := !g.collapsed[nodeID]
	g.collapsed[nodeID] = !g.collapsed[nodeID]

	// Recompute list order and positions to update visibility
	g.computeListOrder()
	g.positionNodesForList()

	// Handle selection recovery if we just collapsed and selected node is now invisible
	if wasExpanded && g.selected != "" {
		g.recoverSelectionAfterCollapse()
	}
}

// recoverSelectionAfterCollapse ensures the selected node is visible after a collapse.
// If the selected node is now invisible, finds the nearest visible node.
// Must be called with mu held.
func (g *Graph) recoverSelectionAfterCollapse() {
	// Check if selected node is still visible
	if g.isNodeVisible(g.selected) {
		return
	}

	// First: try to find a visible ancestor
	ancestor := g.findVisibleAncestor(g.selected)
	if ancestor != "" {
		g.selected = ancestor
		g.adjustViewport()
		return
	}

	// Second: find nearest visible node by list index
	// Find where selected node was in list order
	selectedIdx := -1
	for i, item := range g.listOrder {
		if item.ID == g.selected {
			selectedIdx = i
			break
		}
	}

	if selectedIdx >= 0 {
		// Look backward for nearest visible node
		for i := selectedIdx - 1; i >= 0; i-- {
			if g.listOrder[i].Visible {
				g.selected = g.listOrder[i].ID
				g.adjustViewport()
				return
			}
		}
		// Look forward for nearest visible node
		for i := selectedIdx + 1; i < len(g.listOrder); i++ {
			if g.listOrder[i].Visible {
				g.selected = g.listOrder[i].ID
				g.adjustViewport()
				return
			}
		}
	}

	// Fallback: select first visible node
	for _, item := range g.listOrder {
		if item.Visible {
			g.selected = item.ID
			g.adjustViewport()
			return
		}
	}

	// No visible nodes at all
	g.selected = ""
}

// findVisibleAncestor walks the parent chain and returns the first ancestor
// where all its ancestors are expanded (making it visible).
// Returns empty string if no visible ancestor exists.
// Must be called with mu held.
func (g *Graph) findVisibleAncestor(nodeID string) string {
	parentID := g.getParent(nodeID)
	if parentID == "" {
		return "" // No parent, no visible ancestor
	}

	// If parent is visible, return it
	if g.isNodeVisible(parentID) {
		return parentID
	}

	// Otherwise, recurse to find a visible ancestor of the parent
	return g.findVisibleAncestor(parentID)
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

// UpdateNode updates a node's properties in the graph.
// This is used to overlay additional state (like workqueue status) onto nodes.
func (g *Graph) UpdateNode(node *GraphNode) {
	if node == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()

	if existing := g.nodes[node.ID]; existing != nil {
		// Update fields that can be overlaid
		existing.WQStatus = node.WQStatus
		existing.Attempts = node.Attempts
		existing.InBackoff = node.InBackoff
	}
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

// nodeIcon returns the display icon for a node, considering workqueue state.
// Failed beads in backoff get "!" prefix, abandoned beads get "X" prefix.
func nodeIcon(node *GraphNode) string {
	baseIcon := statusIcon(node.Status)
	switch node.WQStatus {
	case "failed":
		if node.InBackoff {
			return "!" + baseIcon
		}
		return baseIcon
	case "abandoned":
		return "X" + baseIcon
	default:
		return baseIcon
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
		g.positionNodesForList()
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

	return g.renderListMode(width, height)
}

// renderListMode renders the graph as a vertical list with tree glyphs.
// Must be called with mu held (RLock).
func (g *Graph) renderListMode(width, height int) string {
	var lines []string

	// Build adjacency list for children to determine sibling relationships
	children := make(map[string][]string)
	for _, edge := range g.edges {
		if edge.Type == EdgeHierarchy {
			children[edge.From] = append(children[edge.From], edge.To)
		}
	}
	for parent := range children {
		sort.Strings(children[parent])
	}

	// Build visible list items (respecting viewport offset)
	var visibleItems []ListNode
	for _, item := range g.listOrder {
		if item.Visible {
			visibleItems = append(visibleItems, item)
		}
	}

	// Apply viewport offset
	// OffsetY is in lines, convert to item index based on node height
	nodeH := g.nodeHeight()
	startIdx := 0
	if nodeH > 0 && g.viewport.OffsetY > 0 {
		startIdx = g.viewport.OffsetY / nodeH
	}
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx >= len(visibleItems) {
		startIdx = 0
	}

	// height is in screen lines, convert to item count
	itemsPerScreen := height
	if nodeH > 1 {
		itemsPerScreen = height / nodeH
	}
	if itemsPerScreen < 1 {
		itemsPerScreen = 1
	}
	endIdx := startIdx + itemsPerScreen
	if endIdx > len(visibleItems) {
		endIdx = len(visibleItems)
	}

	// Track which items are last at their depth for glyph calculation
	// We need to know: for each visible item, is it the last sibling at its depth?
	isLastAtDepth := make(map[int]bool) // depth -> isLast for current branch

	for i := startIdx; i < endIdx; i++ {
		item := visibleItems[i]
		node := g.nodes[item.ID]
		if node == nil {
			continue
		}

		// Determine if this is the last sibling at its depth
		isLast := g.isLastSibling(item.ID, visibleItems, i, children)

		// Update depth tracking for ancestors
		isLastAtDepth[item.Depth] = isLast

		// Build tree glyphs
		glyphs := g.buildTreeGlyphs(item.Depth, isLastAtDepth, visibleItems, i)

		// Format node content
		line := g.formatListNode(node, item, glyphs, width, children)
		lines = append(lines, line)
	}

	// Pad with empty lines if needed
	for len(lines) < height {
		lines = append(lines, strings.Repeat(" ", width))
	}

	return strings.Join(lines, "\n")
}

// isLastSibling determines if a node is the last sibling among visible nodes.
func (g *Graph) isLastSibling(nodeID string, visibleItems []ListNode, currentIdx int, children map[string][]string) bool {
	if currentIdx >= len(visibleItems) {
		return true
	}

	currentItem := visibleItems[currentIdx]
	currentDepth := currentItem.Depth

	// Look ahead for siblings at the same depth
	for i := currentIdx + 1; i < len(visibleItems); i++ {
		nextItem := visibleItems[i]
		if nextItem.Depth < currentDepth {
			// We've moved to a higher level (parent's sibling), so current is last
			return true
		}
		if nextItem.Depth == currentDepth {
			// Found a sibling at the same depth
			return false
		}
		// nextItem.Depth > currentDepth means we're looking at children, continue
	}

	// No more items or no siblings found
	return true
}

// buildTreeGlyphs builds the tree glyph prefix for a list item.
func (g *Graph) buildTreeGlyphs(depth int, isLastAtDepth map[int]bool, visibleItems []ListNode, currentIdx int) string {
	if depth == 0 {
		return ""
	}

	var glyphs strings.Builder

	// For each ancestor level, determine if we need a continuation line
	for d := 1; d < depth; d++ {
		// Check if there are more siblings below at this depth
		hasMoreAtDepth := g.hasMoreSiblingsAtDepth(d, visibleItems, currentIdx)
		if hasMoreAtDepth {
			glyphs.WriteString("│  ")
		} else {
			glyphs.WriteString("   ")
		}
	}

	// Add the glyph for the current node
	if g.isLastSibling(visibleItems[currentIdx].ID, visibleItems, currentIdx, nil) {
		glyphs.WriteString("└─ ")
	} else {
		glyphs.WriteString("├─ ")
	}

	return glyphs.String()
}

// hasMoreSiblingsAtDepth checks if there are more nodes at the given depth after currentIdx.
func (g *Graph) hasMoreSiblingsAtDepth(targetDepth int, visibleItems []ListNode, currentIdx int) bool {
	// Look ahead to see if any item at targetDepth appears before we go back to a lower depth
	foundDepthOrLower := false
	for i := currentIdx + 1; i < len(visibleItems); i++ {
		item := visibleItems[i]
		if item.Depth < targetDepth {
			// We've gone above the target depth, no more siblings
			return foundDepthOrLower
		}
		if item.Depth == targetDepth {
			return true
		}
	}
	return false
}

// formatListNode formats a node for list display.
// Output varies by density setting:
// - Compact: glyphs + icon + ID only
// - Standard: glyphs + icon + ID + truncated title
// - Detailed: glyphs + icon + ID + priority + title + cost/attempts
func (g *Graph) formatListNode(node *GraphNode, item ListNode, glyphs string, width int, children map[string][]string) string {
	icon := nodeIcon(node)
	isCurrent := node.ID == g.currentBead
	isSelected := node.ID == g.selected
	isCollapsedEpic := node.IsEpic && g.collapsed[node.ID]
	density := ParseDensity(g.config.Density)

	// Calculate available width
	glyphWidth := len(glyphs)
	iconWidth := len(icon) + 1 // icon + space (icon can be 1-2 chars with WQ prefix)
	idWidth := len(node.ID) + 1 // ID + space

	// Collapsed indicator
	collapsedBadge := ""
	if isCollapsedEpic {
		childCount := len(children[node.ID])
		if childCount > 0 {
			collapsedBadge = fmt.Sprintf(" +%d", childCount)
		}
	}
	collapsedWidth := len(collapsedBadge)

	// Build content based on density
	var content strings.Builder
	content.WriteString(glyphs)
	content.WriteString(icon)
	content.WriteString(" ")
	content.WriteString(node.ID)

	switch density {
	case DensityCompact:
		// Compact: glyphs + icon + ID + deps badge + collapsed badge
		depCount := g.countBlockingDeps(node.ID)
		if depCount > 0 {
			if depCount == 1 {
				content.WriteString(" [1 dep]")
			} else {
				content.WriteString(fmt.Sprintf(" [%d deps]", depCount))
			}
		}
		content.WriteString(collapsedBadge)

	case DensityDetailed:
		// Detailed: glyphs + icon + ID + priority + title + cost/attempts
		priority := priorityLabel(node.Priority)
		priorityWidth := len(priority) + 1 // priority + space

		// Build metrics suffix
		metrics := ""
		if node.Cost > 0 || node.Attempts > 0 {
			metrics = fmt.Sprintf(" $%.2f/%da", node.Cost, node.Attempts)
		}
		metricsWidth := len(metrics)

		// Calculate title width
		titleWidth := width - glyphWidth - iconWidth - idWidth - priorityWidth - collapsedWidth - metricsWidth
		if titleWidth < 0 {
			titleWidth = 0
		}

		title := truncate(node.Title, titleWidth)

		content.WriteString(" ")
		content.WriteString(priority)
		content.WriteString(" ")
		content.WriteString(title)
		content.WriteString(collapsedBadge)
		content.WriteString(metrics)

	default: // DensityStandard
		// Standard: glyphs + icon + ID + truncated title + deps badge
		depCount := g.countBlockingDeps(node.ID)
		badge := ""
		if depCount > 0 {
			if depCount == 1 {
				badge = " [1 dep]"
			} else {
				badge = fmt.Sprintf(" [%d deps]", depCount)
			}
		}
		badgeWidth := len(badge)

		// Calculate title width
		titleWidth := width - glyphWidth - iconWidth - idWidth - badgeWidth - collapsedWidth
		if titleWidth < 0 {
			titleWidth = 0
		}

		title := truncate(node.Title, titleWidth)

		content.WriteString(" ")
		content.WriteString(title)
		content.WriteString(collapsedBadge)
		content.WriteString(badge)
	}

	// Pad to full width
	line := content.String()
	if len(line) < width {
		line += strings.Repeat(" ", width-len(line))
	} else if len(line) > width {
		line = line[:width]
	}

	// Check if node is in active top-level subtree
	inActiveSubtree := g.isInActiveTopLevelSubtree(node.ID)

	// Apply styling (priority: current > selected > abandoned > failed > dimmed > default)
	style := graphStyles.Node
	if isCurrent {
		style = graphStyles.NodeCurrent
	} else if isSelected {
		style = graphStyles.NodeSelected
	} else if node.WQStatus == "abandoned" {
		style = graphStyles.NodeAbandoned
	} else if node.WQStatus == "failed" && node.InBackoff {
		style = graphStyles.NodeFailed
	} else if node.OutOfView || node.OutOfScope || !inActiveSubtree {
		style = graphStyles.NodeDimmed
	}

	return style.Render(line)
}

// countBlockingDeps counts the number of blocking dependencies for a node.
func (g *Graph) countBlockingDeps(nodeID string) int {
	count := 0
	for _, edge := range g.edges {
		if edge.To == nodeID && edge.Type == EdgeDependency {
			count++
		}
	}
	return count
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
	parentID := g.getParent(nodeID)
	if parentID == "" {
		return true // Root nodes are always visible
	}
	if g.collapsed[parentID] {
		return false
	}
	return g.isNodeVisible(parentID)
}

// SetEpicFilter sets the epic filter. Nodes outside the epic's subtree will be
// marked as OutOfScope and rendered with dimmed styling.
func (g *Graph) SetEpicFilter(epicID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.epicFilter = epicID
}

// GetEpicFilter returns the current epic filter ID (empty if none).
func (g *Graph) GetEpicFilter() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.epicFilter
}

// SetActiveTopLevel sets the active top-level item for subtree highlighting.
// Nodes in the active subtree will be rendered normally, while nodes outside
// will be dimmed (similar to epic filter but for highlighting, not filtering).
func (g *Graph) SetActiveTopLevel(topLevelID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.activeTopLevel = topLevelID
}

// GetActiveTopLevel returns the current active top-level ID (empty if none).
func (g *Graph) GetActiveTopLevel() string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.activeTopLevel
}

// IsInActiveTopLevelSubtree returns true if the given node is part of the
// active top-level item's subtree. Returns true for all nodes if no active
// top-level is set. Must be called with mu held.
func (g *Graph) isInActiveTopLevelSubtree(nodeID string) bool {
	if g.activeTopLevel == "" {
		return true // No active top-level, all nodes are considered "in subtree"
	}
	descendants := g.computeTopLevelDescendants()
	return descendants[nodeID]
}

// computeTopLevelDescendants builds a set of node IDs that are descendants of the
// active top-level item (including the top-level itself). Uses iterative expansion.
// Must be called with mu held.
func (g *Graph) computeTopLevelDescendants() map[string]bool {
	if g.activeTopLevel == "" {
		return nil
	}

	// Build parent map from hierarchy edges
	parentOf := make(map[string]string)
	for _, edge := range g.edges {
		if edge.Type == EdgeHierarchy {
			parentOf[edge.To] = edge.From
		}
	}

	descendants := map[string]bool{g.activeTopLevel: true}

	for {
		added := false
		for id := range g.nodes {
			parent := parentOf[id]
			if parent == "" {
				continue
			}
			if descendants[parent] && !descendants[id] {
				descendants[id] = true
				added = true
			}
		}
		if !added {
			break
		}
	}

	return descendants
}

// computeEpicDescendants builds a set of node IDs that are descendants of the
// epic filter (including the epic itself). Uses iterative expansion: starting with
// the epic, repeatedly add any node whose parent (via hierarchy edge) is already in the set.
// Must be called with mu held.
func (g *Graph) computeEpicDescendants() map[string]bool {
	if g.epicFilter == "" {
		return nil
	}

	// Build parent map from hierarchy edges
	parentOf := make(map[string]string)
	for _, edge := range g.edges {
		if edge.Type == EdgeHierarchy {
			parentOf[edge.To] = edge.From
		}
	}

	descendants := map[string]bool{g.epicFilter: true}

	for {
		added := false
		for id := range g.nodes {
			parent := parentOf[id]
			if parent == "" {
				continue
			}
			if descendants[parent] && !descendants[id] {
				descendants[id] = true
				added = true
			}
		}
		if !added {
			break
		}
	}

	return descendants
}

