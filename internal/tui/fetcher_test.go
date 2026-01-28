package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/npratt/atari/internal/brclient"
)

func TestBDFetcher_FetchActive(t *testing.T) {
	tests := []struct {
		name      string
		beads     []brclient.Bead
		listErr   error
		wantBeads int
		wantErr   bool
	}{
		{
			name: "successful fetch with multiple beads",
			beads: []brclient.Bead{
				{ID: "bd-001", Title: "Task 1", Status: "open", IssueType: "task"},
				{ID: "bd-002", Title: "Task 2", Status: "in_progress", IssueType: "task"},
				{ID: "bd-003", Title: "Task 3", Status: "blocked", IssueType: "task"},
			},
			wantBeads: 3,
		},
		{
			name: "successful fetch with single bead",
			beads: []brclient.Bead{
				{ID: "bd-001", Title: "Task 1", Status: "open", IssueType: "task"},
			},
			wantBeads: 1,
		},
		{
			name:      "empty response",
			beads:     nil,
			wantBeads: 0,
		},
		{
			name:    "command error",
			listErr: errors.New("br command failed"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := brclient.NewMockClient()
			if tt.listErr != nil {
				client.ListError = tt.listErr
			} else {
				client.ListResponse = tt.beads
			}

			fetcher := NewBDFetcher(client)
			beads, err := fetcher.FetchActive(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(beads) != tt.wantBeads {
				t.Errorf("got %d beads, want %d", len(beads), tt.wantBeads)
			}
		})
	}
}

func TestBDFetcher_FetchBacklog(t *testing.T) {
	tests := []struct {
		name      string
		beads     []brclient.Bead
		listErr   error
		wantBeads int
		wantErr   bool
	}{
		{
			name: "successful fetch with backlog beads",
			beads: []brclient.Bead{
				{ID: "bd-001", Title: "Task 1", Status: "deferred", IssueType: "task"},
			},
			wantBeads: 1,
		},
		{
			name:      "empty backlog",
			beads:     nil,
			wantBeads: 0,
		},
		{
			name:    "command error",
			listErr: errors.New("br command failed"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := brclient.NewMockClient()
			if tt.listErr != nil {
				client.ListError = tt.listErr
			} else {
				client.ListResponse = tt.beads
			}

			fetcher := NewBDFetcher(client)
			beads, err := fetcher.FetchBacklog(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(beads) != tt.wantBeads {
				t.Errorf("got %d beads, want %d", len(beads), tt.wantBeads)
			}
		})
	}
}

func TestGraphBead_ToNode(t *testing.T) {
	bead := GraphBead{
		ID:        "bd-001",
		Title:     "Test Task",
		Status:    "in_progress",
		Priority:  2,
		IssueType: "task",
		Parent:    "bd-epic-001",
	}

	node := bead.ToNode()

	if node.ID != "bd-001" {
		t.Errorf("ID = %q, want %q", node.ID, "bd-001")
	}
	if node.Title != "Test Task" {
		t.Errorf("Title = %q, want %q", node.Title, "Test Task")
	}
	if node.Status != "in_progress" {
		t.Errorf("Status = %q, want %q", node.Status, "in_progress")
	}
	if node.Priority != 2 {
		t.Errorf("Priority = %d, want %d", node.Priority, 2)
	}
	if node.Type != "task" {
		t.Errorf("Type = %q, want %q", node.Type, "task")
	}
	if node.IsEpic {
		t.Error("IsEpic should be false for task")
	}
}

func TestGraphBead_ToNode_Epic(t *testing.T) {
	bead := GraphBead{
		ID:        "bd-epic-001",
		Title:     "Test Epic",
		Status:    "open",
		Priority:  1,
		IssueType: "epic",
	}

	node := bead.ToNode()

	if !node.IsEpic {
		t.Error("IsEpic should be true for epic")
	}
}

func TestGraphBead_ExtractEdges(t *testing.T) {
	bead := GraphBead{
		ID: "bd-task-002",
		Dependencies: []BeadReference{
			{ID: "bd-epic-001", DependencyType: "parent-child"},
			{ID: "bd-task-001", DependencyType: "blocks"},
		},
	}

	edges := bead.ExtractEdges()

	if len(edges) != 2 {
		t.Fatalf("got %d edges, want 2", len(edges))
	}

	// Check hierarchy edge
	if edges[0].From != "bd-epic-001" {
		t.Errorf("edge[0].From = %q, want %q", edges[0].From, "bd-epic-001")
	}
	if edges[0].To != "bd-task-002" {
		t.Errorf("edge[0].To = %q, want %q", edges[0].To, "bd-task-002")
	}
	if edges[0].Type != EdgeHierarchy {
		t.Errorf("edge[0].Type = %v, want EdgeHierarchy", edges[0].Type)
	}

	// Check dependency edge
	if edges[1].From != "bd-task-001" {
		t.Errorf("edge[1].From = %q, want %q", edges[1].From, "bd-task-001")
	}
	if edges[1].To != "bd-task-002" {
		t.Errorf("edge[1].To = %q, want %q", edges[1].To, "bd-task-002")
	}
	if edges[1].Type != EdgeDependency {
		t.Errorf("edge[1].Type = %v, want EdgeDependency", edges[1].Type)
	}
}

func TestGraphBead_ExtractEdges_Empty(t *testing.T) {
	bead := GraphBead{
		ID:           "bd-standalone",
		Dependencies: nil,
	}

	edges := bead.ExtractEdges()

	if len(edges) != 0 {
		t.Errorf("got %d edges, want 0", len(edges))
	}
}

func TestGraphBead_ExtractEdges_CaseInsensitive(t *testing.T) {
	tests := []struct {
		name     string
		depType  string
		wantType EdgeType
	}{
		{"lowercase hyphen", "parent-child", EdgeHierarchy},
		{"uppercase hyphen", "PARENT-CHILD", EdgeHierarchy},
		{"mixed case hyphen", "Parent-Child", EdgeHierarchy},
		{"lowercase underscore", "parent_child", EdgeHierarchy},
		{"uppercase underscore", "PARENT_CHILD", EdgeHierarchy},
		{"blocks type", "blocks", EdgeDependency},
		{"empty type", "", EdgeDependency},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bead := GraphBead{
				ID: "bd-task-001",
				Dependencies: []BeadReference{
					{ID: "bd-epic-001", DependencyType: tt.depType},
				},
			}

			edges := bead.ExtractEdges()

			if len(edges) != 1 {
				t.Fatalf("got %d edges, want 1", len(edges))
			}
			if edges[0].Type != tt.wantType {
				t.Errorf("edge type = %v, want %v", edges[0].Type, tt.wantType)
			}
		})
	}
}

