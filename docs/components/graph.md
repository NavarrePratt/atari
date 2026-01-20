# Bead Graph Visualizer

Interactive TUI pane for visualizing bead relationships, status, and hierarchy.

## Purpose

The graph visualizer provides real-time visibility into project bead structure:

- View all active beads (open, in_progress, blocked) in a navigable graph
- See epic/task hierarchy and blocking dependencies at a glance
- Highlight the currently processing bead
- Navigate and inspect individual beads without leaving the TUI

**Primary use cases**:
- Understand project structure and dependencies
- Monitor progress across multiple epics
- Identify blocked work and dependency chains

## Interface

```go
// Graph manages the bead graph visualization state.
type Graph struct {
    config   *config.GraphConfig
    fetcher  BeadFetcher

    mu       sync.RWMutex
    nodes    map[string]*GraphNode
    edges    []GraphEdge
    layout   *Layout
    selected string // Selected node ID
    viewport Viewport
    collapsed map[string]bool // Collapsed epic IDs
}

type GraphConfig struct {
    Enabled             bool          // Default: true
    Density             string        // "compact", "standard", "detailed" (default: "standard")
    Layout              string        // "horizontal" or "vertical" (inherited from TUI config)
    RefreshOnEvent      bool          // Auto-refresh on BeadStatusEvent (default: false)
    AutoRefreshInterval time.Duration // Interval-based refresh (0 = disabled, min 1s)
}

type GraphNode struct {
    ID           string
    Title        string
    Status       string // open, in_progress, blocked, closed
    Priority     int
    Type         string // epic, task, bug, etc.
    Parent       string // Parent epic ID (empty for top-level)
    IsEpic       bool
    Cost         float64 // Total cost if tracked
    Attempts     int     // Processing attempts
}

type GraphEdge struct {
    From     string
    To       string
    Type     EdgeType // Hierarchy or Dependency
}

type EdgeType int

const (
    EdgeHierarchy  EdgeType = iota // Epic -> Task (solid line)
    EdgeDependency                  // Blocks relationship (dashed line)
)

type Viewport struct {
    OffsetX int
    OffsetY int
    Width   int
    Height  int
}

type Layout struct {
    Direction LayoutDirection // TopDown or LeftRight
    Layers    [][]string      // Node IDs organized by layer
    Positions map[string]Position
}

type LayoutDirection int

const (
    LayoutTopDown   LayoutDirection = iota // Epics at top, tasks below
    LayoutLeftRight                        // Epics on left, tasks to right
)

type Position struct {
    X int
    Y int
    W int // Node width
    H int // Node height
}

// BeadFetcher retrieves bead data from bd CLI.
type BeadFetcher interface {
    FetchActive(ctx context.Context) ([]Bead, error)   // open, in_progress, blocked
    FetchBacklog(ctx context.Context) ([]Bead, error)  // deferred, draft
}

// Public API
func NewGraph(cfg *config.GraphConfig, fetcher BeadFetcher) *Graph
func (g *Graph) Refresh(ctx context.Context) error
func (g *Graph) SetView(view GraphView)        // Active or Backlog
func (g *Graph) Select(nodeID string)
func (g *Graph) SelectNext()                   // Arrow key navigation
func (g *Graph) SelectPrev()
func (g *Graph) SelectParent()
func (g *Graph) SelectChild()
func (g *Graph) ToggleCollapse(nodeID string)  // Collapse/expand epic
func (g *Graph) GetSelected() *GraphNode
func (g *Graph) Render(width, height int) string
func (g *Graph) SetCurrentBead(beadID string)  // Highlight processing bead
```

## Dependencies

| Component | Usage |
|-----------|-------|
| config.GraphConfig | Density, layout, refresh settings |
| BeadFetcher | Data retrieval from br CLI |
| TUI model | Panel integration, keyboard routing |

External:
- `br list --json --status ...` for bead data with dependencies

## Design Decisions

### Data Source

Bead data is fetched via `br list --json` with status filtering:

```bash
# Active view (default)
br list --json --status open --status in_progress --status blocked

# Backlog view
br list --json --status deferred
```

Each bead includes:
- `dependencies` array with full nested objects
- `dependency_type` field: `blocks`, `parent-child`, `relates_to`
- `parent` field for epic membership

### Graph Construction

1. Fetch filtered beads via `br list --json --status ...`
2. Build node map from returned beads
3. Extract hierarchy edges from `parent` field
4. Extract dependency edges from `dependencies` array (where `dependency_type == "blocks"`)
5. Compute layout layers (epics at layer 0, tasks by dependency depth)

### Layout Algorithm

