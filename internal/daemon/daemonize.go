package daemon

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/npratt/atari/internal/config"
)

const (
	// daemonEnvVar is the environment variable set in the child process
	// to indicate it's running as the daemonized process.
	daemonEnvVar = "ATARI_DAEMONIZED"

	// socketWaitTimeout is how long the parent waits for the socket to appear
	// before printing success.
	socketWaitTimeout = 2 * time.Second

	// socketCheckInterval is how often to check for socket availability.
	socketCheckInterval = 50 * time.Millisecond
)

// Daemonize forks the current process as a background daemon using the re-exec pattern.
// It returns (true, nil) if the current process is the parent and should exit,
// (false, nil) if the current process is the daemonized child and should continue,
// or (false, err) if an error occurred.
//
// The parent process:
//   - Spawns a child with ATARI_DAEMONIZED=1
//   - Waits for the socket to appear (up to 2s)
//   - Prints "Started daemon (pid X)" and returns shouldExit=true
//
// The child process:
//   - Detects ATARI_DAEMONIZED=1 and returns shouldExit=false to continue
func Daemonize(cfg *config.Config) (shouldExit bool, pid int, err error) {
	// Check if we're already the daemonized process
	if os.Getenv(daemonEnvVar) == "1" {
		// We are the child - continue running as daemon
		return false, os.Getpid(), nil
	}

	// We are the parent - spawn the daemon child
	executable, err := os.Executable()
	if err != nil {
		return false, 0, fmt.Errorf("get executable path: %w", err)
	}

	cmd := exec.Command(executable, os.Args[1:]...)
	cmd.Env = append(os.Environ(), daemonEnvVar+"=1")

	// Detach from terminal
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	// Create a new session (setsid equivalent)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	if err := cmd.Start(); err != nil {
		return false, 0, fmt.Errorf("start daemon: %w", err)
	}

	childPID := cmd.Process.Pid

	// Wait for socket to appear before declaring success
	if err := waitForSocketReady(cfg.Paths.Socket, socketWaitTimeout); err != nil {
		// Don't fail - daemon may still be starting up
		fmt.Printf("Started daemon (pid %d) - socket not yet available\n", childPID)
	} else {
		fmt.Printf("Started daemon (pid %d)\n", childPID)
	}

	return true, childPID, nil
}

// IsDaemonized returns true if the current process is running as a daemonized child.
func IsDaemonized() bool {
	return os.Getenv(daemonEnvVar) == "1"
}

// waitForSocketReady waits for the socket to accept connections.
func waitForSocketReady(socketPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("unix", socketPath, socketCheckInterval)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(socketCheckInterval)
	}
	return fmt.Errorf("socket not available after %v", timeout)
}
