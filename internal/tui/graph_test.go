package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/npratt/atari/internal/config"
)

// mockFetcher implements BeadFetcher for testing.
type mockFetcher struct {
	activeBeads  []GraphBead
	backlogBeads []GraphBead
	closedBeads  []GraphBead
	beadByID     map[string]GraphBead
	activeErr    error
	backlogErr   error
	closedErr    error
	beadErr      error
}

func (m *mockFetcher) FetchActive(ctx context.Context) ([]GraphBead, error) {
	if m.activeErr != nil {
		return nil, m.activeErr
	}
	return m.activeBeads, nil
}

func (m *mockFetcher) FetchBacklog(ctx context.Context) ([]GraphBead, error) {
	if m.backlogErr != nil {
		return nil, m.backlogErr
	}
	return m.backlogBeads, nil
}

func (m *mockFetcher) FetchClosed(ctx context.Context) ([]GraphBead, error) {
	if m.closedErr != nil {
		return nil, m.closedErr
	}
	return m.closedBeads, nil
}

func (m *mockFetcher) FetchBead(ctx context.Context, id string) (*GraphBead, error) {
	if m.beadErr != nil {
		return nil, m.beadErr
	}
	if m.beadByID != nil {
		if bead, ok := m.beadByID[id]; ok {
			return &bead, nil
		}
	}
	// Search in active beads
	for _, b := range m.activeBeads {
		if b.ID == id {
			return &b, nil
		}
	}
	// Search in backlog beads
	for _, b := range m.backlogBeads {
		if b.ID == id {
			return &b, nil
		}
	}
	// Search in closed beads
	for _, b := range m.closedBeads {
		if b.ID == id {
			return &b, nil
		}
	}
	return nil, errors.New("bead not found")
}

func defaultGraphConfig() *config.GraphConfig {
	return &config.GraphConfig{
		Enabled:        true,
		Density:        "standard",
		RefreshOnEvent: false,
	}
}

func TestNewGraph(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{}

	g := NewGraph(cfg, fetcher, "horizontal")

	if g == nil {
		t.Fatal("NewGraph returned nil")
	}
	if g.config != cfg {
		t.Error("config not set")
	}
	if g.fetcher != fetcher {
		t.Error("fetcher not set")
	}
	if g.layout != "horizontal" {
		t.Errorf("layout = %q, want horizontal", g.layout)
	}
	if g.view != ViewActive {
		t.Errorf("view = %v, want ViewActive", g.view)
	}
}

func TestGraph_Refresh_Active(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-001", Title: "Task 1", Status: "open", IssueType: "task"},
			{ID: "bd-002", Title: "Task 2", Status: "in_progress", IssueType: "task"},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	err := g.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	if g.NodeCount() != 2 {
		t.Errorf("NodeCount = %d, want 2", g.NodeCount())
	}
}

