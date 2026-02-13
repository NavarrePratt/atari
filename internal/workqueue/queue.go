// Package workqueue manages work discovery by polling br ready.
package workqueue

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/npratt/atari/internal/brclient"
	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/events"
	"github.com/npratt/atari/internal/viewmodel"
)

// Re-export types from events package for convenience.
type BeadHistory = events.BeadHistory
type HistoryStatus = events.HistoryStatus

// Re-export HistoryStatus constants.
const (
	HistoryPending   = events.HistoryPending
	HistoryWorking   = events.HistoryWorking
	HistoryCompleted = events.HistoryCompleted
	HistoryFailed    = events.HistoryFailed
	HistoryAbandoned = events.HistoryAbandoned
	HistorySkipped   = events.HistorySkipped
)

// SelectionReason indicates why a bead selection returned the result it did.
type SelectionReason int

const (
	ReasonSuccess    SelectionReason = iota // Bead selected successfully
	ReasonNoReady                           // No ready beads available
	ReasonBackoff                           // All ready beads in backoff
	ReasonMaxFailure                        // All ready beads hit max failures
)

// String returns a human-readable description of the selection reason.
func (r SelectionReason) String() string {
	switch r {
	case ReasonSuccess:
		return "success"
	case ReasonNoReady:
		return "no ready beads"
	case ReasonBackoff:
		return "all beads in backoff"
	case ReasonMaxFailure:
		return "all beads hit max failures"
	default:
		return "unknown"
	}
}

// Bead is an alias to brclient.Bead for backward compatibility.
type Bead = brclient.Bead

// Manager discovers available work by polling br ready.
type Manager struct {
	config         *config.Config
	client         brclient.WorkQueueClient
	history        map[string]*BeadHistory
	activeTopLevel string // Runtime state: currently active top-level item ID
	logger         *slog.Logger
	mu             sync.RWMutex
}

// New creates a Manager with the given config and br client.
// If logger is nil, slog.Default() is used.
func New(cfg *config.Config, client brclient.WorkQueueClient, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		config:  cfg,
		client:  client,
		history: make(map[string]*BeadHistory),
		logger:  logger,
	}
}

// Poll executes br ready --json and returns available beads.
// It applies the configured label filter.
// Returns nil slice (not error) when no work is available.
func (m *Manager) Poll(ctx context.Context) ([]Bead, error) {
	opts := &brclient.ReadyOptions{
		Label:          m.config.WorkQueue.Label,
		UnassignedOnly: m.config.WorkQueue.UnassignedOnly,
	}

	beads, err := m.client.Ready(ctx, opts)
	if err != nil {
		return nil, err
	}

	if len(beads) == 0 {
		return nil, nil
	}

	return beads, nil
}

// Next polls for available work, filters by history, and returns the
// highest-priority eligible bead. Returns nil if no work is available
// or all beads are in backoff. The SelectionReason indicates why no bead
// was selected when the result is nil.
func (m *Manager) Next(ctx context.Context) (*Bead, SelectionReason, error) {
	beads, err := m.Poll(ctx)
	if err != nil {
		return nil, ReasonNoReady, err
	}
	if len(beads) == 0 {
		return nil, ReasonNoReady, nil
	}

	// If epic filter is configured, fetch descendants to filter by
	var epicDescendants map[string]bool
	if m.config.WorkQueue.Epic != "" {
		epicDescendants, err = m.fetchDescendants(ctx, m.config.WorkQueue.Epic)
		if err != nil {
			return nil, ReasonNoReady, fmt.Errorf("fetch epic descendants: %w", err)
		}
	}

	result := m.filterEligible(beads, epicDescendants)
	if len(result.eligible) == 0 {
		return nil, result.reason(), nil
	}

	// Sort by priority (lower = higher priority), then by created_at
	sort.Slice(result.eligible, func(i, j int) bool {
		if result.eligible[i].Priority != result.eligible[j].Priority {
			return result.eligible[i].Priority < result.eligible[j].Priority
		}
		return result.eligible[i].CreatedAt.Before(result.eligible[j].CreatedAt)
	})

	selected := result.eligible[0]

	// Mark as working and increment attempts
	m.mu.Lock()
	if m.history[selected.ID] == nil {
		m.history[selected.ID] = &BeadHistory{ID: selected.ID}
	}
	m.history[selected.ID].Status = HistoryWorking
	m.history[selected.ID].Attempts++
	m.history[selected.ID].LastAttempt = time.Now()
	m.mu.Unlock()

	return &selected, ReasonSuccess, nil
}

// filterResult holds the result of filtering beads along with skip reason counts.
type filterResult struct {
	eligible         []Bead
	skippedBackoff   int
	skippedMaxFailed int
}

