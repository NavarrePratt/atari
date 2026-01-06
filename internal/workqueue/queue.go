// Package workqueue manages work discovery by polling bd ready.
package workqueue

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/events"
	"github.com/npratt/atari/internal/testutil"
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
)

// Bead represents an issue from bd ready --json output.
type Bead struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	Priority    int       `json:"priority"`
	IssueType   string    `json:"issue_type"`
	Labels      []string  `json:"labels,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	CreatedBy   string    `json:"created_by"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Manager discovers available work by polling bd ready.
type Manager struct {
	config  *config.Config
	runner  testutil.CommandRunner
	history map[string]*BeadHistory
	mu      sync.RWMutex
}

// New creates a Manager with the given config and command runner.
func New(cfg *config.Config, runner testutil.CommandRunner) *Manager {
	return &Manager{
		config:  cfg,
		runner:  runner,
		history: make(map[string]*BeadHistory),
	}
}

// Poll executes bd ready --json and returns available beads.
// It applies the configured label filter and uses a 30 second timeout.
// Returns nil slice (not error) when no work is available.
func (m *Manager) Poll(ctx context.Context) ([]Bead, error) {
	// Apply 30 second timeout for bd command
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	args := []string{"ready", "--json"}
	if m.config.WorkQueue.Label != "" {
		args = append(args, "--label", m.config.WorkQueue.Label)
	}
	if m.config.WorkQueue.UnassignedOnly {
		args = append(args, "--unassigned")
	}

	output, err := m.runner.Run(ctx, "bd", args...)
	if err != nil {
		return nil, fmt.Errorf("bd ready failed: %w", err)
	}

	// Empty output means no work available
	if len(output) == 0 {
		return nil, nil
	}

	var beads []Bead
	if err := json.Unmarshal(output, &beads); err != nil {
		return nil, fmt.Errorf("parse bd ready output: %w", err)
	}

	// Empty array also means no work available
	if len(beads) == 0 {
		return nil, nil
	}

	return beads, nil
}

// Next polls for available work, filters by history, and returns the
// highest-priority eligible bead. Returns nil if no work is available
// or all beads are in backoff.
func (m *Manager) Next(ctx context.Context) (*Bead, error) {
	beads, err := m.Poll(ctx)
	if err != nil {
		return nil, err
	}
	if len(beads) == 0 {
		return nil, nil
	}

	eligible := m.filterEligible(beads)
	if len(eligible) == 0 {
		return nil, nil
	}

	// Sort by priority (lower = higher priority), then by created_at
	sort.Slice(eligible, func(i, j int) bool {
		if eligible[i].Priority != eligible[j].Priority {
			return eligible[i].Priority < eligible[j].Priority
		}
		return eligible[i].CreatedAt.Before(eligible[j].CreatedAt)
	})

	selected := eligible[0]

	// Mark as working and increment attempts
	m.mu.Lock()
	if m.history[selected.ID] == nil {
		m.history[selected.ID] = &BeadHistory{ID: selected.ID}
	}
	m.history[selected.ID].Status = HistoryWorking
	m.history[selected.ID].Attempts++
	m.history[selected.ID].LastAttempt = time.Now()
	m.mu.Unlock()

	return &selected, nil
}

// filterEligible returns beads that are not completed, abandoned, in backoff, or have excluded labels.
func (m *Manager) filterEligible(beads []Bead) []Bead {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var eligible []Bead
	now := time.Now()

	for _, bead := range beads {
		// Skip epics - they are containers, not work items
		if bead.IssueType == "epic" {
			continue
		}

		// Skip beads with excluded labels
		if m.hasExcludedLabel(bead.Labels) {
			continue
		}

		history := m.history[bead.ID]
		if history == nil {
			// Never seen before - eligible
			eligible = append(eligible, bead)
			continue
		}

		// Skip completed or abandoned beads
		if history.Status == HistoryCompleted || history.Status == HistoryAbandoned {
			continue
		}

		// Check failed beads for backoff and max failures
		if history.Status == HistoryFailed {
			// Check if we've hit max failures
			if m.config.Backoff.MaxFailures > 0 && history.Attempts >= m.config.Backoff.MaxFailures {
				// Would be abandoned - skip
				continue
			}
			// Check if still in backoff period
			backoff := m.calculateBackoff(history.Attempts)
			if now.Sub(history.LastAttempt) < backoff {
				continue
			}
		}

		eligible = append(eligible, bead)
	}

	return eligible
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
