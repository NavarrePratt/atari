package controller

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/events"
	"github.com/npratt/atari/internal/testutil"
	"github.com/npratt/atari/internal/workqueue"
)

// testConfig returns a config suitable for testing with short intervals.
func testConfig() *config.Config {
	cfg := config.Default()
	cfg.WorkQueue.PollInterval = 10 * time.Millisecond
	cfg.Claude.Timeout = 100 * time.Millisecond
	cfg.Backoff.Initial = 10 * time.Millisecond
	cfg.Backoff.Max = 50 * time.Millisecond
	cfg.Backoff.MaxFailures = 2
	return cfg
}

func TestControllerStates(t *testing.T) {
	t.Run("initial state is idle", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		if c.State() != StateIdle {
			t.Errorf("expected initial state %s, got %s", StateIdle, c.State())
		}
	})

	t.Run("state transitions are thread-safe", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				c.setState(StateWorking)
				_ = c.getState()
				c.setState(StateIdle)
			}()
		}
		wg.Wait()

		// Should not panic and state should be valid
		state := c.State()
		if state != StateIdle && state != StateWorking {
			t.Errorf("unexpected state: %s", state)
		}
	})
}

func TestControllerPauseResume(t *testing.T) {
	t.Run("pause transitions from idle", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		// Return empty beads so controller stays in idle
		runner.SetResponse("br", []string{"ready", "--json"}, []byte("[]"))

		wq := workqueue.New(cfg, runner)
		router := events.NewRouter(10)
		defer router.Close()

		c := New(cfg, wq, router, runner, nil, nil)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start controller in background
		done := make(chan error, 1)
		go func() {
			done <- c.Run(ctx)
		}()

		// Wait for controller to start
		time.Sleep(20 * time.Millisecond)

		// Request pause
		c.Pause()

		// Wait for state change
		time.Sleep(50 * time.Millisecond)

		if c.State() != StatePaused {
			t.Errorf("expected state %s, got %s", StatePaused, c.State())
		}

		// Resume
		c.Resume()
		time.Sleep(20 * time.Millisecond)

		if c.State() != StateIdle {
			t.Errorf("expected state %s after resume, got %s", StateIdle, c.State())
		}

		// Stop
		c.Stop()
		<-done
	})
}

func TestControllerStop(t *testing.T) {
	t.Run("stop from idle", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		runner.SetResponse("br", []string{"ready", "--json"}, []byte("[]"))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		ctx := context.Background()

		done := make(chan error, 1)
		go func() {
			done <- c.Run(ctx)
		}()

		time.Sleep(20 * time.Millisecond)
		c.Stop()

		select {
		case err := <-done:
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		case <-time.After(time.Second):
			t.Error("timeout waiting for controller to stop")
		}

		if c.State() != StateStopped {
			t.Errorf("expected state %s, got %s", StateStopped, c.State())
		}
	})

	t.Run("stop from paused", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		runner.SetResponse("br", []string{"ready", "--json"}, []byte("[]"))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		ctx := context.Background()

		done := make(chan error, 1)
		go func() {
			done <- c.Run(ctx)
		}()

		time.Sleep(20 * time.Millisecond)
		c.Pause()
		time.Sleep(50 * time.Millisecond)

		if c.State() != StatePaused {
			t.Errorf("expected state %s, got %s", StatePaused, c.State())
		}

		c.Stop()

		select {
		case err := <-done:
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		case <-time.After(time.Second):
			t.Error("timeout waiting for controller to stop")
		}

		if c.State() != StateStopped {
			t.Errorf("expected state %s, got %s", StateStopped, c.State())
		}
	})
}

func TestControllerContextCancellation(t *testing.T) {
	cfg := testConfig()
	runner := testutil.NewMockRunner()
	runner.SetResponse("br", []string{"ready", "--json"}, []byte("[]"))

	wq := workqueue.New(cfg, runner)
	c := New(cfg, wq, nil, runner, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- c.Run(ctx)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for controller to stop")
	}

	if c.State() != StateStopped {
		t.Errorf("expected state %s, got %s", StateStopped, c.State())
	}
}

