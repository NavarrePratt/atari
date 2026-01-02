package daemon

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestNewPIDFile(t *testing.T) {
	path := "/tmp/test.pid"
	pf := NewPIDFile(path)

	if pf.Path() != path {
		t.Errorf("expected path %s, got %s", path, pf.Path())
	}
}

func TestPIDFile_WriteAndRead(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.pid")
	pf := NewPIDFile(path)

	// Write PID file
	if err := pf.Write(); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	defer func() { _ = pf.Remove() }()

	// Read should return current PID
	pid := pf.Read()
	if pid != os.Getpid() {
		t.Errorf("expected pid %d, got %d", os.Getpid(), pid)
	}

	// File should exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("PID file should exist after Write()")
	}
}

func TestPIDFile_WriteCreatesDirectory(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "subdir", "test.pid")
	pf := NewPIDFile(path)

	if err := pf.Write(); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	defer func() { _ = pf.Remove() }()

	// Directory should be created
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("directory should be created by Write()")
	}
}

func TestPIDFile_Remove(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.pid")
	pf := NewPIDFile(path)

	if err := pf.Write(); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	if err := pf.Remove(); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	// File should be gone
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("PID file should be removed after Remove()")
	}
}

func TestPIDFile_ReadNonExistent(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "nonexistent.pid")
	pf := NewPIDFile(path)

	pid := pf.Read()
	if pid != 0 {
		t.Errorf("expected 0 for nonexistent file, got %d", pid)
	}
}

func TestPIDFile_ReadInvalidContent(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.pid")

	// Write invalid content
	if err := os.WriteFile(path, []byte("not-a-number\n"), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	pf := NewPIDFile(path)
	pid := pf.Read()
	if pid != 0 {
		t.Errorf("expected 0 for invalid content, got %d", pid)
	}
}

func TestPIDFile_FlockPreventsDoubleLock(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.pid")

	pf1 := NewPIDFile(path)
	pf2 := NewPIDFile(path)

	// First lock should succeed
	if err := pf1.Write(); err != nil {
		t.Fatalf("first Write() error: %v", err)
	}
	defer func() { _ = pf1.Remove() }()

	// Second lock should fail
	err := pf2.Write()
	if err == nil {
		_ = pf2.Remove()
		t.Fatal("expected error for second Write(), got nil")
	}
}

func TestIsProcessRunning_CurrentProcess(t *testing.T) {
	if !IsProcessRunning(os.Getpid()) {
		t.Error("current process should be running")
	}
}

func TestIsProcessRunning_InvalidPID(t *testing.T) {
	if IsProcessRunning(0) {
		t.Error("PID 0 should not be running")
	}
	if IsProcessRunning(-1) {
		t.Error("negative PID should not be running")
	}
}

func TestIsProcessRunning_NonExistentPID(t *testing.T) {
	// Use a very high PID that is unlikely to exist
	// Note: This test may be flaky on systems with many processes
	if IsProcessRunning(999999999) {
		t.Error("nonexistent PID should not be running")
	}
}

func TestPIDFile_IsRunning(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.pid")
	pf := NewPIDFile(path)

	// Write current PID
	if err := pf.Write(); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	defer func() { _ = pf.Remove() }()

	// Should report as running
	if !pf.IsRunning() {
		t.Error("expected IsRunning() to return true for current process")
	}
}

func TestPIDFile_IsRunning_DeadProcess(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.pid")

	// Write a PID that doesn't exist
	if err := os.WriteFile(path, []byte("999999999\n"), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	pf := NewPIDFile(path)
	if pf.IsRunning() {
		t.Error("expected IsRunning() to return false for dead process")
	}
}

func TestPIDFile_CleanupStale_DeadProcess(t *testing.T) {
	tmp := t.TempDir()
	pidPath := filepath.Join(tmp, "test.pid")
	sockPath := filepath.Join(tmp, "test.sock")

	// Create stale files
	if err := os.WriteFile(pidPath, []byte("999999999\n"), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	if err := os.WriteFile(sockPath, []byte(""), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	pf := NewPIDFile(pidPath)
	pf.CleanupStale(sockPath)

	// Both files should be removed
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("PID file should be removed")
	}
	if _, err := os.Stat(sockPath); !os.IsNotExist(err) {
		t.Error("socket file should be removed")
	}
}

func TestPIDFile_CleanupStale_LiveProcess(t *testing.T) {
	tmp := t.TempDir()
	pidPath := filepath.Join(tmp, "test.pid")
	sockPath := filepath.Join(tmp, "test.sock")

	// Create files with current PID (live process)
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(os.Getpid())+"\n"), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	if err := os.WriteFile(sockPath, []byte(""), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	pf := NewPIDFile(pidPath)
	pf.CleanupStale(sockPath)

	// Files should NOT be removed (process is alive)
	if _, err := os.Stat(pidPath); os.IsNotExist(err) {
		t.Error("PID file should NOT be removed for live process")
	}
	if _, err := os.Stat(sockPath); os.IsNotExist(err) {
		t.Error("socket file should NOT be removed for live process")
	}
}

func TestPIDFile_CleanupStale_EmptySocketPath(t *testing.T) {
	tmp := t.TempDir()
	pidPath := filepath.Join(tmp, "test.pid")

	// Create stale PID file
	if err := os.WriteFile(pidPath, []byte("999999999\n"), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	pf := NewPIDFile(pidPath)
	pf.CleanupStale("") // Empty socket path

	// PID file should be removed
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("PID file should be removed")
	}
}
