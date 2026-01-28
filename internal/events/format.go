package events

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

const (
	maxTextLength    = 200
	maxToolInput     = 100
	maxTitleLength   = 50
	truncateIndicator = "..."
)

// Format converts an event to a human-readable string for display.
// Returns empty string for nil or unknown event types.
func Format(event Event) string {
	if event == nil {
		return ""
	}

	switch e := event.(type) {
	case *ClaudeTextEvent:
		return formatClaudeText(e)
	case *ClaudeToolUseEvent:
		return formatClaudeToolUse(e)
	case *ClaudeToolResultEvent:
		return formatClaudeToolResult(e)
	case *SessionStartEvent:
		return formatSessionStart(e)
	case *SessionEndEvent:
		return formatSessionEnd(e)
	case *SessionTimeoutEvent:
		return formatSessionTimeout(e)
	case *DrainStartEvent:
		return formatDrainStart(e)
	case *DrainStopEvent:
		return formatDrainStop(e)
	case *DrainStateChangedEvent:
		return formatDrainStateChanged(e)
	case *IterationStartEvent:
		return formatIterationStart(e)
	case *IterationEndEvent:
		return formatIterationEnd(e)
	case *TurnCompleteEvent:
		return formatTurnComplete(e)
	case *BeadAbandonedEvent:
		return formatBeadAbandoned(e)
	case *BeadCreatedEvent:
		return formatBeadCreated(e)
	case *BeadStatusEvent:
		return formatBeadStatus(e)
	case *BeadUpdatedEvent:
		return formatBeadUpdated(e)
	case *BeadCommentEvent:
		return formatBeadComment(e)
	case *BeadClosedEvent:
		return formatBeadClosed(e)
	case *BeadChangedEvent:
		return formatBeadChanged(e)
	case *EpicClosedEvent:
		return formatEpicClosed(e)
	case *ErrorEvent:
		return formatError(e)
	case *ParseErrorEvent:
		return formatParseError(e)
	default:
		return ""
	}
}

// FormatWithTimestamp formats an event with a timestamp prefix.
// Used for observer context and log display.
func FormatWithTimestamp(event Event) string {
	if event == nil {
		return ""
	}
	ts := event.Timestamp().Format("15:04:05")
	detail := Format(event)
	if detail == "" {
		return fmt.Sprintf("[%s] %s", ts, event.Type())
	}
	return fmt.Sprintf("[%s] %s", ts, detail)
}

func formatClaudeText(e *ClaudeTextEvent) string {
	text := SafeString(e.Text)
	return Truncate(text, maxTextLength)
}

func formatClaudeToolUse(e *ClaudeToolUseEvent) string {
	toolName := SafeString(e.ToolName)
	if toolName == "" {
		return "tool: (unknown)"
	}

	detail := ExtractToolDetail(toolName, e.Input)
	if detail != "" {
		return fmt.Sprintf("tool: %s %s", toolName, detail)
	}
	return fmt.Sprintf("tool: %s", toolName)
}

func formatClaudeToolResult(e *ClaudeToolResultEvent) string {
	if e.IsError {
		return "tool result: ERROR"
	}
	return "tool result: ok"
}

func formatSessionStart(e *SessionStartEvent) string {
	beadID := SafeString(e.BeadID)
	title := SafeString(e.Title)
	if title != "" {
		return fmt.Sprintf("session started: %s - %s", beadID, Truncate(title, maxTitleLength))
	}
	return fmt.Sprintf("session started: %s", beadID)
}

func formatSessionEnd(e *SessionEndEvent) string {
	if e.TotalCostUSD > 0 {
		return fmt.Sprintf("session ended: %d turns, $%.4f", e.NumTurns, e.TotalCostUSD)
	}
	return fmt.Sprintf("session ended: %d turns", e.NumTurns)
}

func formatSessionTimeout(e *SessionTimeoutEvent) string {
	return fmt.Sprintf("session timeout after %s", e.Duration)
}

func formatDrainStart(e *DrainStartEvent) string {
	return fmt.Sprintf("drain started: %s", SafeString(e.WorkDir))
}

func formatDrainStop(e *DrainStopEvent) string {
	reason := SafeString(e.Reason)
	if reason != "" {
		return fmt.Sprintf("drain stopped: %s", reason)
	}
	return "drain stopped"
}

func formatDrainStateChanged(e *DrainStateChangedEvent) string {
	from := SafeString(e.From)
	to := SafeString(e.To)
	return fmt.Sprintf("state: %s -> %s", from, to)
}

func formatIterationStart(e *IterationStartEvent) string {
	beadID := SafeString(e.BeadID)
	title := SafeString(e.Title)
	if title != "" {
		return fmt.Sprintf("iteration: %s (P%d) - %s", beadID, e.Priority, Truncate(title, 40))
	}
	return fmt.Sprintf("iteration: %s (P%d)", beadID, e.Priority)
}

