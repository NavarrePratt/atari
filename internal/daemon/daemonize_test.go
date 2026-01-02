package daemon

import (
	"net"
	"os"
	"testing"
	"time"
)

func TestIsDaemonized_False(t *testing.T) {
	// Ensure env var is not set
	_ = os.Unsetenv(daemonEnvVar)

	if IsDaemonized() {
		t.Error("expected IsDaemonized() to return false")
	}
}

func TestIsDaemonized_True(t *testing.T) {
	// Set env var
	t.Setenv(daemonEnvVar, "1")

	if !IsDaemonized() {
		t.Error("expected IsDaemonized() to return true")
	}
}

func TestIsDaemonized_WrongValue(t *testing.T) {
	// Set env var to non-"1" value
	t.Setenv(daemonEnvVar, "true")

	if IsDaemonized() {
		t.Error("expected IsDaemonized() to return false for non-1 value")
	}
}

func TestWaitForSocketReady_Success(t *testing.T) {
	sockPath := shortSocketPath(t)

	// Start listener in background
	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = listener.Close() }()

	// Wait should succeed immediately
	if err := waitForSocketReady(sockPath, time.Second); err != nil {
		t.Errorf("waitForSocketReady() error: %v", err)
	}
}

func TestWaitForSocketReady_Timeout(t *testing.T) {
	sockPath := shortSocketPath(t)

	// No listener - should timeout
	start := time.Now()
	err := waitForSocketReady(sockPath, 200*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected timeout error")
	}

	// Should have waited close to the timeout
	if elapsed < 150*time.Millisecond {
		t.Errorf("waited only %v, expected ~200ms", elapsed)
	}
}

func TestWaitForSocketReady_DelayedStart(t *testing.T) {
	sockPath := shortSocketPath(t)

	// Start listener after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		listener, err := net.Listen("unix", sockPath)
		if err != nil {
			return
		}
		defer func() { _ = listener.Close() }()
		// Keep listener alive for a bit
		time.Sleep(500 * time.Millisecond)
	}()

	// Wait should succeed after listener starts
	if err := waitForSocketReady(sockPath, time.Second); err != nil {
		t.Errorf("waitForSocketReady() error: %v", err)
	}
}

// Note: Testing Daemonize() fully would require spawning a subprocess
// and verifying process relationships, which is complex and platform-specific.
// The core re-exec logic is tested by integration tests instead.
//
// Here we test the path where we're already the daemon (env var set).

func TestDaemonize_AlreadyDaemonized(t *testing.T) {
	// Set env var to simulate being the daemon child
	t.Setenv(daemonEnvVar, "1")

	shouldExit, pid, err := Daemonize(nil)
	if err != nil {
		t.Errorf("Daemonize() error: %v", err)
	}
	if shouldExit {
		t.Error("shouldExit should be false when already daemonized")
	}
	if pid != os.Getpid() {
		t.Errorf("pid should be current process PID, got %d", pid)
	}
}
