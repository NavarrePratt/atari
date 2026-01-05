package observer

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/events"
)

const (
	// defaultRecentEvents is the default number of recent events to include.
	defaultRecentEvents = 20

	// maxSessionHistory is the maximum number of completed beads to show.
	maxSessionHistory = 5

	// Text truncation limits.
	truncateText        = 200
	truncateToolSummary = 40
	truncateToolResult  = 100
	truncateTitle       = 30
	truncateToolID      = 15
)

// DrainState holds the current state of the drain for context building.
type DrainState struct {
	Status       string
	Uptime       time.Duration
	TotalCost    float64
	CurrentBead  *CurrentBeadInfo
	CurrentTurns int // turns completed in current session
}

// CurrentBeadInfo holds information about the currently active bead.
type CurrentBeadInfo struct {
	ID        string
	Title     string
	StartedAt time.Time
}

// SessionHistory holds information about a completed bead session.
type SessionHistory struct {
	BeadID  string
	Title   string
	Outcome string
	Cost    float64
	Turns   int
}

// ContextBuilder assembles structured context from log events for observer queries.
type ContextBuilder struct {
	logReader *LogReader
	config    *config.ObserverConfig
}

// NewContextBuilder creates a new ContextBuilder with the given log reader and config.
func NewContextBuilder(logReader *LogReader, cfg *config.ObserverConfig) *ContextBuilder {
	return &ContextBuilder{
		logReader: logReader,
		config:    cfg,
	}
}

// Build assembles the full context string for an observer query.
// The conversation parameter contains prior Q&A exchanges for session continuity.
func (b *ContextBuilder) Build(state DrainState, conversation []Exchange) (string, error) {
	var sb strings.Builder

	// System prompt
	sb.WriteString(b.buildSystemPrompt())
	sb.WriteString("\n")

	// Drain status section
	sb.WriteString(b.buildDrainStatusSection(state))
	sb.WriteString("\n")

	// Bead session history section (completed beads)
	history, err := b.loadSessionHistory()
	if err == nil && len(history) > 0 {
		sb.WriteString(b.buildSessionHistorySection(history))
		sb.WriteString("\n")
	}

	// Current bead section
	if state.CurrentBead != nil {
		section, err := b.buildCurrentBeadSection(state.CurrentBead)
		if err == nil && section != "" {
			sb.WriteString(section)
			sb.WriteString("\n")
		}
	}

	// Tips section
	sb.WriteString(b.buildTipsSection())

	// Conversation history section (prior Q&A in this observer session)
	if len(conversation) > 0 {
		sb.WriteString("\n")
		sb.WriteString(b.buildConversationHistorySection(conversation))
	}

	return sb.String(), nil
}

// buildSystemPrompt returns the system prompt for the observer.
func (b *ContextBuilder) buildSystemPrompt() string {
	return `You are an observer assistant helping the user understand what's happening in an automated bead processing session (Atari drain).

Your role:
- Answer questions about current activity
- Help identify issues or unexpected behavior
- Suggest when manual intervention might be needed
- Explain Claude's decision-making based on visible events

You have access to tools. If you need more details about an event, you can use grep to look it up in the log file.
`
}

// buildDrainStatusSection builds the drain status section of the context.
func (b *ContextBuilder) buildDrainStatusSection(state DrainState) string {
	var sb strings.Builder
	sb.WriteString("## Drain Status\n")

	statusLine := fmt.Sprintf("State: %s | Uptime: %s | Total cost: $%.2f",
		state.Status,
		formatDuration(state.Uptime),
		state.TotalCost)

	if state.CurrentBead != nil && state.CurrentTurns > 0 {
		statusLine += fmt.Sprintf(" | Turn: %d", state.CurrentTurns)
	}

	sb.WriteString(statusLine + "\n")
	return sb.String()
}

// buildSessionHistorySection builds the session history table.
func (b *ContextBuilder) buildSessionHistorySection(history []SessionHistory) string {
	var sb strings.Builder
	sb.WriteString("## Session History\n")
	sb.WriteString("| Bead | Title | Outcome | Cost | Turns |\n")
	sb.WriteString("|------|-------|---------|------|-------|\n")
	for _, h := range history {
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | $%.2f | %d |\n",
			h.BeadID, truncate(h.Title, truncateTitle), h.Outcome, h.Cost, h.Turns))
	}
	return sb.String()
}

