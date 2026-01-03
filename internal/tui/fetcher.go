package tui

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/npratt/atari/internal/testutil"
)

// BeadFetcher retrieves bead data for graph visualization.
type BeadFetcher interface {
	// FetchActive retrieves beads with open, in_progress, or blocked status.
	FetchActive(ctx context.Context) ([]GraphBead, error)
	// FetchBacklog retrieves beads with deferred status.
	FetchBacklog(ctx context.Context) ([]GraphBead, error)
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
func (f *BDFetcher) FetchActive(ctx context.Context) ([]GraphBead, error) {
	output, err := f.cmdRunner.Run(ctx, "bd", "list", "--json",
		"--status", "open",
		"--status", "in_progress",
		"--status", "blocked")
	if err != nil {
		return nil, fmt.Errorf("bd list active failed: %w", err)
	}

	return parseBeads(output)
}

// FetchBacklog retrieves beads with deferred status.
func (f *BDFetcher) FetchBacklog(ctx context.Context) ([]GraphBead, error) {
	output, err := f.cmdRunner.Run(ctx, "bd", "list", "--json", "--status", "deferred")
	if err != nil {
		return nil, fmt.Errorf("bd list backlog failed: %w", err)
	}

	return parseBeads(output)
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
