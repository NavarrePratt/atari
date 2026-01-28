package tui

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/npratt/atari/internal/brclient"
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

// BDFetcher implements BeadFetcher using the brclient interface.
type BDFetcher struct {
	client brclient.BeadReader
}

// NewBDFetcher creates a BDFetcher with the given bead reader.
func NewBDFetcher(client brclient.BeadReader) *BDFetcher {
	return &BDFetcher{client: client}
}

// FetchActive retrieves beads with open, in_progress, or blocked status.
// Agent beads are filtered out as they are internal tracking beads.
func (f *BDFetcher) FetchActive(ctx context.Context) ([]GraphBead, error) {
	brBeads, err := f.client.List(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("br list active failed: %w", err)
	}

	beads := beadsToGraphBeads(brBeads)
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
	brBeads, err := f.client.List(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("br list backlog failed: %w", err)
	}

	beads := beadsToGraphBeads(brBeads)
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
	brBeads, err := f.client.List(ctx, &brclient.ListOptions{Status: "closed"})
	if err != nil {
		return nil, fmt.Errorf("br list closed failed: %w", err)
	}

	beads := beadsToGraphBeads(brBeads)

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
	brBead, err := f.client.Show(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("br show %s failed: %w", id, err)
	}
	if brBead == nil {
		return nil, fmt.Errorf("bead not found: %s", id)
	}

	bead := beadToGraphBead(brBead)

	// Fetch labels separately
	labels, err := f.client.Labels(ctx, id)
	if err == nil {
		bead.Labels = labels
	}
	// Ignore label fetch errors - labels are optional

	return &bead, nil
}

// maxConcurrentFetches is the maximum number of parallel br show commands.
const maxConcurrentFetches = 5

// enrichBeadsWithDetails fetches full dependency data for each bead in parallel.
// Uses a semaphore to limit concurrency to maxConcurrentFetches.
// On individual failures, the original bead data is retained.
// Panics in goroutines are recovered and logged; if any panic occurs, an error is returned.
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
	var panicOccurred bool

	for i := range beads {
		if err := sem.Acquire(ctx, 1); err != nil {
			wg.Wait()
			return result, err
		}

		wg.Add(1)
		go func(idx int) {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("panic in bead enrichment",
						"bead_id", beads[idx].ID,
						"panic", r,
						"stack", string(debug.Stack()))
					mu.Lock()
					panicOccurred = true
					mu.Unlock()
				}
				wg.Done()
				sem.Release(1)
			}()

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

	if panicOccurred {
		return result, fmt.Errorf("panic occurred during bead enrichment")
	}

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
	brBead, err := f.client.Show(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("br show %s failed: %w", id, err)
	}
	if brBead == nil {
		return nil, fmt.Errorf("bead not found: %s", id)
	}

	bead := beadToGraphBead(brBead)
	return &bead, nil
}

// beadToGraphBead converts a brclient.Bead to a GraphBead.
func beadToGraphBead(b *brclient.Bead) GraphBead {
	var deps []BeadReference
	for _, d := range b.Dependencies {
		deps = append(deps, BeadReference{
			ID:             d.ID,
			Title:          d.Title,
			Status:         d.Status,
			DependencyType: d.DependencyType,
		})
	}

	var dependents []BeadReference
	for _, d := range b.Dependents {
		dependents = append(dependents, BeadReference{
			ID:             d.ID,
			Title:          d.Title,
			Status:         d.Status,
			DependencyType: d.DependencyType,
		})
	}

	return GraphBead{
		ID:              b.ID,
		Title:           b.Title,
		Description:     b.Description,
		Status:          b.Status,
		Priority:        b.Priority,
		IssueType:       b.IssueType,
		CreatedAt:       b.CreatedAt.Format(time.RFC3339),
		CreatedBy:       b.CreatedBy,
		UpdatedAt:       b.UpdatedAt.Format(time.RFC3339),
		ClosedAt:        b.ClosedAt,
		Parent:          b.Parent,
		Notes:           b.Notes,
		Labels:          b.Labels,
		DependencyCount: b.DependencyCount,
		DependentCount:  b.DependentCount,
		Dependencies:    deps,
		Dependents:      dependents,
	}
}

// beadsToGraphBeads converts a slice of brclient.Bead to GraphBead slice.
func beadsToGraphBeads(brBeads []brclient.Bead) []GraphBead {
	if len(brBeads) == 0 {
		return nil
	}

	result := make([]GraphBead, len(brBeads))
	for i := range brBeads {
		result[i] = beadToGraphBead(&brBeads[i])
	}
	return result
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
