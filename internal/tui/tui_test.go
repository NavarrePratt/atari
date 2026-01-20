package tui

import (
	"strings"
	"testing"

	"github.com/npratt/atari/internal/events"
	"github.com/npratt/atari/internal/observer"
	"github.com/npratt/atari/internal/testutil"
)

// Note: mockStatsGetter is defined in update_test.go

func TestNew_AppliesOptions(t *testing.T) {
	eventChan := make(chan events.Event)
	pauseCalled := false
	resumeCalled := false
	quitCalled := false

	tui := New(eventChan,
		WithOnPause(func() { pauseCalled = true }),
		WithOnResume(func() { resumeCalled = true }),
		WithOnQuit(func() { quitCalled = true }),
	)

	if tui.eventChan != eventChan {
		t.Error("eventChan not set")
	}
	if tui.onPause == nil {
		t.Error("onPause not set")
	}
	if tui.onResume == nil {
		t.Error("onResume not set")
	}
	if tui.onQuit == nil {
		t.Error("onQuit not set")
	}

	// Verify callbacks work
	tui.onPause()
	tui.onResume()
	tui.onQuit()

	if !pauseCalled {
		t.Error("onPause callback not invoked")
	}
	if !resumeCalled {
		t.Error("onResume callback not invoked")
	}
	if !quitCalled {
		t.Error("onQuit callback not invoked")
	}
}

func TestNew_WithStatsGetter(t *testing.T) {
	eventChan := make(chan events.Event)
	stats := &mockStatsGetter{}

	tui := New(eventChan, WithStatsGetter(stats))

	if tui.statsGetter != stats {
		t.Error("statsGetter not set")
	}
}

func TestNew_WithGraphFetcher(t *testing.T) {
	eventChan := make(chan events.Event)
	runner := testutil.NewMockRunner()
	fetcher := NewBDFetcher(runner)

	tui := New(eventChan, WithGraphFetcher(fetcher))

	if tui.graphFetcher != fetcher {
		t.Error("graphFetcher not set")
	}
}

func TestNew_WithObserver(t *testing.T) {
	eventChan := make(chan events.Event)
	// Create a minimal observer (nil dependencies are ok for this test)
	obs := observer.NewObserver(nil, nil, nil, nil)

	tui := New(eventChan, WithObserver(obs))

	if tui.observer != obs {
		t.Error("observer not set")
	}
}

func TestNew_WithEpicID(t *testing.T) {
	eventChan := make(chan events.Event)

	tui := New(eventChan, WithEpicID("bd-epic-123"))

	if tui.epicID != "bd-epic-123" {
		t.Errorf("expected epicID 'bd-epic-123', got %q", tui.epicID)
	}
}

func TestNewModel_EpicIDWired(t *testing.T) {
	eventChan := make(chan events.Event)

	m := newModel(eventChan, nil, nil, nil, nil, nil, nil, nil, "bd-epic-456")

	if m.epicID != "bd-epic-456" {
		t.Errorf("expected model epicID 'bd-epic-456', got %q", m.epicID)
	}
}

func TestRenderStatus_WithEpic(t *testing.T) {
	eventChan := make(chan events.Event)
	m := newModel(eventChan, nil, nil, nil, nil, nil, nil, nil, "bd-test-epic")
	m.status = "idle"

	status := m.renderStatus()

	// Should contain the epic suffix
	if !strings.Contains(status, "(epic: bd-test-epic)") {
		t.Errorf("expected status to contain epic suffix, got %q", status)
	}
}

func TestRenderStatus_WithoutEpic(t *testing.T) {
	eventChan := make(chan events.Event)
	m := newModel(eventChan, nil, nil, nil, nil, nil, nil, nil, "")
	m.status = "idle"

	status := m.renderStatus()

	// Should NOT contain epic suffix
	if strings.Contains(status, "(epic:") {
		t.Errorf("expected status without epic suffix, got %q", status)
	}
}

// TestNewModel_GraphPaneHasFetcher verifies the model passes the fetcher to GraphPane.
// This is the test that would have caught the missing WithGraphFetcher bug.
func TestNewModel_GraphPaneHasFetcher(t *testing.T) {
	eventChan := make(chan events.Event)
	runner := testutil.NewMockRunner()
	fetcher := NewBDFetcher(runner)

	m := newModel(eventChan, nil, nil, nil, nil, nil, fetcher, nil, "")

	// The graph pane should have the fetcher
	if m.graphPane.fetcher == nil {
		t.Error("graphPane.fetcher is nil - fetcher not wired to graph pane")
	}
	if m.graphPane.fetcher != fetcher {
		t.Error("graphPane.fetcher doesn't match provided fetcher")
	}
}

// TestNewModel_GraphPaneNilFetcher verifies behavior when no fetcher is provided.
func TestNewModel_GraphPaneNilFetcher(t *testing.T) {
	eventChan := make(chan events.Event)

	m := newModel(eventChan, nil, nil, nil, nil, nil, nil, nil, "")

	// With nil fetcher, graph pane should still be created but fetcher is nil
	if m.graphPane.fetcher != nil {
		t.Error("expected graphPane.fetcher to be nil when no fetcher provided")
	}
}

// TestNewModel_DetailModalHasFetcher verifies the detail modal receives the fetcher.
func TestNewModel_DetailModalHasFetcher(t *testing.T) {
	eventChan := make(chan events.Event)
	runner := testutil.NewMockRunner()
	fetcher := NewBDFetcher(runner)

	m := newModel(eventChan, nil, nil, nil, nil, nil, fetcher, nil, "")

	if m.detailModal == nil {
		t.Fatal("detailModal is nil")
	}
	if m.detailModal.fetcher == nil {
		t.Error("detailModal.fetcher is nil - fetcher not wired to detail modal")
	}
}