func TestGraph_Refresh_Backlog(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		backlogBeads: []GraphBead{
			{ID: "bd-003", Title: "Deferred Task", Status: "deferred", IssueType: "task"},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	g.SetView(ViewBacklog)

	err := g.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	if g.NodeCount() != 1 {
		t.Errorf("NodeCount = %d, want 1", g.NodeCount())
	}
}

func TestGraph_Refresh_Error(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeErr: errors.New("fetch failed"),
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	err := g.Refresh(context.Background())
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestGraph_BuildFromBeads(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{
				ID:        "bd-epic",
				Title:     "Epic",
				Status:    "open",
				IssueType: "epic",
			},
			{
				ID:        "bd-task-1",
				Title:     "Task 1",
				Status:    "open",
				IssueType: "task",
				Dependencies: []BeadReference{
					{ID: "bd-epic", DependencyType: "parent-child"},
				},
			},
			{
				ID:        "bd-task-2",
				Title:     "Task 2",
				Status:    "blocked",
				IssueType: "task",
				Dependencies: []BeadReference{
					{ID: "bd-epic", DependencyType: "parent-child"},
					{ID: "bd-task-1", DependencyType: "blocks"},
				},
			},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	err := g.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// Check nodes
	if g.NodeCount() != 3 {
		t.Errorf("NodeCount = %d, want 3", g.NodeCount())
	}

	// Check edges: 2 hierarchy + 1 dependency = 3
	if g.EdgeCount() != 3 {
		t.Errorf("EdgeCount = %d, want 3", g.EdgeCount())
	}

	// Verify edge types
	edges := g.GetEdges()
	hierarchyCount := 0
	dependencyCount := 0
	for _, e := range edges {
		switch e.Type {
		case EdgeHierarchy:
			hierarchyCount++
		case EdgeDependency:
			dependencyCount++
		}
	}
	if hierarchyCount != 2 {
		t.Errorf("hierarchy edges = %d, want 2", hierarchyCount)
	}
	if dependencyCount != 1 {
		t.Errorf("dependency edges = %d, want 1", dependencyCount)
	}
}

func TestGraph_ComputeLayout(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-epic", Title: "Epic", Status: "open", IssueType: "epic"},
			{
				ID:        "bd-task-1",
				Title:     "Task 1",
				Status:    "open",
				IssueType: "task",
				Dependencies: []BeadReference{
					{ID: "bd-epic", DependencyType: "parent-child"},
				},
			},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	err := g.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	layout := g.GetLayout()
	if layout == nil {
		t.Fatal("layout is nil")
	}

	// Should have 2 layers: epic at layer 0, task at layer 1
	if len(layout.Layers) != 2 {
		t.Errorf("layers = %d, want 2", len(layout.Layers))
	}

	// Check direction
	if layout.Direction != LayoutTopDown {
		t.Errorf("Direction = %v, want LayoutTopDown", layout.Direction)
	}

	// Check positions exist
	if len(layout.Positions) != 2 {
		t.Errorf("positions = %d, want 2", len(layout.Positions))
	}
}

func TestGraph_ComputeLayout_LeftRight(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-001", Title: "Task", Status: "open", IssueType: "task"},
		},
	}

	g := NewGraph(cfg, fetcher, "vertical")
	err := g.Refresh(context.Background())
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	layout := g.GetLayout()
	if layout.Direction != LayoutLeftRight {
		t.Errorf("Direction = %v, want LayoutLeftRight", layout.Direction)
	}
}

func TestGraph_Select(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-001", Title: "Task 1", Status: "open", IssueType: "task"},
			{ID: "bd-002", Title: "Task 2", Status: "open", IssueType: "task"},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	g.Select("bd-002")

	if g.GetSelectedID() != "bd-002" {
		t.Errorf("selected = %q, want bd-002", g.GetSelectedID())
	}

	node := g.GetSelected()
	if node == nil {
		t.Fatal("GetSelected returned nil")
	}
	if node.ID != "bd-002" {
		t.Errorf("selected node ID = %q, want bd-002", node.ID)
	}
}

func TestGraph_Select_Invalid(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-001", Title: "Task 1", Status: "open", IssueType: "task"},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}
	g.Select("bd-001")

	// Try to select non-existent node
	g.Select("bd-nonexistent")

	// Should still have bd-001 selected
	if g.GetSelectedID() != "bd-001" {
		t.Errorf("selected = %q, want bd-001", g.GetSelectedID())
	}
}

func TestGraph_SelectNext(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-001", Title: "Task 1", Status: "open", IssueType: "task"},
			{ID: "bd-002", Title: "Task 2", Status: "open", IssueType: "task"},
			{ID: "bd-003", Title: "Task 3", Status: "open", IssueType: "task"},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}
	g.Select("bd-001")

	g.SelectNext()
	if g.GetSelectedID() != "bd-002" {
		t.Errorf("after SelectNext: selected = %q, want bd-002", g.GetSelectedID())
	}

	g.SelectNext()
	if g.GetSelectedID() != "bd-003" {
		t.Errorf("after SelectNext: selected = %q, want bd-003", g.GetSelectedID())
	}

	// Wrap around
	g.SelectNext()
	if g.GetSelectedID() != "bd-001" {
		t.Errorf("after SelectNext (wrap): selected = %q, want bd-001", g.GetSelectedID())
	}
}