**Top-down layout** (when TUI split is horizontal - panels are tall):
```
    [Epic A]              [Epic B]
       |                     |
  +----+----+           +----+----+
  |         |           |         |
[Task 1] [Task 2]    [Task 3] [Task 4]
             \          /
              [Task 5]
```

**Left-right layout** (when TUI split is vertical - panels are wide):
```
[Epic A] --- [Task 1]
         \-- [Task 2] --\
                         [Task 5]
[Epic B] --- [Task 3] --/
         \-- [Task 4]
```

Layout direction is determined by the TUI split configuration:
- `layout: horizontal` (panels side-by-side) -> graph uses top-down
- `layout: vertical` (panels stacked) -> graph uses left-right

### Node Rendering

Three density levels, cycled with `d`:

**Compact**:
```
+------------+
| bd-xxx  o  |
+------------+
```

**Standard** (default):
```
+------------------------+
| bd-xxx  o              |
| Fix the login bug      |
+------------------------+
```

**Detailed**:
```
+------------------------+
| bd-xxx  o  P1          |
| Fix the login bug      |
| attempts: 2  $0.42     |
+------------------------+
```

Status indicators:
- `o` = open
- `*` = in_progress (highlighted)
- `x` = blocked
- `-` = deferred

### Edge Rendering

Two edge types with distinct visual styles:

**Hierarchy edges** (epic -> task): Solid lines
```
[Epic] ---- [Task]
```

**Dependency edges** (blocks): Dashed lines
```
[Task A] - - - [Task B]
```

Box-drawing characters for terminals:
- Solid: `─`, `│`, `┌`, `┐`, `└`, `┘`, `├`, `┤`, `┬`, `┴`, `┼`
- Dashed: `╌`, `╎` (or `-`, `|` for compatibility)

### Views

Two views toggled with `a`:

**Active view** (default): Shows beads with status open, in_progress, or blocked.
These are beads that are currently relevant to the drain.

**Backlog view**: Shows beads with status deferred.
These are beads that have been pushed to the backlog.

### Collapsible Epics

Epics can be collapsed to hide their child tasks:

```
Expanded:               Collapsed:
[Epic A]               [Epic A +3]
   |
+--+--+
|     |
[T1] [T2]
```

The `+3` indicator shows the count of hidden children.

Toggle collapse with `c` when an epic is selected.

### Detail Modal

Pressing `Enter` on a selected node opens a modal with full bead details:

```
+-- bd-drain-abc -------------------------------------------+
|                                                           |
| Title: Fix the authentication bug in login flow           |
| Status: in_progress    Priority: 1    Type: bug           |
|                                                           |
| Description:                                              |
| The login flow fails when users have special characters   |
| in their password. This is caused by improper escaping    |
| in the auth handler.                                      |
|                                                           |
| Dependencies:                                             |
|   - bd-drain-xyz (blocks) - "Add input validation"        |
|                                                           |
| Notes:                                                    |
| COMPLETED: Added escaping for special chars               |
| IN PROGRESS: Testing edge cases                           |
|                                                           |
| Created: 2026-01-02 by npratt                             |
| Updated: 2026-01-02                                       |
|                                                           |
+-- [Enter/Esc] close  [o] open in bd ----------------------+
```

Modal behavior:
- Takes ~90% of the graph pane area
- `Esc` or `Enter` to close
- `o` to open `br show <id>` in external terminal (future)

### Current Bead Highlighting

The bead currently being processed by atari is visually highlighted:

```
+------------------------+
| bd-xxx  *  <-- ACTIVE  |  <- Bright/bold styling
| Fix the login bug      |
+------------------------+
```

The graph receives `SetCurrentBead(beadID)` calls from the controller
when iterations start/end.

### Scrolling and Navigation

**Viewport scrolling**: When the graph is larger than the available space,
the viewport scrolls to keep the selected node visible.

**Navigation keys**:
- Arrow keys: Move selection between adjacent nodes
- `h/j/k/l`: Vim-style navigation (optional)
- Selection follows graph structure (parent/child/sibling relationships)

**Auto-scroll**: When selection changes, viewport adjusts to keep
the selected node centered (or at least visible).

## Configuration

```yaml
graph:
  enabled: true
  density: standard           # compact, standard, detailed
  refresh_on_event: false     # Auto-refresh on bead status changes
  auto_refresh_interval: 0    # Interval-based refresh (0 = disabled, min 1s)
```

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `enabled` | bool | true | Enable graph panel in TUI |
| `density` | string | "standard" | Node detail level |
| `refresh_on_event` | bool | false | Auto-refresh on BeadStatusEvent |
| `auto_refresh_interval` | duration | 0 | Interval for auto-refresh (0 = disabled, minimum 1s) |