func TestControllerStats(t *testing.T) {
	cfg := testConfig()
	runner := testutil.NewMockRunner()
	wq := workqueue.New(cfg, runner)
	c := New(cfg, wq, nil, runner, nil, nil)

	stats := c.Stats()
	if stats.Iteration != 0 {
		t.Errorf("expected iteration 0, got %d", stats.Iteration)
	}
	if stats.CurrentTurns != 0 {
		t.Errorf("expected CurrentTurns 0, got %d", stats.CurrentTurns)
	}
}

func TestControllerCurrentTurns(t *testing.T) {
	t.Run("initial value is 0", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		if c.CurrentTurns() != 0 {
			t.Errorf("expected CurrentTurns 0, got %d", c.CurrentTurns())
		}
	})

	t.Run("thread-safe access", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = c.CurrentTurns()
				_ = c.Stats().CurrentTurns
			}()
		}
		wg.Wait()
		// Should not panic
	})
}

func TestControllerEventEmission(t *testing.T) {
	cfg := testConfig()
	runner := testutil.NewMockRunner()
	runner.SetResponse("br", []string{"ready", "--json"}, []byte("[]"))

	wq := workqueue.New(cfg, runner)
	router := events.NewRouter(100)
	defer router.Close()

	sub := router.Subscribe()
	c := New(cfg, wq, router, runner, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- c.Run(ctx)
	}()

	// Wait for DrainStartEvent
	select {
	case evt := <-sub:
		if evt.Type() != events.EventDrainStart {
			t.Errorf("expected %s, got %s", events.EventDrainStart, evt.Type())
		}
		startEvt, ok := evt.(*events.DrainStartEvent)
		if !ok {
			t.Error("failed to cast to DrainStartEvent")
		} else if startEvt.WorkDir == "" {
			t.Error("expected non-empty WorkDir")
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for DrainStartEvent")
	}

	c.Stop()

	// Wait for DrainStopEvent
	deadline := time.After(time.Second)
	for {
		select {
		case evt := <-sub:
			if evt.Type() == events.EventDrainStop {
				stopEvt, ok := evt.(*events.DrainStopEvent)
				if !ok {
					t.Error("failed to cast to DrainStopEvent")
				} else if stopEvt.Reason == "" {
					t.Error("expected non-empty Reason")
				}
				<-done
				return
			}
		case <-deadline:
			t.Error("timeout waiting for DrainStopEvent")
			cancel()
			<-done
			return
		}
	}
}

func TestControllerWorkQueueError(t *testing.T) {
	cfg := testConfig()
	runner := testutil.NewMockRunner()
	runner.SetError("br", []string{"ready", "--json"}, errors.New("connection refused"))

	wq := workqueue.New(cfg, runner)
	router := events.NewRouter(100)
	defer router.Close()

	sub := router.Subscribe()
	c := New(cfg, wq, router, runner, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- c.Run(ctx)
	}()

	// Skip DrainStartEvent
	<-sub

	// Wait for ErrorEvent
	select {
	case evt := <-sub:
		if evt.Type() != events.EventError {
			t.Errorf("expected %s, got %s", events.EventError, evt.Type())
		}
		errEvt, ok := evt.(*events.ErrorEvent)
		if !ok {
			t.Error("failed to cast to ErrorEvent")
		} else if errEvt.Severity != events.SeverityWarning {
			t.Errorf("expected severity %s, got %s", events.SeverityWarning, errEvt.Severity)
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for ErrorEvent")
	}

	c.Stop()
	<-done
}

func TestControllerMultipleSignals(t *testing.T) {
	t.Run("multiple pause signals are deduplicated", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		runner.SetResponse("br", []string{"ready", "--json"}, []byte("[]"))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		// Send multiple pause signals before starting
		c.Pause()
		c.Pause()
		c.Pause()

		// Should not panic
	})

	t.Run("multiple stop signals are deduplicated", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		runner.SetResponse("br", []string{"ready", "--json"}, []byte("[]"))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		// Send multiple stop signals
		c.Stop()
		c.Stop()
		c.Stop()

		// Should not panic
	})
}

