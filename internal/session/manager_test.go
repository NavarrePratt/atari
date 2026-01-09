package session

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/events"
)

// Tests for LimitedWriter

func TestLimitedWriter_BasicWrite(t *testing.T) {
	w := NewLimitedWriter(100)

	n, err := w.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5 {
		t.Errorf("expected n=5, got %d", n)
	}
	if w.String() != "hello" {
		t.Errorf("expected 'hello', got %q", w.String())
	}
	if w.Len() != 5 {
		t.Errorf("expected len=5, got %d", w.Len())
	}
}

func TestLimitedWriter_MultipleWrites(t *testing.T) {
	w := NewLimitedWriter(100)

	_, _ = w.Write([]byte("hello"))
	_, _ = w.Write([]byte(" "))
	_, _ = w.Write([]byte("world"))

	if w.String() != "hello world" {
		t.Errorf("expected 'hello world', got %q", w.String())
	}
}

func TestLimitedWriter_ExactLimit(t *testing.T) {
	w := NewLimitedWriter(5)

	n, _ := w.Write([]byte("hello"))
	if n != 5 {
		t.Errorf("expected n=5, got %d", n)
	}
	if w.String() != "hello" {
		t.Errorf("expected 'hello', got %q", w.String())
	}
}

func TestLimitedWriter_TruncatesAtLimit(t *testing.T) {
	w := NewLimitedWriter(5)

	n, _ := w.Write([]byte("hello world"))
	// Should report full length even though we only kept 5
	if n != 5 {
		t.Errorf("expected n=5 (truncated), got %d", n)
	}
	if w.String() != "hello" {
		t.Errorf("expected 'hello', got %q", w.String())
	}
}

func TestLimitedWriter_DiscardsAfterLimit(t *testing.T) {
	w := NewLimitedWriter(5)

	_, _ = w.Write([]byte("hello"))
	n, _ := w.Write([]byte(" world"))

	// Reports full length even though discarded
	if n != 6 {
		t.Errorf("expected n=6 (discarded), got %d", n)
	}
	// Buffer unchanged
	if w.String() != "hello" {
		t.Errorf("expected 'hello', got %q", w.String())
	}
	if w.Len() != 5 {
		t.Errorf("expected len=5, got %d", w.Len())
	}
}

func TestLimitedWriter_PartialFill(t *testing.T) {
	w := NewLimitedWriter(10)

	_, _ = w.Write([]byte("hello")) // 5 bytes, 5 remaining
	n, _ := w.Write([]byte("1234567890"))

	// Should only keep first 5 of second write
	if n != 5 {
		t.Errorf("expected n=5 (partial), got %d", n)
	}
	if w.String() != "hello12345" {
		t.Errorf("expected 'hello12345', got %q", w.String())
	}
}

func TestLimitedWriter_BytesReturnsCopy(t *testing.T) {
	w := NewLimitedWriter(100)
	_, _ = w.Write([]byte("hello"))

	b := w.Bytes()
	b[0] = 'X' // Modify the returned slice

	// Original should be unchanged
	if w.String() != "hello" {
		t.Errorf("Bytes() should return a copy, but original was modified")
	}
}

func TestLimitedWriter_Concurrent(t *testing.T) {
	w := NewLimitedWriter(1000)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = w.Write([]byte("test"))
		}()
	}
	wg.Wait()

	// Should have captured up to 1000 bytes
	if w.Len() > 1000 {
		t.Errorf("exceeded max size: %d > 1000", w.Len())
	}
	// Should be a multiple of 4 (or less if truncated)
	if w.Len()%4 != 0 && w.Len() != 1000 {
		t.Errorf("unexpected length: %d", w.Len())
	}
}

// Tests for Manager

func TestNew_CreatesManager(t *testing.T) {
	cfg := config.Default()
	router := events.NewRouter(100)
	defer router.Close()

	m := New(cfg, router)
	if m == nil {
		t.Fatal("expected non-nil Manager")
	}
	if m.config != cfg {
		t.Error("expected config to be set")
	}
	if m.events != router {
		t.Error("expected events router to be set")
	}
	if m.stderr == nil {
		t.Error("expected stderr writer to be initialized")
	}
	if m.done == nil {
		t.Error("expected done channel to be initialized")
	}
}

func TestNew_InitializesLastActive(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, nil)

	before := time.Now()
	last := m.lastActive.Load().(time.Time)

	if last.Before(before.Add(-time.Second)) {
		t.Errorf("lastActive should be recent, got %v", last)
	}
}

func TestManager_UpdateActivity(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, nil)

	// Set to old time
	oldTime := time.Now().Add(-time.Hour)
	m.lastActive.Store(oldTime)

	m.UpdateActivity()

	newTime := m.lastActive.Load().(time.Time)
	if newTime.Before(time.Now().Add(-time.Second)) {
		t.Errorf("UpdateActivity should set recent time, got %v", newTime)
	}
	if !newTime.After(oldTime) {
		t.Errorf("UpdateActivity should update time, old=%v new=%v", oldTime, newTime)
	}
}

