package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/npratt/atari/internal/events"
)

func TestSafeWidth(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"positive", 100, 100},
		{"zero", 0, 1},
		{"negative", -10, 1},
		{"one", 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := safeWidth(tt.input)
			if result != tt.expected {
				t.Errorf("safeWidth(%d) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSafeScroll(t *testing.T) {
	tests := []struct {
		name         string
		pos          int
		totalLines   int
		visibleLines int
		expected     int
	}{
		{"normal position", 5, 20, 10, 5},
		{"negative position", -5, 20, 10, 0},
		{"at max", 10, 20, 10, 10},
		{"past max", 15, 20, 10, 10},
		{"more visible than total", 5, 5, 10, 0},
		{"zero total", 0, 0, 10, 0},
		{"zero visible", 5, 20, 0, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := safeScroll(tt.pos, tt.totalLines, tt.visibleLines)
			if result != tt.expected {
				t.Errorf("safeScroll(%d, %d, %d) = %d, want %d",
					tt.pos, tt.totalLines, tt.visibleLines, result, tt.expected)
			}
		})
	}
}

func TestVisibleLines(t *testing.T) {
	tests := []struct {
		name     string
		height   int
		expected int
	}{
		{"normal height", 20, 12},   // 20 - 8 = 12
		{"minimum height", 15, 7},   // 15 - 8 = 7
		{"small height", 9, 1},      // max(1, 9-8) = 1
		{"very small", 3, 1},        // max(1, 3-8) = 1 (negative clamped)
		{"zero height", 0, 1},       // max(1, 0-8) = 1
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := model{height: tt.height}
			result := m.visibleLines()
			if result != tt.expected {
				t.Errorf("visibleLines() with height %d = %d, want %d",
					tt.height, result, tt.expected)
			}
		})
	}
}

func TestRenderTooSmall(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"narrow", 50, 20},
		{"short", 80, 10},
		{"both small", 30, 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := model{width: tt.width, height: tt.height}
			result := m.renderTooSmall()

			if !strings.Contains(result, "Terminal too small") {
				t.Errorf("renderTooSmall() should contain 'Terminal too small', got: %s", result)
			}
			if !strings.Contains(result, "60x15") {
				t.Errorf("renderTooSmall() should mention minimum size 60x15, got: %s", result)
			}
		})
	}
}

func TestViewTooSmall(t *testing.T) {
	tests := []struct {
		name        string
		width       int
		height      int
		shouldBeToo bool
	}{
		{"too narrow", 50, 20, true},
		{"too short", 80, 10, true},
		{"both small", 30, 8, true},
		{"minimum size", 60, 15, false},
		{"larger", 100, 30, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := model{width: tt.width, height: tt.height, status: "idle"}
			result := m.View()

			hasTooSmall := strings.Contains(result, "Terminal too small")
			if hasTooSmall != tt.shouldBeToo {
				t.Errorf("View() with %dx%d: hasTooSmall=%v, want %v",
					tt.width, tt.height, hasTooSmall, tt.shouldBeToo)
			}
		})
	}
}

func TestViewLoading(t *testing.T) {
	m := model{width: 0, height: 0}
	result := m.View()

	if result != "Loading..." {
		t.Errorf("View() with zero dimensions = %q, want %q", result, "Loading...")
	}
}

func TestRenderHeader(t *testing.T) {
	t.Run("idle with no bead", func(t *testing.T) {
		m := model{
			width:  80,
			height: 25,
			status: "idle",
			stats: modelStats{
				TotalCost:  0.1234,
				TotalTurns: 10,
				Completed:  5,
				Failed:     2,
				Abandoned:  1,
			},
		}

		result := m.renderHeader()

		if !strings.Contains(result, "IDLE") {
			t.Error("header should contain status IDLE")
		}
		if !strings.Contains(result, "0.1234") {
			t.Error("header should contain cost")
		}
		if !strings.Contains(result, "no active bead") {
			t.Error("header should show no active bead")
		}
		if !strings.Contains(result, "turns: 10") {
			t.Error("header should show turns count")
		}
		if !strings.Contains(result, "completed: 5") {
			t.Error("header should show completed count")
		}
	})

	t.Run("working with bead", func(t *testing.T) {
		m := model{
			width:  80,
			height: 25,
			status: "working",
			currentBead: &beadInfo{
				ID:    "bd-123",
				Title: "Test bead title",
			},
			stats: modelStats{
				TotalCost: 0.5,
			},
		}

		result := m.renderHeader()

		if !strings.Contains(result, "WORKING") {
			t.Error("header should contain status WORKING")
		}
		if !strings.Contains(result, "bd-123") {
			t.Error("header should contain bead ID")
		}
		if !strings.Contains(result, "Test bead title") {
			t.Error("header should contain bead title")
		}
	})

	t.Run("paused status", func(t *testing.T) {
		m := model{
			width:  80,
			height: 25,
			status: "paused",
		}

		result := m.renderHeader()

		if !strings.Contains(result, "PAUSED") {
			t.Error("header should contain status PAUSED")
		}
	})
}

