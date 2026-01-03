package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/npratt/atari/internal/events"
)

// mockStatsGetter is a mock implementation of StatsGetter for testing.
type mockStatsGetter struct {
	iteration   int
	completed   int
	failed      int
	abandoned   int
	currentBead string
}

func (m *mockStatsGetter) Iteration() int    { return m.iteration }
func (m *mockStatsGetter) Completed() int    { return m.completed }
func (m *mockStatsGetter) Failed() int       { return m.failed }
func (m *mockStatsGetter) Abandoned() int    { return m.abandoned }
func (m *mockStatsGetter) CurrentBead() string { return m.currentBead }

func TestHandleKey_Quit(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"q key", "q"},
		{"ctrl+c", "ctrl+c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quitCalled := false
			m := model{
				status: "idle",
				onQuit: func() { quitCalled = true },
			}

			newM, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)})

			if !quitCalled {
				t.Error("onQuit callback should be called")
			}
			if cmd == nil {
				t.Error("should return tea.Quit command")
			}
			_ = newM // model returned
		})
	}
}

func TestHandleKey_Pause(t *testing.T) {
	pauseCalled := false
	m := model{
		status:  "idle",
		onPause: func() { pauseCalled = true },
	}

	newM, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})

	if !pauseCalled {
		t.Error("onPause callback should be called")
	}
	if cmd != nil {
		t.Error("should return nil command")
	}
	if newM.(model).status != "pausing..." {
		t.Errorf("status should be 'pausing...', got %q", newM.(model).status)
	}
}

func TestHandleKey_Resume(t *testing.T) {
	resumeCalled := false
	m := model{
		status:   "paused",
		onResume: func() { resumeCalled = true },
	}

	newM, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})

	if !resumeCalled {
		t.Error("onResume callback should be called")
	}
	if cmd != nil {
		t.Error("should return nil command")
	}
	if newM.(model).status != "resuming..." {
		t.Errorf("status should be 'resuming...', got %q", newM.(model).status)
	}
}

func TestHandleKey_ScrollUp(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		startPos int
		endPos   int
	}{
		{"up key from middle", "up", 5, 4},
		{"k key from middle", "k", 5, 4},
		{"up key from top", "up", 0, 0},
		{"k key from top", "k", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := model{
				scrollPos:  tt.startPos,
				autoScroll: true,
				height:     20,
			}

			newM, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)})
			resultM := newM.(model)

			if resultM.scrollPos != tt.endPos {
				t.Errorf("scrollPos should be %d, got %d", tt.endPos, resultM.scrollPos)
			}
			if resultM.autoScroll {
				t.Error("autoScroll should be disabled after scroll up")
			}
		})
	}
}

func TestHandleKey_ScrollDown(t *testing.T) {
	eventLines := make([]eventLine, 30)
	for i := range eventLines {
		eventLines[i] = eventLine{Text: "test"}
	}

	tests := []struct {
		name           string
		key            string
		startPos       int
		expectedPos    int
		expectedAuto   bool
	}{
		{"down key in middle", "down", 5, 6, false},
		{"j key in middle", "j", 5, 6, false},
		{"down at bottom enables autoscroll", "down", 17, 18, true}, // maxScroll = 30 - 12 = 18
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := model{
				scrollPos:  tt.startPos,
				autoScroll: false,
				height:     20, // visibleLines = 12
				eventLines: eventLines,
			}

			newM, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)})
			resultM := newM.(model)

			if resultM.scrollPos != tt.expectedPos {
				t.Errorf("scrollPos should be %d, got %d", tt.expectedPos, resultM.scrollPos)
			}
			if resultM.autoScroll != tt.expectedAuto {
				t.Errorf("autoScroll should be %v, got %v", tt.expectedAuto, resultM.autoScroll)
			}
		})
	}
}

func TestHandleKey_Home(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"home key", "home"},
		{"g key", "g"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := model{
				scrollPos:  10,
				autoScroll: true,
			}

			newM, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)})
			resultM := newM.(model)

			if resultM.scrollPos != 0 {
				t.Errorf("scrollPos should be 0, got %d", resultM.scrollPos)
			}
			if resultM.autoScroll {
				t.Error("autoScroll should be disabled after home")
			}
		})
	}
}

