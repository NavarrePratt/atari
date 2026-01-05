package tui

import (
	"context"
	"errors"
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
				runner.SetError("bd", []string{"list", "--json", "--status", "open", "--status", "in_progress", "--status", "blocked"}, tt.err)
			} else {
				runner.SetResponse("bd", []string{"list", "--json", "--status", "open", "--status", "in_progress", "--status", "blocked"}, tt.response)
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
				runner.SetError("bd", []string{"list", "--json", "--status", "deferred"}, tt.err)
			} else {
				runner.SetResponse("bd", []string{"list", "--json", "--status", "deferred"}, tt.response)
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