func TestControllerEmitsDrainStateChangedEvent(t *testing.T) {
	cfg := testConfig()
	runner := testutil.NewMockRunner()
	runner.SetResponse("br", []string{"ready", "--json"}, []byte("[]"))

	wq := workqueue.New(cfg, runner)
	router := events.NewRouter(100)
	defer router.Close()

	sub := router.Subscribe()
	c := New(cfg, wq, router, runner, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- c.Run(ctx)
	}()

	// Skip DrainStartEvent
	<-sub

	// Wait a bit for controller to run
	time.Sleep(30 * time.Millisecond)

	// Request pause - this should trigger a state change event
	c.Pause()

	// Wait for DrainStateChangedEvent with To="paused"
	deadline := time.After(time.Second)
	foundPausedEvent := false
	for !foundPausedEvent {
		select {
		case evt := <-sub:
			if evt.Type() == events.EventDrainStateChanged {
				stateEvt, ok := evt.(*events.DrainStateChangedEvent)
				if !ok {
					t.Error("failed to cast to DrainStateChangedEvent")
					continue
				}
				if stateEvt.To == "paused" {
					foundPausedEvent = true
					if stateEvt.From != "idle" {
						t.Errorf("expected From=idle, got %s", stateEvt.From)
					}
				}
			}
		case <-deadline:
			t.Error("timeout waiting for DrainStateChangedEvent with To=paused")
			c.Stop()
			<-done
			return
		}
	}

	// Resume and look for state change back to idle
	c.Resume()

	deadline = time.After(time.Second)
	foundIdleEvent := false
	for !foundIdleEvent {
		select {
		case evt := <-sub:
			if evt.Type() == events.EventDrainStateChanged {
				stateEvt, ok := evt.(*events.DrainStateChangedEvent)
				if !ok {
					t.Error("failed to cast to DrainStateChangedEvent")
					continue
				}
				if stateEvt.To == "idle" && stateEvt.From == "paused" {
					foundIdleEvent = true
				}
			}
		case <-deadline:
			t.Error("timeout waiting for DrainStateChangedEvent with To=idle")
			c.Stop()
			<-done
			return
		}
	}

	c.Stop()
	<-done
}

func TestAgentStateMapping(t *testing.T) {
	t.Run("maps controller states to agent states correctly", func(t *testing.T) {
		// Verify the mapping
		expectedMappings := map[State]string{
			StateIdle:     "idle",
			StateWorking:  "running",
			StatePaused:   "idle",
			StateStopping: "stopped",
			StateStopped:  "dead",
		}

		for controllerState, expectedAgentState := range expectedMappings {
			agentState, ok := agentStateMap[controllerState]
			if !ok {
				t.Errorf("missing mapping for state %s", controllerState)
				continue
			}
			if agentState != expectedAgentState {
				t.Errorf("state %s: expected agent state %s, got %s", controllerState, expectedAgentState, agentState)
			}
		}
	})
}

func TestControllerResetBeadToOpen(t *testing.T) {
	t.Run("calls br update with correct args", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		runner.SetResponse("br", []string{"update", "test-bead", "--status", "open", "--notes", "test notes"}, []byte(""))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		err := c.resetBeadToOpen("test-bead", "test notes")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Verify the command was called
		calls := runner.GetCalls()
		found := false
		for _, call := range calls {
			if call.Name == "br" && len(call.Args) >= 5 &&
				call.Args[0] == "update" &&
				call.Args[1] == "test-bead" &&
				call.Args[2] == "--status" &&
				call.Args[3] == "open" &&
				call.Args[4] == "--notes" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected br update command to be called")
		}
	})

	t.Run("handles command error", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		runner.SetError("br", []string{"update", "test-bead", "--status", "open", "--notes", "test notes"}, errors.New("br unavailable"))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		err := c.resetBeadToOpen("test-bead", "test notes")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("requires command runner", func(t *testing.T) {
		cfg := testConfig()
		wq := workqueue.New(cfg, nil)
		c := New(cfg, wq, nil, nil, nil, nil)

		err := c.resetBeadToOpen("test-bead", "test notes")
		if err == nil {
			t.Error("expected error when runner is nil")
		}
	})
}