func TestHandleKey_End(t *testing.T) {
	eventLines := make([]eventLine, 30)
	for i := range eventLines {
		eventLines[i] = eventLine{Text: "test"}
	}

	tests := []struct {
		name string
		key  string
	}{
		{"end key", "end"},
		{"G key", "G"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := model{
				scrollPos:  0,
				autoScroll: false,
				height:     20, // visibleLines = 12
				eventLines: eventLines,
			}

			newM, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)})
			resultM := newM.(model)

			expectedPos := 30 - 12 // maxScroll
			if resultM.scrollPos != expectedPos {
				t.Errorf("scrollPos should be %d, got %d", expectedPos, resultM.scrollPos)
			}
			if !resultM.autoScroll {
				t.Error("autoScroll should be enabled after end")
			}
		})
	}
}

func TestHandleKey_NilCallbacks(t *testing.T) {
	// Ensure nil callbacks don't panic
	m := model{status: "idle"}

	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	// No panic = success
}

func TestHandleEvent_DrainStateChanged(t *testing.T) {
	m := model{status: "idle"}

	event := &events.DrainStateChangedEvent{
		BaseEvent: events.NewInternalEvent(events.EventDrainStateChanged),
		From:      "idle",
		To:        "running",
	}

	m.handleEvent(event)

	if m.status != "running" {
		t.Errorf("status should be 'running', got %q", m.status)
	}
}

func TestHandleEvent_IterationStart(t *testing.T) {
	m := model{status: "idle"}

	event := &events.IterationStartEvent{
		BaseEvent: events.NewInternalEvent(events.EventIterationStart),
		BeadID:    "bd-123",
		Title:     "Test bead",
		Priority:  2,
	}

	m.handleEvent(event)

	if m.status != "working" {
		t.Errorf("status should be 'working', got %q", m.status)
	}
	if m.currentBead == nil {
		t.Fatal("currentBead should not be nil")
	}
	if m.currentBead.ID != "bd-123" {
		t.Errorf("currentBead.ID should be 'bd-123', got %q", m.currentBead.ID)
	}
	if m.currentBead.Title != "Test bead" {
		t.Errorf("currentBead.Title should be 'Test bead', got %q", m.currentBead.Title)
	}
	if m.currentBead.Priority != 2 {
		t.Errorf("currentBead.Priority should be 2, got %d", m.currentBead.Priority)
	}
}

func TestHandleEvent_IterationEnd(t *testing.T) {
	tests := []struct {
		name            string
		success         bool
		expectedComp    int
		expectedFailed  int
	}{
		{"success", true, 1, 0},
		{"failure", false, 0, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := model{
				status: "working",
				currentBead: &beadInfo{
					ID: "bd-123",
				},
				stats: modelStats{},
			}

			event := &events.IterationEndEvent{
				BaseEvent:    events.NewInternalEvent(events.EventIterationEnd),
				BeadID:       "bd-123",
				Success:      tt.success,
				NumTurns:     10,
				TotalCostUSD: 0.05,
			}

			m.handleEvent(event)

			if m.status != "idle" {
				t.Errorf("status should be 'idle', got %q", m.status)
			}
			if m.currentBead != nil {
				t.Error("currentBead should be nil after iteration end")
			}
			if m.stats.Completed != tt.expectedComp {
				t.Errorf("stats.Completed should be %d, got %d", tt.expectedComp, m.stats.Completed)
			}
			if m.stats.Failed != tt.expectedFailed {
				t.Errorf("stats.Failed should be %d, got %d", tt.expectedFailed, m.stats.Failed)
			}
			if m.stats.TotalTurns != 10 {
				t.Errorf("stats.TotalTurns should be 10, got %d", m.stats.TotalTurns)
			}
			if m.stats.TotalCost != 0.05 {
				t.Errorf("stats.TotalCost should be 0.05, got %f", m.stats.TotalCost)
			}
		})
	}
}

