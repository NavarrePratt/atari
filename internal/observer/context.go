package observer

import (
	"fmt"
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

	// truncateTitle is used for title display in session history.
	truncateTitle = 30
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
			h.BeadID, events.Truncate(h.Title, truncateTitle), h.Outcome, h.Cost, h.Turns))
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
  br show <bead-id>
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
// Delegates to events.FormatWithTimestamp for consistent formatting.
func FormatEvent(e events.Event) string {
	return events.FormatWithTimestamp(e)
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