func formatIterationEnd(e *IterationEndEvent) string {
	beadID := SafeString(e.BeadID)
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

func formatTurnComplete(e *TurnCompleteEvent) string {
	return fmt.Sprintf("turn %d complete (%d tools, %dms)", e.TurnNumber, e.ToolCount, e.ToolElapsedMs)
}

func formatBeadAbandoned(e *BeadAbandonedEvent) string {
	beadID := SafeString(e.BeadID)
	return fmt.Sprintf("[!] %s abandoned after %d/%d attempts", beadID, e.Attempts, e.MaxFailures)
}

func formatBeadCreated(e *BeadCreatedEvent) string {
	beadID := SafeString(e.BeadID)
	title := SafeString(e.Title)
	actor := SafeString(e.Actor)
	if title != "" {
		return fmt.Sprintf("bead created: %s - %s (by %s)", beadID, Truncate(title, 40), actor)
	}
	return fmt.Sprintf("bead created: %s (by %s)", beadID, actor)
}

func formatBeadStatus(e *BeadStatusEvent) string {
	beadID := SafeString(e.BeadID)
	oldStatus := SafeString(e.OldStatus)
	newStatus := SafeString(e.NewStatus)
	symbol := StatusSymbol(newStatus)
	return fmt.Sprintf("[%s] %s: %s -> %s", symbol, beadID, oldStatus, newStatus)
}

func formatBeadUpdated(e *BeadUpdatedEvent) string {
	beadID := SafeString(e.BeadID)
	actor := SafeString(e.Actor)
	return fmt.Sprintf("bead updated: %s (by %s)", beadID, actor)
}

func formatBeadComment(e *BeadCommentEvent) string {
	beadID := SafeString(e.BeadID)
	actor := SafeString(e.Actor)
	return fmt.Sprintf("comment on %s (by %s)", beadID, actor)
}

func formatBeadClosed(e *BeadClosedEvent) string {
	beadID := SafeString(e.BeadID)
	actor := SafeString(e.Actor)
	return fmt.Sprintf("[+] %s closed (by %s)", beadID, actor)
}

func formatBeadChanged(e *BeadChangedEvent) string {
	beadID := SafeString(e.BeadID)

	// Detect what changed
	if e.OldState == nil && e.NewState != nil {
		return fmt.Sprintf("bead created: %s - %s", beadID, Truncate(SafeString(e.NewState.Title), 40))
	}
	if e.OldState != nil && e.NewState == nil {
		return fmt.Sprintf("bead deleted: %s", beadID)
	}
	if e.OldState != nil && e.NewState != nil {
		if e.OldState.Status != e.NewState.Status {
			symbol := StatusSymbol(e.NewState.Status)
			return fmt.Sprintf("[%s] %s: %s -> %s", symbol, beadID, e.OldState.Status, e.NewState.Status)
		}
		return fmt.Sprintf("bead changed: %s", beadID)
	}
	return fmt.Sprintf("bead changed: %s", beadID)
}

func formatEpicClosed(e *EpicClosedEvent) string {
	epicID := SafeString(e.EpicID)
	title := SafeString(e.Title)
	if title != "" {
		return fmt.Sprintf("[+] epic %s closed: %s (%d children)", epicID, Truncate(title, 30), e.TotalChildren)
	}
	return fmt.Sprintf("[+] epic %s closed (%d children)", epicID, e.TotalChildren)
}

func formatError(e *ErrorEvent) string {
	msg := SafeString(e.Message)
	severity := SafeString(e.Severity)
	if severity == "" {
		severity = "error"
	}
	prefix := strings.ToUpper(severity)
	if e.BeadID != "" {
		return fmt.Sprintf("%s: %s - %s", prefix, e.BeadID, Truncate(msg, 100))
	}
	return fmt.Sprintf("%s: %s", prefix, Truncate(msg, 100))
}

func formatParseError(e *ParseErrorEvent) string {
	errMsg := SafeString(e.Error)
	return fmt.Sprintf("PARSE ERROR: %s", Truncate(errMsg, 100))
}

// ExtractToolDetail extracts relevant detail from tool input based on tool name.
func ExtractToolDetail(toolName string, input map[string]any) string {
	if input == nil {
		return ""
	}

	switch toolName {
	case "Bash":
		if cmd, ok := getStringValue(input, "command"); ok {
			return Truncate(cmd, maxToolInput)
		}
	case "Read", "Write", "Edit":
		if path, ok := getStringValue(input, "file_path"); ok {
			return filepath.Base(path)
		}
	case "Glob":
		if pattern, ok := getStringValue(input, "pattern"); ok {
			return Truncate(pattern, maxToolInput)
		}
	case "Grep":
		if pattern, ok := getStringValue(input, "pattern"); ok {
			return Truncate(pattern, maxToolInput)
		}
	case "Task":
		if desc, ok := getStringValue(input, "description"); ok {
			return Truncate(desc, maxToolInput)
		}
	case "WebFetch":
		if url, ok := getStringValue(input, "url"); ok {
			return Truncate(url, maxToolInput)
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

// Truncate shortens text to maxLen, adding indicator if truncated.
func Truncate(s string, maxLen int) string {
	s = SafeString(s)
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= len(truncateIndicator) {
		return truncateIndicator
	}
	return s[:maxLen-len(truncateIndicator)] + truncateIndicator
}

// ansiRegex matches ANSI escape sequences.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// StripANSI removes ANSI escape sequences from a string.
func StripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// SafeString sanitizes a string for display by removing control characters
// and limiting newlines.
func SafeString(s string) string {
	// Remove ANSI escape sequences
	s = StripANSI(s)

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

// StatusSymbol returns a symbol for a bead status.
func StatusSymbol(status string) string {
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