func TestGraph_SelectPrev(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-001", Title: "Task 1", Status: "open", IssueType: "task"},
			{ID: "bd-002", Title: "Task 2", Status: "open", IssueType: "task"},
			{ID: "bd-003", Title: "Task 3", Status: "open", IssueType: "task"},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}
	g.Select("bd-002")

	g.SelectPrev()
	if g.GetSelectedID() != "bd-001" {
		t.Errorf("after SelectPrev: selected = %q, want bd-001", g.GetSelectedID())
	}

	// Wrap around
	g.SelectPrev()
	if g.GetSelectedID() != "bd-003" {
		t.Errorf("after SelectPrev (wrap): selected = %q, want bd-003", g.GetSelectedID())
	}
}

func TestGraph_SelectParent(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-epic", Title: "Epic", Status: "open", IssueType: "epic"},
			{
				ID:        "bd-task",
				Title:     "Task",
				Status:    "open",
				IssueType: "task",
				Dependencies: []BeadReference{
					{ID: "bd-epic", DependencyType: "parent-child"},
				},
			},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}
	g.Select("bd-task")

	g.SelectParent()
	if g.GetSelectedID() != "bd-epic" {
		t.Errorf("after SelectParent: selected = %q, want bd-epic", g.GetSelectedID())
	}
}

func TestGraph_SelectChild(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-epic", Title: "Epic", Status: "open", IssueType: "epic"},
			{
				ID:        "bd-task",
				Title:     "Task",
				Status:    "open",
				IssueType: "task",
				Dependencies: []BeadReference{
					{ID: "bd-epic", DependencyType: "parent-child"},
				},
			},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}
	g.Select("bd-epic")

	g.SelectChild()
	if g.GetSelectedID() != "bd-task" {
		t.Errorf("after SelectChild: selected = %q, want bd-task", g.GetSelectedID())
	}
}

func TestGraph_ToggleCollapse(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-epic", Title: "Epic", Status: "open", IssueType: "epic"},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	if g.IsCollapsed("bd-epic") {
		t.Error("epic should not be collapsed initially")
	}

	g.ToggleCollapse("bd-epic")
	if !g.IsCollapsed("bd-epic") {
		t.Error("epic should be collapsed after toggle")
	}

	g.ToggleCollapse("bd-epic")
	if g.IsCollapsed("bd-epic") {
		t.Error("epic should not be collapsed after second toggle")
	}
}

func TestGraph_ToggleCollapse_NonEpic(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-task", Title: "Task", Status: "open", IssueType: "task"},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// Should be no-op for non-epics
	g.ToggleCollapse("bd-task")
	if g.IsCollapsed("bd-task") {
		t.Error("non-epic should not be collapsible")
	}
}

func TestGraph_SelectChild_Collapsed(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-epic", Title: "Epic", Status: "open", IssueType: "epic"},
			{
				ID:        "bd-task",
				Title:     "Task",
				Status:    "open",
				IssueType: "task",
				Dependencies: []BeadReference{
					{ID: "bd-epic", DependencyType: "parent-child"},
				},
			},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}
	g.Select("bd-epic")
	g.ToggleCollapse("bd-epic")

	// SelectChild should not work when epic is collapsed
	g.SelectChild()
	if g.GetSelectedID() != "bd-epic" {
		t.Errorf("should stay on epic when collapsed, got %q", g.GetSelectedID())
	}
}

func TestGraph_SetCurrentBead(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{}

	g := NewGraph(cfg, fetcher, "horizontal")

	if g.GetCurrentBead() != "" {
		t.Errorf("initial currentBead = %q, want empty", g.GetCurrentBead())
	}

	g.SetCurrentBead("bd-001")
	if g.GetCurrentBead() != "bd-001" {
		t.Errorf("currentBead = %q, want bd-001", g.GetCurrentBead())
	}

	g.SetCurrentBead("")
	if g.GetCurrentBead() != "" {
		t.Errorf("currentBead after clear = %q, want empty", g.GetCurrentBead())
	}
}