// TestNewModel_CallbacksWired verifies callbacks are passed to model.
func TestNewModel_CallbacksWired(t *testing.T) {
	eventChan := make(chan events.Event)
	pauseCalled := false
	resumeCalled := false
	quitCalled := false

	m := newModel(
		eventChan,
		func() { pauseCalled = true },
		func() { resumeCalled = true },
		func() { quitCalled = true },
		nil, nil, nil, nil, "",
	)

	if m.onPause == nil {
		t.Error("onPause not wired to model")
	}
	if m.onResume == nil {
		t.Error("onResume not wired to model")
	}
	if m.onQuit == nil {
		t.Error("onQuit not wired to model")
	}

	// Verify they're the right callbacks
	m.onPause()
	m.onResume()
	m.onQuit()

	if !pauseCalled || !resumeCalled || !quitCalled {
		t.Error("callbacks not correctly wired")
	}
}

// TestNewModel_DefaultState verifies the model starts with correct defaults.
func TestNewModel_DefaultState(t *testing.T) {
	eventChan := make(chan events.Event)

	m := newModel(eventChan, nil, nil, nil, nil, nil, nil, nil, "")

	if m.status != "idle" {
		t.Errorf("expected status 'idle', got %q", m.status)
	}
	if !m.autoScroll {
		t.Error("autoScroll should be true by default")
	}
	if !m.eventsOpen {
		t.Error("eventsOpen should be true by default")
	}
	if m.graphOpen {
		t.Error("graphOpen should be false by default")
	}
	if m.observerOpen {
		t.Error("observerOpen should be false by default")
	}
}

// TestNewModel_ObserverPaneCreated verifies observer pane is always created.
func TestNewModel_ObserverPaneCreated(t *testing.T) {
	eventChan := make(chan events.Event)

	m := newModel(eventChan, nil, nil, nil, nil, nil, nil, nil, "")

	// Observer pane should exist even without an observer
	// (it just won't be functional)
	// This is tested implicitly by the model having observerPane field
	_ = m.observerPane.View() // Should not panic
}

// TestTUI_FullConfiguration verifies that when all options are provided,
// the TUI is fully configured with no nil dependencies.
// This test documents the expected "production" configuration.
func TestTUI_FullConfiguration(t *testing.T) {
	eventChan := make(chan events.Event)
	runner := testutil.NewMockRunner()
	fetcher := NewBDFetcher(runner)
	stats := &mockStatsGetter{}
	obs := observer.NewObserver(nil, nil, nil, nil)

	tui := New(eventChan,
		WithOnPause(func() {}),
		WithOnResume(func() {}),
		WithOnQuit(func() {}),
		WithStatsGetter(stats),
		WithObserver(obs),
		WithGraphFetcher(fetcher),
	)

	// Verify all fields are set (this is what main.go should do)
	if tui.eventChan == nil {
		t.Error("eventChan is nil")
	}
	if tui.onPause == nil {
		t.Error("onPause is nil - main.go should set WithOnPause")
	}
	if tui.onResume == nil {
		t.Error("onResume is nil - main.go should set WithOnResume")
	}
	if tui.onQuit == nil {
		t.Error("onQuit is nil - main.go should set WithOnQuit")
	}
	if tui.statsGetter == nil {
		t.Error("statsGetter is nil - main.go should set WithStatsGetter")
	}
	// Note: observer can be nil if observer mode is disabled
	if tui.graphFetcher == nil {
		t.Error("graphFetcher is nil - main.go should set WithGraphFetcher")
	}
}

// TestNewModel_FullConfiguration verifies model is properly initialized
// when all dependencies are provided.
func TestNewModel_FullConfiguration(t *testing.T) {
	eventChan := make(chan events.Event)
	runner := testutil.NewMockRunner()
	fetcher := NewBDFetcher(runner)
	stats := &mockStatsGetter{}
	obs := observer.NewObserver(nil, nil, nil, nil)

	m := newModel(
		eventChan,
		func() {}, // onPause
		func() {}, // onResume
		func() {}, // onQuit
		stats,
		obs,
		fetcher,
		nil, // beadStateGetter
		"",  // epicID
	)

	// Verify model state
	if m.eventChan == nil {
		t.Error("eventChan not set in model")
	}
	if m.onPause == nil {
		t.Error("onPause not set in model")
	}
	if m.onResume == nil {
		t.Error("onResume not set in model")
	}
	if m.onQuit == nil {
		t.Error("onQuit not set in model")
	}
	if m.statsGetter == nil {
		t.Error("statsGetter not set in model")
	}

	// Verify sub-components are wired
	if m.graphPane.fetcher == nil {
		t.Error("graphPane.fetcher not wired")
	}
	if m.detailModal == nil {
		t.Error("detailModal not created")
	}
	if m.detailModal.fetcher == nil {
		t.Error("detailModal.fetcher not wired")
	}
}

// TestNewModel_GraphPaneAutoRefreshEnabled verifies the graph pane is created
// with auto-refresh enabled by default. This was a regression where the config
// was created without AutoRefreshInterval, causing auto-refresh to silently fail.
func TestNewModel_GraphPaneAutoRefreshEnabled(t *testing.T) {
	eventChan := make(chan events.Event)

	m := newModel(eventChan, nil, nil, nil, nil, nil, nil, nil, "")

	// The graphPane's autoRefreshCmd should return a non-nil command
	// when auto-refresh is enabled (interval > 0)
	cmd := m.graphPane.autoRefreshCmd()
	if cmd == nil {
		t.Error("graphPane auto-refresh is disabled; newModel should configure AutoRefreshInterval > 0")
	}
}
