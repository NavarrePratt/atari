package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// PIDFile manages a PID file with flock-based locking to prevent concurrent daemon instances.
type PIDFile struct {
	path string
	file *os.File
}

// NewPIDFile creates a PIDFile instance for the given path.
func NewPIDFile(path string) *PIDFile {
	return &PIDFile{path: path}
}

// Path returns the PID file path.
func (p *PIDFile) Path() string {
	return p.path
}

// Write creates and locks the PID file, writing the current process ID.
// Returns an error if another process holds the lock.
func (p *PIDFile) Write() error {
	// Ensure directory exists
	dir := filepath.Dir(p.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create pid directory: %w", err)
	}

	// Open file for writing (create if not exists)
	file, err := os.OpenFile(p.path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return fmt.Errorf("open pid file: %w", err)
	}

	// Try to acquire exclusive lock (non-blocking)
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if err == syscall.EWOULDBLOCK {
			return fmt.Errorf("daemon already running (pid file locked)")
		}
		return fmt.Errorf("lock pid file: %w", err)
	}

	// Truncate and write PID
	if err := file.Truncate(0); err != nil {
		p.unlockAndClose(file)
		return fmt.Errorf("truncate pid file: %w", err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		p.unlockAndClose(file)
		return fmt.Errorf("seek pid file: %w", err)
	}
	if _, err := fmt.Fprintf(file, "%d\n", os.Getpid()); err != nil {
		p.unlockAndClose(file)
		return fmt.Errorf("write pid: %w", err)
	}
	if err := file.Sync(); err != nil {
		p.unlockAndClose(file)
		return fmt.Errorf("sync pid file: %w", err)
	}

	p.file = file
	return nil
}

// Read returns the PID from the file, or 0 if the file doesn't exist or is invalid.
func (p *PIDFile) Read() int {
	data, err := os.ReadFile(p.path)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return pid
}

// Remove releases the lock and removes the PID file.
func (p *PIDFile) Remove() error {
	if p.file != nil {
		// Release lock and close
		p.unlockAndClose(p.file)
		p.file = nil
	}
	// Remove file (ignore error if already gone)
	_ = os.Remove(p.path)
	return nil
}

// unlockAndClose releases the flock and closes the file.
func (p *PIDFile) unlockAndClose(file *os.File) {
	_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	_ = file.Close()
}

// IsProcessRunning checks if the given PID represents a running process.
// On Unix, this sends signal 0 to check process existence.
func IsProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds - send signal 0 to check existence
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// IsRunning checks if the daemon is running based on the PID file.
// Returns true if the PID file exists and the process is alive.
func (p *PIDFile) IsRunning() bool {
	pid := p.Read()
	return IsProcessRunning(pid)
}

// CleanupStale removes stale PID and socket files if the daemon is not running.
// This handles crash recovery where files were left behind.
func (p *PIDFile) CleanupStale(socketPath string) {
	if p.IsRunning() {
		return
	}
	// Remove stale PID file (ignore errors - file may not exist)
	_ = os.Remove(p.path)
	// Remove stale socket (ignore errors - file may not exist)
	if socketPath != "" {
		_ = os.Remove(socketPath)
	}
}
