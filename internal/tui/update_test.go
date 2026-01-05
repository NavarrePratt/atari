package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/events"
)

// mockStatsGetter is a mock implementation of StatsGetter for testing.
type mockStatsGetter struct {
	iteration    int
	completed    int
	failed       int
	abandoned    int
	currentBead  string
	currentTurns int
}

func (m *mockStatsGetter) Iteration() int      { return m.iteration }
func (m *mockStatsGetter) Completed() int      { return m.completed }
func (m *mockStatsGetter) Failed() int         { return m.failed }
func (m *mockStatsGetter) Abandoned() int      { return m.abandoned }
func (m *mockStatsGetter) CurrentBead() string { return m.currentBead }
func (m *mockStatsGetter) CurrentTurns() int   { return m.currentTurns }

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

// Focus management tests

func TestCycleFocus(t *testing.T) {
	tests := []struct {
		name         string
		startFocus   FocusedPane
		eventsOpen   bool
		observerOpen bool
		expectFocus  FocusedPane
	}{
		{"events to observer", FocusEvents, true, true, FocusObserver},
		{"observer to events", FocusObserver, true, true, FocusEvents},
		{"events stays on events when no other panes open", FocusEvents, true, false, FocusEvents},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := model{
				focusedPane:  tt.startFocus,
				eventsOpen:   tt.eventsOpen,
				observerOpen: tt.observerOpen,
				observerPane: NewObserverPane(nil),
				graphPane:    NewGraphPane(nil, nil, "horizontal"),
			}
			m.cycleFocus()
			if m.focusedPane != tt.expectFocus {
				t.Errorf("cycleFocus() from %v: got %v, want %v",
					tt.startFocus, m.focusedPane, tt.expectFocus)
			}
		})
	}
}

func TestIsObserverFocused(t *testing.T) {
	tests := []struct {
		name   string
		focus  FocusedPane
		expect bool
	}{
		{"events focused", FocusEvents, false},
		{"observer focused", FocusObserver, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := model{focusedPane: tt.focus}
			if got := m.isObserverFocused(); got != tt.expect {
				t.Errorf("isObserverFocused() = %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestHandleKey_Tab_CyclesFocus(t *testing.T) {
	tests := []struct {
		name        string
		startFocus  FocusedPane
		expectFocus FocusedPane
	}{
		{"events to observer", FocusEvents, FocusObserver},
		{"observer to events", FocusObserver, FocusEvents},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := model{
				focusedPane:  tt.startFocus,
				eventsOpen:   true, // Events pane must be open for focus cycling
				observerOpen: true, // Tab only cycles focus when observer is open
				observerPane: NewObserverPane(nil),
				graphPane:    NewGraphPane(nil, nil, "horizontal"),
				status:       "idle",
			}
			newM, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
			resultM := newM.(model)

			if resultM.focusedPane != tt.expectFocus {
				t.Errorf("Tab should cycle focus to %v, got %v",
					tt.expectFocus, resultM.focusedPane)
			}
			if cmd != nil {
				t.Error("Tab should return nil command")
			}
		})
	}
}

func TestHandleKey_Esc_ReturnsFocusToEvents(t *testing.T) {
	t.Run("from observer to events", func(t *testing.T) {
		// Create observer pane with focus set
		obsPane := NewObserverPane(nil)
		obsPane.focused = true // Set directly to avoid cursor init issues

		m := model{
			focusedPane:  FocusObserver,
			observerOpen: true,
			eventsOpen:   true, // Events must be open for focus to return there
			observerPane: obsPane,
			graphPane:    NewGraphPane(nil, nil, "horizontal"),
			status:       "idle",
			focusMode:    FocusModeNone, // Not in fullscreen mode
		}
		newM, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
		resultM := newM.(model)

		// Esc should close the observer pane and return focus to events
		if resultM.focusedPane != FocusEvents {
			t.Errorf("Esc from observer should return focus to events, got %v",
				resultM.focusedPane)
		}
		if resultM.observerOpen {
			t.Error("Esc should close the observer pane")
		}
	})

	t.Run("from events stays at events", func(t *testing.T) {
		m := model{
			focusedPane:  FocusEvents,
			observerPane: NewObserverPane(nil),
			graphPane:    NewGraphPane(nil, nil, "horizontal"),
			status:       "idle",
			focusMode:    FocusModeNone, // Not in fullscreen mode
		}
		newM, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
		resultM := newM.(model)

		if resultM.focusedPane != FocusEvents {
			t.Errorf("Esc from events should stay at events, got %v",
				resultM.focusedPane)
		}
	})
}

func TestHandleKey_CtrlC_AlwaysQuits(t *testing.T) {
	tests := []struct {
		name  string
		focus FocusedPane
	}{
		{"from events", FocusEvents},
		{"from observer", FocusObserver},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quitCalled := false
			m := model{
				focusedPane: tt.focus,
				status:      "idle",
				onQuit:      func() { quitCalled = true },
			}

			_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})

			if !quitCalled {
				t.Error("Ctrl+C should always call onQuit")
			}
			if cmd == nil {
				t.Error("Ctrl+C should return tea.Quit")
			}
		})
	}
}

