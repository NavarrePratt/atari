package events

import (
	"encoding/json"
	"log/slog"
)

// eventEnvelope is used for initial JSON parsing to determine event type.
type eventEnvelope struct {
	Type EventType `json:"type"`
}

// ParseEvent parses a JSON line into a typed Event.
// Returns nil with no error for unknown event types (for forward compatibility).
func ParseEvent(line []byte) (Event, error) {
	// First pass: determine event type
	var envelope eventEnvelope
	if err := json.Unmarshal(line, &envelope); err != nil {
		return nil, err
	}

	// Second pass: unmarshal into the correct type
	var ev Event
	var err error

	switch envelope.Type {
	case EventSessionStart:
		var e SessionStartEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case EventSessionEnd:
		var e SessionEndEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case EventSessionTimeout:
		var e SessionTimeoutEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case EventClaudeText:
		var e ClaudeTextEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case EventClaudeToolUse:
		var e ClaudeToolUseEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case EventClaudeToolResult:
		var e ClaudeToolResultEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case EventDrainStart:
		var e DrainStartEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case EventDrainStop:
		var e DrainStopEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case EventDrainStateChanged:
		var e DrainStateChangedEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case EventIterationStart:
		var e IterationStartEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case EventIterationEnd:
		var e IterationEndEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case EventTurnComplete:
		var e TurnCompleteEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case EventBeadAbandoned:
		var e BeadAbandonedEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case EventEpicClosed:
		var e EpicClosedEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case EventBeadCreated:
		var e BeadCreatedEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case EventBeadStatus:
		var e BeadStatusEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case EventBeadUpdated:
		var e BeadUpdatedEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case EventBeadComment:
		var e BeadCommentEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case EventBeadClosed:
		var e BeadClosedEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case EventBeadChanged:
		var e BeadChangedEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case EventError:
		var e ErrorEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	case EventParseError:
		var e ParseErrorEvent
		err = json.Unmarshal(line, &e)
		ev = &e

	default:
		// Unknown event type - skip it for forward compatibility
		slog.Debug("unknown event type", "type", envelope.Type)
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return ev, nil
}

// GetBeadID extracts the bead ID from an event, if present.
// Returns empty string for events without an associated bead.
func GetBeadID(ev Event) string {
	switch e := ev.(type) {
	case *SessionStartEvent:
		return e.BeadID
	case *IterationStartEvent:
		return e.BeadID
	case *IterationEndEvent:
		return e.BeadID
	case *BeadAbandonedEvent:
		return e.BeadID
	case *BeadCreatedEvent:
		return e.BeadID
	case *BeadStatusEvent:
		return e.BeadID
	case *BeadUpdatedEvent:
		return e.BeadID
	case *BeadCommentEvent:
		return e.BeadID
	case *BeadClosedEvent:
		return e.BeadID
	case *BeadChangedEvent:
		return e.BeadID
	case *EpicClosedEvent:
		return e.EpicID
	case *ErrorEvent:
		return e.BeadID
	default:
		return ""
	}
}