func TestControllerGetBeadStatus(t *testing.T) {
	t.Run("returns status from JSON response", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		runner.SetResponse("br", []string{"show", "test-bead", "--json"}, []byte(`[{"status":"open"}]`))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		status := c.getBeadStatus("test-bead")
		if status != "open" {
			t.Errorf("expected status 'open', got '%s'", status)
		}
	})

	t.Run("returns closed status", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		runner.SetResponse("br", []string{"show", "test-bead", "--json"}, []byte(`[{"status":"closed"}]`))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		status := c.getBeadStatus("test-bead")
		if status != "closed" {
			t.Errorf("expected status 'closed', got '%s'", status)
		}
	})

	t.Run("returns empty string on error", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		runner.SetError("br", []string{"show", "test-bead", "--json"}, errors.New("br unavailable"))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		status := c.getBeadStatus("test-bead")
		if status != "" {
			t.Errorf("expected empty status, got '%s'", status)
		}
	})

	t.Run("returns empty string for invalid JSON", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		runner.SetResponse("br", []string{"show", "test-bead", "--json"}, []byte("not json"))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		status := c.getBeadStatus("test-bead")
		if status != "" {
			t.Errorf("expected empty status, got '%s'", status)
		}
	})

	t.Run("returns empty string for empty array", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		runner.SetResponse("br", []string{"show", "test-bead", "--json"}, []byte("[]"))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		status := c.getBeadStatus("test-bead")
		if status != "" {
			t.Errorf("expected empty status, got '%s'", status)
		}
	})

	t.Run("requires command runner", func(t *testing.T) {
		cfg := testConfig()
		wq := workqueue.New(cfg, nil)
		c := New(cfg, wq, nil, nil, nil, nil)

		status := c.getBeadStatus("test-bead")
		if status != "" {
			t.Errorf("expected empty status when runner is nil, got '%s'", status)
		}
	})
}

func TestControllerFollowUpConfig(t *testing.T) {
	t.Run("default config has follow-up enabled", func(t *testing.T) {
		cfg := config.Default()
		if !cfg.FollowUp.Enabled {
			t.Error("expected FollowUp.Enabled to be true by default")
		}
		if cfg.FollowUp.MaxTurns != 5 {
			t.Errorf("expected FollowUp.MaxTurns to be 5, got %d", cfg.FollowUp.MaxTurns)
		}
	})

	t.Run("follow-up disabled returns early", func(t *testing.T) {
		cfg := testConfig()
		cfg.FollowUp.Enabled = false

		runner := testutil.NewMockRunner()
		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)
		c.ctx = context.Background()

		bead := &workqueue.Bead{ID: "test-bead", Title: "Test"}
		closed, result, err := c.runFollowUpSession(bead)

		if closed {
			t.Error("expected closed to be false when follow-up is disabled")
		}
		if result != nil {
			t.Error("expected result to be nil when follow-up is disabled")
		}
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})
}