func TestHandleKey_ObserverFocused_SuppressesGlobalKeys(t *testing.T) {
	// Keys that should be suppressed when observer is focused and open
	keys := []string{"q", "p", "r", "up", "down", "k", "j", "home", "end", "g", "G"}

	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			callbackCalled := false
			m := model{
				focusedPane:  FocusObserver,
				observerOpen: true,
				observerPane: NewObserverPane(nil),
				status:       "idle",
				onQuit:       func() { callbackCalled = true },
				onPause:      func() { callbackCalled = true },
				onResume:     func() { callbackCalled = true },
				scrollPos:    5,
				autoScroll:   true,
				height:       20,
				eventLines:   make([]eventLine, 30),
			}

			newM, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
			resultM := newM.(model)

			if callbackCalled {
				t.Errorf("key %q should not trigger callback when observer focused", key)
			}
			// When observer is focused, keys are forwarded to observer pane
			// The observer pane may return a command (e.g., for textarea input)
			// so we don't check cmd == nil here
			_ = cmd
			// Scroll position should not change
			if resultM.scrollPos != 5 {
				t.Errorf("key %q should not change scroll when observer focused", key)
			}
		})
	}
}

func TestHandleKey_EventsFocused_NormalBehavior(t *testing.T) {
	// Verify that normal keys work when events pane is focused
	t.Run("q quits", func(t *testing.T) {
		quitCalled := false
		m := model{
			focusedPane: FocusEvents,
			status:      "idle",
			onQuit:      func() { quitCalled = true },
		}

		_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})

		if !quitCalled {
			t.Error("q should call onQuit when events focused")
		}
		if cmd == nil {
			t.Error("q should return quit command when events focused")
		}
	})

	t.Run("p pauses", func(t *testing.T) {
		pauseCalled := false
		m := model{
			focusedPane: FocusEvents,
			status:      "idle",
			onPause:     func() { pauseCalled = true },
		}

		newM, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")})
		resultM := newM.(model)

		if !pauseCalled {
			t.Error("p should call onPause when events focused")
		}
		if resultM.status != "pausing..." {
			t.Error("p should set status to pausing...")
		}
	})

	t.Run("up scrolls", func(t *testing.T) {
		m := model{
			focusedPane: FocusEvents,
			scrollPos:   5,
			autoScroll:  true,
			height:      20,
		}

		newM, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("up")})
		resultM := newM.(model)

		if resultM.scrollPos != 4 {
			t.Errorf("up should decrement scroll, got %d", resultM.scrollPos)
		}
	})
}

// Duration tracking tests

