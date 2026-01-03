package tui

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/npratt/atari/internal/events"
)

const (
	maxTextLength    = 200
	maxToolInput     = 100
	truncateIndicator = "..."
)

// Format converts an event to a human-readable string for display.
// Returns empty string for nil or unknown event types.
func Format(event events.Event) string {
	if event == nil {
		return ""
	}

	switch e := event.(type) {
	case *events.ClaudeTextEvent:
		return formatClaudeText(e)
	case *events.ClaudeToolUseEvent:
		return formatClaudeToolUse(e)
	case *events.ClaudeToolResultEvent:
		return formatClaudeToolResult(e)
	case *events.SessionStartEvent:
		return formatSessionStart(e)
	case *events.SessionEndEvent:
		return formatSessionEnd(e)
	case *events.SessionTimeoutEvent:
		return formatSessionTimeout(e)
	case *events.DrainStartEvent:
		return formatDrainStart(e)
	case *events.DrainStopEvent:
		return formatDrainStop(e)
	case *events.DrainStateChangedEvent:
		return formatDrainStateChanged(e)
	case *events.IterationStartEvent:
		return formatIterationStart(e)
	case *events.IterationEndEvent:
		return formatIterationEnd(e)
	case *events.TurnCompleteEvent:
		return formatTurnComplete(e)
	case *events.BeadAbandonedEvent:
		return formatBeadAbandoned(e)
	case *events.BeadCreatedEvent:
		return formatBeadCreated(e)
	case *events.BeadStatusEvent:
		return formatBeadStatus(e)
	case *events.BeadUpdatedEvent:
		return formatBeadUpdated(e)
	case *events.BeadCommentEvent:
		return formatBeadComment(e)
	case *events.BeadClosedEvent:
		return formatBeadClosed(e)
	case *events.ErrorEvent:
		return formatError(e)
	case *events.ParseErrorEvent:
		return formatParseError(e)
	default:
		return ""
	}
}

func formatClaudeText(e *events.ClaudeTextEvent) string {
	text := safeString(e.Text)
	return truncate(text, maxTextLength)
}

func formatClaudeToolUse(e *events.ClaudeToolUseEvent) string {
	toolName := safeString(e.ToolName)
	if toolName == "" {
		return "tool: (unknown)"
	}

	detail := extractToolDetail(toolName, e.Input)
	if detail != "" {
		return fmt.Sprintf("tool: %s %s", toolName, detail)
	}
	return fmt.Sprintf("tool: %s", toolName)
}

func formatClaudeToolResult(e *events.ClaudeToolResultEvent) string {
	if e.IsError {
		return "tool result: ERROR"
	}
	return "tool result: ok"
}

func formatSessionStart(e *events.SessionStartEvent) string {
	beadID := safeString(e.BeadID)
	title := safeString(e.Title)
	if title != "" {
		return fmt.Sprintf("session started: %s - %s", beadID, truncate(title, 50))
	}
	return fmt.Sprintf("session started: %s", beadID)
}

func formatSessionEnd(e *events.SessionEndEvent) string {
	if e.TotalCostUSD > 0 {
		return fmt.Sprintf("session ended: %d turns, $%.4f", e.NumTurns, e.TotalCostUSD)
	}
	return fmt.Sprintf("session ended: %d turns", e.NumTurns)
}

func formatSessionTimeout(e *events.SessionTimeoutEvent) string {
	return fmt.Sprintf("session timeout after %s", e.Duration)
}

func formatDrainStart(e *events.DrainStartEvent) string {
	return fmt.Sprintf("drain started: %s", safeString(e.WorkDir))
}

func formatDrainStop(e *events.DrainStopEvent) string {
	reason := safeString(e.Reason)
	if reason != "" {
		return fmt.Sprintf("drain stopped: %s", reason)
	}
	return "drain stopped"
}

func formatDrainStateChanged(e *events.DrainStateChangedEvent) string {
	from := safeString(e.From)
	to := safeString(e.To)
	return fmt.Sprintf("state: %s -> %s", from, to)
}

func formatIterationStart(e *events.IterationStartEvent) string {
	beadID := safeString(e.BeadID)
	title := safeString(e.Title)
	if title != "" {
		return fmt.Sprintf("iteration: %s (P%d) - %s", beadID, e.Priority, truncate(title, 40))
	}
	return fmt.Sprintf("iteration: %s (P%d)", beadID, e.Priority)
}

func formatIterationEnd(e *events.IterationEndEvent) string {
	beadID := safeString(e.BeadID)
	symbol := "+"
	status := "completed"
	if !e.Success {
		symbol = "x"
		status = "failed"
	}
	if e.TotalCostUSD > 0 {
		return fmt.Sprintf("[%s] %s %s: %d turns, $%.4f", symbol, beadID, status, e.NumTurns, e.TotalCostUSD)
	}
	return fmt.Sprintf("[%s] %s %s: %d turns", symbol, beadID, status, e.NumTurns)
}

func formatTurnComplete(e *events.TurnCompleteEvent) string {
	return fmt.Sprintf("turn %d complete (%d tools, %dms)", e.TurnNumber, e.ToolCount, e.ToolElapsedMs)
}

func formatBeadAbandoned(e *events.BeadAbandonedEvent) string {
	beadID := safeString(e.BeadID)
	return fmt.Sprintf("[!] %s abandoned after %d/%d attempts", beadID, e.Attempts, e.MaxFailures)
}

