// Package daemon provides integration tests for the daemon package.
// These tests verify end-to-end functionality including RPC communication,
// controller integration, and path resolution.
package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/controller"
	"github.com/npratt/atari/internal/events"
	"github.com/npratt/atari/internal/testutil"
	"github.com/npratt/atari/internal/workqueue"
)

// testDaemonEnv holds the test environment for daemon integration tests.
type testDaemonEnv struct {
	t          *testing.T
	tmpDir     string
	cfg        *config.Config
	runner     *testutil.MockRunner
	router     *events.Router
	workQueue  *workqueue.Manager
	controller *controller.Controller
	daemon     *Daemon
	client     *Client
}

// newTestDaemonEnv creates a test environment with controller and daemon.
func newTestDaemonEnv(t *testing.T) *testDaemonEnv {
	t.Helper()

	tmpDir := t.TempDir()

	cfg := config.Default()
	cfg.WorkQueue.PollInterval = 10 * time.Millisecond
	cfg.Paths.Socket = shortSocketPath(t)
	cfg.Paths.PID = filepath.Join(tmpDir, "test.pid")
	cfg.Paths.State = filepath.Join(tmpDir, "state.json")
	cfg.Paths.Log = filepath.Join(tmpDir, "events.log")

	runner := testutil.NewMockRunner()
	// Setup empty bd ready response
	runner.SetResponse("bd", []string{"ready", "--json"}, []byte("[]"))

	router := events.NewRouter(1000)
	wq := workqueue.New(cfg, runner)
	ctrl := controller.New(cfg, wq, router, runner, nil)

	d := New(cfg, ctrl, nil)
	client := NewClient(cfg.Paths.Socket)

	return &testDaemonEnv{
		t:          t,
		tmpDir:     tmpDir,
		cfg:        cfg,
		runner:     runner,
		router:     router,
		workQueue:  wq,
		controller: ctrl,
		daemon:     d,
		client:     client,
	}
}

// cleanup releases resources.
func (e *testDaemonEnv) cleanup() {
	e.router.Close()
}

// startDaemon starts the daemon in a goroutine and waits for it to be ready.
// Returns the daemon error channel.
func (e *testDaemonEnv) startDaemon(ctx context.Context) <-chan error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- e.daemon.Start(ctx)
	}()

	// Wait for socket to be ready
	waitForSocket(e.t, e.cfg.Paths.Socket, 2*time.Second)
	return errCh
}

// startDaemonWithController starts both the daemon and controller.
// Returns the daemon error channel.
func (e *testDaemonEnv) startDaemonWithController(ctx context.Context) <-chan error {
	// Start controller in background
	go func() {
		_ = e.controller.Run(ctx)
	}()

	// Start daemon
	return e.startDaemon(ctx)
}

func TestDaemonLifecycle_WithController(t *testing.T) {
	env := newTestDaemonEnv(t)
	defer env.cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start daemon
	errCh := env.startDaemon(ctx)

	// Verify daemon is running
	if !env.daemon.Running() {
		t.Error("daemon should be running after start")
	}

	// Verify socket accepts connections
	if !env.client.IsRunning() {
		t.Error("client.IsRunning() should return true")
	}

	// Send status RPC - should succeed now that we have a controller
	status, err := env.client.Status()
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}

	// Verify status response
	if status.Status != string(controller.StateIdle) {
		t.Errorf("expected status %s, got %s", controller.StateIdle, status.Status)
	}
	if status.Uptime == "" {
		t.Error("expected non-empty uptime")
	}
	if status.StartTime == "" {
		t.Error("expected non-empty start time")
	}

	// Stop daemon
	cancel()

	// Wait for shutdown
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("daemon Start() returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for daemon to stop")
	}

	// Verify cleanup
	if env.daemon.Running() {
		t.Error("daemon should not be running after stop")
	}

	// Socket should be removed
	if _, err := os.Stat(env.cfg.Paths.Socket); !os.IsNotExist(err) {
		t.Error("socket file should be removed after stop")
	}
}

func TestDaemonPauseResume_WithController(t *testing.T) {
	env := newTestDaemonEnv(t)
	defer env.cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start daemon with controller running
	errCh := env.startDaemonWithController(ctx)

	// Wait for controller to be in idle state
	deadline := time.After(2 * time.Second)
	for {
		status, err := env.client.Status()
		if err == nil && status.Status == string(controller.StateIdle) {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timeout waiting for controller to reach idle state")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Send pause via client
	if err := env.client.Pause(); err != nil {
		t.Fatalf("Pause() error: %v", err)
	}

	// Wait for controller to be paused
	deadline = time.After(2 * time.Second)
	for {
		state := env.controller.State()
		if state == controller.StatePaused {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for paused state, got %s", env.controller.State())
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Verify status shows paused
	status, err := env.client.Status()
	if err != nil {
		t.Fatalf("Status() error after pause: %v", err)
	}
	if status.Status != string(controller.StatePaused) {
		t.Errorf("expected status %s, got %s", controller.StatePaused, status.Status)
	}

	// Send resume via client
	if err := env.client.Resume(); err != nil {
		t.Fatalf("Resume() error: %v", err)
	}

	// Wait for controller to resume (not paused)
	deadline = time.After(2 * time.Second)
	for {
		state := env.controller.State()
		if state != controller.StatePaused {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for resume, still %s", env.controller.State())
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Stop daemon
	cancel()

	select {
	case <-errCh:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for daemon to stop")
	}
}

func TestDaemonForceStop(t *testing.T) {
	env := newTestDaemonEnv(t)
	defer env.cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start daemon
	errCh := env.startDaemon(ctx)

	// Wait for daemon to be ready
	time.Sleep(100 * time.Millisecond)

	// Send force stop via client
	start := time.Now()
	if err := env.client.Stop(true); err != nil {
		t.Fatalf("Stop(force=true) error: %v", err)
	}

	// Wait for daemon to stop
	select {
	case err := <-errCh:
		elapsed := time.Since(start)
		if err != nil {
			t.Errorf("daemon Start() returned error: %v", err)
		}
		// Force stop should be fast
		if elapsed > 500*time.Millisecond {
			t.Errorf("force stop took too long: %v", elapsed)
		}
		t.Logf("force stop completed in %v", elapsed)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for daemon to stop")
	}
}

func TestDaemonGracefulStop(t *testing.T) {
	env := newTestDaemonEnv(t)
	defer env.cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start daemon
	errCh := env.startDaemon(ctx)

	// Wait for daemon to be ready
	time.Sleep(100 * time.Millisecond)

	// Send graceful stop via client
	if err := env.client.Stop(false); err != nil {
		t.Fatalf("Stop(force=false) error: %v", err)
	}

	// Wait for daemon to stop
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("daemon Start() returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for daemon to stop")
	}

	// Verify daemon is stopped
	if env.daemon.Running() {
		t.Error("daemon should not be running after graceful stop")
	}
}

func TestDaemonStatus_Stats(t *testing.T) {
	env := newTestDaemonEnv(t)
	defer env.cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start daemon
	errCh := env.startDaemon(ctx)

	// Wait for daemon to be ready
	time.Sleep(100 * time.Millisecond)

	// Get status with stats
	status, err := env.client.Status()
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}

	// Verify stats are present
	// Note: values may be 0 since we haven't processed any beads
	t.Logf("Status: %+v", status)
	t.Logf("Stats: iteration=%d, total_seen=%d, completed=%d",
		status.Stats.Iteration, status.Stats.TotalSeen, status.Stats.Completed)

	// Stop daemon
	cancel()
	<-errCh
}