func TestGraph_SetView(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{}

	g := NewGraph(cfg, fetcher, "horizontal")

	if g.GetView() != ViewActive {
		t.Errorf("initial view = %v, want ViewActive", g.GetView())
	}

	g.SetView(ViewBacklog)
	if g.GetView() != ViewBacklog {
		t.Errorf("view = %v, want ViewBacklog", g.GetView())
	}
}

func TestGraph_SetViewport(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{}

	g := NewGraph(cfg, fetcher, "horizontal")
	g.SetViewport(100, 50)

	vp := g.GetViewport()
	if vp.Width != 100 {
		t.Errorf("viewport width = %d, want 100", vp.Width)
	}
	if vp.Height != 50 {
		t.Errorf("viewport height = %d, want 50", vp.Height)
	}
}

func TestGraph_MissingDependencies(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{
				ID:        "bd-task",
				Title:     "Task",
				Status:    "blocked",
				IssueType: "task",
				Dependencies: []BeadReference{
					{ID: "bd-missing-1", DependencyType: "blocks"},
					{ID: "bd-missing-2", DependencyType: "blocks"},
				},
			},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// Should have 2 nodes: the task and the pseudo-node
	if g.NodeCount() != 2 {
		t.Errorf("NodeCount = %d, want 2", g.NodeCount())
	}

	nodes := g.GetNodes()
	pseudo := nodes["_hidden_deps"]
	if pseudo == nil {
		t.Fatal("pseudo-node not created")
	}
	if pseudo.Title != "2 deps hidden" {
		t.Errorf("pseudo-node title = %q, want '2 deps hidden'", pseudo.Title)
	}
}

func TestGraph_ChildCount(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-epic", Title: "Epic", Status: "open", IssueType: "epic"},
			{
				ID:        "bd-task-1",
				Title:     "Task 1",
				Status:    "open",
				IssueType: "task",
				Dependencies: []BeadReference{
					{ID: "bd-epic", DependencyType: "parent-child"},
				},
			},
			{
				ID:        "bd-task-2",
				Title:     "Task 2",
				Status:    "open",
				IssueType: "task",
				Dependencies: []BeadReference{
					{ID: "bd-epic", DependencyType: "parent-child"},
				},
			},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	count := g.ChildCount("bd-epic")
	if count != 2 {
		t.Errorf("ChildCount = %d, want 2", count)
	}

	count = g.ChildCount("bd-task-1")
	if count != 0 {
		t.Errorf("ChildCount for task = %d, want 0", count)
	}
}

func TestGraph_AutoSelectFirstNode(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-002", Title: "Task 2", Status: "open", IssueType: "task"},
			{ID: "bd-001", Title: "Task 1", Status: "open", IssueType: "task"},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// Should auto-select first node in first layer (sorted)
	selected := g.GetSelectedID()
	if selected == "" {
		t.Error("no node auto-selected")
	}
}

func TestGraph_NodeDimensions(t *testing.T) {
	tests := []struct {
		density string
		wantW   int
		wantH   int
	}{
		{"compact", 16, 1},
		{"standard", 26, 2},
		{"detailed", 26, 3},
	}

	for _, tt := range tests {
		t.Run(tt.density, func(t *testing.T) {
			cfg := &config.GraphConfig{Density: tt.density}
			fetcher := &mockFetcher{}
			g := NewGraph(cfg, fetcher, "horizontal")

			w, h := g.nodeDimensions()
			if w != tt.wantW {
				t.Errorf("width = %d, want %d", w, tt.wantW)
			}
			if h != tt.wantH {
				t.Errorf("height = %d, want %d", h, tt.wantH)
			}
		})
	}
}