func formatBeadCreated(e *events.BeadCreatedEvent) string {
	beadID := safeString(e.BeadID)
	title := safeString(e.Title)
	actor := safeString(e.Actor)
	if title != "" {
		return fmt.Sprintf("bead created: %s - %s (by %s)", beadID, truncate(title, 40), actor)
	}
	return fmt.Sprintf("bead created: %s (by %s)", beadID, actor)
}

func formatBeadStatus(e *events.BeadStatusEvent) string {
	beadID := safeString(e.BeadID)
	oldStatus := safeString(e.OldStatus)
	newStatus := safeString(e.NewStatus)
	symbol := statusSymbol(newStatus)
	return fmt.Sprintf("[%s] %s: %s -> %s", symbol, beadID, oldStatus, newStatus)
}

func formatBeadUpdated(e *events.BeadUpdatedEvent) string {
	beadID := safeString(e.BeadID)
	actor := safeString(e.Actor)
	return fmt.Sprintf("bead updated: %s (by %s)", beadID, actor)
}

func formatBeadComment(e *events.BeadCommentEvent) string {
	beadID := safeString(e.BeadID)
	actor := safeString(e.Actor)
	return fmt.Sprintf("comment on %s (by %s)", beadID, actor)
}

func formatBeadClosed(e *events.BeadClosedEvent) string {
	beadID := safeString(e.BeadID)
	actor := safeString(e.Actor)
	return fmt.Sprintf("[+] %s closed (by %s)", beadID, actor)
}

func formatError(e *events.ErrorEvent) string {
	msg := safeString(e.Message)
	severity := safeString(e.Severity)
	if severity == "" {
		severity = "error"
	}
	prefix := strings.ToUpper(severity)
	if e.BeadID != "" {
		return fmt.Sprintf("%s: %s - %s", prefix, e.BeadID, truncate(msg, 100))
	}
	return fmt.Sprintf("%s: %s", prefix, truncate(msg, 100))
}

func formatParseError(e *events.ParseErrorEvent) string {
	errMsg := safeString(e.Error)
	return fmt.Sprintf("PARSE ERROR: %s", truncate(errMsg, 100))
}

// extractToolDetail extracts relevant detail from tool input based on tool name.
func extractToolDetail(toolName string, input map[string]any) string {
	if input == nil {
		return ""
	}

	switch toolName {
	case "Bash":
		if cmd, ok := getStringValue(input, "command"); ok {
			return truncate(cmd, maxToolInput)
		}
	case "Read", "Write", "Edit":
		if path, ok := getStringValue(input, "file_path"); ok {
			return truncate(path, maxToolInput)
		}
	case "Glob":
		if pattern, ok := getStringValue(input, "pattern"); ok {
			return truncate(pattern, maxToolInput)
		}
	case "Grep":
		if pattern, ok := getStringValue(input, "pattern"); ok {
			return truncate(pattern, maxToolInput)
		}
	case "Task":
		if desc, ok := getStringValue(input, "description"); ok {
			return truncate(desc, maxToolInput)
		}
	case "WebFetch":
		if url, ok := getStringValue(input, "url"); ok {
			return truncate(url, maxToolInput)
		}
	case "TodoWrite":
		return "(updating todos)"
	}

	return ""
}

// getStringValue safely extracts a string value from a map.
func getStringValue(m map[string]any, key string) (string, bool) {
	if m == nil {
		return "", false
	}
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// truncate shortens text to maxLen, adding indicator if truncated.
func truncate(s string, maxLen int) string {
	s = safeString(s)
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= len(truncateIndicator) {
		return truncateIndicator
	}
	return s[:maxLen-len(truncateIndicator)] + truncateIndicator
}

// safeString sanitizes a string for display by removing control characters
// and limiting newlines.
func safeString(s string) string {
	// Remove ANSI escape sequences
	s = stripANSI(s)

	// Replace newlines with spaces
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")

	// Remove other control characters (except space)
	var sb strings.Builder
	sb.Grow(len(s))
	for _, r := range s {
		if r == ' ' || !unicode.IsControl(r) {
			sb.WriteRune(r)
		}
	}

	// Collapse multiple spaces
	result := sb.String()
	for strings.Contains(result, "  ") {
		result = strings.ReplaceAll(result, "  ", " ")
	}

	return strings.TrimSpace(result)
}

// ansiRegex matches ANSI escape sequences.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// stripANSI removes ANSI escape sequences from a string.
func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// statusSymbol returns a symbol for a bead status.
func statusSymbol(status string) string {
	switch status {
	case "ready":
		return ">"
	case "in_progress":
		return "~"
	case "blocked":
		return "!"
	case "closed":
		return "+"
	default:
		return "-"
	}
}

// formatDurationHuman formats milliseconds as a human-readable duration.
// Returns "<60s" for under a minute, "Xm" for minutes, "Xh" for hours only,
// or "Xh Ym" for hours and minutes.
func formatDurationHuman(ms int64) string {
	if ms <= 0 {
		return "<60s"
	}

	totalSeconds := ms / 1000
	if totalSeconds < 60 {
		return "<60s"
	}

	totalMinutes := totalSeconds / 60
	hours := totalMinutes / 60
	minutes := totalMinutes % 60

	if hours == 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	if minutes == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh %dm", hours, minutes)
}
