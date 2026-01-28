package tui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/npratt/atari/internal/testutil"
	"golang.org/x/sync/semaphore"
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
	output, err := f.cmdRunner.Run(ctx, "br", "list", "--json")
	if err != nil {
		return nil, fmt.Errorf("br list active failed: %w", err)
	}

	beads, err := parseBeads(output)
	if err != nil {
		return nil, err
	}

	beads = filterByStatus(beads, "open", "in_progress", "blocked")

	beads, err = f.enrichBeadsWithDetails(ctx, beads)
	if err != nil {
		return nil, fmt.Errorf("failed to enrich active beads: %w", err)
	}

	return filterOutAgentBeads(beads), nil
}

// FetchBacklog retrieves beads with deferred status.
// Agent beads are filtered out as they are internal tracking beads.
func (f *BDFetcher) FetchBacklog(ctx context.Context) ([]GraphBead, error) {
	output, err := f.cmdRunner.Run(ctx, "br", "list", "--json")
	if err != nil {
		return nil, fmt.Errorf("br list backlog failed: %w", err)
	}

	beads, err := parseBeads(output)
	if err != nil {
		return nil, err
	}

	beads = filterByStatus(beads, "deferred")

	beads, err = f.enrichBeadsWithDetails(ctx, beads)
	if err != nil {
		return nil, fmt.Errorf("failed to enrich backlog beads: %w", err)
	}

	return filterOutAgentBeads(beads), nil
}

// FetchClosed retrieves beads closed within the last 7 days.
// Agent beads are filtered out as they are internal tracking beads.
// Note: br CLI lacks --closed-after flag, so date filtering is done in Go.
func (f *BDFetcher) FetchClosed(ctx context.Context) ([]GraphBead, error) {
	output, err := f.cmdRunner.Run(ctx, "br", "list", "--status", "closed", "--json")
	if err != nil {
		return nil, fmt.Errorf("br list closed failed: %w", err)
	}

	beads, err := parseBeads(output)
	if err != nil {
		return nil, err
	}

	// Filter to beads closed within last 7 days (br lacks --closed-after)
	cutoff := time.Now().AddDate(0, 0, -7)
	beads = filterClosedAfter(beads, cutoff)

	beads, err = f.enrichBeadsWithDetails(ctx, beads)
	if err != nil {
		return nil, fmt.Errorf("failed to enrich closed beads: %w", err)
	}

	return filterOutAgentBeads(beads), nil
}

// FetchBead retrieves full details for a single bead by ID.
func (f *BDFetcher) FetchBead(ctx context.Context, id string) (*GraphBead, error) {
	output, err := f.cmdRunner.Run(ctx, "br", "show", id, "--json")
	if err != nil {
		return nil, fmt.Errorf("br show %s failed: %w", id, err)
	}

	bead, err := parseBead(output)
	if err != nil {
		return nil, err
	}

	// Fetch labels separately
	labelsOutput, err := f.cmdRunner.Run(ctx, "br", "label", "list", id, "--json")
	if err == nil {
		labels, parseErr := parseLabels(labelsOutput)
		if parseErr == nil {
			bead.Labels = labels
		}
	}
	// Ignore label fetch errors - labels are optional

	return bead, nil
}

// maxConcurrentFetches is the maximum number of parallel br show commands.
const maxConcurrentFetches = 5

// enrichBeadsWithDetails fetches full dependency data for each bead in parallel.
// Uses a semaphore to limit concurrency to maxConcurrentFetches.
// On individual failures, the original bead data is retained.
func (f *BDFetcher) enrichBeadsWithDetails(ctx context.Context, beads []GraphBead) ([]GraphBead, error) {
	if len(beads) == 0 {
		return beads, nil
	}

	result := make([]GraphBead, len(beads))
	copy(result, beads)

	sem := semaphore.NewWeighted(maxConcurrentFetches)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var failedIDs []string

	for i := range beads {
		if err := sem.Acquire(ctx, 1); err != nil {
			return result, err
		}

		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			defer sem.Release(1)

			bead := beads[idx]
			enriched, err := f.fetchBeadDetails(ctx, bead.ID)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					slog.Debug("bead enrichment cancelled", "bead_id", bead.ID)
				} else {
					slog.Warn("failed to enrich bead, using basic data",
						"bead_id", bead.ID,
						"error", err)
					mu.Lock()
					failedIDs = append(failedIDs, bead.ID)
					mu.Unlock()
				}
				return
			}

			mu.Lock()
			result[idx] = *enriched
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	if len(failedIDs) > 0 {
		slog.Warn("enrichment partially failed",
			"total", len(beads),
			"failed", len(failedIDs),
			"failed_ids", failedIDs)
	}

	return result, nil
}

// fetchBeadDetails fetches full bead details without labels.
// Used by enrichBeadsWithDetails for parallel fetching.
func (f *BDFetcher) fetchBeadDetails(ctx context.Context, id string) (*GraphBead, error) {
	output, err := f.cmdRunner.Run(ctx, "br", "show", id, "--json")
	if err != nil {
		return nil, fmt.Errorf("br show %s failed: %w", id, err)
	}

	return parseBead(output)
}

// parseBead parses JSON output from br show into a single GraphBead.
func parseBead(data []byte) (*GraphBead, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty response")
	}

	// br show --json returns an array with one element
	var beads []GraphBead
	if err := json.Unmarshal(data, &beads); err != nil {
		return nil, fmt.Errorf("failed to parse bead data: %w", err)
	}

	if len(beads) == 0 {
		return nil, fmt.Errorf("bead not found")
	}

	return &beads[0], nil
}

// parseBeads parses JSON output from br list into GraphBead slice.
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

// parseLabels parses JSON output from br label list into a string slice.
func parseLabels(data []byte) ([]string, error) {
	// Handle empty response
	if len(data) == 0 {
		return nil, nil
	}

	var labels []string
	if err := json.Unmarshal(data, &labels); err != nil {
		return nil, fmt.Errorf("failed to parse labels: %w", err)
	}

	return labels, nil
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

// filterClosedAfter returns beads with ClosedAt after the given cutoff time.
// Beads without a valid ClosedAt timestamp are excluded.
func filterClosedAfter(beads []GraphBead, cutoff time.Time) []GraphBead {
	if len(beads) == 0 {
		return nil
	}

	result := make([]GraphBead, 0, len(beads))
	for _, b := range beads {
		if b.ClosedAt == "" {
			continue
		}
		closedAt, err := time.Parse(time.RFC3339, b.ClosedAt)
		if err != nil {
			continue
		}
		if closedAt.After(cutoff) {
			result = append(result, b)
		}
	}

	return result
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
