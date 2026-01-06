package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/npratt/atari/internal/testutil"
)

// BeadFetcher retrieves bead data for graph visualization.
type BeadFetcher interface {
	// FetchActive retrieves beads with open, in_progress, or blocked status.
	FetchActive(ctx context.Context) ([]GraphBead, error)
	// FetchBacklog retrieves beads with deferred status.
	FetchBacklog(ctx context.Context) ([]GraphBead, error)
	// FetchClosed retrieves beads closed within the last 7 days.
	FetchClosed(ctx context.Context) ([]GraphBead, error)
	// FetchBead retrieves full details for a single bead by ID.
	FetchBead(ctx context.Context, id string) (*GraphBead, error)
}

// BDFetcher implements BeadFetcher using the bd CLI.
type BDFetcher struct {
	cmdRunner testutil.CommandRunner
}

// NewBDFetcher creates a BDFetcher with the given command runner.
func NewBDFetcher(runner testutil.CommandRunner) *BDFetcher {
	return &BDFetcher{cmdRunner: runner}
}

// FetchActive retrieves beads with open, in_progress, or blocked status.
// Agent beads are filtered out as they are internal tracking beads.
func (f *BDFetcher) FetchActive(ctx context.Context) ([]GraphBead, error) {
	output, err := f.cmdRunner.Run(ctx, "bd", "list", "--json")
	if err != nil {
		return nil, fmt.Errorf("bd list active failed: %w", err)
	}

	beads, err := parseBeads(output)
	if err != nil {
		return nil, err
	}

	beads = filterByStatus(beads, "open", "in_progress", "blocked")
	return filterOutAgentBeads(beads), nil
}

// FetchBacklog retrieves beads with deferred status.
// Agent beads are filtered out as they are internal tracking beads.
func (f *BDFetcher) FetchBacklog(ctx context.Context) ([]GraphBead, error) {
	output, err := f.cmdRunner.Run(ctx, "bd", "list", "--json")
	if err != nil {
		return nil, fmt.Errorf("bd list backlog failed: %w", err)
	}

	beads, err := parseBeads(output)
	if err != nil {
		return nil, err
	}

	beads = filterByStatus(beads, "deferred")
	return filterOutAgentBeads(beads), nil
}

// FetchClosed retrieves beads closed within the last 7 days.
// Agent beads are filtered out as they are internal tracking beads.
func (f *BDFetcher) FetchClosed(ctx context.Context) ([]GraphBead, error) {
	output, err := f.cmdRunner.Run(ctx, "bd", "list", "--json")
	if err != nil {
		return nil, fmt.Errorf("bd list closed failed: %w", err)
	}

	beads, err := parseBeads(output)
	if err != nil {
		return nil, err
	}

	beads = filterByStatus(beads, "closed")
	beads = filterClosedWithinDays(beads, 7)
	return filterOutAgentBeads(beads), nil
}

// FetchBead retrieves full details for a single bead by ID.
func (f *BDFetcher) FetchBead(ctx context.Context, id string) (*GraphBead, error) {
	output, err := f.cmdRunner.Run(ctx, "bd", "show", id, "--json")
	if err != nil {
		return nil, fmt.Errorf("bd show %s failed: %w", id, err)
	}

	return parseBead(output)
}

// parseBead parses JSON output from bd show into a single GraphBead.
func parseBead(data []byte) (*GraphBead, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty response")
	}

	// bd show --json returns an array with one element
	var beads []GraphBead
	if err := json.Unmarshal(data, &beads); err != nil {
		return nil, fmt.Errorf("failed to parse bead data: %w", err)
	}

	if len(beads) == 0 {
		return nil, fmt.Errorf("bead not found")
	}

	return &beads[0], nil
}

// parseBeads parses JSON output from bd list into GraphBead slice.
func parseBeads(data []byte) ([]GraphBead, error) {
	// Handle empty response
	if len(data) == 0 {
		return nil, nil
	}

	var beads []GraphBead
	if err := json.Unmarshal(data, &beads); err != nil {
		return nil, fmt.Errorf("failed to parse bead data: %w", err)
	}

	return beads, nil
}

// filterByStatus returns beads matching any of the given statuses.
func filterByStatus(beads []GraphBead, statuses ...string) []GraphBead {
	if len(beads) == 0 {
		return nil
	}

	statusSet := make(map[string]bool, len(statuses))
	for _, s := range statuses {
		statusSet[s] = true
	}

	result := make([]GraphBead, 0, len(beads))
	for _, b := range beads {
		if statusSet[b.Status] {
			result = append(result, b)
		}
	}

	return result
}

// filterOutAgentBeads removes beads with issue_type="agent" from the slice.
// Agent beads are internal tracking beads that should not appear in the graph.
func filterOutAgentBeads(beads []GraphBead) []GraphBead {
	if len(beads) == 0 {
		return nil
	}

	result := make([]GraphBead, 0, len(beads))
	for _, b := range beads {
		if b.IssueType != "agent" {
			result = append(result, b)
		}
	}

	return result
}

// filterClosedWithinDays returns beads whose ClosedAt timestamp is within the last N days.
func filterClosedWithinDays(beads []GraphBead, days int) []GraphBead {
	if len(beads) == 0 {
		return nil
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	result := make([]GraphBead, 0, len(beads))

	for _, b := range beads {
		if b.ClosedAt == "" {
			continue
		}
		// Try parsing common timestamp formats
		closedTime, err := parseTimestamp(b.ClosedAt)
		if err != nil {
			continue
		}
		if closedTime.After(cutoff) {
			result = append(result, b)
		}
	}

	return result
}

// parseTimestamp parses a timestamp string in common formats.
func parseTimestamp(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", s)
}

// ToNodesAndEdges converts a slice of GraphBeads to nodes and edges.
// This is a convenience function for graph construction.
func ToNodesAndEdges(beads []GraphBead) ([]GraphNode, []GraphEdge) {
	nodes := make([]GraphNode, 0, len(beads))
	var edges []GraphEdge

	for i := range beads {
		nodes = append(nodes, beads[i].ToNode())
		edges = append(edges, beads[i].ExtractEdges()...)
	}

	return nodes, edges
}