Layout direction is inherited from the TUI `layout` setting.

### Auto-Refresh Behavior

When `auto_refresh_interval` is configured:

- Refresh triggers at the specified interval (e.g., `5s`, `10s`)
- Refresh only occurs when the graph pane is visible
- Existing loading guard prevents overlapping requests
- Manual refresh with `R` key remains available
- Minimum interval is enforced at 1 second to prevent excessive load

## TUI Integration

### Panel System

The graph is one of three toggleable panels:

| Key | Action |
|-----|--------|
| `e` | Toggle events panel |
| `o` | Toggle observer panel |
| `g` | Toggle graph panel |
| `E` | Focus events (fullscreen) |
| `O` | Focus observer (fullscreen) |
| `G` | Focus graph (fullscreen) |
| `Esc` | Exit focus mode |
| `Tab` | Cycle focus between visible panels |

Panel ordering: Panels appear in the order they were enabled.
First enabled = leftmost (horizontal) or topmost (vertical).

### Graph-Specific Keys

| Key | Action |
|-----|--------|
| `Arrow keys` | Navigate between nodes |
| `Enter` | Open detail modal for selected node |
| `a` | Toggle Active/Backlog view |
| `c` | Collapse/expand selected epic |
| `d` | Cycle density (compact/standard/detailed) |
| `r` | Refresh graph data |

### All Panels Disabled

When all panels are disabled (events, observer, graph), the TUI shows
an expanded stats view:

```
+-- ATARI -------------------------------------------------+
|                                                          |
|  Status: WORKING                                         |
|  Uptime: 2h 15m                                          |
|                                                          |
|  Current Bead: bd-drain-abc                              |
|  Title: Fix the authentication bug                       |
|  Started: 3m 42s ago                                     |
|                                                          |
|  Session Stats:                                          |
|    Beads completed: 5                                    |
|    Beads failed: 1                                       |
|    Total cost: $4.23                                     |
|    Total turns: 127                                      |
|                                                          |
|  Press [e] events  [o] observer  [g] graph               |
|                                                          |
+----------------------------------------------------------+
```

## Implementation

### Fetching Beads

```go
type BRFetcher struct {
    cmdRunner CommandRunner
}

func (f *BRFetcher) FetchActive(ctx context.Context) ([]Bead, error) {
    args := []string{"list", "--json", "--status", "open", "--status", "in_progress", "--status", "blocked"}
    output, err := f.cmdRunner.Run(ctx, "br", args...)
    if err != nil {
        return nil, fmt.Errorf("br list failed: %w", err)
    }

    var beads []Bead
    if err := json.Unmarshal(output, &beads); err != nil {
        return nil, fmt.Errorf("parse beads: %w", err)
    }
    return beads, nil
}

func (f *BRFetcher) FetchBacklog(ctx context.Context) ([]Bead, error) {
    args := []string{"list", "--json", "--status", "deferred"}
    // ... same pattern
}
```

### Building the Graph

```go
func (g *Graph) buildFromBeads(beads []Bead) {
    g.nodes = make(map[string]*GraphNode)
    g.edges = nil

    // Build nodes
    for _, b := range beads {
        g.nodes[b.ID] = &GraphNode{
            ID:       b.ID,
            Title:    b.Title,
            Status:   b.Status,
            Priority: b.Priority,
            Type:     b.IssueType,
            Parent:   b.Parent,
            IsEpic:   b.IssueType == "epic",
        }
    }

    // Build edges from parent relationships (hierarchy)
    for _, b := range beads {
        if b.Parent != "" {
            g.edges = append(g.edges, GraphEdge{
                From: b.Parent,
                To:   b.ID,
                Type: EdgeHierarchy,
            })
        }
    }

    // Build edges from dependencies (blocks)
    for _, b := range beads {
        for _, dep := range b.Dependencies {
            if dep.DependencyType == "blocks" {
                g.edges = append(g.edges, GraphEdge{
                    From: dep.ID,
                    To:   b.ID,
                    Type: EdgeDependency,
                })
            }
        }
    }

    // Compute layout
    g.computeLayout()
}
```

### Layout Computation