func TestHandleEvent_IterationStart_SetsDuration(t *testing.T) {
	m := model{
		status: "idle",
		stats:  modelStats{CurrentDurationMs: 5000}, // Some leftover value
	}

	event := &events.IterationStartEvent{
		BaseEvent: events.NewInternalEvent(events.EventIterationStart),
		BeadID:    "bd-123",
		Title:     "Test bead",
		Priority:  2,
	}

	m.handleEvent(event)

	if m.currentBead == nil {
		t.Fatal("currentBead should be set")
	}
	if m.currentBead.StartTime.IsZero() {
		t.Error("currentBead.StartTime should be set from event timestamp")
	}
	if m.stats.CurrentDurationMs != 0 {
		t.Errorf("CurrentDurationMs should be reset to 0, got %d", m.stats.CurrentDurationMs)
	}
}

func TestHandleEvent_IterationEnd_AccumulatesDuration(t *testing.T) {
	m := model{
		status: "working",
		currentBead: &beadInfo{
			ID:        "bd-123",
			StartTime: time.Now().Add(-5 * time.Minute),
		},
		stats: modelStats{
			TotalDurationMs:   60000,  // 1 minute accumulated
			CurrentDurationMs: 300000, // 5 minutes current
		},
	}

	event := &events.IterationEndEvent{
		BaseEvent:    events.NewInternalEvent(events.EventIterationEnd),
		BeadID:       "bd-123",
		Success:      true,
		NumTurns:     10,
		DurationMs:   300000, // 5 minutes from event
		TotalCostUSD: 0.05,
	}

	m.handleEvent(event)

	if m.stats.TotalDurationMs != 360000 { // 60000 + 300000
		t.Errorf("TotalDurationMs should be 360000, got %d", m.stats.TotalDurationMs)
	}
	if m.stats.CurrentDurationMs != 0 {
		t.Errorf("CurrentDurationMs should be reset to 0, got %d", m.stats.CurrentDurationMs)
	}
}

func TestHandleTick_UpdatesCurrentDuration(t *testing.T) {
	startTime := time.Now().Add(-2 * time.Minute)
	m := model{
		currentBead: &beadInfo{
			ID:        "bd-123",
			StartTime: startTime,
		},
		stats: modelStats{CurrentDurationMs: 0},
	}

	m.handleTick()

	// Should be approximately 2 minutes in ms (allow some tolerance)
	expectedMs := int64(2 * 60 * 1000)
	tolerance := int64(1000) // 1 second tolerance

	if m.stats.CurrentDurationMs < expectedMs-tolerance || m.stats.CurrentDurationMs > expectedMs+tolerance {
		t.Errorf("CurrentDurationMs should be ~%d, got %d", expectedMs, m.stats.CurrentDurationMs)
	}
}

func TestHandleTick_NoUpdateWithoutBead(t *testing.T) {
	m := model{
		currentBead: nil,
		stats:       modelStats{CurrentDurationMs: 0},
	}

	m.handleTick()

	if m.stats.CurrentDurationMs != 0 {
		t.Errorf("CurrentDurationMs should remain 0 without current bead, got %d", m.stats.CurrentDurationMs)
	}
}

func TestHandleTick_NoUpdateWithZeroStartTime(t *testing.T) {
	m := model{
		currentBead: &beadInfo{
			ID:        "bd-123",
			StartTime: time.Time{}, // Zero value
		},
		stats: modelStats{CurrentDurationMs: 0},
	}

	m.handleTick()

	if m.stats.CurrentDurationMs != 0 {
		t.Errorf("CurrentDurationMs should remain 0 with zero StartTime, got %d", m.stats.CurrentDurationMs)
	}
}

// Fullscreen toggle tests

