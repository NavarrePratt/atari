package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"

	"github.com/npratt/atari/internal/events"
)

// ScannerBufferSize is the buffer size for parsing stream-json (1MB).
const ScannerBufferSize = 1024 * 1024

// StreamEvent represents a raw Claude stream-json event.
// The structure varies by event type, so we use json.RawMessage for nested content.
type StreamEvent struct {
	Type    string `json:"type"`
	Subtype string `json:"subtype,omitempty"`

	// For assistant events
	Message *StreamMessage `json:"message,omitempty"`

	// For result events
	SessionID    string  `json:"session_id,omitempty"`
	NumTurns     int     `json:"num_turns,omitempty"`
	DurationMs   int64   `json:"duration_ms,omitempty"`
	TotalCostUSD float64 `json:"total_cost_usd,omitempty"`
	Result       string  `json:"result,omitempty"`

	// For system init events
	Model string   `json:"model,omitempty"`
	Tools []string `json:"tools,omitempty"`
}

// StreamMessage is the message field in assistant/user events.
type StreamMessage struct {
	Content []json.RawMessage `json:"content"`
}

// ContentBlock represents a content item within a message.
type ContentBlock struct {
	Type string `json:"type"`

	// Text content
	Text string `json:"text,omitempty"`

	// Thinking content
	Thinking string `json:"thinking,omitempty"`

	// Tool use content
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`

	// Tool result content
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

// Parser reads Claude stream-json output and emits typed events.
type Parser struct {
	scanner *bufio.Scanner
	router  *events.Router
	manager *Manager
}

// NewParser creates a Parser for the given reader.
// The router receives parsed events. The manager is updated on each event.
func NewParser(r io.Reader, router *events.Router, manager *Manager) *Parser {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, ScannerBufferSize)
	scanner.Buffer(buf, ScannerBufferSize)

	return &Parser{
		scanner: scanner,
		router:  router,
		manager: manager,
	}
}

// Parse reads the stream and emits events until EOF or error.
// Parse errors are emitted as ParseErrorEvent but do not stop parsing.
func (p *Parser) Parse() error {
	for p.scanner.Scan() {
		line := p.scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event StreamEvent
		if err := json.Unmarshal(line, &event); err != nil {
			p.emitParseError(string(line), err)
			continue
		}

		// Update activity timestamp on each valid event
		if p.manager != nil {
			p.manager.UpdateActivity()
		}

		// Convert and emit internal events
		p.convertAndEmit(&event)
	}

	if err := p.scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}
	return nil
}

// emitParseError emits a ParseErrorEvent for a failed line.
func (p *Parser) emitParseError(line string, err error) {
	if p.router == nil {
		return
	}
	p.router.Emit(&events.ParseErrorEvent{
		BaseEvent: events.NewClaudeEvent(events.EventParseError),
		Line:      line,
		Error:     err.Error(),
	})
}

// convertAndEmit converts a StreamEvent to internal event(s) and emits them.
func (p *Parser) convertAndEmit(e *StreamEvent) {
	if p.router == nil {
		return
	}

	switch e.Type {
	case "system":
		p.handleSystemEvent(e)
	case "assistant":
		p.handleAssistantEvent(e)
	case "user":
		p.handleUserEvent(e)
	case "result":
		p.handleResultEvent(e)
	default:
		// Unknown event types are silently ignored (logged at debug level in production)
	}
}

// handleSystemEvent processes system events (init, compact_boundary, etc).
func (p *Parser) handleSystemEvent(e *StreamEvent) {
	switch e.Subtype {
	case "init":
		// System init events contain model and tools info
		// Currently we don't have a dedicated event type for this
		// Could emit a SessionStartEvent but that's for bead context
	case "compact_boundary":
		// Context compaction event - could be useful for logging
	default:
		// Other system events (hooks, etc) are ignored
	}
}

// handleAssistantEvent processes assistant message events.
func (p *Parser) handleAssistantEvent(e *StreamEvent) {
	if e.Message == nil {
		return
	}

	for _, rawContent := range e.Message.Content {
		var block ContentBlock
		if err := json.Unmarshal(rawContent, &block); err != nil {
			p.emitParseError(string(rawContent), err)
			continue
		}

		switch block.Type {
		case "text":
			p.router.Emit(&events.ClaudeTextEvent{
				BaseEvent: events.NewClaudeEvent(events.EventClaudeText),
				Text:      block.Text,
			})
		case "thinking":
			// Thinking is emitted as text for now
			p.router.Emit(&events.ClaudeTextEvent{
				BaseEvent: events.NewClaudeEvent(events.EventClaudeText),
				Text:      block.Thinking,
			})
		case "tool_use":
			p.router.Emit(&events.ClaudeToolUseEvent{
				BaseEvent: events.NewClaudeEvent(events.EventClaudeToolUse),
				ToolID:    block.ID,
				ToolName:  block.Name,
				Input:     block.Input,
			})
		}
	}
}

// handleUserEvent processes user message events (tool results).
func (p *Parser) handleUserEvent(e *StreamEvent) {
	if e.Message == nil {
		return
	}

	for _, rawContent := range e.Message.Content {
		var block ContentBlock
		if err := json.Unmarshal(rawContent, &block); err != nil {
			p.emitParseError(string(rawContent), err)
			continue
		}

		if block.Type == "tool_result" {
			p.router.Emit(&events.ClaudeToolResultEvent{
				BaseEvent: events.NewClaudeEvent(events.EventClaudeToolResult),
				ToolID:    block.ToolUseID,
				Content:   block.Content,
				IsError:   block.IsError,
			})
		}
	}
}

// handleResultEvent processes session result events.
func (p *Parser) handleResultEvent(e *StreamEvent) {
	p.router.Emit(&events.SessionEndEvent{
		BaseEvent:    events.NewClaudeEvent(events.EventSessionEnd),
		SessionID:    e.SessionID,
		NumTurns:     e.NumTurns,
		DurationMs:   e.DurationMs,
		TotalCostUSD: e.TotalCostUSD,
		Result:       e.Result,
	})
}