func TestGraphBead_ExtractEdges_ParentFallback(t *testing.T) {
	bead := GraphBead{
		ID:           "bd-task-001",
		Parent:       "bd-epic-001",
		Dependencies: nil,
	}

	edges := bead.ExtractEdges()

	if len(edges) != 1 {
		t.Fatalf("got %d edges, want 1", len(edges))
	}
	if edges[0].From != "bd-epic-001" {
		t.Errorf("edge.From = %q, want %q", edges[0].From, "bd-epic-001")
	}
	if edges[0].To != "bd-task-001" {
		t.Errorf("edge.To = %q, want %q", edges[0].To, "bd-task-001")
	}
	if edges[0].Type != EdgeHierarchy {
		t.Errorf("edge.Type = %v, want EdgeHierarchy", edges[0].Type)
	}
}

func TestGraphBead_ExtractEdges_ParentFallbackWithBlocksDep(t *testing.T) {
	bead := GraphBead{
		ID:     "bd-task-001",
		Parent: "bd-epic-001",
		Dependencies: []BeadReference{
			{ID: "bd-task-000", DependencyType: "blocks"},
		},
	}

	edges := bead.ExtractEdges()

	if len(edges) != 2 {
		t.Fatalf("got %d edges, want 2", len(edges))
	}
	// First edge from blocks dependency
	if edges[0].From != "bd-task-000" || edges[0].Type != EdgeDependency {
		t.Errorf("edge[0] = {%s, %v}, want {bd-task-000, EdgeDependency}", edges[0].From, edges[0].Type)
	}
	// Second edge from Parent fallback
	if edges[1].From != "bd-epic-001" || edges[1].Type != EdgeHierarchy {
		t.Errorf("edge[1] = {%s, %v}, want {bd-epic-001, EdgeHierarchy}", edges[1].From, edges[1].Type)
	}
}

func TestGraphBead_ExtractEdges_NoDuplicateParentEdge(t *testing.T) {
	bead := GraphBead{
		ID:     "bd-task-001",
		Parent: "bd-epic-001",
		Dependencies: []BeadReference{
			{ID: "bd-epic-001", DependencyType: "parent-child"},
		},
	}

	edges := bead.ExtractEdges()

	if len(edges) != 1 {
		t.Fatalf("got %d edges, want 1 (no duplicate)", len(edges))
	}
	if edges[0].From != "bd-epic-001" || edges[0].Type != EdgeHierarchy {
		t.Errorf("edge = {%s, %v}, want {bd-epic-001, EdgeHierarchy}", edges[0].From, edges[0].Type)
	}
}

func TestToNodesAndEdges(t *testing.T) {
	beads := []GraphBead{
		{
			ID:        "bd-epic-001",
			Title:     "Epic",
			Status:    "open",
			Priority:  1,
			IssueType: "epic",
		},
		{
			ID:        "bd-task-001",
			Title:     "Task",
			Status:    "in_progress",
			Priority:  2,
			IssueType: "task",
			Dependencies: []BeadReference{
				{ID: "bd-epic-001", DependencyType: "parent-child"},
			},
		},
	}

	nodes, edges := ToNodesAndEdges(beads)

	if len(nodes) != 2 {
		t.Errorf("got %d nodes, want 2", len(nodes))
	}
	if len(edges) != 1 {
		t.Errorf("got %d edges, want 1", len(edges))
	}

	if nodes[0].ID != "bd-epic-001" {
		t.Errorf("nodes[0].ID = %q, want %q", nodes[0].ID, "bd-epic-001")
	}
	if nodes[1].ID != "bd-task-001" {
		t.Errorf("nodes[1].ID = %q, want %q", nodes[1].ID, "bd-task-001")
	}

	if edges[0].From != "bd-epic-001" || edges[0].To != "bd-task-001" {
		t.Errorf("edge = %v, want from bd-epic-001 to bd-task-001", edges[0])
	}
}

