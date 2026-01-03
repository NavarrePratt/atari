// Package events defines the event type taxonomy and base structures for the
// Atari event system. This is the foundation for all event-driven communication
// between components.
package events

import "time"

// EventType identifies the category and nature of an event.
type EventType string

// MVP event types - only the events needed for Phase 1.
const (
	// Session events
	EventSessionStart   EventType = "session.start"
	EventSessionEnd     EventType = "session.end"
	EventSessionTimeout EventType = "session.timeout"

	// Claude content events
	EventClaudeText       EventType = "claude.text"
	EventClaudeToolUse    EventType = "claude.tool_use"
	EventClaudeToolResult EventType = "claude.tool_result"

	// Drain control events
	EventDrainStart        EventType = "drain.start"
	EventDrainStop         EventType = "drain.stop"
	EventDrainStateChanged EventType = "drain.state_changed"

	// Iteration events
	EventIterationStart EventType = "iteration.start"
	EventIterationEnd   EventType = "iteration.end"

	// Turn events
	EventTurnComplete EventType = "turn.complete"

	// Bead events (internal)
	EventBeadAbandoned EventType = "bead.abandoned"

	// BD activity events (from bd activity --follow)
	EventBeadCreated EventType = "bead.created"
	EventBeadStatus  EventType = "bead.status"
	EventBeadUpdated EventType = "bead.updated"
	EventBeadComment EventType = "bead.comment"
	EventBeadClosed  EventType = "bead.closed"

	// Error events
	EventError      EventType = "error"
	EventParseError EventType = "error.parse"
)

// Source constants identify the origin of events.
const (
	SourceClaude   = "claude"
	SourceBD       = "bd"
	SourceInternal = "atari"
)

// Event is the base interface for all events in the system.
type Event interface {
	Type() EventType
	Timestamp() time.Time
	Source() string
}

// BaseEvent provides the common fields for all events.
type BaseEvent struct {
	EventType EventType `json:"type"`
	Time      time.Time `json:"timestamp"`
	Src       string    `json:"source"`
}

// Type returns the event type.
func (e BaseEvent) Type() EventType {
	return e.EventType
}

// Timestamp returns when the event occurred.
func (e BaseEvent) Timestamp() time.Time {
	return e.Time
}

// Source returns the origin of the event.
func (e BaseEvent) Source() string {
	return e.Src
}

// SessionStartEvent is emitted when a Claude session begins.
type SessionStartEvent struct {
	BaseEvent
	BeadID string `json:"bead_id"`
	Title  string `json:"title"`
}

// SessionEndEvent is emitted when a Claude session completes.
type SessionEndEvent struct {
	BaseEvent
	SessionID    string  `json:"session_id"`
	NumTurns     int     `json:"num_turns"`
	DurationMs   int64   `json:"duration_ms"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Result       string  `json:"result,omitempty"`
}

// SessionTimeoutEvent is emitted when a session is killed due to inactivity.
type SessionTimeoutEvent struct {
	BaseEvent
	Duration time.Duration `json:"duration"`
}

// ClaudeTextEvent is emitted for assistant text output.
type ClaudeTextEvent struct {
	BaseEvent
	Text string `json:"text"`
}

// ClaudeToolUseEvent is emitted when Claude invokes a tool.
type ClaudeToolUseEvent struct {
	BaseEvent
	ToolID   string         `json:"tool_id"`
	ToolName string         `json:"tool_name"`
	Input    map[string]any `json:"input"`
}

// ClaudeToolResultEvent is emitted after tool execution.
type ClaudeToolResultEvent struct {
	BaseEvent
	ToolID  string `json:"tool_id"`
	Content string `json:"content"`
	IsError bool   `json:"is_error,omitempty"`
}

// DrainStartEvent is emitted when atari starts.
type DrainStartEvent struct {
	BaseEvent
	WorkDir string `json:"work_dir"`
}

// DrainStopEvent is emitted when atari stops.
type DrainStopEvent struct {
	BaseEvent
	Reason string `json:"reason,omitempty"`
}

// DrainStateChangedEvent is emitted when the controller state changes.
// This enables TUI and other observers to track state transitions.
type DrainStateChangedEvent struct {
	BaseEvent
	From string `json:"from"`
	To   string `json:"to"`
}

// IterationStartEvent is emitted when beginning work on a bead.
type IterationStartEvent struct {
	BaseEvent
	BeadID   string `json:"bead_id"`
	Title    string `json:"title"`
	Priority int    `json:"priority"`
	Attempt  int    `json:"attempt"`
}

// IterationEndEvent is emitted when bead work completes.
type IterationEndEvent struct {
	BaseEvent
	BeadID       string  `json:"bead_id"`
	Success      bool    `json:"success"`
	NumTurns     int     `json:"num_turns"`
	DurationMs   int64   `json:"duration_ms"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Error        string  `json:"error,omitempty"`
}