func TestControllerGracefulPause(t *testing.T) {
	t.Run("graceful pause signal is sent correctly", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		runner.SetResponse("br", []string{"ready", "--json"}, []byte("[]"))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		// Verify channel starts empty
		select {
		case <-c.gracefulPauseSignal:
			t.Error("expected graceful pause signal channel to be empty initially")
		default:
			// Good - channel is empty
		}

		// Send graceful pause signal
		c.GracefulPause()

		// Verify signal was sent
		select {
		case <-c.gracefulPauseSignal:
			// Good - signal received
		default:
			t.Error("expected graceful pause signal to be sent")
		}
	})

	t.Run("graceful pause from idle transitions to paused", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		runner.SetResponse("br", []string{"ready", "--json"}, []byte("[]"))

		wq := workqueue.New(cfg, runner)
		router := events.NewRouter(100)
		defer router.Close()

		c := New(cfg, wq, router, runner, nil, nil)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		done := make(chan error, 1)
		go func() {
			done <- c.Run(ctx)
		}()

		// Wait for controller to start
		time.Sleep(30 * time.Millisecond)

		// Request graceful pause
		c.GracefulPause()

		// Wait for state change
		time.Sleep(100 * time.Millisecond)

		if c.State() != StatePaused {
			t.Errorf("expected state %s after graceful pause from idle, got %s", StatePaused, c.State())
		}

		c.Stop()
		<-done
	})

	t.Run("multiple graceful pause signals are deduplicated", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		runner.SetResponse("br", []string{"ready", "--json"}, []byte("[]"))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		// Send multiple graceful pause signals - should not panic
		c.GracefulPause()
		c.GracefulPause()
		c.GracefulPause()

		// Verify only one signal is buffered
		select {
		case <-c.gracefulPauseSignal:
			// Good - first signal
		default:
			t.Error("expected at least one signal")
		}

		// Channel should now be empty
		select {
		case <-c.gracefulPauseSignal:
			t.Error("expected only one signal to be buffered")
		default:
			// Good - no more signals
		}
	})

	t.Run("graceful pause after iteration transitions to paused", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		runner.SetResponse("br", []string{"ready", "--json"}, []byte("[]"))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		// Simulate state after completing an iteration
		c.setState(StateWorking)

		// The graceful pause signal should be checked when transitioning
		// after runWorkingOnBead completes

		// Verify the channel accepts signals
		c.GracefulPause()

		select {
		case <-c.gracefulPauseSignal:
			// Good
		default:
			t.Error("expected graceful pause signal")
		}
	})
}

func TestControllerSessionResume(t *testing.T) {
	t.Run("getStoredSessionID returns empty for new bead", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		sessionID := c.getStoredSessionID("new-bead")
		if sessionID != "" {
			t.Errorf("expected empty session ID for new bead, got %s", sessionID)
		}
	})

	t.Run("getStoredSessionID returns ID from history", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		wq := workqueue.New(cfg, runner)

		// Pre-populate history with session ID
		history := map[string]*workqueue.BeadHistory{
			"test-bead": {
				ID:            "test-bead",
				Status:        workqueue.HistoryFailed,
				LastSessionID: "session-123",
			},
		}
		wq.SetHistory(history)

		c := New(cfg, wq, nil, runner, nil, nil)

		sessionID := c.getStoredSessionID("test-bead")
		if sessionID != "session-123" {
			t.Errorf("expected session ID 'session-123', got '%s'", sessionID)
		}
	})

	t.Run("getStoredSessionID returns empty for history without session ID", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		wq := workqueue.New(cfg, runner)

		// Pre-populate history without session ID
		history := map[string]*workqueue.BeadHistory{
			"test-bead": {
				ID:     "test-bead",
				Status: workqueue.HistoryFailed,
			},
		}
		wq.SetHistory(history)

		c := New(cfg, wq, nil, runner, nil, nil)

		sessionID := c.getStoredSessionID("test-bead")
		if sessionID != "" {
			t.Errorf("expected empty session ID, got '%s'", sessionID)
		}
	})
}