func TestHandleEvent_BeadAbandoned(t *testing.T) {
	m := model{stats: modelStats{Abandoned: 0}}

	event := &events.BeadAbandonedEvent{
		BaseEvent: events.NewInternalEvent(events.EventBeadAbandoned),
		BeadID:    "bd-123",
	}

	m.handleEvent(event)

	if m.stats.Abandoned != 1 {
		t.Errorf("stats.Abandoned should be 1, got %d", m.stats.Abandoned)
	}
}

func TestHandleEvent_DrainStop(t *testing.T) {
	m := model{status: "running"}

	event := &events.DrainStopEvent{
		BaseEvent: events.NewInternalEvent(events.EventDrainStop),
		Reason:    "user requested",
	}

	m.handleEvent(event)

	if m.status != "stopped" {
		t.Errorf("status should be 'stopped', got %q", m.status)
	}
}

func TestHandleEvent_AddsToEventLog(t *testing.T) {
	m := model{
		autoScroll: true,
		height:     20,
	}

	event := &events.SessionStartEvent{
		BaseEvent: events.NewInternalEvent(events.EventSessionStart),
		BeadID:    "bd-123",
		Title:     "Test bead",
	}

	m.handleEvent(event)

	if len(m.eventLines) != 1 {
		t.Errorf("should have 1 event line, got %d", len(m.eventLines))
	}
}

func TestHandleEvent_BufferTrimming(t *testing.T) {
	// Create a model with almost max lines
	m := model{
		eventLines: make([]eventLine, maxEventLines-5),
		autoScroll: true,
		height:     20,
		scrollPos:  maxEventLines - 20,
	}

	// Add events that push over the limit
	for i := 0; i < 10; i++ {
		event := &events.SessionStartEvent{
			BaseEvent: events.NewInternalEvent(events.EventSessionStart),
			BeadID:    "bd-123",
			Title:     "Test bead",
		}
		m.handleEvent(event)
	}

	// Buffer should be trimmed
	if len(m.eventLines) > maxEventLines {
		t.Errorf("buffer should be trimmed, got %d lines", len(m.eventLines))
	}

	// After trimming, should have: (995 + 10) - 100 = 905 lines
	expected := maxEventLines - 5 + 10 - trimEventLines
	if len(m.eventLines) != expected {
		t.Errorf("buffer should have %d lines after trim, got %d", expected, len(m.eventLines))
	}
}

func TestHandleEvent_ScrollPosAdjustedAfterTrim(t *testing.T) {
	// Create a model with max lines and scroll position
	m := model{
		eventLines: make([]eventLine, maxEventLines),
		autoScroll: false,
		height:     20,
		scrollPos:  maxEventLines - 50, // scrollPos = 950
	}

	// Add event that triggers trim
	event := &events.SessionStartEvent{
		BaseEvent: events.NewInternalEvent(events.EventSessionStart),
		BeadID:    "bd-123",
		Title:     "Test bead",
	}
	m.handleEvent(event)

	// Scroll pos should be adjusted by trimEventLines
	expectedPos := (maxEventLines - 50) - trimEventLines // 950 - 100 = 850
	if m.scrollPos != expectedPos {
		t.Errorf("scrollPos should be adjusted to %d, got %d", expectedPos, m.scrollPos)
	}
}

func TestHandleEvent_AutoScrollToBottom(t *testing.T) {
	m := model{
		eventLines: make([]eventLine, 10),
		autoScroll: true,
		height:     20, // visibleLines = 12
		scrollPos:  0,
	}

	event := &events.SessionStartEvent{
		BaseEvent: events.NewInternalEvent(events.EventSessionStart),
		BeadID:    "bd-123",
		Title:     "Test bead",
	}
	m.handleEvent(event)

	// With 11 events and 12 visible lines, no scroll needed (all fit)
	// But if we add more, scroll should follow
	for i := 0; i < 20; i++ {
		m.handleEvent(event)
	}

	// Now we have 31 events, maxScroll = 31 - 12 = 19
	expectedPos := len(m.eventLines) - m.visibleLines()
	if m.scrollPos != expectedPos {
		t.Errorf("scrollPos should be %d for autoscroll, got %d", expectedPos, m.scrollPos)
	}
}