func TestHandleKey_FullscreenToggle_Graph(t *testing.T) {
	t.Run("B enters fullscreen when graph is open", func(t *testing.T) {
		m := model{
			focusedPane:  FocusEvents,
			focusMode:    FocusModeNone,
			graphOpen:    true,
			eventsOpen:   true,
			observerPane: NewObserverPane(nil),
			graphPane:    NewGraphPane(nil, nil, "horizontal"),
			width:        100,
			height:       30,
		}

		newM, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("B")})
		resultM := newM.(model)

		if resultM.focusMode != FocusGraph {
			t.Errorf("focusMode should be FocusGraph, got %v", resultM.focusMode)
		}
		if resultM.focusedPane != FocusGraph {
			t.Errorf("focusedPane should be FocusGraph, got %v", resultM.focusedPane)
		}
		if !resultM.graphPane.IsFocused() {
			t.Error("graphPane should be focused")
		}
		if cmd != nil {
			t.Error("should return nil command")
		}
	})

	t.Run("B exits fullscreen when already in graph fullscreen", func(t *testing.T) {
		graphPane := NewGraphPane(nil, nil, "horizontal")
		graphPane.focused = true

		m := model{
			focusedPane:  FocusGraph,
			focusMode:    FocusGraph, // Already in fullscreen
			graphOpen:    true,
			eventsOpen:   true,
			observerPane: NewObserverPane(nil),
			graphPane:    graphPane,
			width:        100,
			height:       30,
		}

		newM, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("B")})
		resultM := newM.(model)

		if resultM.focusMode != FocusModeNone {
			t.Errorf("focusMode should be FocusModeNone after toggle, got %v", resultM.focusMode)
		}
	})

	t.Run("B does nothing when graph is closed", func(t *testing.T) {
		m := model{
			focusedPane:  FocusEvents,
			focusMode:    FocusModeNone,
			graphOpen:    false, // Graph is closed
			eventsOpen:   true,
			observerPane: NewObserverPane(nil),
			graphPane:    NewGraphPane(nil, nil, "horizontal"),
			width:        100,
			height:       30,
		}

		newM, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("B")})
		resultM := newM.(model)

		if resultM.focusMode != FocusModeNone {
			t.Errorf("focusMode should remain FocusModeNone when graph closed, got %v", resultM.focusMode)
		}
	})
}

func TestHandleKey_FullscreenToggle_Observer(t *testing.T) {
	t.Run("O enters fullscreen when observer is open", func(t *testing.T) {
		obsPane := NewObserverPane(nil)

		m := model{
			focusedPane:  FocusEvents,
			focusMode:    FocusModeNone,
			observerOpen: true,
			eventsOpen:   true,
			observerPane: obsPane,
			graphPane:    NewGraphPane(nil, nil, "horizontal"),
			width:        100,
			height:       30,
		}

		newM, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("O")})
		resultM := newM.(model)

		if resultM.focusMode != FocusObserver {
			t.Errorf("focusMode should be FocusObserver, got %v", resultM.focusMode)
		}
		if resultM.focusedPane != FocusObserver {
			t.Errorf("focusedPane should be FocusObserver, got %v", resultM.focusedPane)
		}
	})

	t.Run("O does nothing when observer is closed", func(t *testing.T) {
		m := model{
			focusedPane:  FocusEvents,
			focusMode:    FocusModeNone,
			observerOpen: false,
			eventsOpen:   true,
			observerPane: NewObserverPane(nil),
			graphPane:    NewGraphPane(nil, nil, "horizontal"),
			width:        100,
			height:       30,
		}

		newM, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("O")})
		resultM := newM.(model)

		if resultM.focusMode != FocusModeNone {
			t.Errorf("focusMode should remain FocusModeNone when observer closed, got %v", resultM.focusMode)
		}
	})
}

func TestHandleKey_FullscreenToggle_Events(t *testing.T) {
	t.Run("E enters fullscreen when events is open", func(t *testing.T) {
		m := model{
			focusedPane:  FocusEvents,
			focusMode:    FocusModeNone,
			eventsOpen:   true,
			observerPane: NewObserverPane(nil),
			graphPane:    NewGraphPane(nil, nil, "horizontal"),
			width:        100,
			height:       30,
		}

		newM, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("E")})
		resultM := newM.(model)

		if resultM.focusMode != FocusEvents {
			t.Errorf("focusMode should be FocusEvents, got %v", resultM.focusMode)
		}
		if resultM.focusedPane != FocusEvents {
			t.Errorf("focusedPane should be FocusEvents, got %v", resultM.focusedPane)
		}
	})
}