func TestPluralize(t *testing.T) {
	tests := []struct {
		count    int
		singular string
		plural   string
		want     string
	}{
		{0, "dep hidden", "deps hidden", "0 deps hidden"},
		{1, "dep hidden", "deps hidden", "1 dep hidden"},
		{2, "dep hidden", "deps hidden", "2 deps hidden"},
		{10, "item", "items", "10 items"},
	}

	for _, tt := range tests {
		got := pluralize(tt.count, tt.singular, tt.plural)
		if got != tt.want {
			t.Errorf("pluralize(%d, %q, %q) = %q, want %q", tt.count, tt.singular, tt.plural, got, tt.want)
		}
	}
}

func TestStatusIcon(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"open", "o"},
		{"in_progress", "*"},
		{"blocked", "x"},
		{"deferred", "-"},
		{"closed", "."},
		{"unknown", "?"},
		{"", "?"},
	}

	for _, tt := range tests {
		got := statusIcon(tt.status)
		if got != tt.want {
			t.Errorf("statusIcon(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestPriorityLabel(t *testing.T) {
	tests := []struct {
		priority int
		want     string
	}{
		{0, "P0"},
		{1, "P1"},
		{2, "P2"},
		{3, "P3"},
		{4, "P4"},
		{-1, "P?"},
		{5, "P?"},
	}

	for _, tt := range tests {
		got := priorityLabel(tt.priority)
		if got != tt.want {
			t.Errorf("priorityLabel(%d) = %q, want %q", tt.priority, got, tt.want)
		}
	}
}

func TestGraph_CycleDensity(t *testing.T) {
	cfg := &config.GraphConfig{Density: "compact"}
	fetcher := &mockFetcher{}

	g := NewGraph(cfg, fetcher, "horizontal")

	// Start with compact
	if g.GetDensity() != DensityCompact {
		t.Errorf("initial density = %v, want DensityCompact", g.GetDensity())
	}

	// Cycle to standard
	g.CycleDensity()
	if g.GetDensity() != DensityStandard {
		t.Errorf("after first cycle = %v, want DensityStandard", g.GetDensity())
	}

	// Cycle to detailed
	g.CycleDensity()
	if g.GetDensity() != DensityDetailed {
		t.Errorf("after second cycle = %v, want DensityDetailed", g.GetDensity())
	}

	// Cycle back to compact
	g.CycleDensity()
	if g.GetDensity() != DensityCompact {
		t.Errorf("after third cycle = %v, want DensityCompact", g.GetDensity())
	}
}

func TestGraph_Render_Empty(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{}

	g := NewGraph(cfg, fetcher, "horizontal")
	output := g.Render(40, 5)

	if output == "" {
		t.Error("Render returned empty string for empty graph")
	}
	if !strings.Contains(output, "No beads") {
		t.Errorf("expected empty message, got %q", output)
	}
}

func TestGraph_Render_SingleNode(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-001", Title: "Test Task", Status: "open", IssueType: "task"},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	output := g.Render(60, 10)

	if output == "" {
		t.Error("Render returned empty string")
	}
	if !strings.Contains(output, "bd-001") {
		t.Errorf("output should contain node ID, got %q", output)
	}
}

func TestGraph_Render_WithCurrentBead(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-001", Title: "Task 1", Status: "open", IssueType: "task"},
			{ID: "bd-002", Title: "Task 2", Status: "in_progress", IssueType: "task"},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	g.SetCurrentBead("bd-002")
	output := g.Render(60, 10)

	// Should contain both nodes
	if !strings.Contains(output, "bd-001") || !strings.Contains(output, "bd-002") {
		t.Errorf("output should contain both nodes, got %q", output)
	}
}

func TestGraph_Render_CollapsedEpic(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-epic", Title: "Epic", Status: "open", IssueType: "epic"},
			{
				ID:        "bd-task",
				Title:     "Child Task",
				Status:    "open",
				IssueType: "task",
				Dependencies: []BeadReference{
					{ID: "bd-epic", DependencyType: "parent-child"},
				},
			},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// Before collapse: both visible
	output := g.Render(60, 10)
	if !strings.Contains(output, "bd-task") {
		t.Errorf("child should be visible before collapse")
	}

	// Collapse epic
	g.ToggleCollapse("bd-epic")
	output = g.Render(60, 10)

	// Epic should show "+1" indicator
	if !strings.Contains(output, "+1") {
		t.Errorf("collapsed epic should show child count indicator, got %q", output)
	}
}

func TestGraph_Render_DensityLevels(t *testing.T) {
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-001", Title: "A Longer Task Title", Status: "open", Priority: 1, IssueType: "task"},
		},
	}

	tests := []struct {
		density  string
		contains []string
	}{
		{"compact", []string{"bd-001", "o"}},
		{"standard", []string{"bd-001", "o", "A Longer"}},
		{"detailed", []string{"bd-001", "o", "P1"}},
	}

	for _, tt := range tests {
		t.Run(tt.density, func(t *testing.T) {
			cfg := &config.GraphConfig{Density: tt.density}
			g := NewGraph(cfg, fetcher, "horizontal")
			if err := g.Refresh(context.Background()); err != nil {
				t.Fatalf("Refresh failed: %v", err)
			}

			output := g.Render(60, 10)
			for _, want := range tt.contains {
				if !strings.Contains(output, want) {
					t.Errorf("density %s: output should contain %q, got %q", tt.density, want, output)
				}
			}
		})
	}
}