// reason computes the SelectionReason from the filter result.
// Returns ReasonSuccess if there are eligible beads.
// Otherwise returns the most specific reason for why all beads were filtered.
func (r *filterResult) reason() SelectionReason {
	if len(r.eligible) > 0 {
		return ReasonSuccess
	}
	// Prioritize max failure over backoff as the reason
	if r.skippedMaxFailed > 0 && r.skippedBackoff == 0 {
		return ReasonMaxFailure
	}
	if r.skippedBackoff > 0 {
		return ReasonBackoff
	}
	// No beads were skipped due to backoff or max failures,
	// so there simply weren't any ready beads (or all were filtered by other criteria)
	return ReasonNoReady
}

// filterEligible returns beads that are not completed, abandoned, in backoff, or have excluded labels.
// If epicDescendants is non-nil, only beads in that set are considered.
// Also tracks skip reasons to determine why selection failed.
func (m *Manager) filterEligible(beads []Bead, epicDescendants map[string]bool) filterResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result filterResult
	now := time.Now()

	for _, bead := range beads {
		// Skip epics - they are containers, not work items
		if bead.IssueType == "epic" {
			continue
		}

		// If epic filter is active, skip beads not in descendant set
		if epicDescendants != nil && !epicDescendants[bead.ID] {
			continue
		}

		// Skip beads with excluded labels
		if m.hasExcludedLabel(bead.Labels) {
			continue
		}

		history := m.history[bead.ID]
		if history == nil {
			// Never seen before - eligible
			result.eligible = append(result.eligible, bead)
			continue
		}

		// Skip completed, abandoned, or skipped beads
		if history.Status == HistoryCompleted || history.Status == HistoryAbandoned || history.Status == HistorySkipped {
			continue
		}

		// Check failed beads for backoff and max failures
		if history.Status == HistoryFailed {
			// Check if we've hit max failures
			if m.config.Backoff.MaxFailures > 0 && history.Attempts >= m.config.Backoff.MaxFailures {
				// Log only on first skip (status hasn't been marked abandoned yet)
				m.logger.Warn("bead hit max failures, skipping",
					"bead_id", bead.ID,
					"attempts", history.Attempts,
					"max_failures", m.config.Backoff.MaxFailures,
					"last_error", history.LastError)
				result.skippedMaxFailed++
				continue
			}
			// Check if still in backoff period
			backoff := m.calculateBackoff(history.Attempts)
			if now.Sub(history.LastAttempt) < backoff {
				result.skippedBackoff++
				continue
			}
		}

		result.eligible = append(result.eligible, bead)
	}

	return result
}

// calculateBackoff returns the backoff duration for a given number of attempts.
// Returns 0 for first attempt, Initial for second attempt, then exponential growth capped at max.
func (m *Manager) calculateBackoff(attempts int) time.Duration {
	if attempts <= 1 {
		return 0
	}

	backoff := m.config.Backoff.Initial
	for i := 2; i < attempts; i++ {
		backoff = time.Duration(float64(backoff) * m.config.Backoff.Multiplier)
		if backoff > m.config.Backoff.Max {
			return m.config.Backoff.Max
		}
	}

	return backoff
}

// hasExcludedLabel returns true if any of the bead's labels are in the exclude list.
func (m *Manager) hasExcludedLabel(beadLabels []string) bool {
	if len(m.config.WorkQueue.ExcludeLabels) == 0 {
		return false
	}
	for _, beadLabel := range beadLabels {
		for _, excludeLabel := range m.config.WorkQueue.ExcludeLabels {
			if beadLabel == excludeLabel {
				return true
			}
		}
	}
	return false
}

// fetchDescendants fetches all beads and builds a set of IDs that are descendants
// of the given epic ID. Returns a map where keys are bead IDs that are descendants
// (including the epic itself). The algorithm iteratively adds beads whose parent
// is already in the set until no new beads are found.
func (m *Manager) fetchDescendants(ctx context.Context, epicID string) (map[string]bool, error) {
	beads, err := m.fetchAllBeads(ctx)
	if err != nil {
		return nil, err
	}

	if len(beads) == 0 {
		return map[string]bool{epicID: true}, nil
	}

	return buildDescendantSet(epicID, beads), nil
}

// buildDescendantSet builds a set of bead IDs that are descendants of the given epic.
// The epic ID itself is included in the set. Uses iterative expansion: starting with
// the epic, repeatedly add any bead whose parent is already in the set.
func buildDescendantSet(epicID string, beads []Bead) map[string]bool {
	descendants := map[string]bool{epicID: true}

	for {
		added := false
		for _, bead := range beads {
			if bead.Parent == "" {
				continue
			}
			if descendants[bead.Parent] && !descendants[bead.ID] {
				descendants[bead.ID] = true
				added = true
			}
		}
		if !added {
			break
		}
	}

	return descendants
}

