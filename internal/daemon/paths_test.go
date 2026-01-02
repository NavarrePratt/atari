package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/npratt/atari/internal/config"
)

func TestResolvePaths_RelativePaths(t *testing.T) {
	tmp := t.TempDir()

	paths := config.PathsConfig{
		State:  ".atari/state.json",
		Log:    ".atari/atari.log",
		Socket: ".atari/atari.sock",
		PID:    ".atari/atari.pid",
	}

	resolved, err := ResolvePaths(paths, tmp)
	if err != nil {
		t.Fatalf("ResolvePaths() error: %v", err)
	}

	expected := config.PathsConfig{
		State:  filepath.Join(tmp, ".atari/state.json"),
		Log:    filepath.Join(tmp, ".atari/atari.log"),
		Socket: filepath.Join(tmp, ".atari/atari.sock"),
		PID:    filepath.Join(tmp, ".atari/atari.pid"),
	}

	if resolved.State != expected.State {
		t.Errorf("State: expected %q, got %q", expected.State, resolved.State)
	}
	if resolved.Log != expected.Log {
		t.Errorf("Log: expected %q, got %q", expected.Log, resolved.Log)
	}
	if resolved.Socket != expected.Socket {
		t.Errorf("Socket: expected %q, got %q", expected.Socket, resolved.Socket)
	}
	if resolved.PID != expected.PID {
		t.Errorf("PID: expected %q, got %q", expected.PID, resolved.PID)
	}
}

func TestResolvePaths_AbsolutePaths(t *testing.T) {
	tmp := t.TempDir()

	// Absolute paths should remain unchanged
	paths := config.PathsConfig{
		State:  "/absolute/state.json",
		Log:    "/absolute/atari.log",
		Socket: "/absolute/atari.sock",
		PID:    "/absolute/atari.pid",
	}

	resolved, err := ResolvePaths(paths, tmp)
	if err != nil {
		t.Fatalf("ResolvePaths() error: %v", err)
	}

	if resolved.State != paths.State {
		t.Errorf("State: expected %q, got %q (should remain absolute)", paths.State, resolved.State)
	}
	if resolved.Log != paths.Log {
		t.Errorf("Log: expected %q, got %q", paths.Log, resolved.Log)
	}
}

func TestResolvePaths_MixedPaths(t *testing.T) {
	tmp := t.TempDir()

	paths := config.PathsConfig{
		State:  "relative/state.json",
		Log:    "/absolute/atari.log",
		Socket: "relative/atari.sock",
		PID:    "/absolute/atari.pid",
	}

	resolved, err := ResolvePaths(paths, tmp)
	if err != nil {
		t.Fatalf("ResolvePaths() error: %v", err)
	}

	if resolved.State != filepath.Join(tmp, "relative/state.json") {
		t.Errorf("State should be resolved to absolute")
	}
	if resolved.Log != "/absolute/atari.log" {
		t.Errorf("Log should remain absolute")
	}
}

func TestFindProjectRoot_WithGit(t *testing.T) {
	tmp := t.TempDir()

	// Create .git directory
	gitDir := filepath.Join(tmp, ".git")
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatalf("create .git: %v", err)
	}

	// Create a subdirectory
	subDir := filepath.Join(tmp, "sub", "dir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("create subdir: %v", err)
	}

	// Find project root from subdirectory
	root := FindProjectRoot(subDir)
	if root != tmp {
		t.Errorf("expected root %q, got %q", tmp, root)
	}
}

func TestFindProjectRoot_WithBeads(t *testing.T) {
	tmp := t.TempDir()

	// Create .beads directory
	beadsDir := filepath.Join(tmp, ".beads")
	if err := os.Mkdir(beadsDir, 0755); err != nil {
		t.Fatalf("create .beads: %v", err)
	}

	// Create a subdirectory
	subDir := filepath.Join(tmp, "deep", "nested", "dir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("create subdir: %v", err)
	}

	root := FindProjectRoot(subDir)
	if root != tmp {
		t.Errorf("expected root %q, got %q", tmp, root)
	}
}

func TestFindProjectRoot_NoMarker(t *testing.T) {
	tmp := t.TempDir()

	// No markers - should return start directory
	subDir := filepath.Join(tmp, "sub")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("create subdir: %v", err)
	}

	root := FindProjectRoot(subDir)

	// Convert to absolute for comparison
	absSubDir, _ := filepath.Abs(subDir)
	if root != absSubDir {
		t.Errorf("expected %q (start dir), got %q", absSubDir, root)
	}
}

func TestFindProjectRoot_FromRoot(t *testing.T) {
	tmp := t.TempDir()

	// Create .git directory
	gitDir := filepath.Join(tmp, ".git")
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatalf("create .git: %v", err)
	}

	// Find from root itself
	root := FindProjectRoot(tmp)
	if root != tmp {
		t.Errorf("expected root %q, got %q", tmp, root)
	}
}

