package tui

import (
	"fmt"

	"github.com/npratt/atari/internal/events"
)

// Re-export constants for backward compatibility with existing TUI code
const (
	maxTextLength     = 200
	truncateIndicator = "..."
)

// Format converts an event to a human-readable string for display.
// Delegates to events.Format for consistent formatting across all consumers.
func Format(event events.Event) string {
	return events.Format(event)
}

// truncate shortens text to maxLen, adding indicator if truncated.
// Delegates to events.Truncate.
func truncate(s string, maxLen int) string {
	return events.Truncate(s, maxLen)
}

// safeString sanitizes a string for display.
// Delegates to events.SafeString.
func safeString(s string) string {
	return events.SafeString(s)
}

// stripANSI removes ANSI escape sequences from a string.
// Delegates to events.StripANSI.
func stripANSI(s string) string {
	return events.StripANSI(s)
}

// statusSymbol returns a symbol for a bead status.
// Delegates to events.StatusSymbol.
func statusSymbol(status string) string {
	return events.StatusSymbol(status)
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
