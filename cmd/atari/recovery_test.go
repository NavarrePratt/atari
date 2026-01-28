package main

import (
	"testing"
	"time"

	"github.com/npratt/atari/internal/events"
)

func TestNormalizeHistoryForRecovery_EmptyHistory(t *testing.T) {
	input := make(map[string]*events.BeadHistory)
	result := normalizeHistoryForRecovery(input)

	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestNormalizeHistoryForRecovery_WorkingToFailed(t *testing.T) {
	// A bead that was being worked on when atari crashed should be normalized
	// to failed status so backoff logic applies correctly on restart.
	input := map[string]*events.BeadHistory{
		"bd-123": {
			ID:          "bd-123",
			Status:      events.HistoryWorking,
			Attempts:    2,
			LastAttempt: time.Now().Add(-5 * time.Minute),
		},
	}

	result := normalizeHistoryForRecovery(input)

	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}

	entry := result["bd-123"]
	if entry.Status != events.HistoryFailed {
		t.Errorf("expected status HistoryFailed, got %v", entry.Status)
	}
	if entry.Attempts != 2 {
		t.Errorf("expected attempts 2 (preserved), got %d", entry.Attempts)
	}
	if entry.LastError != "session interrupted (atari restart)" {
		t.Errorf("expected specific LastError, got %q", entry.LastError)
	}
}

func TestNormalizeHistoryForRecovery_PreservesOtherStatuses(t *testing.T) {
	// Non-working statuses should be preserved without modification.
	input := map[string]*events.BeadHistory{
		"bd-completed": {
			ID:       "bd-completed",
			Status:   events.HistoryCompleted,
			Attempts: 1,
		},
		"bd-failed": {
			ID:        "bd-failed",
			Status:    events.HistoryFailed,
			Attempts:  3,
			LastError: "previous error",
		},
		"bd-abandoned": {
			ID:        "bd-abandoned",
			Status:    events.HistoryAbandoned,
			Attempts:  5,
			LastError: "max failures",
		},
		"bd-pending": {
			ID:       "bd-pending",
			Status:   events.HistoryPending,
			Attempts: 0,
		},
	}

	result := normalizeHistoryForRecovery(input)

	if len(result) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(result))
	}

	// Completed should stay completed
	if result["bd-completed"].Status != events.HistoryCompleted {
		t.Errorf("completed status should be preserved, got %v", result["bd-completed"].Status)
	}

	// Failed should stay failed with original error
	if result["bd-failed"].Status != events.HistoryFailed {
		t.Errorf("failed status should be preserved, got %v", result["bd-failed"].Status)
	}
	if result["bd-failed"].LastError != "previous error" {
		t.Errorf("failed LastError should be preserved, got %q", result["bd-failed"].LastError)
	}

	// Abandoned should stay abandoned
	if result["bd-abandoned"].Status != events.HistoryAbandoned {
		t.Errorf("abandoned status should be preserved, got %v", result["bd-abandoned"].Status)
	}

	// Pending should stay pending
	if result["bd-pending"].Status != events.HistoryPending {
		t.Errorf("pending status should be preserved, got %v", result["bd-pending"].Status)
	}
}

func TestNormalizeHistoryForRecovery_DoesNotMutateInput(t *testing.T) {
	// Verify that the input map is not mutated.
	input := map[string]*events.BeadHistory{
		"bd-123": {
			ID:       "bd-123",
			Status:   events.HistoryWorking,
			Attempts: 2,
		},
	}

	_ = normalizeHistoryForRecovery(input)

	// Original should still be working
	if input["bd-123"].Status != events.HistoryWorking {
		t.Errorf("input should not be mutated, got status %v", input["bd-123"].Status)
	}
}

func TestNormalizeHistoryForRecovery_PreservesSessionID(t *testing.T) {
	// Session ID should be preserved for resume capability.
	input := map[string]*events.BeadHistory{
		"bd-123": {
			ID:            "bd-123",
			Status:        events.HistoryWorking,
			Attempts:      1,
			LastSessionID: "session-abc-123",
		},
	}

	result := normalizeHistoryForRecovery(input)

	if result["bd-123"].LastSessionID != "session-abc-123" {
		t.Errorf("expected LastSessionID to be preserved, got %q", result["bd-123"].LastSessionID)
	}
}