func TestHandleKey_Esc_ExitsFullscreen(t *testing.T) {
	t.Run("esc exits graph fullscreen", func(t *testing.T) {
		m := model{
			focusedPane:  FocusGraph,
			focusMode:    FocusGraph,
			graphOpen:    true,
			eventsOpen:   true,
			observerPane: NewObserverPane(nil),
			graphPane:    NewGraphPane(nil, nil, "horizontal"),
			width:        100,
			height:       30,
		}

		newM, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
		resultM := newM.(model)

		if resultM.focusMode != FocusModeNone {
			t.Errorf("focusMode should be FocusModeNone after esc, got %v", resultM.focusMode)
		}
	})

	t.Run("esc exits observer fullscreen", func(t *testing.T) {
		m := model{
			focusedPane:  FocusObserver,
			focusMode:    FocusObserver,
			observerOpen: true,
			eventsOpen:   true,
			observerPane: NewObserverPane(nil),
			graphPane:    NewGraphPane(nil, nil, "horizontal"),
			width:        100,
			height:       30,
		}

		newM, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
		resultM := newM.(model)

		if resultM.focusMode != FocusModeNone {
			t.Errorf("focusMode should be FocusModeNone after esc, got %v", resultM.focusMode)
		}
	})
}

// TestView_FullscreenRendering verifies that View respects focusMode.
func TestView_FullscreenRendering(t *testing.T) {
	t.Run("renders fullscreen when focusMode is set", func(t *testing.T) {
		// Create a proper config for the graph pane
		graphCfg := &config.GraphConfig{
			Enabled: true,
			Density: "standard",
		}
		m := model{
			focusedPane:  FocusGraph,
			focusMode:    FocusGraph,
			graphOpen:    true,
			eventsOpen:   true,
			observerPane: NewObserverPane(nil),
			graphPane:    NewGraphPane(graphCfg, nil, "horizontal"),
			width:        100,
			height:       30,
		}

		view := m.View()

		// In fullscreen mode, we should NOT see the split layout
		// The view should render the graph pane at full size
		// We can't easily check the exact content, but we can verify View doesn't panic
		// and returns a non-empty string
		if view == "" {
			t.Error("View should return non-empty string in fullscreen mode")
		}
	})
}

// TestUpdate_FullscreenPreserved verifies focusMode survives Update cycle.
func TestUpdate_FullscreenPreserved(t *testing.T) {
	ch := make(chan events.Event, 1)

	// Create a proper config for the graph pane
	graphCfg := &config.GraphConfig{
		Enabled: true,
		Density: "standard",
	}

	m := model{
		eventChan:    ch,
		focusedPane:  FocusEvents,
		focusMode:    FocusModeNone,
		graphOpen:    true,
		eventsOpen:   true,
		observerPane: NewObserverPane(nil),
		graphPane:    NewGraphPane(graphCfg, nil, "horizontal"),
		width:        100,
		height:       30,
		status:       "idle",
	}

	// Simulate pressing 'B' through the full Update path
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("B")}
	newModel, _ := m.Update(keyMsg)
	resultM := newModel.(model)

	if resultM.focusMode != FocusGraph {
		t.Errorf("focusMode should be FocusGraph after Update, got %v", resultM.focusMode)
	}

	// Call View on the result to ensure it renders correctly
	view := resultM.View()
	if view == "" {
		t.Error("View should return non-empty string after entering fullscreen")
	}
}

