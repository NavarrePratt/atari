// Package workqueue manages work discovery by polling bd ready.
package workqueue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/testutil"
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
	config *config.Config
	runner testutil.CommandRunner
}

// New creates a Manager with the given config and command runner.
func New(cfg *config.Config, runner testutil.CommandRunner) *Manager {
	return &Manager{
		config: cfg,
		runner: runner,
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
