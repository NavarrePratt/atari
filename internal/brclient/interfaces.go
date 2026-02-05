// Package brclient provides interfaces and implementations for interacting with the br CLI.
// It abstracts br CLI operations to enable unit testing with mocks.
package brclient

import (
	"context"
	"time"
)

// Bead represents an issue from br CLI output.
type Bead struct {
	ID              string          `json:"id"`
	Title           string          `json:"title"`
	Description     string          `json:"description"`
	Status          string          `json:"status"`
	Priority        int             `json:"priority"`
	IssueType       string          `json:"issue_type"`
	Labels          []string        `json:"labels,omitempty"`
	Parent          string          `json:"parent,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	CreatedBy       string          `json:"created_by"`
	UpdatedAt       time.Time       `json:"updated_at"`
	ClosedAt        string          `json:"closed_at,omitempty"`
	ClosedBy        string          `json:"closed_by,omitempty"`
	CloseReason     string          `json:"close_reason,omitempty"`
	BlockedBy       []string        `json:"blocked_by,omitempty"`
	Blocks          []string        `json:"blocks,omitempty"`
	Notes           string          `json:"notes,omitempty"`
	DependencyCount int             `json:"dependency_count,omitempty"`
	DependentCount  int             `json:"dependent_count,omitempty"`
	Dependencies    []BeadReference `json:"dependencies,omitempty"`
	Dependents      []BeadReference `json:"dependents,omitempty"`
}

// BeadReference represents a reference to another bead in dependencies/dependents.
type BeadReference struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Status         string `json:"status"`
	DependencyType string `json:"dependency_type"`
}

// EpicCloseResult represents a closed epic from br epic close-eligible output.
type EpicCloseResult struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	DependentCount int    `json:"dependent_count"`
}

// ReadyOptions configures the Ready query.
type ReadyOptions struct {
	Label          string
	UnassignedOnly bool
}

// ListOptions configures the List query.
type ListOptions struct {
	Status string
}

// BeadReader provides read operations for beads.
// Use this interface when you only need to query bead data.
type BeadReader interface {
	// Show retrieves details for a single bead by ID.
	Show(ctx context.Context, id string) (*Bead, error)

	// List retrieves beads, optionally filtered by options.
	List(ctx context.Context, opts *ListOptions) ([]Bead, error)

	// Labels retrieves labels for a bead.
	Labels(ctx context.Context, id string) ([]string, error)
}

// BeadUpdater provides write operations for beads.
// Use this interface when you need to modify bead status.
type BeadUpdater interface {
	// UpdateStatus changes a bead's status and optionally adds notes.
	UpdateStatus(ctx context.Context, id, status, notes string) error

	// Comment adds a comment to a bead.
	Comment(ctx context.Context, id, message string) error

	// Close closes a bead with a reason.
	Close(ctx context.Context, id, reason string) error

	// CloseEligibleEpics closes all epics where all children are completed.
	// Returns the list of closed epics.
	CloseEligibleEpics(ctx context.Context) ([]EpicCloseResult, error)
}

// WorkQueueClient provides operations for work discovery.
// Use this interface for finding and claiming work.
type WorkQueueClient interface {
	// Ready retrieves beads that are ready for work.
	Ready(ctx context.Context, opts *ReadyOptions) ([]Bead, error)

	// List retrieves all beads for hierarchy analysis.
	List(ctx context.Context, opts *ListOptions) ([]Bead, error)
}

// Client combines all bead operations.
// Use this when you need the full br CLI interface.
type Client interface {
	BeadReader
	BeadUpdater
	WorkQueueClient
}