func TestManager_Running_BeforeStart(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, nil)

	if m.Running() {
		t.Error("expected Running() to be false before Start()")
	}
}

func TestManager_Start_DoubleStartFails(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, nil)

	// Mark as started without actually starting (to avoid needing claude)
	m.mu.Lock()
	m.started = true
	m.mu.Unlock()

	err := m.Start(context.Background(), "test")
	if err == nil {
		t.Error("expected error on double start")
	}
	if err.Error() != "session already started" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestManager_Stderr(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, nil)

	// Write directly to stderr buffer
	_, _ = m.stderr.Write([]byte("test error"))

	if m.Stderr() != "test error" {
		t.Errorf("expected 'test error', got %q", m.Stderr())
	}
}

func TestDefaultStderrCap(t *testing.T) {
	if DefaultStderrCap != 64*1024 {
		t.Errorf("expected DefaultStderrCap=64KB, got %d", DefaultStderrCap)
	}
}

// Test that watchdog emits timeout event (using short timeout)
func TestManager_WatchdogEmitsTimeout(t *testing.T) {
	cfg := config.Default()
	cfg.Claude.Timeout = 50 * time.Millisecond

	router := events.NewRouter(100)
	defer router.Close()

	sub := router.Subscribe()

	m := New(cfg, router)

	// Set lastActive to old time to trigger immediate timeout
	m.lastActive.Store(time.Now().Add(-time.Hour))

	// Start watchdog directly (simulating started state)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go m.watchdog(ctx)

	// Wait for timeout event
	select {
	case event := <-sub:
		if event.Type() != events.EventSessionTimeout {
			t.Errorf("expected EventSessionTimeout, got %v", event.Type())
		}
		timeout, ok := event.(*events.SessionTimeoutEvent)
		if !ok {
			t.Errorf("expected *SessionTimeoutEvent, got %T", event)
		}
		if timeout.Duration < time.Hour-time.Minute {
			t.Errorf("expected duration ~1h, got %v", timeout.Duration)
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for SessionTimeoutEvent")
	}
}

// Test that watchdog respects context cancellation
func TestManager_WatchdogRespectsContext(t *testing.T) {
	cfg := config.Default()
	cfg.Claude.Timeout = time.Hour // Long timeout

	m := New(cfg, nil)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		m.watchdog(ctx)
		close(done)
	}()

	// Cancel context
	cancel()

	// Watchdog should exit quickly
	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Error("watchdog did not exit after context cancellation")
	}
}

// Test that watchdog respects done channel
func TestManager_WatchdogRespectsDone(t *testing.T) {
	cfg := config.Default()
	cfg.Claude.Timeout = time.Hour // Long timeout

	m := New(cfg, nil)

	ctx := context.Background()

	finished := make(chan struct{})
	go func() {
		m.watchdog(ctx)
		close(finished)
	}()

	// Close done channel
	close(m.done)

	// Watchdog should exit quickly
	select {
	case <-finished:
		// Success
	case <-time.After(time.Second):
		t.Error("watchdog did not exit after done channel closed")
	}
}

// Test graceful pause functionality

func TestManager_PauseRequested_InitiallyFalse(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, nil)

	if m.PauseRequested() {
		t.Error("expected PauseRequested() to be false initially")
	}
}

func TestManager_RequestPause(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, nil)

	if m.PauseRequested() {
		t.Fatal("precondition failed: pause already requested")
	}

	m.RequestPause()

	if !m.PauseRequested() {
		t.Error("expected PauseRequested() to be true after RequestPause()")
	}
}

func TestManager_RequestPause_Idempotent(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, nil)

	m.RequestPause()
	m.RequestPause()
	m.RequestPause()

	if !m.PauseRequested() {
		t.Error("expected PauseRequested() to remain true after multiple calls")
	}
}

// Test wrap-up functionality

func TestManager_WrapUpSent_InitiallyFalse(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, nil)

	if m.WrapUpSent() {
		t.Error("expected WrapUpSent() to be false initially")
	}
}

func TestManager_SendWrapUp_AlwaysSucceeds(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, nil)

	// SendWrapUp is now a no-op that always succeeds
	err := m.SendWrapUp("test wrap-up prompt")
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if !m.WrapUpSent() {
		t.Error("expected WrapUpSent() to be true after SendWrapUp")
	}
}

func TestManager_SendWrapUp_Idempotent(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, nil)

	// SendWrapUp can be called multiple times (it's a no-op)
	err := m.SendWrapUp("first prompt")
	if err != nil {
		t.Errorf("expected no error on first call, got: %v", err)
	}

	err = m.SendWrapUp("second prompt")
	if err != nil {
		t.Errorf("expected no error on second call, got: %v", err)
	}

	if !m.WrapUpSent() {
		t.Error("expected WrapUpSent() to be true")
	}
}

func TestManager_CloseStdin_NilStdin(t *testing.T) {
	cfg := config.Default()
	m := New(cfg, nil)

	// Should not error when stdin is nil
	err := m.CloseStdin()
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}