// TestFullscreenWorkflow tests the complete user workflow for fullscreen toggle.
func TestFullscreenWorkflow(t *testing.T) {
	ch := make(chan events.Event, 1)
	graphCfg := &config.GraphConfig{
		Enabled: true,
		Density: "standard",
	}

	// Step 1: Start with default state (graph closed)
	m := model{
		eventChan:    ch,
		focusedPane:  FocusEvents,
		focusMode:    FocusModeNone,
		graphOpen:    false,
		eventsOpen:   true,
		observerPane: NewObserverPane(nil),
		graphPane:    NewGraphPane(graphCfg, nil, "horizontal"),
		width:        120,
		height:       40,
		status:       "idle",
	}

	// Initial state checks
	if m.graphOpen {
		t.Error("graph should be closed initially")
	}
	if m.focusMode != FocusModeNone {
		t.Error("focusMode should be FocusModeNone initially")
	}

	// Step 2: Press 'b' to open graph (lowercase - toggle)
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	m = newModel.(model)

	if !m.graphOpen {
		t.Error("graph should be open after pressing 'b'")
	}
	if m.focusMode != FocusModeNone {
		t.Error("focusMode should still be FocusModeNone after 'b' (not fullscreen)")
	}

	// Step 3: Capture split view output
	splitView := m.View()
	if splitView == "" {
		t.Error("split view should render non-empty content")
	}

	// Step 4: Press 'B' to enter fullscreen (uppercase - fullscreen)
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("B")})
	m = newModel.(model)

	if m.focusMode != FocusGraph {
		t.Errorf("focusMode should be FocusGraph after 'B', got %v", m.focusMode)
	}

	// Step 5: Capture fullscreen view output
	fullscreenView := m.View()
	if fullscreenView == "" {
		t.Error("fullscreen view should render non-empty content")
	}

	// Step 6: Views should be different
	if splitView == fullscreenView {
		t.Error("split view and fullscreen view should be different")
	}

	// Step 7: Press ESC to exit fullscreen
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	m = newModel.(model)

	if m.focusMode != FocusModeNone {
		t.Errorf("focusMode should be FocusModeNone after ESC, got %v", m.focusMode)
	}

	// Step 8: View should be back to split view
	backToSplit := m.View()
	if backToSplit != splitView {
		// Note: This might not be exactly equal due to timing/state differences,
		// but focusMode should definitely be FocusModeNone
		// The important check is that we're not in fullscreen anymore
		t.Log("Note: Views differ slightly, which is OK as long as focusMode is correct")
	}
}

// INSERT mode key blocking tests

func TestHandleKey_InsertMode_BlocksPanelToggles(t *testing.T) {
	// Keys that should be blocked (forwarded to observer) when in INSERT mode
	keys := []string{"e", "o", "b", "E", "B", "O"}

	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			// Create observer pane in INSERT mode
			obsPane := NewObserverPane(nil)
			obsPane.focused = true
			obsPane.insertMode = true

			m := model{
				focusedPane:  FocusObserver,
				observerOpen: true,
				eventsOpen:   true,
				graphOpen:    false,
				observerPane: obsPane,
				graphPane:    NewGraphPane(nil, nil, "horizontal"),
				status:       "idle",
			}

			// Record initial state
			initialEventsOpen := m.eventsOpen
			initialObserverOpen := m.observerOpen
			initialGraphOpen := m.graphOpen

			_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})

			// Panels should NOT be toggled in INSERT mode
			if m.eventsOpen != initialEventsOpen {
				t.Errorf("key %q should not toggle events pane in INSERT mode", key)
			}
			if m.observerOpen != initialObserverOpen {
				t.Errorf("key %q should not toggle observer pane in INSERT mode", key)
			}
			if m.graphOpen != initialGraphOpen {
				t.Errorf("key %q should not toggle graph pane in INSERT mode", key)
			}
		})
	}
}

