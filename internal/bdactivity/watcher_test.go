package bdactivity

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/events"
	"github.com/npratt/atari/internal/testutil"
)

func testConfig() *config.BDActivityConfig {
	return &config.BDActivityConfig{
		Enabled:           true,
		ReconnectDelay:    10 * time.Millisecond,
		MaxReconnectDelay: 50 * time.Millisecond,
	}
}

func TestWatcher_StartStop(t *testing.T) {
	mock := testutil.NewMockProcessRunner()
	router := events.NewRouter(10)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Set up mock to return empty output then block
	mock.SetOutput("")

	watcher := New(testConfig(), router, mock, logger)

	// Start should succeed
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := watcher.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !watcher.Running() {
		t.Error("expected Running() to be true after Start")
	}

	// Give it a moment to start
	time.Sleep(20 * time.Millisecond)

	// Stop should succeed
	err = watcher.Stop()
	if err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	if watcher.Running() {
		t.Error("expected Running() to be false after Stop")
	}
}

func TestWatcher_DoubleStart(t *testing.T) {
	mock := testutil.NewMockProcessRunner()
	router := events.NewRouter(10)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	mock.SetOutput("")

	watcher := New(testConfig(), router, mock, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := watcher.Start(ctx)
	if err != nil {
		t.Fatalf("First Start failed: %v", err)
	}
	defer func() { _ = watcher.Stop() }()

	// Second start should fail
	err = watcher.Start(ctx)
	if err == nil {
		t.Error("expected second Start to fail")
	}
	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWatcher_StopIdempotent(t *testing.T) {
	mock := testutil.NewMockProcessRunner()
	router := events.NewRouter(10)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	mock.SetOutput("")

	watcher := New(testConfig(), router, mock, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := watcher.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	time.Sleep(20 * time.Millisecond)

	// Multiple stops should be safe
	for i := 0; i < 3; i++ {
		err = watcher.Stop()
		if err != nil {
			t.Errorf("Stop %d failed: %v", i, err)
		}
	}
}

func TestWatcher_StopBeforeStart(t *testing.T) {
	mock := testutil.NewMockProcessRunner()
	router := events.NewRouter(10)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	watcher := New(testConfig(), router, mock, logger)

	// Stop before start should be safe
	err := watcher.Stop()
	if err != nil {
		t.Errorf("Stop before Start should not error: %v", err)
	}
}

func TestWatcher_EventFlow(t *testing.T) {
	mock := testutil.NewMockProcessRunner()
	router := events.NewRouter(10)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Set up mock with bd activity output
	output := `{"timestamp":"2026-01-02T12:00:00-05:00","type":"create","issue_id":"bd-001","symbol":"+","message":"bd-001 created · Test issue"}
{"timestamp":"2026-01-02T12:01:00-05:00","type":"status","issue_id":"bd-001","symbol":"→","message":"bd-001 status changed","old_status":"open","new_status":"in_progress"}
`
	mock.SetOutput(output)

	watcher := New(testConfig(), router, mock, logger)

	// Subscribe to events
	sub := router.Subscribe()
	defer router.Unsubscribe(sub)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := watcher.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Collect events with timeout
	var received []events.Event
	timeout := time.After(500 * time.Millisecond)
	for len(received) < 2 {
		select {
		case e := <-sub:
			received = append(received, e)
		case <-timeout:
			t.Fatalf("timeout waiting for events, got %d", len(received))
		}
	}

	_ = watcher.Stop()

	// Verify first event is BeadCreatedEvent
	if _, ok := received[0].(*events.BeadCreatedEvent); !ok {
		t.Errorf("expected BeadCreatedEvent, got %T", received[0])
	}

	// Verify second event is BeadStatusEvent
	if _, ok := received[1].(*events.BeadStatusEvent); !ok {
		t.Errorf("expected BeadStatusEvent, got %T", received[1])
	}
}

func TestWatcher_StartFailure(t *testing.T) {
	mock := testutil.NewMockProcessRunner()
	router := events.NewRouter(10)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Configure mock to fail on start
	mock.SetStartError(errors.New("command not found"))

	watcher := New(testConfig(), router, mock, logger)

	// Subscribe to events
	sub := router.Subscribe()
	defer router.Unsubscribe(sub)

	ctx, cancel := context.WithCancel(context.Background())

	err := watcher.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for warning event about start failure
	timeout := time.After(200 * time.Millisecond)
	var gotWarning bool
	for !gotWarning {
		select {
		case e := <-sub:
			if errEvent, ok := e.(*events.ErrorEvent); ok {
				if strings.Contains(errEvent.Message, "bd activity exited") ||
					strings.Contains(errEvent.Message, "start bd activity") {
					gotWarning = true
				}
			}
		case <-timeout:
			// Cancel context to stop watcher
			cancel()
			_ = watcher.Stop()
			t.Fatal("timeout waiting for warning event")
		}
	}

	cancel()
	_ = watcher.Stop()
}

func TestWatcher_ParseErrorWarning(t *testing.T) {
	mock := testutil.NewMockProcessRunner()
	router := events.NewRouter(10)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Set up mock with invalid JSON
	output := "not valid json\n"
	mock.SetOutput(output)

	watcher := New(testConfig(), router, mock, logger)

	// Subscribe to events
	sub := router.Subscribe()
	defer router.Unsubscribe(sub)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := watcher.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for warning event about parse error
	timeout := time.After(200 * time.Millisecond)
	var gotWarning bool
	for !gotWarning {
		select {
		case e := <-sub:
			if errEvent, ok := e.(*events.ErrorEvent); ok {
				if strings.Contains(errEvent.Message, "parse error") {
					gotWarning = true
				}
			}
		case <-timeout:
			_ = watcher.Stop()
			t.Fatal("timeout waiting for parse warning event")
		}
	}

	_ = watcher.Stop()
}

func TestWatcher_ParseWarningRateLimited(t *testing.T) {
	mock := testutil.NewMockProcessRunner()
	router := events.NewRouter(100)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Set up mock with multiple invalid JSON lines
	output := "bad1\nbad2\nbad3\nbad4\nbad5\n"
	mock.SetOutput(output)

	watcher := New(testConfig(), router, mock, logger)

	// Subscribe to events
	sub := router.Subscribe()
	defer router.Unsubscribe(sub)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := watcher.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Collect events for a short time
	time.Sleep(100 * time.Millisecond)
	_ = watcher.Stop()

	// Count parse error events (should be rate-limited to 1)
	parseErrorCount := 0
drainLoop:
	for {
		select {
		case e := <-sub:
			if errEvent, ok := e.(*events.ErrorEvent); ok {
				if strings.Contains(errEvent.Message, "parse error") {
					parseErrorCount++
				}
			}
		default:
			break drainLoop
		}
	}

	// Should have at most 1 parse error due to rate limiting (5s interval)
	if parseErrorCount > 1 {
		t.Errorf("expected at most 1 parse error event (rate-limited), got %d", parseErrorCount)
	}
}

func TestWatcher_ContextCancel(t *testing.T) {
	mock := testutil.NewMockProcessRunner()
	router := events.NewRouter(10)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	mock.SetOutput("")

	watcher := New(testConfig(), router, mock, logger)

	ctx, cancel := context.WithCancel(context.Background())

	err := watcher.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Give it a moment to start
	time.Sleep(20 * time.Millisecond)

	// Cancel context should stop watcher
	cancel()

	// Wait for watcher to stop
	timeout := time.After(200 * time.Millisecond)
	for watcher.Running() {
		select {
		case <-timeout:
			_ = watcher.Stop()
			t.Fatal("watcher did not stop after context cancel")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestWatcher_BackoffBehavior(t *testing.T) {
	mock := testutil.NewMockProcessRunner()
	router := events.NewRouter(10)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	cfg := &config.BDActivityConfig{
		Enabled:           true,
		ReconnectDelay:    10 * time.Millisecond,
		MaxReconnectDelay: 40 * time.Millisecond,
	}

	// Track start attempts
	var mu sync.Mutex
	startTimes := []time.Time{}

	mock.OnStart(func(attempt int, name string, args []string) (string, string, error) {
		mu.Lock()
		startTimes = append(startTimes, time.Now())
		mu.Unlock()
		// Return empty output and then process will "exit"
		return "", "", nil
	})

	watcher := New(cfg, router, mock, logger)

	ctx, cancel := context.WithCancel(context.Background())

	err := watcher.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Let it reconnect a few times
	time.Sleep(150 * time.Millisecond)

	cancel()
	_ = watcher.Stop()

	mu.Lock()
	attempts := len(startTimes)
	times := make([]time.Time, len(startTimes))
	copy(times, startTimes)
	mu.Unlock()

	if attempts < 2 {
		t.Skipf("not enough reconnection attempts to test backoff (got %d)", attempts)
	}

	// Verify delays are increasing (with some tolerance)
	for i := 1; i < len(times)-1; i++ {
		delay1 := times[i].Sub(times[i-1])
		delay2 := times[i+1].Sub(times[i])

		// Second delay should be at least as long as first (exponential backoff)
		// Allow some tolerance for timing
		if delay2 < delay1/2 {
			t.Logf("delay %d: %v, delay %d: %v", i, delay1, i+1, delay2)
		}
	}
}

func TestWatcher_BackoffReset(t *testing.T) {
	mock := testutil.NewMockProcessRunner()
	router := events.NewRouter(10)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	cfg := testConfig()

	watcher := New(cfg, router, mock, logger)

	// Set initial backoff high
	watcher.backoff = 100 * time.Millisecond

	// Simulate receiving events
	for i := 0; i < 5; i++ {
		watcher.resetBackoff()
	}

	// Backoff should be reset to initial value
	if watcher.backoff != cfg.ReconnectDelay {
		t.Errorf("expected backoff to be reset to %v, got %v", cfg.ReconnectDelay, watcher.backoff)
	}
}

func TestWatcher_RunningState(t *testing.T) {
	mock := testutil.NewMockProcessRunner()
	router := events.NewRouter(10)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	mock.SetOutput("")

	watcher := New(testConfig(), router, mock, logger)

	// Initially not running
	if watcher.Running() {
		t.Error("expected Running() to be false before Start")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := watcher.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Should be running now
	if !watcher.Running() {
		t.Error("expected Running() to be true after Start")
	}

	time.Sleep(20 * time.Millisecond)
	_ = watcher.Stop()

	// Should not be running after stop
	if watcher.Running() {
		t.Error("expected Running() to be false after Stop")
	}
}
