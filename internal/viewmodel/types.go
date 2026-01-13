// Package viewmodel provides shared types for TUI stats display.
package viewmodel

import "time"

// BlockedBeadInfo represents a bead that is currently in backoff.
type BlockedBeadInfo struct {
	BeadID       string        // ID of the blocked bead
	FailureCount int           // Number of failed attempts
	RetryIn      time.Duration // Time remaining until retry is allowed
	LastError    string        // Error message from last attempt
}

// TUIStats provides a snapshot of controller statistics for TUI display.
type TUIStats struct {
	Completed      int              // Number of successfully completed beads
	Failed         int              // Number of failed beads (still retryable)
	Abandoned      int              // Number of beads that exceeded max failures
	InBackoff      int              // Number of beads currently in backoff period
	CurrentBead    string           // ID of bead being worked on (empty if idle)
	CurrentTurns   int              // Turns completed in current session
	TopBlockedBead *BlockedBeadInfo // Bead with shortest remaining backoff (nil if none)
}
