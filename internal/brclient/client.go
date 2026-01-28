package brclient

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/npratt/atari/internal/exec"
)

// DefaultTimeout is the default timeout for br CLI commands.
const DefaultTimeout = 30 * time.Second

// CLIClient implements Client by shelling out to the br CLI.
type CLIClient struct {
	runner  exec.CommandRunner
	timeout time.Duration
}

// NewCLIClient creates a CLIClient with the given command runner.
func NewCLIClient(runner exec.CommandRunner) *CLIClient {
	return &CLIClient{
		runner:  runner,
		timeout: DefaultTimeout,
	}
}

// WithTimeout returns a new CLIClient with the specified timeout.
func (c *CLIClient) WithTimeout(d time.Duration) *CLIClient {
	return &CLIClient{
		runner:  c.runner,
		timeout: d,
	}
}

// Show retrieves details for a single bead by ID.
func (c *CLIClient) Show(ctx context.Context, id string) (*Bead, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	output, err := c.runner.Run(ctx, "br", "show", id, "--json")
	if err != nil {
		return nil, fmt.Errorf("br show %s failed: %w", id, err)
	}

	if len(output) == 0 {
		return nil, fmt.Errorf("bead not found: %s", id)
	}

	var beads []Bead
	if err := json.Unmarshal(output, &beads); err != nil {
		return nil, fmt.Errorf("parse br show output: %w", err)
	}

	if len(beads) == 0 {
		return nil, fmt.Errorf("bead not found: %s", id)
	}

	return &beads[0], nil
}

// List retrieves beads, optionally filtered by options.
func (c *CLIClient) List(ctx context.Context, opts *ListOptions) ([]Bead, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	args := []string{"list", "--json"}
	if opts != nil && opts.Status != "" {
		args = append(args, "--status", opts.Status)
	}

	output, err := c.runner.Run(ctx, "br", args...)
	if err != nil {
		return nil, fmt.Errorf("br list failed: %w", err)
	}

	if len(output) == 0 {
		return nil, nil
	}

	var beads []Bead
	if err := json.Unmarshal(output, &beads); err != nil {
		return nil, fmt.Errorf("parse br list output: %w", err)
	}

	return beads, nil
}

// Labels retrieves labels for a bead.
func (c *CLIClient) Labels(ctx context.Context, id string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	output, err := c.runner.Run(ctx, "br", "label", "list", id, "--json")
	if err != nil {
		return nil, fmt.Errorf("br label list %s failed: %w", id, err)
	}

	if len(output) == 0 {
		return nil, nil
	}

	var labels []string
	if err := json.Unmarshal(output, &labels); err != nil {
		return nil, fmt.Errorf("parse br label list output: %w", err)
	}

	return labels, nil
}

// Ready retrieves beads that are ready for work.
func (c *CLIClient) Ready(ctx context.Context, opts *ReadyOptions) ([]Bead, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	args := []string{"ready", "--json"}
	if opts != nil {
		if opts.Label != "" {
			args = append(args, "--label", opts.Label)
		}
		if opts.UnassignedOnly {
			args = append(args, "--unassigned")
		}
	}

	output, err := c.runner.Run(ctx, "br", args...)
	if err != nil {
		return nil, fmt.Errorf("br ready failed: %w", err)
	}

	if len(output) == 0 {
		return nil, nil
	}

	var beads []Bead
	if err := json.Unmarshal(output, &beads); err != nil {
		return nil, fmt.Errorf("parse br ready output: %w", err)
	}

	return beads, nil
}

// UpdateStatus changes a bead's status and optionally adds notes.
func (c *CLIClient) UpdateStatus(ctx context.Context, id, status, notes string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	args := []string{"update", id, "--status", status}
	if notes != "" {
		args = append(args, "--notes", notes)
	}

	_, err := c.runner.Run(ctx, "br", args...)
	if err != nil {
		return fmt.Errorf("br update %s failed: %w", id, err)
	}

	return nil
}

// Close closes a bead with a reason.
func (c *CLIClient) Close(ctx context.Context, id, reason string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	_, err := c.runner.Run(ctx, "br", "close", id, "--reason", reason)
	if err != nil {
		return fmt.Errorf("br close %s failed: %w", id, err)
	}

	return nil
}

// CloseEligibleEpics closes all epics where all children are completed.
func (c *CLIClient) CloseEligibleEpics(ctx context.Context) ([]EpicCloseResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	output, err := c.runner.Run(ctx, "br", "epic", "close-eligible", "--json")
	if err != nil {
		return nil, fmt.Errorf("br epic close-eligible failed: %w", err)
	}

	if len(output) == 0 {
		return nil, nil
	}

	var results []EpicCloseResult
	if err := json.Unmarshal(output, &results); err != nil {
		return nil, fmt.Errorf("parse br epic close-eligible output: %w", err)
	}

	return results, nil
}

// Verify CLIClient implements Client interface.
var _ Client = (*CLIClient)(nil)