func TestCharGrid(t *testing.T) {
	grid := newGrid(10, 5)

	if grid.width != 10 || grid.height != 5 {
		t.Errorf("grid dimensions = %dx%d, want 10x5", grid.width, grid.height)
	}

	// Write a rune
	grid.writeRune(0, 0, 'X')
	if grid.cells[0][0] != 'X' {
		t.Errorf("cell[0][0] = %c, want X", grid.cells[0][0])
	}

	// Write a string
	grid.writeString(2, 1, "Hello")
	if string(grid.cells[1][2:7]) != "Hello" {
		t.Errorf("row 1 = %q, want 'Hello' at position 2", string(grid.cells[1]))
	}

	// Boundary check: out of bounds writes should be ignored
	grid.writeRune(-1, 0, 'Y')
	grid.writeRune(100, 0, 'Z')
	// No panic is success

	// String output
	output := grid.String()
	lines := strings.Split(output, "\n")
	if len(lines) != 5 {
		t.Errorf("String() returned %d lines, want 5", len(lines))
	}
}

func TestGraph_Render_ViewportClipping(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-001", Title: "Task 1", Status: "open", IssueType: "task"},
			{ID: "bd-002", Title: "Task 2", Status: "open", IssueType: "task"},
			{ID: "bd-003", Title: "Task 3", Status: "open", IssueType: "task"},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// Set a small viewport that might clip some nodes
	g.SetViewport(30, 5)

	// Render should not panic with small viewport
	output := g.Render(30, 5)
	if output == "" {
		t.Error("Render returned empty string")
	}
}

func TestGraph_FormatNodeCompact(t *testing.T) {
	cfg := &config.GraphConfig{Density: "compact"}
	fetcher := &mockFetcher{}
	g := NewGraph(cfg, fetcher, "horizontal")

	node := &GraphNode{
		ID:     "bd-test",
		Title:  "Test Node",
		Status: "in_progress",
	}

	text := g.formatNodeCompact(node, false, 0)
	if text != "bd-test *" {
		t.Errorf("formatNodeCompact = %q, want 'bd-test *'", text)
	}

	// With collapsed indicator
	text = g.formatNodeCompact(node, true, 3)
	if text != "bd-test * +3" {
		t.Errorf("formatNodeCompact (collapsed) = %q, want 'bd-test * +3'", text)
	}
}

func TestGraph_FormatNodeStandard(t *testing.T) {
	cfg := &config.GraphConfig{Density: "standard"}
	fetcher := &mockFetcher{}
	g := NewGraph(cfg, fetcher, "horizontal")

	node := &GraphNode{
		ID:     "bd-test",
		Title:  "A Very Long Title That Gets Truncated",
		Status: "blocked",
	}

	text := g.formatNodeStandard(node, false, 0)
	if !strings.Contains(text, "bd-test") || !strings.Contains(text, "x") {
		t.Errorf("formatNodeStandard should contain ID and status icon, got %q", text)
	}
	if !strings.Contains(text, "...") {
		t.Errorf("formatNodeStandard should truncate long title, got %q", text)
	}
}