```go
func (g *Graph) computeLayout() {
    g.layout = &Layout{
        Direction: g.layoutDirection(),
        Positions: make(map[string]Position),
    }

    // Find root nodes (epics with no parent, or nodes with no incoming hierarchy edges)
    roots := g.findRoots()

    // Assign layers using BFS
    layers := g.assignLayers(roots)
    g.layout.Layers = layers

    // Position nodes within layers
    g.positionNodes(layers)
}

func (g *Graph) assignLayers(roots []string) [][]string {
    layers := [][]string{roots}
    visited := make(map[string]bool)
    for _, r := range roots {
        visited[r] = true
    }

    for len(layers[len(layers)-1]) > 0 {
        var nextLayer []string
        for _, nodeID := range layers[len(layers)-1] {
            // Find children (hierarchy edges where From == nodeID)
            for _, edge := range g.edges {
                if edge.From == nodeID && edge.Type == EdgeHierarchy {
                    if !visited[edge.To] {
                        visited[edge.To] = true
                        nextLayer = append(nextLayer, edge.To)
                    }
                }
            }
        }
        if len(nextLayer) == 0 {
            break
        }
        layers = append(layers, nextLayer)
    }

    return layers
}
```

### Rendering

```go
func (g *Graph) Render(width, height int) string {
    var buf strings.Builder

    // Create a 2D grid for the graph
    grid := newGrid(width, height)

    // Render nodes
    for id, pos := range g.layout.Positions {
        node := g.nodes[id]
        if g.isCollapsed(node.Parent) {
            continue // Skip children of collapsed epics
        }

        // Adjust for viewport offset
        x := pos.X - g.viewport.OffsetX
        y := pos.Y - g.viewport.OffsetY

        if x >= 0 && x < width && y >= 0 && y < height {
            nodeStr := g.renderNode(node, pos.W, pos.H)
            grid.place(x, y, nodeStr)
        }
    }

    // Render edges
    for _, edge := range g.edges {
        g.renderEdge(grid, edge)
    }

    return grid.String()
}

func (g *Graph) renderNode(node *GraphNode, w, h int) string {
    style := g.nodeStyle(node)

    switch g.config.Density {
    case "compact":
        return style.Render(fmt.Sprintf(" %s %s ", node.ID, g.statusIcon(node.Status)))
    case "detailed":
        return style.Render(fmt.Sprintf(" %s %s P%d \n %s \n attempts: %d  $%.2f ",
            node.ID, g.statusIcon(node.Status), node.Priority,
            truncate(node.Title, w-4),
            node.Attempts, node.Cost))
    default: // standard
        return style.Render(fmt.Sprintf(" %s %s \n %s ",
            node.ID, g.statusIcon(node.Status),
            truncate(node.Title, w-4)))
    }
}

func (g *Graph) statusIcon(status string) string {
    switch status {
    case "open":
        return "o"
    case "in_progress":
        return "*"
    case "blocked":
        return "x"
    case "deferred":
        return "-"
    default:
        return "?"
    }
}
```

## Testing

### Unit Tests (`internal/tui/graph_test.go`)

- `TestNewGraph`: Graph construction with configuration
- `TestGraph_BuildFromBeads`: Node and edge extraction
- `TestGraph_ComputeLayout`: Layer assignment and positioning
- `TestGraph_Navigation`: Selection movement (next/prev/parent/child)
- `TestGraph_Collapse`: Epic collapse/expand behavior
- `TestGraph_Render_Compact`: Compact density rendering
- `TestGraph_Render_Standard`: Standard density rendering
- `TestGraph_Render_Detailed`: Detailed density rendering
- `TestGraph_Viewport`: Scrolling and viewport adjustment
- `TestGraph_StatusIcons`: Status indicator mapping
- `TestGraph_EdgeRendering`: Hierarchy vs dependency line styles
- `TestGraph_CurrentBeadHighlight`: Active bead styling

### Integration Tests (`internal/integration/graph_test.go`)

- `TestGraphFetchActive`: br list integration for active beads
- `TestGraphFetchBacklog`: br list integration for backlog
- `TestGraphViewToggle`: Active/backlog view switching
- `TestGraphRefresh`: Manual refresh behavior
- `TestGraphWithRealBeads`: End-to-end with mock br
- `TestGraphPanelToggle`: TUI panel system integration
- `TestGraphFocusMode`: Fullscreen graph mode
- `TestGraphDetailModal`: Modal open/close behavior

### Test Infrastructure

- `internal/testutil/mock_br.go`: Mock br CLI for graph tests
- `internal/testutil/graph_fixtures.go`: Sample bead data for tests

## Error Handling

| Error | Action |
|-------|--------|
| br not available | Show error message in graph pane |
| No beads found | Show "No active beads" message |
| Parse error | Log warning, show partial graph |
| Layout overflow | Enable scrolling, show viewport indicator |

## Future Considerations

- **Real-time updates**: Subscribe to BeadStatusEvents for live graph updates
- **Filtering**: Filter by label, assignee, or priority
- **Search**: Jump to node by ID or title substring
- **Export**: Export graph as ASCII, SVG, or DOT format
- **Mini-map**: Small overview showing position in large graphs
- **Keyboard hints**: Show available navigation keys on nodes