func TestRenderEvents(t *testing.T) {
	t.Run("no events", func(t *testing.T) {
		m := model{
			width:  80,
			height: 25,
		}

		result := m.renderEvents()

		if !strings.Contains(result, "Waiting for events") {
			t.Error("should show waiting message when no events")
		}
	})

	t.Run("with events", func(t *testing.T) {
		now := time.Now()
		m := model{
			width:  80,
			height: 25,
			eventLines: []eventLine{
				{Time: now, Text: "test event 1", Style: styles.Tool},
				{Time: now, Text: "test event 2", Style: styles.Session},
			},
		}

		result := m.renderEvents()

		if !strings.Contains(result, "test event 1") {
			t.Error("should show event 1")
		}
		if !strings.Contains(result, "test event 2") {
			t.Error("should show event 2")
		}
	})

	t.Run("scroll position", func(t *testing.T) {
		now := time.Now()
		eventLines := make([]eventLine, 30)
		for i := range eventLines {
			eventLines[i] = eventLine{
				Time:  now,
				Text:  "event " + string(rune('A'+i)),
				Style: styles.Tool,
			}
		}

		m := model{
			width:      80,
			height:     15, // visibleLines = 7
			scrollPos:  10,
			eventLines: eventLines,
		}

		result := m.renderEvents()

		// Should show events starting from position 10
		if !strings.Contains(result, "event K") { // index 10 = 'A' + 10 = 'K'
			t.Error("should show event at scroll position")
		}
	})
}

func TestRenderFooter(t *testing.T) {
	tests := []struct {
		name           string
		status         string
		shouldContain  []string
		shouldNotHave  []string
	}{
		{
			name:          "idle",
			status:        "idle",
			shouldContain: []string{"p: pause", "q: quit", "scroll"},
		},
		{
			name:          "working",
			status:        "working",
			shouldContain: []string{"p: pause", "q: quit"},
		},
		{
			name:           "paused",
			status:         "paused",
			shouldContain:  []string{"r: resume", "q: quit"},
			shouldNotHave:  []string{"p: pause"},
		},
		{
			name:           "stopped",
			status:         "stopped",
			shouldContain:  []string{"q: quit"},
			shouldNotHave:  []string{"p: pause", "r: resume"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := model{status: tt.status}
			result := m.renderFooter()

			for _, s := range tt.shouldContain {
				if !strings.Contains(result, s) {
					t.Errorf("footer for %s should contain %q, got: %s", tt.status, s, result)
				}
			}
			for _, s := range tt.shouldNotHave {
				if strings.Contains(result, s) {
					t.Errorf("footer for %s should not contain %q, got: %s", tt.status, s, result)
				}
			}
		})
	}
}

func TestStyleForEvent(t *testing.T) {
	tests := []struct {
		name  string
		event events.Event
	}{
		{"nil event", nil},
		{"tool use", &events.ClaudeToolUseEvent{}},
		{"tool result", &events.ClaudeToolResultEvent{}},
		{"text", &events.ClaudeTextEvent{}},
		{"session start", &events.SessionStartEvent{}},
		{"session end", &events.SessionEndEvent{}},
		{"iteration start", &events.IterationStartEvent{}},
		{"iteration end", &events.IterationEndEvent{}},
		{"bead created", &events.BeadCreatedEvent{}},
		{"bead status", &events.BeadStatusEvent{}},
		{"error", &events.ErrorEvent{}},
		{"parse error", &events.ParseErrorEvent{}},
		{"drain start", &events.DrainStartEvent{}},
		{"drain stop", &events.DrainStopEvent{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify no panic and returns a style
			style := StyleForEvent(tt.event)
			// Style should have some value - we just check it doesn't panic
			_ = style.Render("test")
		})
	}
}

func TestRenderEventLine(t *testing.T) {
	now := time.Date(2024, 1, 15, 14, 30, 45, 0, time.UTC)

	t.Run("normal line", func(t *testing.T) {
		m := model{width: 80}
		el := eventLine{
			Time:  now,
			Text:  "test event message",
			Style: styles.Tool,
		}

		result := m.renderEventLine(el, 60)

		if !strings.Contains(result, "14:30:45") {
			t.Error("should contain timestamp in HH:MM:SS format")
		}
		if !strings.Contains(result, "test event message") {
			t.Error("should contain event text")
		}
	})

	t.Run("truncated line", func(t *testing.T) {
		m := model{width: 80}
		el := eventLine{
			Time:  now,
			Text:  "this is a very long event message that should be truncated",
			Style: styles.Tool,
		}

		result := m.renderEventLine(el, 30) // Very narrow

		if !strings.Contains(result, "...") {
			t.Error("long text should be truncated with ...")
		}
	})
}
