package tui

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/npratt/atari/internal/testutil"
)

func TestBDFetcher_FetchActive(t *testing.T) {
	tests := []struct {
		name      string
		response  []byte
		err       error
		wantBeads int
		wantErr   bool
	}{
		{
			name:      "successful fetch with multiple beads",
			response:  []byte(testutil.GraphActiveBeadsJSON),
			wantBeads: 3,
		},
		{
			name:      "successful fetch with single bead",
			response:  []byte(testutil.GraphSingleBeadJSON),
			wantBeads: 1,
		},
		{
			name:      "empty response",
			response:  []byte(testutil.GraphEmptyBeadsJSON),
			wantBeads: 0,
		},
		{
			name:    "command error",
			err:     errors.New("bd command failed"),
			wantErr: true,
		},
		{
			name:     "invalid JSON",
			response: []byte("not json"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := testutil.NewMockRunner()
			if tt.err != nil {
				runner.SetError("bd", []string{"list", "--json"}, tt.err)
			} else {
				runner.SetResponse("bd", []string{"list", "--json"}, tt.response)
			}

			fetcher := NewBDFetcher(runner)
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
		response  []byte
		err       error
		wantBeads int
		wantErr   bool
	}{
		{
			name:      "successful fetch with backlog beads",
			response:  []byte(testutil.GraphBacklogBeadsJSON),
			wantBeads: 1,
		},
		{
			name:      "empty backlog",
			response:  []byte(testutil.GraphEmptyBeadsJSON),
			wantBeads: 0,
		},
		{
			name:    "command error",
			err:     errors.New("bd command failed"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := testutil.NewMockRunner()
			if tt.err != nil {
				runner.SetError("bd", []string{"list", "--json"}, tt.err)
			} else {
				runner.SetResponse("bd", []string{"list", "--json"}, tt.response)
			}

			fetcher := NewBDFetcher(runner)
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

func TestParseBeads_EmptyInput(t *testing.T) {
	beads, err := parseBeads(nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if beads != nil {
		t.Errorf("expected nil beads for nil input, got %v", beads)
	}

	beads, err = parseBeads([]byte{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if beads != nil {
		t.Errorf("expected nil beads for empty input, got %v", beads)
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
	runner := testutil.NewMockRunner()
	runner.SetResponse("bd", []string{"list", "--json"}, []byte(testutil.GraphMixedWithAgentJSON))

	fetcher := NewBDFetcher(runner)
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
	runner := testutil.NewMockRunner()

	// Use DynamicResponse to simulate a slow command that respects context
	runner.DynamicResponse = func(ctx context.Context, name string, args []string) ([]byte, error, bool) {
		// Check if context is cancelled before returning
		select {
		case <-ctx.Done():
			return nil, ctx.Err(), true
		default:
			return []byte(testutil.GraphActiveBeadsJSON), nil, true
		}
	}

	fetcher := NewBDFetcher(runner)

	// Create a pre-cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := fetcher.FetchActive(ctx)
	if err == nil {
		t.Error("expected error for cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestBDFetcher_FetchActive_ContextTimeout(t *testing.T) {
	runner := testutil.NewMockRunner()

	// Use DynamicResponse to simulate a command that takes too long
	runner.DynamicResponse = func(ctx context.Context, name string, args []string) ([]byte, error, bool) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err(), true
		case <-time.After(100 * time.Millisecond):
			return []byte(testutil.GraphActiveBeadsJSON), nil, true
		}
	}

	fetcher := NewBDFetcher(runner)

	// Create a context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := fetcher.FetchActive(ctx)
	if err == nil {
		t.Error("expected error for timeout, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded error, got %v", err)
	}
}

func TestBDFetcher_FetchBacklog_ContextCancellation(t *testing.T) {
	runner := testutil.NewMockRunner()

	runner.DynamicResponse = func(ctx context.Context, name string, args []string) ([]byte, error, bool) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err(), true
		default:
			return []byte(testutil.GraphBacklogBeadsJSON), nil, true
		}
	}

	fetcher := NewBDFetcher(runner)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := fetcher.FetchBacklog(ctx)
	if err == nil {
		t.Error("expected error for cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestBDFetcher_FetchBead_ContextCancellation(t *testing.T) {
	runner := testutil.NewMockRunner()

	runner.DynamicResponse = func(ctx context.Context, name string, args []string) ([]byte, error, bool) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err(), true
		default:
			return []byte(`[{"id": "bd-001", "title": "Test", "status": "open", "issue_type": "task"}]`), nil, true
		}
	}

	fetcher := NewBDFetcher(runner)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := fetcher.FetchBead(ctx, "bd-001")
	if err == nil {
		t.Error("expected error for cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "RFC3339Nano with nanoseconds",
			input:   "2024-01-15T10:00:00.123456789Z",
			wantErr: false,
		},
		{
			name:    "RFC3339Nano with microseconds",
			input:   "2024-01-15T10:00:00.123456Z",
			wantErr: false,
		},
		{
			name:    "RFC3339Nano with milliseconds",
			input:   "2024-01-15T10:00:00.123Z",
			wantErr: false,
		},
		{
			name:    "RFC3339 standard",
			input:   "2024-01-15T10:00:00Z",
			wantErr: false,
		},
		{
			name:    "RFC3339 with timezone offset",
			input:   "2024-01-15T10:00:00-05:00",
			wantErr: false,
		},
		{
			name:    "space-separated datetime",
			input:   "2024-01-15 10:00:00",
			wantErr: false,
		},
		{
			name:    "date only",
			input:   "2024-01-15",
			wantErr: false,
		},
		{
			name:    "invalid format",
			input:   "not-a-date",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseTimestamp(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for input %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error for input %q: %v", tt.input, err)
				return
			}
			if result.IsZero() {
				t.Errorf("expected non-zero time for input %q", tt.input)
			}
		})
	}
}

func TestParseTimestamp_RFC3339Nano_Preserves_FractionalSeconds(t *testing.T) {
	// Verify that fractional seconds are preserved
	input := "2024-01-15T10:00:00.123456789Z"
	result, err := parseTimestamp(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that nanoseconds are preserved (at least the precision we need)
	if result.Nanosecond() == 0 {
		t.Error("nanoseconds should be non-zero for RFC3339Nano input")
	}
}

func TestBDFetcher_EnrichBeadsWithDetails(t *testing.T) {
	basicBeads := []GraphBead{
		{ID: "bd-001", Title: "Task 1", Status: "open", IssueType: "task"},
		{ID: "bd-002", Title: "Task 2", Status: "open", IssueType: "task"},
	}

	enrichedBead1 := `[{"id": "bd-001", "title": "Task 1", "status": "open", "issue_type": "task", "dependencies": [{"id": "bd-epic-001", "dependency_type": "parent-child"}]}]`
	enrichedBead2 := `[{"id": "bd-002", "title": "Task 2", "status": "open", "issue_type": "task", "dependencies": [{"id": "bd-001", "dependency_type": "blocks"}]}]`

	runner := testutil.NewMockRunner()
	runner.SetResponse("bd", []string{"show", "bd-001", "--json"}, []byte(enrichedBead1))
	runner.SetResponse("bd", []string{"show", "bd-002", "--json"}, []byte(enrichedBead2))

	fetcher := NewBDFetcher(runner)
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
	runner := testutil.NewMockRunner()
	fetcher := NewBDFetcher(runner)

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

	enrichedBead1 := `[{"id": "bd-001", "title": "Task 1 enriched", "status": "open", "issue_type": "task", "dependencies": [{"id": "bd-epic-001", "dependency_type": "parent-child"}]}]`
	enrichedBead3 := `[{"id": "bd-003", "title": "Task 3 enriched", "status": "open", "issue_type": "task"}]`

	runner := testutil.NewMockRunner()
	runner.SetResponse("bd", []string{"show", "bd-001", "--json"}, []byte(enrichedBead1))
	runner.SetError("bd", []string{"show", "bd-002", "--json"}, errors.New("bead not found"))
	runner.SetResponse("bd", []string{"show", "bd-003", "--json"}, []byte(enrichedBead3))

	fetcher := NewBDFetcher(runner)
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

	runner := testutil.NewMockRunner()
	runner.DynamicResponse = func(ctx context.Context, name string, args []string) ([]byte, error, bool) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err(), true
		default:
			return []byte(`[{"id": "bd-001", "title": "Task", "status": "open"}]`), nil, true
		}
	}

	fetcher := NewBDFetcher(runner)

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

func TestBDFetcher_EnrichBeadsWithDetails_ConcurrencyLimit(t *testing.T) {
	// Create more beads than the concurrency limit
	beadCount := 10
	basicBeads := make([]GraphBead, beadCount)
	for i := 0; i < beadCount; i++ {
		basicBeads[i] = GraphBead{
			ID:        fmt.Sprintf("bd-%03d", i),
			Title:     fmt.Sprintf("Task %d", i),
			Status:    "open",
			IssueType: "task",
		}
	}

	var concurrentCount int64
	var maxConcurrent int64

	runner := testutil.NewMockRunner()
	runner.DynamicResponse = func(ctx context.Context, name string, args []string) ([]byte, error, bool) {
		if name == "bd" && len(args) >= 1 && args[0] == "show" {
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

			id := args[1]
			return []byte(fmt.Sprintf(`[{"id": "%s", "title": "Enriched", "status": "open"}]`, id)), nil, true
		}
		return nil, nil, false
	}

	fetcher := NewBDFetcher(runner)
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