func TestHandleTick_NilStatsGetter(t *testing.T) {
	m := model{statsGetter: nil}
	// Should not panic
	m.handleTick()
}

func TestHandleTick_SyncsStats(t *testing.T) {
	mock := &mockStatsGetter{
		completed: 5,
		failed:    2,
		abandoned: 1,
	}
	m := model{
		statsGetter: mock,
		stats: modelStats{
			Completed: 4, // Drift from controller
			Failed:    2,
			Abandoned: 1,
		},
	}

	m.handleTick()

	if m.stats.Completed != 5 {
		t.Errorf("stats.Completed should be synced to 5, got %d", m.stats.Completed)
	}
}

func TestHandleTick_ClearsBeadOnDrift(t *testing.T) {
	mock := &mockStatsGetter{
		currentBead: "", // Controller says no current bead
	}
	m := model{
		statsGetter: mock,
		currentBead: &beadInfo{ID: "bd-123"}, // TUI thinks there's one
	}

	m.handleTick()

	if m.currentBead != nil {
		t.Error("currentBead should be cleared when controller has none")
	}
}

func TestUpdate_WindowSizeMsg(t *testing.T) {
	m := model{width: 0, height: 0}

	newM, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	resultM := newM.(model)

	if resultM.width != 100 {
		t.Errorf("width should be 100, got %d", resultM.width)
	}
	if resultM.height != 30 {
		t.Errorf("height should be 30, got %d", resultM.height)
	}
	if cmd != nil {
		t.Error("should return nil command for window size")
	}
}

func TestUpdate_EventMsg(t *testing.T) {
	ch := make(chan events.Event, 1)
	m := model{
		eventChan: ch,
		status:    "idle",
	}

	event := &events.DrainStateChangedEvent{
		BaseEvent: events.NewInternalEvent(events.EventDrainStateChanged),
		From:      "idle",
		To:        "running",
	}

	newM, cmd := m.Update(eventMsg(event))
	resultM := newM.(model)

	if resultM.status != "running" {
		t.Errorf("status should be updated to 'running', got %q", resultM.status)
	}
	if cmd == nil {
		t.Error("should return command to wait for next event")
	}
}

func TestUpdate_ChannelClosedMsg(t *testing.T) {
	m := model{status: "running"}

	_, cmd := m.Update(channelClosedMsg{})

	if cmd == nil {
		t.Error("should return tea.Quit command")
	}
}

func TestUpdate_TickMsg(t *testing.T) {
	mock := &mockStatsGetter{completed: 3}
	m := model{
		statsGetter: mock,
		stats:       modelStats{Completed: 2},
	}

	newM, cmd := m.Update(tickMsg(time.Now()))
	resultM := newM.(model)

	if resultM.stats.Completed != 3 {
		t.Errorf("stats should be synced, got Completed=%d", resultM.stats.Completed)
	}
	if cmd == nil {
		t.Error("should return command for next tick")
	}
}

func TestWaitForEvent_ClosedChannel(t *testing.T) {
	ch := make(chan events.Event)
	close(ch)

	cmd := waitForEvent(ch)
	msg := cmd()

	if _, ok := msg.(channelClosedMsg); !ok {
		t.Errorf("should return channelClosedMsg, got %T", msg)
	}
}

func TestWaitForEvent_ReceivesEvent(t *testing.T) {
	ch := make(chan events.Event, 1)
	event := &events.DrainStartEvent{
		BaseEvent: events.NewInternalEvent(events.EventDrainStart),
	}
	ch <- event

	cmd := waitForEvent(ch)
	msg := cmd()

	if evtMsg, ok := msg.(eventMsg); ok {
		if evtMsg.Type() != events.EventDrainStart {
			t.Errorf("should receive DrainStartEvent, got %s", evtMsg.Type())
		}
	} else {
		t.Errorf("should return eventMsg, got %T", msg)
	}
}