// buildCurrentBeadSection builds the current bead section with recent events.
func (b *ContextBuilder) buildCurrentBeadSection(bead *CurrentBeadInfo) (string, error) {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## Current Bead: %s\n", bead.ID))
	sb.WriteString(fmt.Sprintf("Title: %s\n", bead.Title))
	sb.WriteString(fmt.Sprintf("Started: %s ago\n\n", formatDuration(time.Since(bead.StartedAt))))

	// Load recent events for current bead
	limit := b.recentEventsLimit()
	evts, err := b.logReader.ReadByBeadID(bead.ID)
	if err != nil && err != ErrFileNotFound && err != ErrEmptyFile {
		return "", err
	}

	if len(evts) > 0 {
		// Take only the most recent events up to limit
		if len(evts) > limit {
			evts = evts[len(evts)-limit:]
		}

		sb.WriteString("### Recent Activity\n")
		for _, e := range evts {
			sb.WriteString(FormatEvent(e))
			sb.WriteString("\n")
		}
	}

	return sb.String(), nil
}

// buildTipsSection builds the tips section for event lookup.
func (b *ContextBuilder) buildTipsSection() string {
	return `## Retrieving Full Event Details
Events are stored in ` + "`" + `.atari/atari.log` + "`" + ` as JSON lines.
To see full event details:
  grep '<tool_id>' .atari/atari.log | jq .
To see recent events:
  tail -50 .atari/atari.log | jq -s .
To get bead details:
  bd show <bead-id>
`
}

// buildConversationHistorySection formats prior Q&A exchanges for context.
func (b *ContextBuilder) buildConversationHistorySection(conversation []Exchange) string {
	var sb strings.Builder
	sb.WriteString("## Conversation History\n")
	sb.WriteString("Previous exchanges in this observer session:\n\n")

	for i, ex := range conversation {
		sb.WriteString(fmt.Sprintf("### Exchange %d\n", i+1))
		sb.WriteString(fmt.Sprintf("**User:** %s\n\n", ex.Question))
		sb.WriteString(fmt.Sprintf("**Assistant:** %s\n\n", ex.Answer))
	}

	return sb.String()
}

// loadSessionHistory reads completed bead sessions from the log.
func (b *ContextBuilder) loadSessionHistory() ([]SessionHistory, error) {
	allEvents, err := b.logReader.readAllEvents()
	if err != nil {
		if err == ErrFileNotFound || err == ErrEmptyFile {
			return nil, nil
		}
		return nil, err
	}

	// Track iteration start/end pairs
	beadMap := make(map[string]*SessionHistory)
	var history []SessionHistory

	for _, ev := range allEvents {
		switch e := ev.(type) {
		case *events.IterationStartEvent:
			beadMap[e.BeadID] = &SessionHistory{
				BeadID: e.BeadID,
				Title:  e.Title,
			}
		case *events.IterationEndEvent:
			if h, ok := beadMap[e.BeadID]; ok {
				h.Turns = e.NumTurns
				h.Cost = e.TotalCostUSD
				if e.Success {
					h.Outcome = "completed"
				} else {
					h.Outcome = "failed"
				}
				history = append(history, *h)
				delete(beadMap, e.BeadID)
			}
		}
	}

	// Return only the last N sessions
	if len(history) > maxSessionHistory {
		history = history[len(history)-maxSessionHistory:]
	}

	return history, nil
}

// recentEventsLimit returns the configured limit for recent events.
func (b *ContextBuilder) recentEventsLimit() int {
	if b.config != nil && b.config.RecentEvents > 0 {
		return b.config.RecentEvents
	}
	return defaultRecentEvents
}

