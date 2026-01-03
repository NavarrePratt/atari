package tui

import (
	"context"
	"errors"
	"testing"

	"github.com/npratt/atari/internal/config"
)

// mockFetcher implements BeadFetcher for testing.
type mockFetcher struct {
	activeBeads  []GraphBead
	backlogBeads []GraphBead
	activeErr    error
	backlogErr   error
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