// fetchAllBeads retrieves all beads and populates parent fields.
//
// TODO: Refactor back to br list when it includes parent field.
// See: https://github.com/coreweave/beads-rust/issues/XXX
// Current workaround uses br show per-bead which is N+1.
func (m *Manager) fetchAllBeads(ctx context.Context) ([]Bead, error) {
	beads, err := m.client.List(ctx, nil)
	if err != nil {
		return nil, err
	}

	if len(beads) == 0 {
		return nil, nil
	}

	// Populate parent field using br show for beads missing it.
	// br list --json does not include parent, but br show --json does.
	for i := range beads {
		if beads[i].Parent != "" {
			continue
		}

		full, err := m.client.Show(ctx, beads[i].ID)
		if err != nil {
			slog.Warn("failed to fetch parent for bead, skipping",
				"bead_id", beads[i].ID,
				"error", err)
			continue
		}
		if full == nil {
			slog.Warn("br show returned nil for bead, skipping parent lookup",
				"bead_id", beads[i].ID)
			continue
		}

		beads[i].Parent = full.Parent
	}

	return beads, nil
}

// identifyTopLevelItems returns beads that are either epics or have no parent.
// Results are sorted by priority (ascending), then by creation time (oldest first).
func identifyTopLevelItems(beads []Bead) []Bead {
	var topLevel []Bead
	for _, bead := range beads {
		if bead.IssueType == "epic" || bead.Parent == "" {
			topLevel = append(topLevel, bead)
		}
	}

	sort.Slice(topLevel, func(i, j int) bool {
		if topLevel[i].Priority != topLevel[j].Priority {
			return topLevel[i].Priority < topLevel[j].Priority
		}
		return topLevel[i].CreatedAt.Before(topLevel[j].CreatedAt)
	})

	return topLevel
}

// hasReadyDescendants checks if a top-level item has any ready descendants.
// It returns true if there are ready beads that are descendants of the given top-level ID.
func hasReadyDescendants(topLevelID string, readyBeads []Bead, allBeads []Bead) bool {
	descendants := buildDescendantSet(topLevelID, allBeads)

	for _, bead := range readyBeads {
		// Skip epics themselves
		if bead.IssueType == "epic" {
			continue
		}
		// Check if this ready bead is a descendant (or the top-level itself if not an epic)
		if descendants[bead.ID] {
			return true
		}
	}
	return false
}

// selectBestTopLevel finds the highest-priority top-level item that has ready work.
// Returns empty string if no top-level item has ready descendants.
func selectBestTopLevel(topLevelItems []Bead, readyBeads []Bead, allBeads []Bead) string {
	for _, item := range topLevelItems {
		if hasReadyDescendants(item.ID, readyBeads, allBeads) {
			return item.ID
		}
	}
	return ""
}

// NextTopLevel selects the next bead using top-level selection mode.
// This mode groups work by top-level items (epics + standalone beads) and
// focuses on one top-level at a time until exhausted.
//
// Algorithm:
// 1. If activeTopLevel is set and has ready descendants, use it
// 2. Otherwise, identify all top-level items, pick highest priority with ready work
// 3. Filter beads to descendants of selected top-level item
// 4. Apply existing filterEligible logic
// 5. Return highest priority descendant
//
// The SelectionReason indicates why no bead was selected when the result is nil.
func (m *Manager) NextTopLevel(ctx context.Context) (*Bead, SelectionReason, error) {
	// Get ready beads
	readyBeads, err := m.Poll(ctx)
	if err != nil {
		return nil, ReasonNoReady, err
	}
	if len(readyBeads) == 0 {
		return nil, ReasonNoReady, nil
	}

	// Get all beads for hierarchy traversal
	allBeads, err := m.fetchAllBeads(ctx)
	if err != nil {
		return nil, ReasonNoReady, fmt.Errorf("fetch all beads: %w", err)
	}

	m.mu.Lock()
	activeTopLevel := m.activeTopLevel
	m.mu.Unlock()

	// Check if active top-level is still valid and has work
	if activeTopLevel != "" {
		if hasReadyDescendants(activeTopLevel, readyBeads, allBeads) {
			// Continue with current top-level
			return m.selectFromTopLevel(activeTopLevel, readyBeads, allBeads)
		}
		// Active top-level is exhausted, clear it
		m.mu.Lock()
		m.activeTopLevel = ""
		m.mu.Unlock()
	}

	// Select new top-level item
	topLevelItems := identifyTopLevelItems(allBeads)
	newTopLevel := selectBestTopLevel(topLevelItems, readyBeads, allBeads)
	if newTopLevel == "" {
		// No top-level items with ready work; fall back to global selection
		// This handles orphaned beads that somehow aren't under any top-level
		return m.selectFromTopLevel("", readyBeads, allBeads)
	}

	// Set new active top-level
	m.mu.Lock()
	m.activeTopLevel = newTopLevel
	m.mu.Unlock()

	return m.selectFromTopLevel(newTopLevel, readyBeads, allBeads)
}

