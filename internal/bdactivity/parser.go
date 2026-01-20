// Package bdactivity provides parsing for bd JSONL files.
package bdactivity

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/npratt/atari/internal/events"
)

// bdActivity represents a single bd activity JSON event (legacy format).
// Kept for backward compatibility with any code that may use it.
type bdActivity struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	IssueID   string `json:"issue_id"`
	Symbol    string `json:"symbol"`
	Message   string `json:"message"`
	OldStatus string `json:"old_status,omitempty"`
	NewStatus string `json:"new_status,omitempty"`
	Actor     string `json:"actor,omitempty"`
}

// ParseLine parses a single line of bd activity JSON output (legacy format).
// Kept for backward compatibility.
// Returns nil, nil for empty lines or unknown event types (silently skipped).
// Returns nil, error for invalid JSON.
func ParseLine(line []byte) (events.Event, error) {
	if len(line) == 0 || len(strings.TrimSpace(string(line))) == 0 {
		return nil, nil
	}

	var activity bdActivity
	if err := json.Unmarshal(line, &activity); err != nil {
		return nil, err
	}

	return mapToEvent(&activity), nil
}

// mapToEvent converts a bdActivity to the appropriate events.Event type.
// Returns nil for unknown mutation types.
func mapToEvent(a *bdActivity) events.Event {
	timestamp := parseTimestamp(a.Timestamp)

	base := events.BaseEvent{
		Time: timestamp,
		Src:  events.SourceBD,
	}

	switch a.Type {
	case "create":
		base.EventType = events.EventBeadCreated
		return &events.BeadCreatedEvent{
			BaseEvent: base,
			BeadID:    a.IssueID,
			Title:     extractTitle(a.Message, a.IssueID),
			Actor:     a.Actor,
		}

	case "status":
		eventType := mapStatusEventType(a.NewStatus)
		base.EventType = eventType

		if eventType == events.EventBeadClosed {
			return &events.BeadClosedEvent{
				BaseEvent: base,
				BeadID:    a.IssueID,
				Actor:     a.Actor,
			}
		}
		return &events.BeadStatusEvent{
			BaseEvent: base,
			BeadID:    a.IssueID,
			OldStatus: a.OldStatus,
			NewStatus: a.NewStatus,
			Actor:     a.Actor,
		}

	case "update":
		base.EventType = events.EventBeadUpdated
		return &events.BeadUpdatedEvent{
			BaseEvent: base,
			BeadID:    a.IssueID,
			Actor:     a.Actor,
		}

	case "comment":
		base.EventType = events.EventBeadComment
		return &events.BeadCommentEvent{
			BaseEvent: base,
			BeadID:    a.IssueID,
			Actor:     a.Actor,
		}

	default:
		return nil
	}
}

// mapStatusEventType determines whether a status change is a close event.
func mapStatusEventType(newStatus string) events.EventType {
	switch newStatus {
	case "closed", "completed":
		return events.EventBeadClosed
	default:
		return events.EventBeadStatus
	}
}

// parseTimestamp parses an RFC3339 timestamp string.
// Returns time.Now() if the timestamp is empty or invalid.
func parseTimestamp(ts string) time.Time {
	if ts == "" {
		return time.Now()
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			return time.Now()
		}
	}
	return t
}

// extractTitle extracts the title from a bd activity message.
// Messages are formatted as "{issue_id} created · {title}" for create events.
func extractTitle(message, issueID string) string {
	prefix := issueID + " created · "
	if strings.HasPrefix(message, prefix) {
		return strings.TrimPrefix(message, prefix)
	}
	return ""
}
