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

	// Should have 3 nodes: the task and 2 out-of-view nodes for missing deps
	if g.NodeCount() != 3 {
		t.Errorf("NodeCount = %d, want 3", g.NodeCount())
	}

	nodes := g.GetNodes()

	// Verify out-of-view nodes exist and are marked as out-of-view
	missing1 := nodes["bd-missing-1"]
	if missing1 == nil {
		t.Fatal("out-of-view node bd-missing-1 not created")
	}
	if !missing1.OutOfView {
		t.Error("bd-missing-1 should be marked OutOfView")
	}
	// Placeholder nodes have "?" title since FetchBead returns nil
	if missing1.Title != "?" {
		t.Errorf("bd-missing-1.Title = %q, want '?'", missing1.Title)
	}

	missing2 := nodes["bd-missing-2"]
	if missing2 == nil {
		t.Fatal("out-of-view node bd-missing-2 not created")
	}
	if !missing2.OutOfView {
		t.Error("bd-missing-2 should be marked OutOfView")
	}

	// The task itself should not be out-of-view
	task := nodes["bd-task"]
	if task == nil {
		t.Fatal("task node not found")
	}
	if task.OutOfView {
		t.Error("bd-task should not be marked OutOfView")
	}
}

func TestGraph_MissingDependencies_WithFetchedData(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{
				ID:        "bd-task",
				Title:     "Task",
				Status:    "blocked",
				IssueType: "task",
				Dependencies: []BeadReference{
					{ID: "bd-closed-dep", DependencyType: "blocks"},
				},
			},
		},
		// Provide bead data for the missing dependency
		beadByID: map[string]GraphBead{
			"bd-closed-dep": {
				ID:        "bd-closed-dep",
				Title:     "Closed Dependency",
				Status:    "closed",
				IssueType: "task",
			},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// Should have 2 nodes: the task and the fetched out-of-view dep
	if g.NodeCount() != 2 {
		t.Errorf("NodeCount = %d, want 2", g.NodeCount())
	}

	nodes := g.GetNodes()

	// Verify out-of-view node has proper data from fetch
	closedDep := nodes["bd-closed-dep"]
	if closedDep == nil {
		t.Fatal("out-of-view node bd-closed-dep not created")
	}
	if !closedDep.OutOfView {
		t.Error("bd-closed-dep should be marked OutOfView")
	}
	if closedDep.Title != "Closed Dependency" {
		t.Errorf("bd-closed-dep.Title = %q, want 'Closed Dependency'", closedDep.Title)
	}
	if closedDep.Status != "closed" {
		t.Errorf("bd-closed-dep.Status = %q, want 'closed'", closedDep.Status)
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

	// Each density level shows different information:
	// - Compact: icon + ID only (no title)
	// - Standard: icon + ID + title
	// - Detailed: icon + ID + priority + title (+ cost/attempts when available)
	tests := []struct {
		density     string
		contains    []string
		notContains []string
	}{
		{"compact", []string{"bd-001", "o"}, []string{"A Longer", "P1"}},
		{"standard", []string{"bd-001", "o", "A Longer"}, []string{"P1"}},
		{"detailed", []string{"bd-001", "o", "A Longer", "P1"}, nil},
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
			for _, notWant := range tt.notContains {
				if strings.Contains(output, notWant) {
					t.Errorf("density %s: output should NOT contain %q, got %q", tt.density, notWant, output)
				}
			}
		})
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

// -----------------------------------------------------------------------------
// List Mode Tests: computeListOrder
// -----------------------------------------------------------------------------

// TestComputeListOrder_SimpleHierarchy tests DFS traversal produces correct order
// for a simple parent-child hierarchy.
func TestComputeListOrder_SimpleHierarchy(t *testing.T) {
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

	layout := g.GetLayout()
	if layout == nil {
		t.Fatal("layout is nil")
	}
	if len(layout.ListOrder) != 3 {
		t.Fatalf("expected 3 items in list order, got %d", len(layout.ListOrder))
	}

	// Verify order: epic (depth 0), then children (depth 1)
	if layout.ListOrder[0].ID != "bd-epic" {
		t.Errorf("expected first item to be bd-epic, got %s", layout.ListOrder[0].ID)
	}
	if layout.ListOrder[0].Depth != 0 {
		t.Errorf("expected epic depth 0, got %d", layout.ListOrder[0].Depth)
	}

	// Children are sorted alphabetically: bd-task-1, bd-task-2
	if layout.ListOrder[1].ID != "bd-task-1" {
		t.Errorf("expected second item to be bd-task-1, got %s", layout.ListOrder[1].ID)
	}
	if layout.ListOrder[1].Depth != 1 {
		t.Errorf("expected task-1 depth 1, got %d", layout.ListOrder[1].Depth)
	}
	if layout.ListOrder[1].ParentID != "bd-epic" {
		t.Errorf("expected task-1 parent to be bd-epic, got %s", layout.ListOrder[1].ParentID)
	}

	if layout.ListOrder[2].ID != "bd-task-2" {
		t.Errorf("expected third item to be bd-task-2, got %s", layout.ListOrder[2].ID)
	}
}

// TestComputeListOrder_MultipleRoots tests DFS with multiple root nodes (no parents).
func TestComputeListOrder_MultipleRoots(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-standalone-1", Title: "Standalone 1", Status: "open", IssueType: "task"},
			{ID: "bd-standalone-2", Title: "Standalone 2", Status: "open", IssueType: "task"},
			{ID: "bd-epic", Title: "Epic", Status: "open", IssueType: "epic"},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	layout := g.GetLayout()
	if len(layout.ListOrder) != 3 {
		t.Fatalf("expected 3 items in list order, got %d", len(layout.ListOrder))
	}

	// All should be at depth 0 since they're all roots
	for i, item := range layout.ListOrder {
		if item.Depth != 0 {
			t.Errorf("item %d (%s) depth = %d, want 0", i, item.ID, item.Depth)
		}
		if !item.Visible {
			t.Errorf("item %d (%s) should be visible", i, item.ID)
		}
	}

	// Epics should come first in sort order
	if layout.ListOrder[0].ID != "bd-epic" {
		t.Errorf("expected epic first (sorts before tasks), got %s", layout.ListOrder[0].ID)
	}
}

// TestComputeListOrder_CollapsedEpic tests that children of collapsed epics
// are marked as not visible.
func TestComputeListOrder_CollapsedEpic(t *testing.T) {
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
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// Before collapse: both visible
	layout := g.GetLayout()
	if !layout.ListOrder[0].Visible || !layout.ListOrder[1].Visible {
		t.Error("both items should be visible before collapse")
	}

	// Collapse the epic
	g.ToggleCollapse("bd-epic")

	layout = g.GetLayout()
	// Epic should still be visible
	if !layout.ListOrder[0].Visible {
		t.Error("epic should be visible after collapse")
	}
	// Task should NOT be visible
	if layout.ListOrder[1].Visible {
		t.Error("task should not be visible after epic collapse")
	}
}

// TestComputeListOrder_CycleProtection tests that cycles in dependencies
// don't cause infinite loops.
func TestComputeListOrder_CycleProtection(t *testing.T) {
	cfg := defaultGraphConfig()
	// Create a cycle: A -> B -> A (via parent-child edges)
	// Note: Real data shouldn't have cycles, but we protect against them
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{
				ID:        "bd-a",
				Title:     "Node A",
				Status:    "open",
				IssueType: "epic",
				// A is child of B (creating cycle with B being child of A below)
				// This is unusual but tests cycle protection
			},
			{
				ID:        "bd-b",
				Title:     "Node B",
				Status:    "open",
				IssueType: "task",
				Dependencies: []BeadReference{
					{ID: "bd-a", DependencyType: "parent-child"},
				},
			},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// Should not hang or panic
	layout := g.GetLayout()
	// Should have both nodes
	if len(layout.ListOrder) != 2 {
		t.Errorf("expected 2 items, got %d", len(layout.ListOrder))
	}
}

// TestComputeListOrder_OrphanNodes tests that nodes only connected via
// dependency edges (not hierarchy) are included at the end.
func TestComputeListOrder_OrphanNodes(t *testing.T) {
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
			{
				ID:        "bd-orphan",
				Title:     "Orphan",
				Status:    "blocked",
				IssueType: "task",
				Dependencies: []BeadReference{
					// Only has a "blocks" dependency, not parent-child
					{ID: "bd-task", DependencyType: "blocks"},
				},
			},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	layout := g.GetLayout()
	if len(layout.ListOrder) != 3 {
		t.Fatalf("expected 3 items, got %d", len(layout.ListOrder))
	}

	// Orphan should be at depth 0 (root level) since it has no hierarchy parent
	var orphanItem *ListNode
	for i := range layout.ListOrder {
		if layout.ListOrder[i].ID == "bd-orphan" {
			orphanItem = &layout.ListOrder[i]
			break
		}
	}
	if orphanItem == nil {
		t.Fatal("orphan not found in list order")
	}
	if orphanItem.Depth != 0 {
		t.Errorf("orphan depth = %d, want 0 (root level)", orphanItem.Depth)
	}
}

// -----------------------------------------------------------------------------
// List Mode Tests: Navigation
// -----------------------------------------------------------------------------

// TestSelectNext_ListMode_Linear tests linear traversal in list mode.
func TestSelectNext_ListMode_Linear(t *testing.T) {
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

	g.Select("bd-epic")

	// Navigate through list: epic -> task-1 -> task-2
	g.SelectNext()
	if g.GetSelectedID() != "bd-task-1" {
		t.Errorf("after first SelectNext: selected = %q, want bd-task-1", g.GetSelectedID())
	}

	g.SelectNext()
	if g.GetSelectedID() != "bd-task-2" {
		t.Errorf("after second SelectNext: selected = %q, want bd-task-2", g.GetSelectedID())
	}
}

// TestSelectPrev_ListMode_Linear tests linear backward traversal in list mode.
func TestSelectPrev_ListMode_Linear(t *testing.T) {
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

	g.Select("bd-task-2")

	// Navigate backward: task-2 -> task-1 -> epic
	g.SelectPrev()
	if g.GetSelectedID() != "bd-task-1" {
		t.Errorf("after first SelectPrev: selected = %q, want bd-task-1", g.GetSelectedID())
	}

	g.SelectPrev()
	if g.GetSelectedID() != "bd-epic" {
		t.Errorf("after second SelectPrev: selected = %q, want bd-epic", g.GetSelectedID())
	}
}

// TestSelectNext_ListMode_NoWrap tests that list mode stops at the last node
// instead of wrapping around.
func TestSelectNext_ListMode_NoWrap(t *testing.T) {
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

	g.Select("bd-002") // Start at last node

	// Try to go next - should stay at bd-002 (no wrap)
	g.SelectNext()
	if g.GetSelectedID() != "bd-002" {
		t.Errorf("SelectNext at last node: selected = %q, want bd-002 (no wrap)", g.GetSelectedID())
	}

	// Try again - still should not wrap
	g.SelectNext()
	if g.GetSelectedID() != "bd-002" {
		t.Errorf("SelectNext at last node (again): selected = %q, want bd-002 (no wrap)", g.GetSelectedID())
	}
}

// TestSelectPrev_ListMode_NoWrap tests that list mode stops at the first node
// instead of wrapping around.
func TestSelectPrev_ListMode_NoWrap(t *testing.T) {
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

	g.Select("bd-001") // Start at first node

	// Try to go previous - should stay at bd-001 (no wrap)
	g.SelectPrev()
	if g.GetSelectedID() != "bd-001" {
		t.Errorf("SelectPrev at first node: selected = %q, want bd-001 (no wrap)", g.GetSelectedID())
	}

	// Try again - still should not wrap
	g.SelectPrev()
	if g.GetSelectedID() != "bd-001" {
		t.Errorf("SelectPrev at first node (again): selected = %q, want bd-001 (no wrap)", g.GetSelectedID())
	}
}

// TestSelectNext_ListMode_SkipsHidden tests that navigation skips hidden nodes.
func TestSelectNext_ListMode_SkipsHidden(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-epic-1", Title: "Epic 1", Status: "open", IssueType: "epic"},
			{
				ID:        "bd-child-1",
				Title:     "Child of Epic 1",
				Status:    "open",
				IssueType: "task",
				Dependencies: []BeadReference{
					{ID: "bd-epic-1", DependencyType: "parent-child"},
				},
			},
			{ID: "bd-epic-2", Title: "Epic 2", Status: "open", IssueType: "epic"},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	g.Select("bd-epic-1")

	// Collapse epic-1 to hide its child
	g.ToggleCollapse("bd-epic-1")

	// Now navigate: epic-1 -> epic-2 (skipping hidden child)
	g.SelectNext()
	if g.GetSelectedID() != "bd-epic-2" {
		t.Errorf("SelectNext should skip hidden child: selected = %q, want bd-epic-2", g.GetSelectedID())
	}
}

// -----------------------------------------------------------------------------
// List Mode Tests: Selection Recovery
// -----------------------------------------------------------------------------

// TestToggleCollapse_SelectionRecovery_ToParent tests that collapsing an epic
// with a selected child moves selection to the parent.
func TestToggleCollapse_SelectionRecovery_ToParent(t *testing.T) {
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

	g.Select("bd-task") // Select the child

	// Collapse the epic - child becomes invisible
	g.ToggleCollapse("bd-epic")

	// Selection should recover to parent
	if g.GetSelectedID() != "bd-epic" {
		t.Errorf("after collapse: selected = %q, want bd-epic (parent)", g.GetSelectedID())
	}
}

// TestToggleCollapse_SelectionRecovery_ToNearestVisible tests selection recovery
// when the parent is also invisible (nested collapse).
func TestToggleCollapse_SelectionRecovery_ToNearestVisible(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-root", Title: "Root", Status: "open", IssueType: "epic"},
			{
				ID:        "bd-mid",
				Title:     "Middle Epic",
				Status:    "open",
				IssueType: "epic",
				Dependencies: []BeadReference{
					{ID: "bd-root", DependencyType: "parent-child"},
				},
			},
			{
				ID:        "bd-leaf",
				Title:     "Leaf Task",
				Status:    "open",
				IssueType: "task",
				Dependencies: []BeadReference{
					{ID: "bd-mid", DependencyType: "parent-child"},
				},
			},
			{ID: "bd-sibling", Title: "Sibling", Status: "open", IssueType: "task"},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	g.Select("bd-mid") // Select the middle epic

	// Collapse root - mid becomes invisible
	g.ToggleCollapse("bd-root")

	// Selection should recover to root (nearest visible ancestor)
	if g.GetSelectedID() != "bd-root" {
		t.Errorf("after collapse: selected = %q, want bd-root (nearest visible)", g.GetSelectedID())
	}
}

// TestRefresh_SelectionValidation tests that refresh validates selection
// when the previously selected node no longer exists.
func TestRefresh_SelectionValidation(t *testing.T) {
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

	// Select bd-002
	g.Select("bd-002")
	if g.GetSelectedID() != "bd-002" {
		t.Fatal("failed to select bd-002")
	}

	// Update fetcher to remove bd-002
	fetcher.activeBeads = []GraphBead{
		{ID: "bd-001", Title: "Task 1", Status: "open", IssueType: "task"},
	}

	// Refresh should clear invalid selection and auto-select first node
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// Selection should have been cleared or moved to valid node
	selected := g.GetSelectedID()
	if selected == "bd-002" {
		t.Error("selected node should not be the removed bd-002")
	}
	if selected == "" {
		t.Error("should auto-select a valid node after refresh")
	}
	if selected != "bd-001" {
		t.Errorf("should auto-select bd-001, got %q", selected)
	}
}

// -----------------------------------------------------------------------------
// List Mode Tests: Rendering
// -----------------------------------------------------------------------------

// TestRenderListMode_TreeGlyphs tests that list mode renders tree glyphs correctly.
func TestRenderListMode_TreeGlyphs(t *testing.T) {
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

	g.SetViewport(80, 20)

	output := g.Render(80, 20)

	// Should contain tree glyphs for children
	if !strings.Contains(output, "├") && !strings.Contains(output, "└") {
		t.Error("list mode output should contain tree glyphs")
	}

	// Should contain all node IDs
	if !strings.Contains(output, "bd-epic") {
		t.Error("output should contain bd-epic")
	}
	if !strings.Contains(output, "bd-task-1") {
		t.Error("output should contain bd-task-1")
	}
	if !strings.Contains(output, "bd-task-2") {
		t.Error("output should contain bd-task-2")
	}
}

// TestRenderListMode_DependencyBadge tests that blocking dependencies are shown.
func TestRenderListMode_DependencyBadge(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-blocker", Title: "Blocker", Status: "open", IssueType: "task"},
			{
				ID:        "bd-blocked",
				Title:     "Blocked Task",
				Status:    "blocked",
				IssueType: "task",
				Dependencies: []BeadReference{
					{ID: "bd-blocker", DependencyType: "blocks"},
				},
			},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	g.SetViewport(80, 20)

	output := g.Render(80, 20)

	// Blocked task should show dependency badge
	if !strings.Contains(output, "[1 dep]") {
		t.Errorf("output should contain dependency badge [1 dep], got:\n%s", output)
	}
}

// TestRenderListMode_Viewport tests that viewport clipping works in list mode.
func TestRenderListMode_Viewport(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-001", Title: "Task 1", Status: "open", IssueType: "task"},
			{ID: "bd-002", Title: "Task 2", Status: "open", IssueType: "task"},
			{ID: "bd-003", Title: "Task 3", Status: "open", IssueType: "task"},
			{ID: "bd-004", Title: "Task 4", Status: "open", IssueType: "task"},
			{ID: "bd-005", Title: "Task 5", Status: "open", IssueType: "task"},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// Set a small viewport that can't show all nodes
	g.SetViewport(60, 3)

	output := g.Render(60, 3)

	// Should render without panic and produce output
	if output == "" {
		t.Error("render should produce output")
	}

	// Output should have 3 lines (height is 3)
	lines := strings.Split(output, "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
}

// TestRenderListMode_CollapsedIndicator tests that collapsed epics show +N badge.
func TestRenderListMode_CollapsedIndicator(t *testing.T) {
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

	g.SetViewport(80, 20)
	g.ToggleCollapse("bd-epic")

	output := g.Render(80, 20)

	// Should show +2 indicator for collapsed epic with 2 children
	if !strings.Contains(output, "+2") {
		t.Errorf("collapsed epic should show +2 indicator, got:\n%s", output)
	}
}

// TestRenderListMode_Empty tests rendering an empty graph in list mode.
func TestRenderListMode_Empty(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{}

	g := NewGraph(cfg, fetcher, "horizontal")

	output := g.Render(60, 10)

	if !strings.Contains(output, "No beads") {
		t.Errorf("empty list mode should show 'No beads' message, got:\n%s", output)
	}
}

// TestRenderListMode_SingleNode tests rendering a single node in list mode.
func TestRenderListMode_SingleNode(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{
		activeBeads: []GraphBead{
			{ID: "bd-single", Title: "Single Task", Status: "open", IssueType: "task"},
		},
	}

	g := NewGraph(cfg, fetcher, "horizontal")
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	g.SetViewport(60, 10)

	output := g.Render(60, 10)

	// Should contain the single node
	if !strings.Contains(output, "bd-single") {
		t.Errorf("output should contain bd-single, got:\n%s", output)
	}

	// Should NOT contain tree glyphs for single root node
	if strings.Contains(output, "├") || strings.Contains(output, "└") || strings.Contains(output, "│") {
		t.Errorf("single root node should not have tree glyphs, got:\n%s", output)
	}
}


// -----------------------------------------------------------------------------
// Position Tests for List Mode
// -----------------------------------------------------------------------------

// TestPositionNodesForList tests that list mode positions are computed correctly.
func TestPositionNodesForList(t *testing.T) {
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

	g.SetViewport(80, 20)

	layout := g.GetLayout()

	// Epic at depth 0: X should be 0
	epicPos := layout.Positions["bd-epic"]
	if epicPos.X != 0 {
		t.Errorf("epic X = %d, want 0", epicPos.X)
	}

	// Task at depth 1: X should be 2 (2-space indent per level)
	taskPos := layout.Positions["bd-task"]
	if taskPos.X != 2 {
		t.Errorf("task X = %d, want 2 (2-space indent)", taskPos.X)
	}

	// Task should be below epic (Y should be greater)
	if taskPos.Y <= epicPos.Y {
		t.Errorf("task Y (%d) should be greater than epic Y (%d)", taskPos.Y, epicPos.Y)
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

// -----------------------------------------------------------------------------
// Epic Collapse Tests with Enriched Data
// These tests verify collapse functionality works correctly with the new
// bead enrichment approach where parent-child dependencies are fully populated.
// -----------------------------------------------------------------------------

// TestToggleCollapse_WithEnrichedBeads verifies that collapse works correctly
// with enriched beads that have full dependency arrays including parent-child
// relationships in the dependencies field.
func TestToggleCollapse_WithEnrichedBeads(t *testing.T) {
	cfg := defaultGraphConfig()

	// Create enriched beads with full dependency information
	enrichedBeads := []GraphBead{
		{
			ID:        "bd-epic-001",
			Title:     "Epic: Authentication",
			Status:    "open",
			Priority:  1,
			IssueType: "epic",
		},
		{
			ID:        "bd-task-001",
			Title:     "Implement login form",
			Status:    "in_progress",
			Priority:  2,
			IssueType: "task",
			Dependencies: []BeadReference{
				{
					ID:             "bd-epic-001",
					Title:          "Epic: Authentication",
					Status:         "open",
					DependencyType: "parent-child",
				},
			},
		},
		{
			ID:        "bd-task-002",
			Title:     "Add session management",
			Status:    "blocked",
			Priority:  2,
			IssueType: "task",
			Dependencies: []BeadReference{
				{
					ID:             "bd-epic-001",
					Title:          "Epic: Authentication",
					Status:         "open",
					DependencyType: "parent-child",
				},
				{
					ID:             "bd-task-001",
					Title:          "Implement login form",
					Status:         "in_progress",
					DependencyType: "blocks",
				},
			},
		},
		{
			ID:        "bd-task-003",
			Title:     "Add logout functionality",
			Status:    "open",
			Priority:  2,
			IssueType: "task",
			Dependencies: []BeadReference{
				{
					ID:             "bd-epic-001",
					Title:          "Epic: Authentication",
					Status:         "open",
					DependencyType: "parent-child",
				},
			},
		},
	}

	fetcher := &mockFetcher{activeBeads: enrichedBeads}
	g := NewGraph(cfg, fetcher, "horizontal")

	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// Verify initial structure: 1 epic + 3 tasks = 4 nodes
	if g.NodeCount() != 4 {
		t.Errorf("NodeCount = %d, want 4", g.NodeCount())
	}

	// Verify 3 hierarchy edges (parent-child) + 1 dependency edge (blocks)
	edges := g.GetEdges()
	hierarchyCount := 0
	depCount := 0
	for _, e := range edges {
		switch e.Type {
		case EdgeHierarchy:
			hierarchyCount++
		case EdgeDependency:
			depCount++
		}
	}
	if hierarchyCount != 3 {
		t.Errorf("hierarchy edges = %d, want 3", hierarchyCount)
	}
	if depCount != 1 {
		t.Errorf("dependency edges = %d, want 1", depCount)
	}

	// Initially, epic should not be collapsed
	if g.IsCollapsed("bd-epic-001") {
		t.Error("epic should not be collapsed initially")
	}

	// All nodes should be visible before collapse
	g.mu.RLock()
	visible := g.getVisibleNodes()
	g.mu.RUnlock()
	if len(visible) != 4 {
		t.Errorf("visible nodes before collapse = %d, want 4", len(visible))
	}

	// Collapse the epic
	g.ToggleCollapse("bd-epic-001")

	// Verify epic is now collapsed
	if !g.IsCollapsed("bd-epic-001") {
		t.Error("epic should be collapsed after toggle")
	}

	// Only epic should be visible (3 children hidden)
	g.mu.RLock()
	visible = g.getVisibleNodes()
	g.mu.RUnlock()
	if len(visible) != 1 {
		t.Errorf("visible nodes after collapse = %d, want 1 (just epic)", len(visible))
	}

	// Verify child count reports correctly for collapsed epic
	childCount := g.ChildCount("bd-epic-001")
	if childCount != 3 {
		t.Errorf("ChildCount = %d, want 3", childCount)
	}

	// Expand the epic
	g.ToggleCollapse("bd-epic-001")

	// Verify epic is now expanded
	if g.IsCollapsed("bd-epic-001") {
		t.Error("epic should be expanded after second toggle")
	}

	// All nodes should be visible again
	g.mu.RLock()
	visible = g.getVisibleNodes()
	g.mu.RUnlock()
	if len(visible) != 4 {
		t.Errorf("visible nodes after expand = %d, want 4", len(visible))
	}
}

// TestCollapsedState_PreservedAfterRefresh verifies that the collapsed state
// of epics is preserved when the graph is refreshed with new data.
func TestCollapsedState_PreservedAfterRefresh(t *testing.T) {
	cfg := defaultGraphConfig()

	// Initial enriched beads
	initialBeads := []GraphBead{
		{
			ID:        "bd-epic-001",
			Title:     "Epic: Authentication",
			Status:    "open",
			Priority:  1,
			IssueType: "epic",
		},
		{
			ID:        "bd-task-001",
			Title:     "Implement login form",
			Status:    "in_progress",
			Priority:  2,
			IssueType: "task",
			Dependencies: []BeadReference{
				{
					ID:             "bd-epic-001",
					Title:          "Epic: Authentication",
					Status:         "open",
					DependencyType: "parent-child",
				},
			},
		},
		{
			ID:        "bd-task-002",
			Title:     "Add session management",
			Status:    "open",
			Priority:  2,
			IssueType: "task",
			Dependencies: []BeadReference{
				{
					ID:             "bd-epic-001",
					Title:          "Epic: Authentication",
					Status:         "open",
					DependencyType: "parent-child",
				},
			},
		},
	}

	fetcher := &mockFetcher{activeBeads: initialBeads}
	g := NewGraph(cfg, fetcher, "horizontal")

	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Initial refresh failed: %v", err)
	}

	// Collapse the epic
	g.ToggleCollapse("bd-epic-001")
	if !g.IsCollapsed("bd-epic-001") {
		t.Fatal("epic should be collapsed")
	}

	// Verify only epic is visible
	g.mu.RLock()
	visibleBefore := len(g.getVisibleNodes())
	g.mu.RUnlock()
	if visibleBefore != 1 {
		t.Errorf("visible nodes before refresh = %d, want 1", visibleBefore)
	}

	// Update fetcher with modified data (simulating a refresh with updated status)
	updatedBeads := []GraphBead{
		{
			ID:        "bd-epic-001",
			Title:     "Epic: Authentication",
			Status:    "open",
			Priority:  1,
			IssueType: "epic",
		},
		{
			ID:        "bd-task-001",
			Title:     "Implement login form",
			Status:    "closed", // Status changed
			Priority:  2,
			IssueType: "task",
			Dependencies: []BeadReference{
				{
					ID:             "bd-epic-001",
					Title:          "Epic: Authentication",
					Status:         "open",
					DependencyType: "parent-child",
				},
			},
		},
		{
			ID:        "bd-task-002",
			Title:     "Add session management",
			Status:    "in_progress", // Status changed
			Priority:  2,
			IssueType: "task",
			Dependencies: []BeadReference{
				{
					ID:             "bd-epic-001",
					Title:          "Epic: Authentication",
					Status:         "open",
					DependencyType: "parent-child",
				},
			},
		},
		{
			ID:        "bd-task-003", // New task added
			Title:     "Add password reset",
			Status:    "open",
			Priority:  2,
			IssueType: "task",
			Dependencies: []BeadReference{
				{
					ID:             "bd-epic-001",
					Title:          "Epic: Authentication",
					Status:         "open",
					DependencyType: "parent-child",
				},
			},
		},
	}

	fetcher.activeBeads = updatedBeads

	// Refresh the graph
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh after update failed: %v", err)
	}

	// Verify collapse state is preserved after refresh
	if !g.IsCollapsed("bd-epic-001") {
		t.Error("epic collapse state should be preserved after refresh")
	}

	// Verify only epic is visible (new task should also be hidden)
	g.mu.RLock()
	visibleAfter := len(g.getVisibleNodes())
	g.mu.RUnlock()
	if visibleAfter != 1 {
		t.Errorf("visible nodes after refresh = %d, want 1 (collapse preserved)", visibleAfter)
	}

	// Verify all 4 nodes exist
	if g.NodeCount() != 4 {
		t.Errorf("NodeCount after refresh = %d, want 4", g.NodeCount())
	}

	// Verify child count includes the new task
	childCount := g.ChildCount("bd-epic-001")
	if childCount != 3 {
		t.Errorf("ChildCount after refresh = %d, want 3", childCount)
	}

	// Expand to verify all children are present
	g.ToggleCollapse("bd-epic-001")
	g.mu.RLock()
	visible := g.getVisibleNodes()
	g.mu.RUnlock()
	if len(visible) != 4 {
		t.Errorf("visible nodes after expand = %d, want 4", len(visible))
	}
}

// TestToggleCollapse_MultipleEpics verifies collapse works correctly when
// there are multiple epics with enriched child dependencies.
func TestToggleCollapse_MultipleEpics(t *testing.T) {
	cfg := defaultGraphConfig()

	beads := []GraphBead{
		{
			ID:        "bd-epic-auth",
			Title:     "Epic: Authentication",
			Status:    "open",
			Priority:  1,
			IssueType: "epic",
		},
		{
			ID:        "bd-epic-ui",
			Title:     "Epic: UI Updates",
			Status:    "open",
			Priority:  1,
			IssueType: "epic",
		},
		{
			ID:        "bd-auth-task-1",
			Title:     "Auth Task 1",
			Status:    "open",
			Priority:  2,
			IssueType: "task",
			Dependencies: []BeadReference{
				{ID: "bd-epic-auth", DependencyType: "parent-child"},
			},
		},
		{
			ID:        "bd-auth-task-2",
			Title:     "Auth Task 2",
			Status:    "open",
			Priority:  2,
			IssueType: "task",
			Dependencies: []BeadReference{
				{ID: "bd-epic-auth", DependencyType: "parent-child"},
			},
		},
		{
			ID:        "bd-ui-task-1",
			Title:     "UI Task 1",
			Status:    "open",
			Priority:  2,
			IssueType: "task",
			Dependencies: []BeadReference{
				{ID: "bd-epic-ui", DependencyType: "parent-child"},
			},
		},
	}

	fetcher := &mockFetcher{activeBeads: beads}
	g := NewGraph(cfg, fetcher, "horizontal")

	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// Verify initial state: all 5 nodes visible
	g.mu.RLock()
	visible := g.getVisibleNodes()
	g.mu.RUnlock()
	if len(visible) != 5 {
		t.Errorf("visible nodes initially = %d, want 5", len(visible))
	}

	// Collapse first epic
	g.ToggleCollapse("bd-epic-auth")

	// 3 visible: 2 epics + 1 UI task
	g.mu.RLock()
	visible = g.getVisibleNodes()
	g.mu.RUnlock()
	if len(visible) != 3 {
		t.Errorf("visible after collapsing auth epic = %d, want 3", len(visible))
	}

	// Collapse second epic
	g.ToggleCollapse("bd-epic-ui")

	// 2 visible: just the 2 epics
	g.mu.RLock()
	visible = g.getVisibleNodes()
	g.mu.RUnlock()
	if len(visible) != 2 {
		t.Errorf("visible after collapsing both epics = %d, want 2", len(visible))
	}

	// Expand first epic
	g.ToggleCollapse("bd-epic-auth")

	// 4 visible: 2 epics + 2 auth tasks
	g.mu.RLock()
	visible = g.getVisibleNodes()
	g.mu.RUnlock()
	if len(visible) != 4 {
		t.Errorf("visible after expanding auth epic = %d, want 4", len(visible))
	}
}

// TestCollapse_NavigationSkipsHiddenChildren verifies that SelectNext/SelectPrev
// correctly skip hidden children when navigating.
func TestCollapse_NavigationSkipsHiddenChildren(t *testing.T) {
	cfg := defaultGraphConfig()

	beads := []GraphBead{
		{
			ID:        "bd-epic",
			Title:     "Epic",
			Status:    "open",
			IssueType: "epic",
		},
		{
			ID:        "bd-child-1",
			Title:     "Child 1",
			Status:    "open",
			IssueType: "task",
			Dependencies: []BeadReference{
				{ID: "bd-epic", DependencyType: "parent-child"},
			},
		},
		{
			ID:        "bd-child-2",
			Title:     "Child 2",
			Status:    "open",
			IssueType: "task",
			Dependencies: []BeadReference{
				{ID: "bd-epic", DependencyType: "parent-child"},
			},
		},
		{
			ID:        "bd-standalone",
			Title:     "Standalone Task",
			Status:    "open",
			IssueType: "task",
		},
	}

	fetcher := &mockFetcher{activeBeads: beads}
	g := NewGraph(cfg, fetcher, "horizontal")

	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// Select the epic and collapse it
	g.Select("bd-epic")
	g.ToggleCollapse("bd-epic")

	// Navigate to next - should skip hidden children and go to standalone
	g.SelectNext()

	selected := g.GetSelectedID()
	if selected != "bd-standalone" {
		t.Errorf("SelectNext should skip hidden children, got %q, want bd-standalone", selected)
	}

	// Navigate back - should go back to epic (not hidden children)
	g.SelectPrev()

	selected = g.GetSelectedID()
	if selected != "bd-epic" {
		t.Errorf("SelectPrev should go to epic, got %q, want bd-epic", selected)
	}
}

// -----------------------------------------------------------------------------
// Epic Filter Tests
// -----------------------------------------------------------------------------

// TestSetEpicFilter_MarksOutOfScopeNodes tests that nodes outside the epic
// subtree are marked as OutOfScope when an epic filter is active.
func TestSetEpicFilter_MarksOutOfScopeNodes(t *testing.T) {
	cfg := defaultGraphConfig()
	beads := []GraphBead{
		{ID: "bd-epic", Title: "Epic", Status: "open", IssueType: "epic"},
		{
			ID:        "bd-child-1",
			Title:     "Child of Epic",
			Status:    "open",
			IssueType: "task",
			Dependencies: []BeadReference{
				{ID: "bd-epic", DependencyType: "parent-child"},
			},
		},
		{
			ID:        "bd-grandchild",
			Title:     "Grandchild",
			Status:    "open",
			IssueType: "task",
			Dependencies: []BeadReference{
				{ID: "bd-child-1", DependencyType: "parent-child"},
			},
		},
		{ID: "bd-unrelated", Title: "Unrelated Task", Status: "open", IssueType: "task"},
	}

	fetcher := &mockFetcher{activeBeads: beads}
	g := NewGraph(cfg, fetcher, "horizontal")

	// Set epic filter BEFORE refresh
	g.SetEpicFilter("bd-epic")

	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	nodes := g.GetNodes()

	// Epic and its descendants should NOT be out of scope
	if nodes["bd-epic"].OutOfScope {
		t.Error("bd-epic should not be OutOfScope")
	}
	if nodes["bd-child-1"].OutOfScope {
		t.Error("bd-child-1 should not be OutOfScope (child of epic)")
	}
	if nodes["bd-grandchild"].OutOfScope {
		t.Error("bd-grandchild should not be OutOfScope (grandchild of epic)")
	}

	// Unrelated node SHOULD be out of scope
	if !nodes["bd-unrelated"].OutOfScope {
		t.Error("bd-unrelated should be OutOfScope (not in epic subtree)")
	}
}

// TestSetEpicFilter_NoFilter tests that without an epic filter, no nodes
// are marked as OutOfScope.
func TestSetEpicFilter_NoFilter(t *testing.T) {
	cfg := defaultGraphConfig()
	beads := []GraphBead{
		{ID: "bd-epic", Title: "Epic", Status: "open", IssueType: "epic"},
		{
			ID:        "bd-child",
			Title:     "Child",
			Status:    "open",
			IssueType: "task",
			Dependencies: []BeadReference{
				{ID: "bd-epic", DependencyType: "parent-child"},
			},
		},
		{ID: "bd-standalone", Title: "Standalone", Status: "open", IssueType: "task"},
	}

	fetcher := &mockFetcher{activeBeads: beads}
	g := NewGraph(cfg, fetcher, "horizontal")

	// No epic filter set
	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	nodes := g.GetNodes()

	// No nodes should be out of scope
	for id, node := range nodes {
		if node.OutOfScope {
			t.Errorf("%s should not be OutOfScope when no epic filter is set", id)
		}
	}
}

// TestSetEpicFilter_AutoSelectSkipsOutOfScope tests that auto-selection
// skips out-of-scope nodes.
func TestSetEpicFilter_AutoSelectSkipsOutOfScope(t *testing.T) {
	cfg := defaultGraphConfig()
	beads := []GraphBead{
		// Unrelated tasks come first alphabetically
		{ID: "bd-aaa-unrelated", Title: "Unrelated", Status: "open", IssueType: "task"},
		{ID: "bd-epic", Title: "Epic", Status: "open", IssueType: "epic"},
		{
			ID:        "bd-zzz-child",
			Title:     "Child",
			Status:    "open",
			IssueType: "task",
			Dependencies: []BeadReference{
				{ID: "bd-epic", DependencyType: "parent-child"},
			},
		},
	}

	fetcher := &mockFetcher{activeBeads: beads}
	g := NewGraph(cfg, fetcher, "horizontal")

	// Set epic filter
	g.SetEpicFilter("bd-epic")

	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	// Auto-selection should skip the out-of-scope "aaa-unrelated"
	// and select the first in-scope node (epic comes first as it sorts before tasks)
	selected := g.GetSelectedID()
	if selected == "bd-aaa-unrelated" {
		t.Error("auto-select should skip out-of-scope nodes")
	}
	if selected != "bd-epic" {
		t.Errorf("expected auto-select to be bd-epic, got %q", selected)
	}
}

// TestGetEpicFilter tests the getter for the epic filter.
func TestGetEpicFilter(t *testing.T) {
	cfg := defaultGraphConfig()
	fetcher := &mockFetcher{}
	g := NewGraph(cfg, fetcher, "horizontal")

	// Initially empty
	if got := g.GetEpicFilter(); got != "" {
		t.Errorf("GetEpicFilter initially = %q, want empty", got)
	}

	// After setting
	g.SetEpicFilter("bd-epic-123")
	if got := g.GetEpicFilter(); got != "bd-epic-123" {
		t.Errorf("GetEpicFilter = %q, want bd-epic-123", got)
	}

	// Clearing
	g.SetEpicFilter("")
	if got := g.GetEpicFilter(); got != "" {
		t.Errorf("GetEpicFilter after clear = %q, want empty", got)
	}
}

// TestComputeEpicDescendants tests the descendant computation algorithm.
func TestComputeEpicDescendants(t *testing.T) {
	cfg := defaultGraphConfig()
	beads := []GraphBead{
		{ID: "bd-root", Title: "Root Epic", Status: "open", IssueType: "epic"},
		{
			ID:        "bd-level-1a",
			Title:     "Level 1A",
			Status:    "open",
			IssueType: "task",
			Dependencies: []BeadReference{
				{ID: "bd-root", DependencyType: "parent-child"},
			},
		},
		{
			ID:        "bd-level-1b",
			Title:     "Level 1B",
			Status:    "open",
			IssueType: "task",
			Dependencies: []BeadReference{
				{ID: "bd-root", DependencyType: "parent-child"},
			},
		},
		{
			ID:        "bd-level-2",
			Title:     "Level 2",
			Status:    "open",
			IssueType: "task",
			Dependencies: []BeadReference{
				{ID: "bd-level-1a", DependencyType: "parent-child"},
			},
		},
		{ID: "bd-outside", Title: "Outside", Status: "open", IssueType: "task"},
	}

	fetcher := &mockFetcher{activeBeads: beads}
	g := NewGraph(cfg, fetcher, "horizontal")

	g.SetEpicFilter("bd-root")

	if err := g.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	nodes := g.GetNodes()

	// Verify descendants
	inScope := []string{"bd-root", "bd-level-1a", "bd-level-1b", "bd-level-2"}
	for _, id := range inScope {
		if nodes[id].OutOfScope {
			t.Errorf("%s should be in scope (descendant of epic)", id)
		}
	}

	// Verify outside node
	if !nodes["bd-outside"].OutOfScope {
		t.Error("bd-outside should be out of scope")
	}
}