func TestGraph_FormatNodeDetailed(t *testing.T) {
	cfg := &config.GraphConfig{Density: "detailed"}
	fetcher := &mockFetcher{}
	g := NewGraph(cfg, fetcher, "horizontal")

	node := &GraphNode{
		ID:       "bd-test",
		Title:    "Test Node",
		Status:   "open",
		Priority: 1,
		Attempts: 2,
		Cost:     1.50,
	}

	text := g.formatNodeDetailed(node, false, 0)
	if !strings.Contains(text, "bd-test") {
		t.Errorf("should contain ID, got %q", text)
	}
	if !strings.Contains(text, "P1") {
		t.Errorf("should contain priority, got %q", text)
	}
	if !strings.Contains(text, "[2 $1.50]") {
		t.Errorf("should contain attempts and cost, got %q", text)
	}
}

func TestGraph_GetVisibleNodes_WithCollapsedEpic(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-epic", Title: "Epic", Status: "open", IssueType: "epic"},
			{
				ID:        "bd-task-1",
				Title:     "Task 1",
				Status:    "open",
				IssueType: "task",
				Dependencies: []BeadReference{
					{ID: "bd-epic", DependencyType: "parent-child"},
				},
			},
			{
				ID:        "bd-task-2",
				Title:     "Task 2",
				Status:    "open",
				IssueType: "task",
				Dependencies: []BeadReference{
					{ID: "bd-epic", DependencyType: "parent-child"},
				},
			},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// Before collapse
	g.mu.RLock()
	visible := g.getVisibleNodes()
	g.mu.RUnlock()
	if len(visible) != 3 {
		t.Errorf("visible nodes before collapse = %d, want 3", len(visible))
	}

	// After collapse
	g.ToggleCollapse("bd-epic")
	g.mu.RLock()
	visible = g.getVisibleNodes()
	g.mu.RUnlock()
	if len(visible) != 1 {
		t.Errorf("visible nodes after collapse = %d, want 1 (just epic)", len(visible))
	}
}

func TestCharGrid_StyledString_CorrectPositions(t *testing.T) {
	// This test verifies that writeStyledString places characters at correct
	// positions without ANSI escape code corruption. Previously, formatNode()
	// returned pre-styled strings with ANSI codes, and writeString() would
	// iterate over those codes as individual runes, corrupting positions.
	grid := newGrid(20, 3)

	// Write styled text at position (2, 1)
	style := graphStyles.NodeSelected
	grid.writeStyledString(2, 1, "bd-001 o", style)

	// Verify characters are at correct positions
	expectedChars := []struct {
		x    int
		char rune
	}{
		{2, 'b'},
		{3, 'd'},
		{4, '-'},
		{5, '0'},
		{6, '0'},
		{7, '1'},
		{8, ' '},
		{9, 'o'},
	}

	for _, tc := range expectedChars {
		if grid.cells[1][tc.x] != tc.char {
			t.Errorf("cell[1][%d] = %c, want %c", tc.x, grid.cells[1][tc.x], tc.char)
		}
	}

	// Verify style is stored for each character
	for x := 2; x <= 9; x++ {
		if grid.styles[1][x].Render("x") != style.Render("x") {
			t.Errorf("style not stored at position [1][%d]", x)
		}
	}

	// Verify spaces outside the text remain unstyled
	if grid.cells[1][0] != ' ' || grid.cells[1][1] != ' ' {
		t.Error("leading spaces should remain")
	}
}