// selectFromTopLevel filters ready beads to descendants of the given top-level
// and returns the highest priority eligible bead.
func (m *Manager) selectFromTopLevel(topLevelID string, readyBeads []Bead, allBeads []Bead) (*Bead, SelectionReason, error) {
	var epicDescendants map[string]bool
	if topLevelID != "" {
		epicDescendants = buildDescendantSet(topLevelID, allBeads)
	}

	result := m.filterEligible(readyBeads, epicDescendants)
	if len(result.eligible) == 0 {
		return nil, result.reason(), nil
	}

	// Sort by priority (lower = higher priority), then by created_at
	sort.Slice(result.eligible, func(i, j int) bool {
		if result.eligible[i].Priority != result.eligible[j].Priority {
			return result.eligible[i].Priority < result.eligible[j].Priority
		}
		return result.eligible[i].CreatedAt.Before(result.eligible[j].CreatedAt)
	})

	selected := result.eligible[0]

	// Mark as working and increment attempts
	m.mu.Lock()
	if m.history[selected.ID] == nil {
		m.history[selected.ID] = &BeadHistory{ID: selected.ID}
	}
	m.history[selected.ID].Status = HistoryWorking
	m.history[selected.ID].Attempts++
	m.history[selected.ID].LastAttempt = time.Now()
	m.mu.Unlock()

	return &selected, ReasonSuccess, nil
}

// ActiveTopLevel returns the currently active top-level item ID.
// Returns empty string if no top-level is active.
func (m *Manager) ActiveTopLevel() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeTopLevel
}

// ClearActiveTopLevel clears the active top-level item.
// This is useful when you want to force selection of a new top-level.
func (m *Manager) ClearActiveTopLevel() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeTopLevel = ""
}

// SetActiveTopLevel sets the active top-level item ID.
// This is useful for restoring state or testing.
func (m *Manager) SetActiveTopLevel(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeTopLevel = id
}

// HasEligibleReadyDescendants checks if a top-level item has any eligible ready descendants.
// It returns true if there are ready beads that are:
// 1. Descendants of the given top-level ID (or the top-level itself if not an epic)
// 2. Not epics
// 3. Not completed, abandoned, or skipped in history
// 4. Not at max failures
//
// This is used to verify that restoring an active top-level from persisted state
// won't cause immediate re-stall.
func (m *Manager) HasEligibleReadyDescendants(ctx context.Context, topLevelID string) (bool, error) {
	// Get ready beads
	readyBeads, err := m.Poll(ctx)
	if err != nil {
		return false, err
	}
	if len(readyBeads) == 0 {
		return false, nil
	}

	// Get all beads for hierarchy traversal (with parent fields populated)
	allBeads, err := m.fetchAllBeads(ctx)
	if err != nil {
		return false, fmt.Errorf("fetch all beads: %w", err)
	}

	// Build descendant set
	descendants := buildDescendantSet(topLevelID, allBeads)

	// Filter to just ready descendants (excluding epics)
	var descendantBeads []Bead
	for _, bead := range readyBeads {
		if bead.IssueType == "epic" {
			continue
		}
		if descendants[bead.ID] {
			descendantBeads = append(descendantBeads, bead)
		}
	}

	if len(descendantBeads) == 0 {
		return false, nil
	}

	// Check eligibility using existing logic (no additional epic filter needed)
	result := m.filterEligible(descendantBeads, nil)
	return len(result.eligible) > 0, nil
}

// RecordSuccess marks a bead as completed.
func (m *Manager) RecordSuccess(beadID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.history[beadID] == nil {
		m.history[beadID] = &BeadHistory{ID: beadID}
	}
	m.history[beadID].Status = HistoryCompleted
}

