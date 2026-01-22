// Package tui provides terminal user interface components for the atari daemon.
package tui

// EdgeType indicates the relationship type between nodes.
type EdgeType int

const (
	// EdgeHierarchy represents parent-child relationships (solid line).
	EdgeHierarchy EdgeType = iota
	// EdgeDependency represents blocking dependencies (dashed line).
	EdgeDependency
)

// String returns a string representation of the EdgeType.
func (e EdgeType) String() string {
	switch e {
	case EdgeHierarchy:
		return "hierarchy"
	case EdgeDependency:
		return "dependency"
	default:
		return "unknown"
	}
}

// GraphView indicates which set of beads to display.
type GraphView int

const (
	// ViewActive shows open, in_progress, and blocked beads.
	ViewActive GraphView = iota
	// ViewBacklog shows deferred beads.
	ViewBacklog
	// ViewClosed shows beads closed in the last 7 days.
	ViewClosed
)

// String returns a string representation of the GraphView.
func (v GraphView) String() string {
	switch v {
	case ViewActive:
		return "active"
	case ViewBacklog:
		return "backlog"
	case ViewClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// LayoutDirection indicates how nodes should be arranged.
type LayoutDirection int

const (
	// LayoutTopDown arranges nodes vertically (root at top).
	LayoutTopDown LayoutDirection = iota
	// LayoutLeftRight arranges nodes horizontally (root at left).
	LayoutLeftRight
)

// String returns a string representation of the LayoutDirection.
func (d LayoutDirection) String() string {
	switch d {
	case LayoutTopDown:
		return "top-down"
	case LayoutLeftRight:
		return "left-right"
	default:
		return "unknown"
	}
}

// GraphNode represents a bead in the graph visualization.
type GraphNode struct {
	ID        string
	Title     string
	Status    string  // open, in_progress, blocked, deferred
	Priority  int     // 0=critical, 1=high, 2=normal, 3=low, 4=backlog
	Type      string  // epic, task, bug, etc.
	Parent    string  // Parent epic ID for hierarchy edges
	IsEpic    bool    // True if this is an epic
	Cost      float64 // Accumulated cost
	Attempts  int     // Number of work attempts
	OutOfView bool    // True if node is from a different view (e.g., closed dep in Active view)
	OutOfScope bool   // True if node is outside epic filter scope

	// Workqueue state overlay (populated from BeadStateGetter)
	WQStatus  string // "", "failed", "abandoned" - workqueue status
	InBackoff bool   // True if bead is currently in backoff period
}

// GraphEdge represents a relationship between two nodes.
type GraphEdge struct {
	From string   // Source node ID
	To   string   // Target node ID
	Type EdgeType // Hierarchy or dependency
}

// Viewport represents the visible area of the graph.
type Viewport struct {
	OffsetX int // Horizontal scroll offset
	OffsetY int // Vertical scroll offset
	Width   int // Visible width in characters
	Height  int // Visible height in rows
}

// Position represents a node's position and size in the layout.
type Position struct {
	X int // Left position
	Y int // Top position
	W int // Width
	H int // Height
}

// GraphBead represents the raw bead data from bd list --json.
// This is used by the fetcher before conversion to GraphNode.
type GraphBead struct {
	ID              string          `json:"id"`
	Title           string          `json:"title"`
	Description     string          `json:"description"`
	Status          string          `json:"status"`
	Priority        int             `json:"priority"`
	IssueType       string          `json:"issue_type"`
	CreatedAt       string          `json:"created_at"`
	CreatedBy       string          `json:"created_by"`
	UpdatedAt       string          `json:"updated_at"`
	ClosedAt        string          `json:"closed_at,omitempty"`
	Parent          string          `json:"parent,omitempty"`
	Notes           string          `json:"notes,omitempty"`
	Labels          []string        `json:"labels,omitempty"`
	DependencyCount int             `json:"dependency_count,omitempty"`
	DependentCount  int             `json:"dependent_count,omitempty"`
	Dependencies    []BeadReference `json:"dependencies,omitempty"`
	Dependents      []BeadReference `json:"dependents,omitempty"`
}

// BeadReference represents a reference to another bead in dependencies/dependents.
type BeadReference struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Status         string `json:"status"`
	DependencyType string `json:"dependency_type"` // "parent-child" or "blocks"
}

// ToNode converts a GraphBead to a GraphNode for visualization.
func (b *GraphBead) ToNode() GraphNode {
	return GraphNode{
		ID:       b.ID,
		Title:    b.Title,
		Status:   b.Status,
		Priority: b.Priority,
		Type:     b.IssueType,
		Parent:   b.Parent,
		IsEpic:   b.IssueType == "epic",
		Cost:     0, // Cost tracking not yet implemented
		Attempts: 0, // Attempt tracking not yet implemented
	}
}

// ExtractEdges extracts graph edges from the bead's dependencies.
// Returns edges where this bead is the target (dependencies point TO this bead).
func (b *GraphBead) ExtractEdges() []GraphEdge {
	var edges []GraphEdge

	for _, dep := range b.Dependencies {
		edgeType := EdgeDependency
		if dep.DependencyType == "parent-child" {
			edgeType = EdgeHierarchy
		}
		edges = append(edges, GraphEdge{
			From: dep.ID,
			To:   b.ID,
			Type: edgeType,
		})
	}

	return edges
}

// NodeDensity represents the level of detail shown for nodes.
type NodeDensity int

const (
	// DensityCompact shows minimal info (ID only).
	DensityCompact NodeDensity = iota
	// DensityStandard shows ID and truncated title.
	DensityStandard
	// DensityDetailed shows ID, title, status, and priority.
	DensityDetailed
)

// String returns a string representation of the NodeDensity.
func (d NodeDensity) String() string {
	switch d {
	case DensityCompact:
		return "compact"
	case DensityStandard:
		return "standard"
	case DensityDetailed:
		return "detailed"
	default:
		return "unknown"
	}
}

// ParseDensity converts a string to NodeDensity.
func ParseDensity(s string) NodeDensity {
	switch s {
	case "compact":
		return DensityCompact
	case "detailed":
		return DensityDetailed
	default:
		return DensityStandard
	}
}

// ListNode represents a node in the list view with its tree position.
type ListNode struct {
	ID       string // Node ID
	Depth    int    // Tree depth (0 = root)
	ParentID string // Immediate parent ID for selection recovery
	Visible  bool   // Whether this node should be rendered
}