func TestGraph_FormatNode_ReturnsPlainText(t *testing.T) {
	// Verify that formatNode returns plain text without ANSI escape codes.
	// The style is returned separately and applied by charGrid.String().
	cfg := &config.GraphConfig{Density: "compact"}
	fetcher := &mockFetcher{}
	g := NewGraph(cfg, fetcher, "horizontal")

	node := &GraphNode{
		ID:     "bd-test",
		Title:  "Test",
		Status: "open",
	}

	text, _ := g.formatNode(node, 20, false, true, false, 0)

	// Text should not contain ANSI escape codes
	if strings.Contains(text, "\x1b[") {
		t.Errorf("formatNode returned text with ANSI codes: %q", text)
	}

	// Text should be plain
	expected := "bd-test o"
	if text != expected {
		t.Errorf("formatNode text = %q, want %q", text, expected)
	}

	// Style is a lipgloss.Style - verify it was returned (non-zero value type)
	// Note: lipgloss styling in tests may not produce ANSI codes in non-TTY environments
}

func TestGraph_Render_NoPositionCorruption(t *testing.T) {
	// Regression test: verify that styled nodes render at correct positions
	// without corruption from ANSI escape codes.
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-001", Title: "Task 1", Status: "open", IssueType: "task"},
			{ID: "bd-002", Title: "Task 2", Status: "in_progress", IssueType: "task"},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// Verify nodes were loaded
	if g.NodeCount() != 2 {
		t.Fatalf("expected 2 nodes, got %d", g.NodeCount())
	}

	// Set viewport dimensions (required for Select to work correctly)
	g.SetViewport(80, 10)

	// Set current bead to test current highlighting
	g.SetCurrentBead("bd-001")
	g.Select("bd-002")

	output := g.Render(80, 10)

	// Both node IDs should appear in the output as complete strings
	// If ANSI codes corrupted positions, IDs would be fragmented
	if !strings.Contains(output, "bd-001") {
		t.Errorf("output missing bd-001 (current bead)")
	}
	if !strings.Contains(output, "bd-002") {
		t.Errorf("output missing bd-002 (selected)")
	}

	// Count occurrences - each ID should appear exactly once
	if strings.Count(output, "bd-001") != 1 {
		t.Errorf("bd-001 should appear exactly once, got %d", strings.Count(output, "bd-001"))
	}
	if strings.Count(output, "bd-002") != 1 {
		t.Errorf("bd-002 should appear exactly once, got %d", strings.Count(output, "bd-002"))
	}
}

func TestGraph_ConcurrentRebuildAndRender(t *testing.T) {
	// This test verifies that concurrent calls to RebuildFromBeads and Render
	// do not cause data races. Run with -race to detect issues.
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{}
	g := NewGraph(cfg, fetcher, "horizontal")

	beadSets := [][]GraphBead{
		{
			{ID: "bd-001", Title: "Task 1", Status: "open", IssueType: "task"},
			{ID: "bd-002", Title: "Task 2", Status: "in_progress", IssueType: "task"},
		},
		{
			{ID: "bd-003", Title: "Task 3", Status: "blocked", IssueType: "task"},
		},
		{
			{ID: "bd-epic", Title: "Epic", Status: "open", IssueType: "epic"},
			{
				ID:        "bd-task",
				Title:     "Child",
				Status:    "open",
				IssueType: "task",
				Dependencies: []BeadReference{
					{ID: "bd-epic", DependencyType: "parent-child"},
				},
			},
		},
	}

	done := make(chan struct{})
	const iterations = 100

	// Writer goroutine: repeatedly rebuilds with different bead sets
	go func() {
		defer close(done)
		for i := 0; i < iterations; i++ {
			beads := beadSets[i%len(beadSets)]
			g.RebuildFromBeads(beads)
		}
	}()

	// Reader goroutines: repeatedly render and access graph state
	for i := 0; i < 3; i++ {
		go func() {
			for {
				select {
				case <-done:
					return
				default:
					g.Render(60, 20)
					g.GetNodes()
					g.GetEdges()
					g.NodeCount()
					g.GetLayout()
					g.GetSelectedID()
				}
			}
		}()
	}

	// Navigation goroutine: repeatedly navigates selection
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				g.SelectNext()
				g.SelectPrev()
				g.SelectParent()
				g.SelectChild()
			}
		}
	}()

	<-done
}