// TurnCompleteEvent is emitted when a turn boundary is reached during a session.
// A turn boundary occurs when all pending tool uses have received their results.
type TurnCompleteEvent struct {
	BaseEvent
	TurnNumber    int   `json:"turn_number"`
	ToolCount     int   `json:"tool_count"`
	ToolElapsedMs int64 `json:"tool_elapsed_ms"`
}

// BeadAbandonedEvent is emitted when a bead hits the max_failures limit.
type BeadAbandonedEvent struct {
	BaseEvent
	BeadID      string `json:"bead_id"`
	Attempts    int    `json:"attempts"`
	MaxFailures int    `json:"max_failures"`
	LastError   string `json:"last_error"`
}

// BeadCreatedEvent is emitted when a new bead is created via bd activity.
type BeadCreatedEvent struct {
	BaseEvent
	BeadID string `json:"bead_id"`
	Title  string `json:"title"`
	Actor  string `json:"actor"`
}

// BeadStatusEvent is emitted when a bead's status changes via bd activity.
type BeadStatusEvent struct {
	BaseEvent
	BeadID    string `json:"bead_id"`
	OldStatus string `json:"old_status"`
	NewStatus string `json:"new_status"`
	Actor     string `json:"actor"`
}

// BeadUpdatedEvent is emitted when a bead is updated via bd activity.
type BeadUpdatedEvent struct {
	BaseEvent
	BeadID string `json:"bead_id"`
	Actor  string `json:"actor"`
}

// BeadCommentEvent is emitted when a comment is added to a bead via bd activity.
type BeadCommentEvent struct {
	BaseEvent
	BeadID string `json:"bead_id"`
	Actor  string `json:"actor"`
}

// BeadClosedEvent is emitted when a bead is closed via bd activity.
type BeadClosedEvent struct {
	BaseEvent
	BeadID string `json:"bead_id"`
	Actor  string `json:"actor"`
}

// Severity constants for error events.
const (
	SeverityWarning = "warning"
	SeverityError   = "error"
	SeverityFatal   = "fatal"
)

// ErrorEvent is emitted for any error condition.
type ErrorEvent struct {
	BaseEvent
	Message  string            `json:"message"`
	Severity string            `json:"severity"`
	BeadID   string            `json:"bead_id,omitempty"`
	Context  map[string]string `json:"context,omitempty"`
}

// ParseErrorEvent is emitted when stream-json parsing fails on a line.
type ParseErrorEvent struct {
	BaseEvent
	Line  string `json:"line"`
	Error string `json:"error"`
}

// NewEvent creates a BaseEvent with the given type and source.
func NewEvent(eventType EventType, source string) BaseEvent {
	return BaseEvent{
		EventType: eventType,
		Time:      time.Now(),
		Src:       source,
	}
}

// NewClaudeEvent creates a BaseEvent with Claude as the source.
func NewClaudeEvent(eventType EventType) BaseEvent {
	return NewEvent(eventType, SourceClaude)
}

// NewBDEvent creates a BaseEvent with BD as the source.
func NewBDEvent(eventType EventType) BaseEvent {
	return NewEvent(eventType, SourceBD)
}

// NewInternalEvent creates a BaseEvent with Atari as the source.
func NewInternalEvent(eventType EventType) BaseEvent {
	return NewEvent(eventType, SourceInternal)
}

// HistoryStatus represents the state of a bead in the work history.
type HistoryStatus string

// HistoryStatus constants.
const (
	HistoryPending   HistoryStatus = "pending"
	HistoryWorking   HistoryStatus = "working"
	HistoryCompleted HistoryStatus = "completed"
	HistoryFailed    HistoryStatus = "failed"
	HistoryAbandoned HistoryStatus = "abandoned"
)

// BeadHistory tracks the processing history of a bead.
// This type is shared between workqueue and state sink.
type BeadHistory struct {
	ID          string        `json:"id"`
	Status      HistoryStatus `json:"status"`
	Attempts    int           `json:"attempts"`
	LastAttempt time.Time     `json:"last_attempt"`
	LastError   string        `json:"last_error,omitempty"`
}