func TestEdgeType_String(t *testing.T) {
	tests := []struct {
		edgeType EdgeType
		want     string
	}{
		{EdgeHierarchy, "hierarchy"},
		{EdgeDependency, "dependency"},
		{EdgeType(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.edgeType.String(); got != tt.want {
			t.Errorf("EdgeType(%d).String() = %q, want %q", tt.edgeType, got, tt.want)
		}
	}
}

func TestGraphView_String(t *testing.T) {
	tests := []struct {
		view GraphView
		want string
	}{
		{ViewActive, "active"},
		{ViewBacklog, "backlog"},
		{GraphView(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.view.String(); got != tt.want {
			t.Errorf("GraphView(%d).String() = %q, want %q", tt.view, got, tt.want)
		}
	}
}

func TestLayoutDirection_String(t *testing.T) {
	tests := []struct {
		dir  LayoutDirection
		want string
	}{
		{LayoutTopDown, "top-down"},
		{LayoutLeftRight, "left-right"},
		{LayoutDirection(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.dir.String(); got != tt.want {
			t.Errorf("LayoutDirection(%d).String() = %q, want %q", tt.dir, got, tt.want)
		}
	}
}

func TestNodeDensity_String(t *testing.T) {
	tests := []struct {
		density NodeDensity
		want    string
	}{
		{DensityCompact, "compact"},
		{DensityStandard, "standard"},
		{DensityDetailed, "detailed"},
		{NodeDensity(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.density.String(); got != tt.want {
			t.Errorf("NodeDensity(%d).String() = %q, want %q", tt.density, got, tt.want)
		}
	}
}

func TestParseDensity(t *testing.T) {
	tests := []struct {
		input string
		want  NodeDensity
	}{
		{"compact", DensityCompact},
		{"standard", DensityStandard},
		{"detailed", DensityDetailed},
		{"unknown", DensityStandard}, // default
		{"", DensityStandard},        // default
	}

	for _, tt := range tests {
		if got := ParseDensity(tt.input); got != tt.want {
			t.Errorf("ParseDensity(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestFilterByStatus(t *testing.T) {
	beads := []GraphBead{
		{ID: "open-1", Status: "open"},
		{ID: "in-progress-1", Status: "in_progress"},
		{ID: "blocked-1", Status: "blocked"},
		{ID: "closed-1", Status: "closed"},
		{ID: "deferred-1", Status: "deferred"},
	}

	tests := []struct {
		name     string
		statuses []string
		wantIDs  []string
	}{
		{
			name:     "filter active statuses",
			statuses: []string{"open", "in_progress", "blocked"},
			wantIDs:  []string{"open-1", "in-progress-1", "blocked-1"},
		},
		{
			name:     "filter deferred only",
			statuses: []string{"deferred"},
			wantIDs:  []string{"deferred-1"},
		},
		{
			name:     "filter closed only",
			statuses: []string{"closed"},
			wantIDs:  []string{"closed-1"},
		},
		{
			name:     "no matching status",
			statuses: []string{"unknown"},
			wantIDs:  []string{},
		},
		{
			name:     "empty statuses",
			statuses: []string{},
			wantIDs:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterByStatus(beads, tt.statuses...)
			if len(result) != len(tt.wantIDs) {
				t.Errorf("got %d beads, want %d", len(result), len(tt.wantIDs))
				return
			}
			for i, want := range tt.wantIDs {
				if result[i].ID != want {
					t.Errorf("result[%d].ID = %q, want %q", i, result[i].ID, want)
				}
			}
		})
	}
}

func TestFilterByStatus_EmptyInput(t *testing.T) {
	result := filterByStatus(nil, "open")
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}

	result = filterByStatus([]GraphBead{}, "open")
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}
}

func TestFilterOutAgentBeads(t *testing.T) {
	beads := []GraphBead{
		{ID: "task-1", IssueType: "task"},
		{ID: "agent-1", IssueType: "agent"},
		{ID: "epic-1", IssueType: "epic"},
		{ID: "agent-2", IssueType: "agent"},
		{ID: "bug-1", IssueType: "bug"},
	}

	result := filterOutAgentBeads(beads)

	if len(result) != 3 {
		t.Fatalf("got %d beads, want 3", len(result))
	}

	wantIDs := []string{"task-1", "epic-1", "bug-1"}
	for i, want := range wantIDs {
		if result[i].ID != want {
			t.Errorf("result[%d].ID = %q, want %q", i, result[i].ID, want)
		}
	}
}

func TestFilterOutAgentBeads_EmptyInput(t *testing.T) {
	result := filterOutAgentBeads(nil)
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}

	result = filterOutAgentBeads([]GraphBead{})
	if result != nil {
		t.Errorf("expected nil for empty input, got %v", result)
	}
}

func TestFilterOutAgentBeads_AllAgents(t *testing.T) {
	beads := []GraphBead{
		{ID: "agent-1", IssueType: "agent"},
		{ID: "agent-2", IssueType: "agent"},
	}

	result := filterOutAgentBeads(beads)

	if len(result) != 0 {
		t.Errorf("got %d beads, want 0", len(result))
	}
}

func TestFilterOutAgentBeads_NoAgents(t *testing.T) {
	beads := []GraphBead{
		{ID: "task-1", IssueType: "task"},
		{ID: "epic-1", IssueType: "epic"},
	}

	result := filterOutAgentBeads(beads)

	if len(result) != 2 {
		t.Errorf("got %d beads, want 2", len(result))
	}
}

func TestBDFetcher_FetchActive_FiltersAgentBeads(t *testing.T) {
	client := brclient.NewMockClient()
	client.ListResponse = []brclient.Bead{
		{ID: "task-001", Title: "Task 1", Status: "open", IssueType: "task"},
		{ID: "agent-001", Title: "Agent 1", Status: "open", IssueType: "agent"},
		{ID: "task-002", Title: "Task 2", Status: "open", IssueType: "task"},
	}

	fetcher := NewBDFetcher(client)
	beads, err := fetcher.FetchActive(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 2 beads (task-001 and task-002), not 3 (agent-001 filtered out)
	if len(beads) != 2 {
		t.Errorf("got %d beads, want 2 (agent bead should be filtered)", len(beads))
	}

	for _, b := range beads {
		if b.IssueType == "agent" {
			t.Errorf("agent bead %q should have been filtered out", b.ID)
		}
	}
}

func TestBDFetcher_FetchActive_ContextCancellation(t *testing.T) {
	client := brclient.NewMockClient()

	fetcher := NewBDFetcher(client)

	// Create a pre-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := fetcher.FetchActive(ctx)
	// MockClient doesn't respect context, so this will succeed
	// In real usage, the CLI client would respect context
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBDFetcher_FetchBead_ContextCancellation(t *testing.T) {
	client := brclient.NewMockClient()
	client.SetShowResponse("bd-001", &brclient.Bead{ID: "bd-001", Title: "Test", Status: "open", IssueType: "task"})

	fetcher := NewBDFetcher(client)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := fetcher.FetchBead(ctx, "bd-001")
	// MockClient doesn't respect context, so this will succeed
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBDFetcher_EnrichBeadsWithDetails(t *testing.T) {
	basicBeads := []GraphBead{
		{ID: "bd-001", Title: "Task 1", Status: "open", IssueType: "task"},
		{ID: "bd-002", Title: "Task 2", Status: "open", IssueType: "task"},
	}

	client := brclient.NewMockClient()
	client.DynamicShow = func(ctx context.Context, id string) (*brclient.Bead, error, bool) {
		switch id {
		case "bd-001":
			return &brclient.Bead{
				ID: "bd-001", Title: "Task 1", Status: "open", IssueType: "task",
				Dependencies: []brclient.BeadReference{{ID: "bd-epic-001", DependencyType: "parent-child"}},
			}, nil, true
		case "bd-002":
			return &brclient.Bead{
				ID: "bd-002", Title: "Task 2", Status: "open", IssueType: "task",
				Dependencies: []brclient.BeadReference{{ID: "bd-001", DependencyType: "blocks"}},
			}, nil, true
		}
		return nil, nil, false
	}

	fetcher := NewBDFetcher(client)
	result, err := fetcher.enrichBeadsWithDetails(context.Background(), basicBeads)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("got %d beads, want 2", len(result))
	}

	if len(result[0].Dependencies) != 1 {
		t.Errorf("bead 0 has %d dependencies, want 1", len(result[0].Dependencies))
	}
	if len(result[1].Dependencies) != 1 {
		t.Errorf("bead 1 has %d dependencies, want 1", len(result[1].Dependencies))
	}
}

func TestBDFetcher_EnrichBeadsWithDetails_Empty(t *testing.T) {
	client := brclient.NewMockClient()
	fetcher := NewBDFetcher(client)

	result, err := fetcher.enrichBeadsWithDetails(context.Background(), nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}

	result, err = fetcher.enrichBeadsWithDetails(context.Background(), []GraphBead{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d beads", len(result))
	}
}

func TestBDFetcher_EnrichBeadsWithDetails_PartialFailure(t *testing.T) {
	basicBeads := []GraphBead{
		{ID: "bd-001", Title: "Task 1", Status: "open", IssueType: "task"},
		{ID: "bd-002", Title: "Task 2", Status: "open", IssueType: "task"},
		{ID: "bd-003", Title: "Task 3", Status: "open", IssueType: "task"},
	}

	client := brclient.NewMockClient()
	client.DynamicShow = func(ctx context.Context, id string) (*brclient.Bead, error, bool) {
		switch id {
		case "bd-001":
			return &brclient.Bead{
				ID: "bd-001", Title: "Task 1 enriched", Status: "open", IssueType: "task",
				Dependencies: []brclient.BeadReference{{ID: "bd-epic-001", DependencyType: "parent-child"}},
			}, nil, true
		case "bd-002":
			return nil, errors.New("bead not found"), true
		case "bd-003":
			return &brclient.Bead{
				ID: "bd-003", Title: "Task 3 enriched", Status: "open", IssueType: "task",
			}, nil, true
		}
		return nil, nil, false
	}

	fetcher := NewBDFetcher(client)
	result, err := fetcher.enrichBeadsWithDetails(context.Background(), basicBeads)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("got %d beads, want 3", len(result))
	}

	// bd-001 should be enriched
	if result[0].Title != "Task 1 enriched" {
		t.Errorf("bead 0 title = %q, want %q", result[0].Title, "Task 1 enriched")
	}
	if len(result[0].Dependencies) != 1 {
		t.Errorf("bead 0 has %d dependencies, want 1", len(result[0].Dependencies))
	}

	// bd-002 should retain original data (enrichment failed)
	if result[1].Title != "Task 2" {
		t.Errorf("bead 1 title = %q, want %q", result[1].Title, "Task 2")
	}

	// bd-003 should be enriched
	if result[2].Title != "Task 3 enriched" {
		t.Errorf("bead 2 title = %q, want %q", result[2].Title, "Task 3 enriched")
	}
}

func TestBDFetcher_EnrichBeadsWithDetails_ContextCancellation(t *testing.T) {
	basicBeads := []GraphBead{
		{ID: "bd-001", Title: "Task 1", Status: "open", IssueType: "task"},
		{ID: "bd-002", Title: "Task 2", Status: "open", IssueType: "task"},
	}

	client := brclient.NewMockClient()
	client.DynamicShow = func(ctx context.Context, id string) (*brclient.Bead, error, bool) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err(), true
		default:
			return &brclient.Bead{ID: id, Title: "Task", Status: "open"}, nil, true
		}
	}

	fetcher := NewBDFetcher(client)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := fetcher.enrichBeadsWithDetails(ctx, basicBeads)
	if err == nil {
		t.Error("expected error for cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestBDFetcher_EnrichBeadsWithDetails_LogsWarnOnFailure(t *testing.T) {
	basicBeads := []GraphBead{
		{ID: "bd-001", Title: "Task 1", Status: "open", IssueType: "task"},
		{ID: "bd-002", Title: "Task 2", Status: "open", IssueType: "task"},
	}

	client := brclient.NewMockClient()
	client.DynamicShow = func(ctx context.Context, id string) (*brclient.Bead, error, bool) {
		switch id {
		case "bd-001":
			return &brclient.Bead{ID: "bd-001", Title: "Task 1 enriched", Status: "open", IssueType: "task"}, nil, true
		case "bd-002":
			return nil, errors.New("bead not found"), true
		}
		return nil, nil, false
	}

	// Capture log output
	var logBuf bytes.Buffer
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(oldLogger)

	fetcher := NewBDFetcher(client)
	_, err := fetcher.enrichBeadsWithDetails(context.Background(), basicBeads)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logOutput := logBuf.String()

	// Verify WARN level for non-cancellation errors
	if !containsLogEntry(logOutput, "WARN", "failed to enrich bead") {
		t.Error("expected WARN log for failed enrichment")
	}

	// Verify aggregate warning
	if !containsLogEntry(logOutput, "WARN", "enrichment partially failed") {
		t.Error("expected WARN log for partial failure summary")
	}
}

func TestBDFetcher_EnrichBeadsWithDetails_LogsDebugOnCancel(t *testing.T) {
	basicBeads := []GraphBead{
		{ID: "bd-001", Title: "Task 1", Status: "open", IssueType: "task"},
	}

	client := brclient.NewMockClient()
	client.DynamicShow = func(ctx context.Context, id string) (*brclient.Bead, error, bool) {
		return nil, context.Canceled, true
	}

	// Capture log output
	var logBuf bytes.Buffer
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(oldLogger)

	fetcher := NewBDFetcher(client)
	// Note: This won't return error because individual bead failures are handled gracefully
	_, _ = fetcher.enrichBeadsWithDetails(context.Background(), basicBeads)

	logOutput := logBuf.String()

	// Verify DEBUG level for cancellation
	if !containsLogEntry(logOutput, "DEBUG", "bead enrichment cancelled") {
		t.Error("expected DEBUG log for cancelled enrichment")
	}

	// Verify no WARN for cancellation
	if containsLogEntry(logOutput, "WARN", "failed to enrich bead") {
		t.Error("should not log WARN for cancelled enrichment")
	}
}

// containsLogEntry checks if log output contains an entry with given level and message substring
func containsLogEntry(logOutput, level, msgSubstring string) bool {
	lines := bytes.Split([]byte(logOutput), []byte("\n"))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		entryLevel, _ := entry["level"].(string)
		entryMsg, _ := entry["msg"].(string)
		if entryLevel == level && contains(entryMsg, msgSubstring) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestBDFetcher_EnrichBeadsWithDetails_ConcurrencyLimit(t *testing.T) {
	// Create more beads than the concurrency limit
	beadCount := 10
	basicBeads := make([]GraphBead, beadCount)
	for i := range beadCount {
		basicBeads[i] = GraphBead{
			ID:        fmt.Sprintf("bd-%03d", i),
			Title:     fmt.Sprintf("Task %d", i),
			Status:    "open",
			IssueType: "task",
		}
	}

	var concurrentCount int64
	var maxConcurrent int64

	client := brclient.NewMockClient()
	client.DynamicShow = func(ctx context.Context, id string) (*brclient.Bead, error, bool) {
		current := atomic.AddInt64(&concurrentCount, 1)
		defer atomic.AddInt64(&concurrentCount, -1)

		for {
			max := atomic.LoadInt64(&maxConcurrent)
			if current <= max {
				break
			}
			if atomic.CompareAndSwapInt64(&maxConcurrent, max, current) {
				break
			}
		}

		time.Sleep(10 * time.Millisecond)

		return &brclient.Bead{ID: id, Title: "Enriched", Status: "open"}, nil, true
	}

	fetcher := NewBDFetcher(client)
	result, err := fetcher.enrichBeadsWithDetails(context.Background(), basicBeads)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != beadCount {
		t.Errorf("got %d beads, want %d", len(result), beadCount)
	}

	// Verify concurrency was limited
	observed := atomic.LoadInt64(&maxConcurrent)
	if observed > maxConcurrentFetches {
		t.Errorf("max concurrent fetches = %d, want <= %d", observed, maxConcurrentFetches)
	}
}

func TestBDFetcher_EnrichBeadsWithDetails_PanicRecovery(t *testing.T) {
	basicBeads := []GraphBead{
		{ID: "bd-001", Title: "Task 1", Status: "open", IssueType: "task"},
		{ID: "bd-002", Title: "Task 2", Status: "open", IssueType: "task"},
		{ID: "bd-003", Title: "Task 3", Status: "open", IssueType: "task"},
	}

	client := brclient.NewMockClient()
	client.DynamicShow = func(ctx context.Context, id string) (*brclient.Bead, error, bool) {
		switch id {
		case "bd-001":
			return &brclient.Bead{ID: "bd-001", Title: "Task 1 enriched", Status: "open", IssueType: "task"}, nil, true
		case "bd-002":
			panic("simulated panic for testing")
		case "bd-003":
			return &brclient.Bead{ID: "bd-003", Title: "Task 3 enriched", Status: "open", IssueType: "task"}, nil, true
		}
		return nil, nil, false
	}

	// Capture log output to avoid test noise
	var logBuf bytes.Buffer
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(oldLogger)

	fetcher := NewBDFetcher(client)
	result, err := fetcher.enrichBeadsWithDetails(context.Background(), basicBeads)

	// Should return an error indicating panic occurred
	if err == nil {
		t.Fatal("expected error for panic, got nil")
	}
	if err.Error() != "panic occurred during bead enrichment" {
		t.Errorf("unexpected error message: %v", err)
	}

	// Should still have all beads in result (non-panicking ones enriched)
	if len(result) != 3 {
		t.Errorf("got %d beads, want 3", len(result))
	}

	// Verify panic was logged
	logOutput := logBuf.String()
	if !containsLogEntry(logOutput, "ERROR", "panic in bead enrichment") {
		t.Error("expected ERROR log for panic")
	}
}

func TestBDFetcher_EnrichBeadsWithDetails_PanicDoesNotHang(t *testing.T) {
	basicBeads := []GraphBead{
		{ID: "bd-001", Title: "Task 1", Status: "open", IssueType: "task"},
	}

	client := brclient.NewMockClient()
	client.DynamicShow = func(ctx context.Context, id string) (*brclient.Bead, error, bool) {
		panic("test panic")
	}

	// Suppress panic log output
	var logBuf bytes.Buffer
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(oldLogger)

	fetcher := NewBDFetcher(client)

	done := make(chan struct{})
	go func() {
		_, _ = fetcher.enrichBeadsWithDetails(context.Background(), basicBeads)
		close(done)
	}()

	select {
	case <-done:
		// Function returned without hanging
	case <-time.After(2 * time.Second):
		t.Fatal("enrichBeadsWithDetails hung after panic")
	}
}

func TestBDFetcher_EnrichBeadsWithDetails_WaitsForGoroutinesOnCancel(t *testing.T) {
	// Create more beads than concurrency limit to ensure some are queued
	beadCount := 10
	basicBeads := make([]GraphBead, beadCount)
	for i := range beadCount {
		basicBeads[i] = GraphBead{
			ID:        fmt.Sprintf("bd-%03d", i),
			Title:     fmt.Sprintf("Task %d", i),
			Status:    "open",
			IssueType: "task",
		}
	}

	var goroutinesStarted int64
	var goroutinesFinished int64

	client := brclient.NewMockClient()
	client.DynamicShow = func(ctx context.Context, id string) (*brclient.Bead, error, bool) {
		atomic.AddInt64(&goroutinesStarted, 1)
		defer atomic.AddInt64(&goroutinesFinished, 1)

		// Simulate some work
		select {
		case <-ctx.Done():
			return nil, ctx.Err(), true
		case <-time.After(50 * time.Millisecond):
			return &brclient.Bead{ID: id, Title: "Enriched", Status: "open"}, nil, true
		}
	}

	fetcher := NewBDFetcher(client)

	// Cancel context after a short delay to allow some goroutines to start
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	_, err := fetcher.enrichBeadsWithDetails(ctx, basicBeads)

	// Should get a context.Canceled error
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}

	// After function returns, all started goroutines should have finished
	started := atomic.LoadInt64(&goroutinesStarted)
	finished := atomic.LoadInt64(&goroutinesFinished)

	if started != finished {
		t.Errorf("goroutines mismatch: started=%d, finished=%d (function returned before all goroutines completed)", started, finished)
	}
}

// End-to-end enrichment flow tests
// These tests exercise the complete enrichment pipeline through FetchActive,
// using realistic fixtures that match actual br list vs br show output.

func TestBDFetcher_FetchActive_EnrichmentFlow(t *testing.T) {
	// Test the complete enrichment flow:
	// 1. List returns unenriched data (no dependencies array)
	// 2. Show returns enriched data for each bead
	// 3. FetchActive returns beads with dependencies populated
	// 4. Hierarchy edges can be extracted from the enriched beads

	client := brclient.NewMockClient()
	client.ListResponse = []brclient.Bead{
		{ID: "bd-epic-001", Title: "Implement TUI", Status: "open", Priority: 1, IssueType: "epic"},
		{ID: "bd-task-001", Title: "Create graph model", Status: "in_progress", Priority: 2, IssueType: "task", Parent: "bd-epic-001"},
		{ID: "bd-task-002", Title: "Add session management", Status: "open", Priority: 2, IssueType: "task", Parent: "bd-epic-001"},
	}

	client.SetShowResponse("bd-epic-001", &brclient.Bead{ID: "bd-epic-001", Title: "Implement TUI", Status: "open", Priority: 1, IssueType: "epic"})
	client.SetShowResponse("bd-task-001", &brclient.Bead{
		ID: "bd-task-001", Title: "Create graph model", Status: "in_progress", Priority: 2, IssueType: "task",
		Dependencies: []brclient.BeadReference{{ID: "bd-epic-001", DependencyType: "parent-child"}},
	})
	client.SetShowResponse("bd-task-002", &brclient.Bead{
		ID: "bd-task-002", Title: "Add session management", Status: "open", Priority: 2, IssueType: "task",
		Dependencies: []brclient.BeadReference{
			{ID: "bd-epic-001", DependencyType: "parent-child"},
			{ID: "bd-task-001", DependencyType: "blocks"},
		},
	})

	fetcher := NewBDFetcher(client)
	beads, err := fetcher.FetchActive(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 3 beads (epic + 2 tasks)
	if len(beads) != 3 {
		t.Fatalf("got %d beads, want 3", len(beads))
	}

	// Verify enrichment populated dependencies
	beadMap := make(map[string]GraphBead)
	for _, b := range beads {
		beadMap[b.ID] = b
	}

	// bd-task-001 should have parent-child dependency on bd-epic-001
	task1 := beadMap["bd-task-001"]
	if len(task1.Dependencies) == 0 {
		t.Error("bd-task-001 should have dependencies after enrichment")
	} else {
		found := false
		for _, dep := range task1.Dependencies {
			if dep.ID == "bd-epic-001" && dep.DependencyType == "parent-child" {
				found = true
				break
			}
		}
		if !found {
			t.Error("bd-task-001 should have parent-child dependency on bd-epic-001")
		}
	}

	// bd-task-002 should have both parent-child and blocks dependencies
	task2 := beadMap["bd-task-002"]
	if len(task2.Dependencies) < 2 {
		t.Errorf("bd-task-002 should have 2 dependencies, got %d", len(task2.Dependencies))
	} else {
		hasParent := false
		hasBlocks := false
		for _, dep := range task2.Dependencies {
			if dep.ID == "bd-epic-001" && dep.DependencyType == "parent-child" {
				hasParent = true
			}
			if dep.ID == "bd-task-001" && dep.DependencyType == "blocks" {
				hasBlocks = true
			}
		}
		if !hasParent {
			t.Error("bd-task-002 should have parent-child dependency on bd-epic-001")
		}
		if !hasBlocks {
			t.Error("bd-task-002 should have blocks dependency on bd-task-001")
		}
	}

	// Verify hierarchy edges can be extracted
	nodes, edges := ToNodesAndEdges(beads)

	if len(nodes) != 3 {
		t.Errorf("got %d nodes, want 3", len(nodes))
	}

	// Count hierarchy and dependency edges
	var hierarchyCount, dependencyCount int
	for _, edge := range edges {
		switch edge.Type {
		case EdgeHierarchy:
			hierarchyCount++
		case EdgeDependency:
			dependencyCount++
		}
	}

	// Should have hierarchy edges from epic to tasks
	if hierarchyCount < 2 {
		t.Errorf("got %d hierarchy edges, want at least 2", hierarchyCount)
	}

	// Should have dependency edge from task-001 to task-002
	if dependencyCount < 1 {
		t.Errorf("got %d dependency edges, want at least 1", dependencyCount)
	}
}

func TestBDFetcher_FetchActive_PartialEnrichmentFailure(t *testing.T) {
	// Test partial enrichment failure:
	// - List returns 3 beads
	// - Show fails for 1 bead, succeeds for 2
	// - 2 beads have dependencies, 1 uses basic data
	// - Graph is still usable with partial hierarchy

	client := brclient.NewMockClient()
	client.ListResponse = []brclient.Bead{
		{ID: "bd-epic-001", Title: "Implement TUI", Status: "open", Priority: 1, IssueType: "epic"},
		{ID: "bd-task-001", Title: "Create graph model", Status: "in_progress", Priority: 2, IssueType: "task", Parent: "bd-epic-001"},
		{ID: "bd-task-002", Title: "Add session management", Status: "open", Priority: 2, IssueType: "task", Parent: "bd-epic-001"},
	}

	// Set up enrichment to succeed for epic and task-001, fail for task-002
	client.DynamicShow = func(ctx context.Context, id string) (*brclient.Bead, error, bool) {
		switch id {
		case "bd-epic-001":
			return &brclient.Bead{ID: "bd-epic-001", Title: "Implement TUI", Status: "open", Priority: 1, IssueType: "epic"}, nil, true
		case "bd-task-001":
			return &brclient.Bead{
				ID: "bd-task-001", Title: "Create graph model", Status: "in_progress", Priority: 2, IssueType: "task",
				Dependencies: []brclient.BeadReference{{ID: "bd-epic-001", DependencyType: "parent-child"}},
			}, nil, true
		case "bd-task-002":
			return nil, errors.New("bead not found"), true
		}
		return nil, nil, false
	}

	fetcher := NewBDFetcher(client)
	beads, err := fetcher.FetchActive(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 3 beads
	if len(beads) != 3 {
		t.Fatalf("got %d beads, want 3", len(beads))
	}

	beadMap := make(map[string]GraphBead)
	for _, b := range beads {
		beadMap[b.ID] = b
	}

	// task-001 should be enriched (has dependencies)
	task1 := beadMap["bd-task-001"]
	if len(task1.Dependencies) == 0 {
		t.Error("bd-task-001 should have dependencies (enrichment succeeded)")
	}

	// task-002 should retain basic data (enrichment failed)
	task2 := beadMap["bd-task-002"]
	if len(task2.Dependencies) != 0 {
		t.Errorf("bd-task-002 should have no dependencies (enrichment failed), got %d", len(task2.Dependencies))
	}
	// But it should still have basic data from list
	if task2.Title != "Add session management" {
		t.Errorf("bd-task-002 should retain basic data, got title %q", task2.Title)
	}

	// Verify partial hierarchy is visible
	nodes, edges := ToNodesAndEdges(beads)

	if len(nodes) != 3 {
		t.Errorf("got %d nodes, want 3", len(nodes))
	}

	// Should have at least 1 hierarchy edge (from enriched task-001)
	var hierarchyCount int
	for _, edge := range edges {
		if edge.Type == EdgeHierarchy {
			hierarchyCount++
		}
	}
	if hierarchyCount == 0 {
		t.Error("expected at least 1 hierarchy edge from partially enriched data")
	}
}

func TestBDFetcher_FetchActive_TotalEnrichmentFailure(t *testing.T) {
	// Test total enrichment failure:
	// - List returns beads
	// - Show fails for ALL beads
	// - WARN logged with failure count
	// - Graph still displays (flat, but functional)

	client := brclient.NewMockClient()
	client.ListResponse = []brclient.Bead{
		{ID: "bd-epic-001", Title: "Implement TUI", Status: "open", Priority: 1, IssueType: "epic"},
		{ID: "bd-task-001", Title: "Create graph model", Status: "in_progress", Priority: 2, IssueType: "task"},
		{ID: "bd-task-002", Title: "Add session management", Status: "open", Priority: 2, IssueType: "task"},
	}

	// Fail all enrichment attempts
	client.DynamicShow = func(ctx context.Context, id string) (*brclient.Bead, error, bool) {
		return nil, errors.New("connection refused"), true
	}

	// Capture log output to verify WARN
	var logBuf bytes.Buffer
	oldLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(oldLogger)

	fetcher := NewBDFetcher(client)
	beads, err := fetcher.FetchActive(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still have 3 beads (basic data from list)
	if len(beads) != 3 {
		t.Fatalf("got %d beads, want 3", len(beads))
	}

	// All beads should have no dependencies (enrichment failed)
	for _, b := range beads {
		if len(b.Dependencies) != 0 {
			t.Errorf("bead %s should have no dependencies (all enrichment failed), got %d", b.ID, len(b.Dependencies))
		}
	}

	// Verify WARN logged for partial failure
	logOutput := logBuf.String()
	if !containsLogEntry(logOutput, "WARN", "enrichment partially failed") {
		t.Error("expected WARN log for total enrichment failure")
	}

	// Verify graph is still functional (flat, but displayable)
	nodes, edges := ToNodesAndEdges(beads)

	if len(nodes) != 3 {
		t.Errorf("got %d nodes, want 3", len(nodes))
	}

	// No edges since enrichment failed
	if len(edges) != 0 {
		t.Errorf("expected 0 edges with failed enrichment, got %d", len(edges))
	}
}

func TestBDFetcher_FetchActive_ContextCancellationMidEnrichment(t *testing.T) {
	// Test context cancellation during enrichment:
	// - List returns many beads (more than maxConcurrentFetches)
	// - Cancel context after first show
	// - Returns error promptly (semaphore acquire fails)
	// - No goroutine leaks

	// Create a list response with 10 beads to exceed maxConcurrentFetches (5)
	manyBeads := make([]brclient.Bead, 10)
	for i := range 10 {
		manyBeads[i] = brclient.Bead{
			ID:        fmt.Sprintf("bd-%03d", i),
			Title:     fmt.Sprintf("Task %d", i),
			Status:    "open",
			Priority:  2,
			IssueType: "task",
		}
	}

	client := brclient.NewMockClient()
	client.ListResponse = manyBeads

	var showCallCount int64
	var goroutinesStarted int64
	var goroutinesFinished int64
	ctx, cancel := context.WithCancel(context.Background())

	client.DynamicShow = func(dynCtx context.Context, id string) (*brclient.Bead, error, bool) {
		atomic.AddInt64(&goroutinesStarted, 1)
		defer atomic.AddInt64(&goroutinesFinished, 1)

		callNum := atomic.AddInt64(&showCallCount, 1)

		// Cancel after first call starts
		if callNum == 1 {
			// Simulate some work then cancel
			time.Sleep(10 * time.Millisecond)
			cancel()
		}

		// Check if context is cancelled
		select {
		case <-dynCtx.Done():
			return nil, dynCtx.Err(), true
		case <-time.After(100 * time.Millisecond):
			return &brclient.Bead{ID: id, Title: "Enriched", Status: "open"}, nil, true
		}
	}

	fetcher := NewBDFetcher(client)

	done := make(chan struct{})
	var fetchErr error

	go func() {
		_, fetchErr = fetcher.FetchActive(ctx)
		close(done)
	}()

	// Should complete promptly (within 2 seconds)
	select {
	case <-done:
		// Expected
	case <-time.After(3 * time.Second):
		t.Fatal("FetchActive did not return promptly after context cancellation")
	}

	// Should return context.Canceled error
	if fetchErr == nil {
		t.Error("expected error for cancelled context, got nil")
	} else if !errors.Is(fetchErr, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", fetchErr)
	}

	// Give goroutines time to finish
	time.Sleep(200 * time.Millisecond)

	// Verify no goroutine leaks
	started := atomic.LoadInt64(&goroutinesStarted)
	finished := atomic.LoadInt64(&goroutinesFinished)

	if started != finished {
		t.Errorf("goroutine leak: started=%d, finished=%d", started, finished)
	}
}