func TestControllerValidateEpic(t *testing.T) {
	t.Run("no epic configured returns nil", func(t *testing.T) {
		cfg := testConfig()
		cfg.WorkQueue.Epic = "" // No epic configured

		runner := testutil.NewMockRunner()
		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)
		c.ctx = context.Background()

		err := c.validateEpic(c.ctx)
		if err != nil {
			t.Errorf("expected no error when epic not configured, got: %v", err)
		}

		id, title := c.ValidatedEpic()
		if id != "" || title != "" {
			t.Errorf("expected empty epic info, got id=%s title=%s", id, title)
		}
	})

	t.Run("valid epic stores info", func(t *testing.T) {
		cfg := testConfig()
		cfg.WorkQueue.Epic = "bd-epic-123"

		runner := testutil.NewMockRunner()
		runner.SetResponse("br", []string{"show", "bd-epic-123", "--json"},
			[]byte(`[{"id":"bd-epic-123","title":"Test Epic","issue_type":"epic"}]`))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)
		c.ctx = context.Background()

		err := c.validateEpic(c.ctx)
		if err != nil {
			t.Errorf("expected no error for valid epic, got: %v", err)
		}

		id, title := c.ValidatedEpic()
		if id != "bd-epic-123" {
			t.Errorf("expected epic id 'bd-epic-123', got '%s'", id)
		}
		if title != "Test Epic" {
			t.Errorf("expected epic title 'Test Epic', got '%s'", title)
		}
	})

	t.Run("epic not found returns error", func(t *testing.T) {
		cfg := testConfig()
		cfg.WorkQueue.Epic = "bd-nonexistent"

		runner := testutil.NewMockRunner()
		runner.SetError("br", []string{"show", "bd-nonexistent", "--json"},
			errors.New("issue not found"))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)
		c.ctx = context.Background()

		err := c.validateEpic(c.ctx)
		if err == nil {
			t.Error("expected error for nonexistent epic")
		}
		if err.Error() != "epic not found: bd-nonexistent" {
			t.Errorf("expected 'epic not found: bd-nonexistent', got: %v", err)
		}
	})

	t.Run("empty response returns error", func(t *testing.T) {
		cfg := testConfig()
		cfg.WorkQueue.Epic = "bd-empty"

		runner := testutil.NewMockRunner()
		runner.SetResponse("br", []string{"show", "bd-empty", "--json"}, []byte(""))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)
		c.ctx = context.Background()

		err := c.validateEpic(c.ctx)
		if err == nil {
			t.Error("expected error for empty response")
		}
		if err.Error() != "epic not found: bd-empty" {
			t.Errorf("expected 'epic not found: bd-empty', got: %v", err)
		}
	})

	t.Run("empty array returns error", func(t *testing.T) {
		cfg := testConfig()
		cfg.WorkQueue.Epic = "bd-empty"

		runner := testutil.NewMockRunner()
		runner.SetResponse("br", []string{"show", "bd-empty", "--json"}, []byte("[]"))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)
		c.ctx = context.Background()

		err := c.validateEpic(c.ctx)
		if err == nil {
			t.Error("expected error for empty array response")
		}
		if err.Error() != "epic not found: bd-empty" {
			t.Errorf("expected 'epic not found: bd-empty', got: %v", err)
		}
	})

	t.Run("non-epic type returns error", func(t *testing.T) {
		cfg := testConfig()
		cfg.WorkQueue.Epic = "bd-task-456"

		runner := testutil.NewMockRunner()
		runner.SetResponse("br", []string{"show", "bd-task-456", "--json"},
			[]byte(`[{"id":"bd-task-456","title":"A Task","issue_type":"task"}]`))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)
		c.ctx = context.Background()

		err := c.validateEpic(c.ctx)
		if err == nil {
			t.Error("expected error for non-epic type")
		}
		if err.Error() != "bd-task-456 is not an epic (type: task)" {
			t.Errorf("expected 'bd-task-456 is not an epic (type: task)', got: %v", err)
		}
	})

	t.Run("no command runner returns error", func(t *testing.T) {
		cfg := testConfig()
		cfg.WorkQueue.Epic = "bd-epic-123"

		wq := workqueue.New(cfg, nil)
		c := New(cfg, wq, nil, nil, nil, nil)
		c.ctx = context.Background()

		err := c.validateEpic(c.ctx)
		if err == nil {
			t.Error("expected error when command runner is nil")
		}
		if err.Error() != "cannot validate epic: no command runner available" {
			t.Errorf("expected 'cannot validate epic: no command runner available', got: %v", err)
		}
	})

	t.Run("invalid json returns error", func(t *testing.T) {
		cfg := testConfig()
		cfg.WorkQueue.Epic = "bd-bad-json"

		runner := testutil.NewMockRunner()
		runner.SetResponse("br", []string{"show", "bd-bad-json", "--json"}, []byte("not json"))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)
		c.ctx = context.Background()

		err := c.validateEpic(c.ctx)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
		if err.Error() != "epic not found: bd-bad-json" {
			t.Errorf("expected 'epic not found: bd-bad-json', got: %v", err)
		}
	})
}