func TestWriteReadDaemonInfo(t *testing.T) {
	tmp := t.TempDir()
	infoPath := filepath.Join(tmp, ".atari", "daemon.json")

	info := &DaemonInfo{
		SocketPath: "/path/to/socket",
		PIDPath:    "/path/to/pid",
		LogPath:    "/path/to/log",
		StartTime:  time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		PID:        12345,
	}

	// Write
	if err := WriteDaemonInfo(infoPath, info); err != nil {
		t.Fatalf("WriteDaemonInfo() error: %v", err)
	}

	// Read back
	readInfo, err := ReadDaemonInfo(infoPath)
	if err != nil {
		t.Fatalf("ReadDaemonInfo() error: %v", err)
	}

	if readInfo.SocketPath != info.SocketPath {
		t.Errorf("SocketPath: expected %q, got %q", info.SocketPath, readInfo.SocketPath)
	}
	if readInfo.PIDPath != info.PIDPath {
		t.Errorf("PIDPath: expected %q, got %q", info.PIDPath, readInfo.PIDPath)
	}
	if readInfo.LogPath != info.LogPath {
		t.Errorf("LogPath: expected %q, got %q", info.LogPath, readInfo.LogPath)
	}
	if readInfo.PID != info.PID {
		t.Errorf("PID: expected %d, got %d", info.PID, readInfo.PID)
	}
	if !readInfo.StartTime.Equal(info.StartTime) {
		t.Errorf("StartTime: expected %v, got %v", info.StartTime, readInfo.StartTime)
	}
}

func TestWriteDaemonInfo_CreatesDirectory(t *testing.T) {
	tmp := t.TempDir()

	// Path to a non-existent directory
	infoPath := filepath.Join(tmp, "nested", "dirs", "daemon.json")

	info := &DaemonInfo{
		SocketPath: "/path/to/socket",
		PID:        12345,
	}

	if err := WriteDaemonInfo(infoPath, info); err != nil {
		t.Fatalf("WriteDaemonInfo() error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(infoPath); os.IsNotExist(err) {
		t.Error("daemon.json should have been created")
	}
}

func TestReadDaemonInfo_NotFound(t *testing.T) {
	_, err := ReadDaemonInfo("/nonexistent/daemon.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestRemoveDaemonInfo(t *testing.T) {
	tmp := t.TempDir()
	infoPath := filepath.Join(tmp, "daemon.json")

	// Create file
	if err := os.WriteFile(infoPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("create file: %v", err)
	}

	// Remove
	if err := RemoveDaemonInfo(infoPath); err != nil {
		t.Errorf("RemoveDaemonInfo() error: %v", err)
	}

	// Should be gone
	if _, err := os.Stat(infoPath); !os.IsNotExist(err) {
		t.Error("file should have been removed")
	}
}

func TestRemoveDaemonInfo_NotFound(t *testing.T) {
	// Removing nonexistent file should not error
	if err := RemoveDaemonInfo("/nonexistent/daemon.json"); err != nil {
		t.Errorf("RemoveDaemonInfo() should not error for missing file: %v", err)
	}
}

func TestFindDaemonInfo_Found(t *testing.T) {
	tmp := t.TempDir()

	// Create .git marker
	if err := os.Mkdir(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatalf("create .git: %v", err)
	}

	// Create .atari directory and daemon.json
	atariDir := filepath.Join(tmp, ".atari")
	if err := os.Mkdir(atariDir, 0755); err != nil {
		t.Fatalf("create .atari: %v", err)
	}

	info := &DaemonInfo{
		SocketPath: "/path/to/socket",
		PID:        12345,
	}
	infoPath := filepath.Join(atariDir, "daemon.json")
	if err := WriteDaemonInfo(infoPath, info); err != nil {
		t.Fatalf("write daemon info: %v", err)
	}

	// Create subdirectory and search from there
	subDir := filepath.Join(tmp, "sub", "dir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("create subdir: %v", err)
	}

	foundInfo, err := FindDaemonInfo(subDir)
	if err != nil {
		t.Fatalf("FindDaemonInfo() error: %v", err)
	}

	if foundInfo.SocketPath != info.SocketPath {
		t.Errorf("SocketPath: expected %q, got %q", info.SocketPath, foundInfo.SocketPath)
	}
	if foundInfo.PID != info.PID {
		t.Errorf("PID: expected %d, got %d", info.PID, foundInfo.PID)
	}
}

func TestFindDaemonInfo_NotFound(t *testing.T) {
	tmp := t.TempDir()

	// Create .git marker but no daemon.json
	if err := os.Mkdir(filepath.Join(tmp, ".git"), 0755); err != nil {
		t.Fatalf("create .git: %v", err)
	}

	_, err := FindDaemonInfo(tmp)
	if err == nil {
		t.Error("expected error when daemon.json not found")
	}
}

func TestDaemonInfoPath(t *testing.T) {
	path := DaemonInfoPath("/project")
	expected := "/project/.atari/daemon.json"
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}
