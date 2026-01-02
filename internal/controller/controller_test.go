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

// testAgentID is the agent bead ID used in tests.
const testAgentID = "test-agent"

// testConfig returns a config suitable for testing with short intervals.
func testConfig() *config.Config {
	cfg := config.Default()
	cfg.WorkQueue.PollInterval = 10 * time.Millisecond
	cfg.Claude.Timeout = 100 * time.Millisecond
	cfg.Backoff.Initial = 10 * time.Millisecond
	cfg.Backoff.Max = 50 * time.Millisecond
	cfg.Backoff.MaxFailures = 2
	cfg.AgentID = testAgentID
	return cfg
}

// setupAgentStateMocks configures mock responses for all bd agent state commands.
func setupAgentStateMocks(runner *testutil.MockRunner) {
	for _, state := range []string{"idle", "running", "stopped", "dead"} {
		runner.SetResponse("bd", []string{"agent", "state", testAgentID, state}, []byte(""))
	}
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
		setupAgentStateMocks(runner)
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
		setupAgentStateMocks(runner)
		// Return empty beads so controller stays in idle
		runner.SetResponse("bd", []string{"ready", "--json"}, []byte("[]"))

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
		setupAgentStateMocks(runner)
		runner.SetResponse("bd", []string{"ready", "--json"}, []byte("[]"))

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
		setupAgentStateMocks(runner)
		runner.SetResponse("bd", []string{"ready", "--json"}, []byte("[]"))

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
	setupAgentStateMocks(runner)
	runner.SetResponse("bd", []string{"ready", "--json"}, []byte("[]"))

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
}

func TestControllerEventEmission(t *testing.T) {
	cfg := testConfig()
	runner := testutil.NewMockRunner()
	setupAgentStateMocks(runner)
	runner.SetResponse("bd", []string{"ready", "--json"}, []byte("[]"))

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
	setupAgentStateMocks(runner)
	runner.SetError("bd", []string{"ready", "--json"}, errors.New("connection refused"))

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
		setupAgentStateMocks(runner)
		runner.SetResponse("bd", []string{"ready", "--json"}, []byte("[]"))

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
		setupAgentStateMocks(runner)
		runner.SetResponse("bd", []string{"ready", "--json"}, []byte("[]"))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		// Send multiple stop signals
		c.Stop()
		c.Stop()
		c.Stop()

		// Should not panic
	})
}

func TestControllerAgentStateReporting(t *testing.T) {
	t.Run("reports agent state on state transitions", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		setupAgentStateMocks(runner)
		runner.SetResponse("bd", []string{"ready", "--json"}, []byte("[]"))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		done := make(chan error, 1)
		go func() {
			done <- c.Run(ctx)
		}()

		// Wait for controller to run a bit
		time.Sleep(50 * time.Millisecond)
		c.Stop()
		<-done

		// Check that agent state commands were called
		calls := runner.GetCalls()
		var agentStateCalls []testutil.CommandCall
		for _, call := range calls {
			if call.Name == "bd" && len(call.Args) >= 3 && call.Args[0] == "agent" && call.Args[1] == "state" {
				agentStateCalls = append(agentStateCalls, call)
			}
		}

		if len(agentStateCalls) == 0 {
			t.Error("expected at least one agent state call")
		}

		// Verify we see the expected state transitions
		seenStates := make(map[string]bool)
		for _, call := range agentStateCalls {
			if len(call.Args) >= 4 {
				seenStates[call.Args[3]] = true
			}
		}

		// Should have at least stopped and dead states from shutdown sequence
		if !seenStates["stopped"] {
			t.Error("expected to see 'stopped' agent state")
		}
		if !seenStates["dead"] {
			t.Error("expected to see 'dead' agent state")
		}
	})

	t.Run("handles agent state command failure gracefully", func(t *testing.T) {
		cfg := testConfig()
		runner := testutil.NewMockRunner()
		// Set up errors for agent state commands
		runner.SetError("bd", []string{"agent", "state", testAgentID, "idle"}, errors.New("bd not available"))
		runner.SetError("bd", []string{"agent", "state", testAgentID, "running"}, errors.New("bd not available"))
		runner.SetError("bd", []string{"agent", "state", testAgentID, "stopped"}, errors.New("bd not available"))
		runner.SetError("bd", []string{"agent", "state", testAgentID, "dead"}, errors.New("bd not available"))
		runner.SetResponse("bd", []string{"ready", "--json"}, []byte("[]"))

		wq := workqueue.New(cfg, runner)
		c := New(cfg, wq, nil, runner, nil, nil)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		done := make(chan error, 1)
		go func() {
			done <- c.Run(ctx)
		}()

		time.Sleep(30 * time.Millisecond)
		c.Stop()

		// Controller should still stop cleanly even with agent state errors
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