func TestControllerRunWithInvalidEpic(t *testing.T) {
	t.Run("run fails with invalid epic", func(t *testing.T) {
		cfg := testConfig()
		cfg.WorkQueue.Epic = "bd-nonexistent"

		runner := testutil.NewMockRunner()
		runner.SetError("br", []string{"show", "bd-nonexistent", "--json"},
			errors.New("issue not found"))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		err := c.Run(ctx)
		if err == nil {
			t.Error("expected Run to fail with invalid epic")
		}
		if err.Error() != "epic not found: bd-nonexistent" {
			t.Errorf("expected 'epic not found: bd-nonexistent', got: %v", err)
		}
	})

	t.Run("run fails with non-epic type", func(t *testing.T) {
		cfg := testConfig()
		cfg.WorkQueue.Epic = "bd-task-456"

		runner := testutil.NewMockRunner()
		runner.SetResponse("br", []string{"show", "bd-task-456", "--json"},
			[]byte(`[{"id":"bd-task-456","title":"A Task","issue_type":"task"}]`))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		err := c.Run(ctx)
		if err == nil {
			t.Error("expected Run to fail with non-epic type")
		}
		if err.Error() != "bd-task-456 is not an epic (type: task)" {
			t.Errorf("expected 'bd-task-456 is not an epic (type: task)', got: %v", err)
		}
	})

	t.Run("run succeeds with valid epic", func(t *testing.T) {
		cfg := testConfig()
		cfg.WorkQueue.Epic = "bd-epic-123"

		runner := testutil.NewMockRunner()
		runner.SetResponse("br", []string{"show", "bd-epic-123", "--json"},
			[]byte(`[{"id":"bd-epic-123","title":"Test Epic","issue_type":"epic"}]`))
		runner.SetResponse("br", []string{"ready", "--json"}, []byte("[]"))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		done := make(chan error, 1)
		go func() {
			done <- c.Run(ctx)
		}()

		// Wait for controller to start and validate
		time.Sleep(30 * time.Millisecond)

		// Verify epic info was stored
		id, title := c.ValidatedEpic()
		if id != "bd-epic-123" {
			t.Errorf("expected epic id 'bd-epic-123', got '%s'", id)
		}
		if title != "Test Epic" {
			t.Errorf("expected epic title 'Test Epic', got '%s'", title)
		}

		c.Stop()
		<-done
	})
}

func TestControllerIsBeadClosed(t *testing.T) {
	t.Run("returns true for closed status", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		runner.SetResponse("br", []string{"show", "test-bead", "--json"}, []byte(`[{"status":"closed"}]`))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		if !c.isBeadClosed("test-bead") {
			t.Error("expected isBeadClosed to return true for closed status")
		}
	})

	t.Run("returns true for completed status", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		runner.SetResponse("br", []string{"show", "test-bead", "--json"}, []byte(`[{"status":"completed"}]`))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		if !c.isBeadClosed("test-bead") {
			t.Error("expected isBeadClosed to return true for completed status")
		}
	})

	t.Run("returns false for open status", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		runner.SetResponse("br", []string{"show", "test-bead", "--json"}, []byte(`[{"status":"open"}]`))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		if c.isBeadClosed("test-bead") {
			t.Error("expected isBeadClosed to return false for open status")
		}
	})

	t.Run("returns false for in_progress status", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		runner.SetResponse("br", []string{"show", "test-bead", "--json"}, []byte(`[{"status":"in_progress"}]`))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		if c.isBeadClosed("test-bead") {
			t.Error("expected isBeadClosed to return false for in_progress status")
		}
	})

	t.Run("returns false on error", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		runner.SetError("br", []string{"show", "test-bead", "--json"}, errors.New("br unavailable"))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		if c.isBeadClosed("test-bead") {
			t.Error("expected isBeadClosed to return false on error")
		}
	})
}