// RecordFailure marks a bead as failed with the given error.
// If the bead has exceeded max failures, it will be marked as abandoned.
func (m *Manager) RecordFailure(beadID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.history[beadID] == nil {
		m.history[beadID] = &BeadHistory{ID: beadID}
	}
	h := m.history[beadID]
	h.LastError = err.Error()
	h.LastAttempt = time.Now()

	// Check if we've exceeded max failures
	if m.config.Backoff.MaxFailures > 0 && h.Attempts >= m.config.Backoff.MaxFailures {
		h.Status = HistoryAbandoned
	} else {
		h.Status = HistoryFailed
	}
}

// RecordSkipped marks a bead as skipped (user chose to move past it).
func (m *Manager) RecordSkipped(beadID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.history[beadID] == nil {
		m.history[beadID] = &BeadHistory{ID: beadID}
	}
	m.history[beadID].Status = HistorySkipped
}

// ResetHistory clears the history for a bead, allowing it to be retried.
func (m *Manager) ResetHistory(beadID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.history, beadID)
}

// ResetBead clears failure state for a bead, allowing it to be retried.
// Unlike ResetHistory, this preserves the history entry but resets all state.
func (m *Manager) ResetBead(beadID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if h := m.history[beadID]; h != nil {
		h.Status = HistoryPending
		h.Attempts = 0
		h.LastAttempt = time.Time{}
		h.LastError = ""
	}
}

// QueueStats provides statistics about the work queue.
type QueueStats struct {
	TotalSeen int
	Completed int
	Failed    int
	Abandoned int
	InBackoff int
}

// Stats returns current queue statistics.
func (m *Manager) Stats() QueueStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var stats QueueStats
	now := time.Now()

	for _, h := range m.history {
		stats.TotalSeen++
		switch h.Status {
		case HistoryCompleted:
			stats.Completed++
		case HistoryFailed:
			stats.Failed++
			backoff := m.calculateBackoff(h.Attempts)
			if now.Sub(h.LastAttempt) < backoff {
				stats.InBackoff++
			}
		case HistoryAbandoned:
			stats.Abandoned++
		}
	}

	return stats
}

// History returns a copy of the current bead history map.
// This is useful for state persistence.
func (m *Manager) History() map[string]*BeadHistory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*BeadHistory, len(m.history))
	for k, v := range m.history {
		// Copy the struct to prevent mutation
		copy := *v
		result[k] = &copy
	}
	return result
}

// SetHistory restores history from persisted state.
// This is called during recovery after a restart.
func (m *Manager) SetHistory(history map[string]*BeadHistory) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.history = make(map[string]*BeadHistory, len(history))
	for k, v := range history {
		copy := *v
		m.history[k] = &copy
	}
}

// GetBeadState returns the workqueue state for a bead.
// Returns:
//   - status: "", "failed", "abandoned", or "skipped"
//   - attempts: number of attempts (0 if never tried)
//   - inBackoff: true if bead is currently in backoff period
func (m *Manager) GetBeadState(beadID string) (status string, attempts int, inBackoff bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	h, ok := m.history[beadID]
	if !ok {
		return "", 0, false
	}

	attempts = h.Attempts

	switch h.Status {
	case HistoryFailed:
		status = "failed"
		// Check if in backoff
		if !h.LastAttempt.IsZero() {
			backoff := m.calculateBackoff(h.Attempts)
			elapsed := time.Since(h.LastAttempt)
			inBackoff = elapsed < backoff
		}
	case HistoryAbandoned:
		status = "abandoned"
	case HistorySkipped:
		status = "skipped"
	default:
		// pending, working, completed - not relevant for styling
		status = ""
	}

	return status, attempts, inBackoff
}

// GetBlockedBeads returns beads that are currently in backoff, sorted by
// shortest remaining backoff (most urgent first). Only includes beads with
// HistoryFailed status and positive remaining backoff time.
func (m *Manager) GetBlockedBeads() []viewmodel.BlockedBeadInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	var blocked []viewmodel.BlockedBeadInfo

	for _, h := range m.history {
		if h.Status != HistoryFailed {
			continue
		}

		// Skip entries with missing LastAttempt (graceful degradation)
		if h.LastAttempt.IsZero() {
			continue
		}

		backoff := m.calculateBackoff(h.Attempts)
		elapsed := now.Sub(h.LastAttempt)
		remaining := backoff - elapsed

		// Only include if still in backoff (remaining > 0)
		if remaining <= 0 {
			continue
		}

		blocked = append(blocked, viewmodel.BlockedBeadInfo{
			BeadID:       h.ID,
			FailureCount: h.Attempts,
			RetryIn:      remaining,
			LastError:    h.LastError,
		})
	}

	// Sort by shortest remaining backoff (most urgent first)
	sort.Slice(blocked, func(i, j int) bool {
		return blocked[i].RetryIn < blocked[j].RetryIn
	})

	return blocked
}