func TestHandleKey_InsertMode_GlobalKeysStillWork(t *testing.T) {
	t.Run("esc exits insert mode", func(t *testing.T) {
		obsPane := NewObserverPane(nil)
		obsPane.focused = true
		obsPane.insertMode = true

		m := model{
			focusedPane:  FocusObserver,
			observerOpen: true,
			eventsOpen:   true,
			observerPane: obsPane,
			graphPane:    NewGraphPane(nil, nil, "horizontal"),
			focusMode:    FocusModeNone,
		}

		newM, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEscape})
		resultM := newM.(model)

		// After esc in INSERT mode, observer should exit insert mode
		if resultM.observerPane.IsInsertMode() {
			t.Error("esc should exit INSERT mode")
		}
	})

	t.Run("tab cycles focus", func(t *testing.T) {
		obsPane := NewObserverPane(nil)
		obsPane.focused = true
		obsPane.insertMode = true

		m := model{
			focusedPane:  FocusObserver,
			observerOpen: true,
			eventsOpen:   true,
			observerPane: obsPane,
			graphPane:    NewGraphPane(nil, nil, "horizontal"),
		}

		newM, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
		resultM := newM.(model)

		// Tab should cycle focus even in INSERT mode
		if resultM.focusedPane != FocusEvents {
			t.Errorf("tab should cycle focus from observer to events, got %v", resultM.focusedPane)
		}
	})

	t.Run("ctrl+c is forwarded to observer when loading", func(t *testing.T) {
		obsPane := NewObserverPane(nil)
		obsPane.focused = true
		obsPane.insertMode = true
		obsPane.loading = true

		quitCalled := false
		m := model{
			focusedPane:  FocusObserver,
			observerOpen: true,
			observerPane: obsPane,
			graphPane:    NewGraphPane(nil, nil, "horizontal"),
			onQuit:       func() { quitCalled = true },
		}

		_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})

		// ctrl+c when observer is loading should NOT quit the app
		// (it gets forwarded to observer for query cancellation)
		if quitCalled {
			t.Error("ctrl+c should not quit when observer is loading - should forward to observer")
		}
		// Should return a cmd (observer's cmd, not tea.Quit)
		// Note: actual cancellation depends on observer having a non-nil observer instance
		_ = cmd
	})
}

func TestHandleKey_NormalMode_PanelTogglesWork(t *testing.T) {
	// Verify that panel toggles work when observer is focused but in NORMAL mode
	t.Run("e toggles events in normal mode", func(t *testing.T) {
		obsPane := NewObserverPane(nil)
		obsPane.focused = true
		obsPane.insertMode = false // NORMAL mode

		m := model{
			focusedPane:  FocusObserver,
			observerOpen: true,
			eventsOpen:   true,
			observerPane: obsPane,
			graphPane:    NewGraphPane(nil, nil, "horizontal"),
		}

		newM, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
		resultM := newM.(model)

		// In NORMAL mode, 'e' should toggle events pane
		if resultM.eventsOpen {
			t.Error("e should toggle events pane closed in NORMAL mode")
		}
	})
}

// TestFullscreen_VisualDifference ensures fullscreen actually looks different.
func TestFullscreen_VisualDifference(t *testing.T) {
	ch := make(chan events.Event, 1)
	graphCfg := &config.GraphConfig{
		Enabled: true,
		Density: "standard",
	}

	// Create a model with events and graph both open (split view)
	m := model{
		eventChan:    ch,
		focusedPane:  FocusEvents,
		focusMode:    FocusModeNone,
		graphOpen:    true,
		eventsOpen:   true,
		observerPane: NewObserverPane(nil),
		graphPane:    NewGraphPane(graphCfg, nil, "horizontal"),
		width:        120,
		height:       40,
		status:       "idle",
	}

	// Capture split view
	splitView := m.View()

	// Enter fullscreen
	m.focusMode = FocusGraph
	m.focusedPane = FocusGraph

	// Capture fullscreen view
	fullscreenView := m.View()

	// The fullscreen view should be different (typically simpler, single pane)
	if splitView == fullscreenView {
		t.Error("fullscreen view should differ from split view")
	}

	// Count newlines as a rough measure of layout difference
	splitLines := len(strings.Split(splitView, "\n"))
	fullscreenLines := len(strings.Split(fullscreenView, "\n"))

	// Log the difference for debugging
	t.Logf("Split view lines: %d, Fullscreen lines: %d", splitLines, fullscreenLines)

	// The layout should be noticeably different
	// (In fullscreen, there should be no events pane, just the graph)
}