// FormatEvent formats a single event for display in the context.
func FormatEvent(e events.Event) string {
	ts := e.Timestamp().Format("15:04:05")

	switch ev := e.(type) {
	case *events.ClaudeTextEvent:
		return fmt.Sprintf("[%s] Claude: %s", ts, truncate(ev.Text, truncateText))

	case *events.ClaudeToolUseEvent:
		summary := formatToolSummary(ev.ToolName, ev.Input)
		return fmt.Sprintf("[%s] Tool: %s %s (%s)", ts, ev.ToolName, summary, shortID(ev.ToolID))

	case *events.ClaudeToolResultEvent:
		content := truncate(ev.Content, truncateToolResult)
		if ev.IsError {
			return fmt.Sprintf("[%s] Result ERROR: %s (%s)", ts, content, shortID(ev.ToolID))
		}
		return fmt.Sprintf("[%s] Result: %s (%s)", ts, content, shortID(ev.ToolID))

	case *events.SessionStartEvent:
		return fmt.Sprintf("[%s] Session started for %s", ts, ev.BeadID)

	case *events.SessionEndEvent:
		return fmt.Sprintf("[%s] Session ended (turns: %d, cost: $%.2f)", ts, ev.NumTurns, ev.TotalCostUSD)

	case *events.SessionTimeoutEvent:
		return fmt.Sprintf("[%s] Session timed out after %s", ts, ev.Duration)

	case *events.TurnCompleteEvent:
		return fmt.Sprintf("[%s] Turn %d complete (%d tools, %dms)", ts, ev.TurnNumber, ev.ToolCount, ev.ToolElapsedMs)

	case *events.IterationStartEvent:
		return fmt.Sprintf("[%s] Started bead %s: %s", ts, ev.BeadID, truncate(ev.Title, truncateToolSummary))

	case *events.IterationEndEvent:
		outcome := "completed"
		if !ev.Success {
			outcome = "failed"
		}
		return fmt.Sprintf("[%s] Bead %s %s ($%.2f)", ts, ev.BeadID, outcome, ev.TotalCostUSD)

	case *events.BeadAbandonedEvent:
		return fmt.Sprintf("[%s] Bead %s abandoned after %d attempts", ts, ev.BeadID, ev.Attempts)

	case *events.BeadCreatedEvent:
		return fmt.Sprintf("[%s] Bead created: %s - %s", ts, ev.BeadID, truncate(ev.Title, truncateToolSummary))

	case *events.BeadStatusEvent:
		return fmt.Sprintf("[%s] Bead %s: %s -> %s", ts, ev.BeadID, ev.OldStatus, ev.NewStatus)

	case *events.BeadUpdatedEvent:
		return fmt.Sprintf("[%s] Bead %s updated", ts, ev.BeadID)

	case *events.BeadCommentEvent:
		return fmt.Sprintf("[%s] Comment on bead %s", ts, ev.BeadID)

	case *events.BeadClosedEvent:
		return fmt.Sprintf("[%s] Bead %s closed", ts, ev.BeadID)

	case *events.DrainStartEvent:
		return fmt.Sprintf("[%s] Drain started in %s", ts, ev.WorkDir)

	case *events.DrainStopEvent:
		return fmt.Sprintf("[%s] Drain stopped: %s", ts, ev.Reason)

	case *events.DrainStateChangedEvent:
		return fmt.Sprintf("[%s] Drain state: %s -> %s", ts, ev.From, ev.To)

	case *events.ErrorEvent:
		return fmt.Sprintf("[%s] ERROR: %s", ts, ev.Message)

	case *events.ParseErrorEvent:
		return fmt.Sprintf("[%s] Parse error: %s", ts, ev.Error)

	default:
		return fmt.Sprintf("[%s] %s", ts, e.Type())
	}
}

// formatToolSummary extracts a summary from tool input based on tool type.
func formatToolSummary(toolName string, input map[string]any) string {
	switch toolName {
	case "Bash":
		if desc, ok := input["description"].(string); ok {
			return fmt.Sprintf("%q", truncate(desc, truncateToolSummary))
		}
		if cmd, ok := input["command"].(string); ok {
			return fmt.Sprintf("%q", truncate(cmd, truncateToolSummary))
		}
	case "Read", "Write", "Edit":
		if path, ok := input["file_path"].(string); ok {
			return filepath.Base(path)
		}
	case "Glob", "Grep":
		if pattern, ok := input["pattern"].(string); ok {
			return fmt.Sprintf("%q", pattern)
		}
	case "Task":
		if desc, ok := input["description"].(string); ok {
			return fmt.Sprintf("%q", truncate(desc, truncateToolSummary))
		}
	}
	return ""
}

// shortID truncates a tool ID for display.
func shortID(toolID string) string {
	if len(toolID) > truncateToolID {
		return toolID[:truncateToolID] + "..."
	}
	return toolID
}

// truncate truncates a string to the given length with ellipsis.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if minutes == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh %dm", hours, minutes)
}
