package daemon

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/npratt/atari/internal/config"
)

// waitForSocket waits for the socket to be ready to accept connections.
func waitForSocket(t *testing.T, socketPath string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("unix", socketPath, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("socket did not become ready within %v", timeout)
}

// shortSocketPath creates a short socket path to avoid Unix socket length limits.
// macOS has a 104 byte limit, Linux has 108 bytes.
func shortSocketPath(t *testing.T) string {
	t.Helper()
	// Create a temp file to get a unique name, then delete it
	f, err := os.CreateTemp("", "sock")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	path := f.Name()
	_ = f.Close()
	_ = os.Remove(path)
	// Add cleanup to remove socket at test end
	t.Cleanup(func() { _ = os.Remove(path) })
	return path
}

func TestDaemon_StartStop(t *testing.T) {
	tmp := t.TempDir()
	cfg := config.Default()
	cfg.Paths.Socket = filepath.Join(tmp, "test.sock")

	d := New(cfg, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())

	// Start daemon in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Start(ctx)
	}()

	// Wait for socket to appear
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(cfg.Paths.Socket); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !d.Running() {
		t.Error("daemon should be running after Start")
	}

	// Verify socket exists
	if _, err := os.Stat(cfg.Paths.Socket); os.IsNotExist(err) {
		t.Error("socket file should exist after Start")
	}

	// Stop daemon
	cancel()

	// Wait for shutdown
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Start() returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("daemon did not stop within timeout")
	}

	if d.Running() {
		t.Error("daemon should not be running after Stop")
	}
}

func TestDaemon_StartAlreadyRunning(t *testing.T) {
	tmp := t.TempDir()
	cfg := config.Default()
	cfg.Paths.Socket = filepath.Join(tmp, "test.sock")

	d := New(cfg, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start daemon
	go func() {
		_ = d.Start(ctx)
	}()

	// Wait for socket to appear
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(cfg.Paths.Socket); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Try to start again - should fail
	err := d.Start(ctx)
	if err == nil {
		t.Error("expected error when starting already running daemon")
	}
}

func TestDaemon_SocketPermissions(t *testing.T) {
	tmp := t.TempDir()
	cfg := config.Default()
	cfg.Paths.Socket = filepath.Join(tmp, "test.sock")

	d := New(cfg, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = d.Start(ctx)
	}()

	// Wait for socket to appear
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(cfg.Paths.Socket); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Check permissions
	info, err := os.Stat(cfg.Paths.Socket)
	if err != nil {
		t.Fatalf("stat socket: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != socketPermissions {
		t.Errorf("expected socket permissions %o, got %o", socketPermissions, perm)
	}
}

func TestDaemon_HandleConnection_UnknownMethod(t *testing.T) {
	cfg := config.Default()
	cfg.Paths.Socket = shortSocketPath(t)

	d := New(cfg, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = d.Start(ctx)
	}()

	// Wait for socket to be ready
	waitForSocket(t, cfg.Paths.Socket, 2*time.Second)

	// Connect and send request
	conn, err := net.Dial("unix", cfg.Paths.Socket)
	if err != nil {
		t.Fatalf("dial socket: %v", err)
	}
	defer func() { _ = conn.Close() }()

	req := Request{Method: "unknown_method", ID: 1}
	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	if err := encoder.Encode(req); err != nil {
		t.Fatalf("encode request: %v", err)
	}

	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error == "" {
		t.Error("expected error for unknown method")
	}
	if resp.ID != 1 {
		t.Errorf("expected ID 1, got %d", resp.ID)
	}
}

func TestDaemon_HandleConnection_InvalidJSON(t *testing.T) {
	cfg := config.Default()
	cfg.Paths.Socket = shortSocketPath(t)

	d := New(cfg, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = d.Start(ctx)
	}()

	// Wait for socket to be ready
	waitForSocket(t, cfg.Paths.Socket, 2*time.Second)

	// Connect and send invalid JSON
	conn, err := net.Dial("unix", cfg.Paths.Socket)
	if err != nil {
		t.Fatalf("dial socket: %v", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.Write([]byte("not json\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	var resp Response
	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error == "" {
		t.Error("expected error for invalid JSON")
	}
}

func TestDaemon_HandleStatus_NoController(t *testing.T) {
	cfg := config.Default()
	cfg.Paths.Socket = shortSocketPath(t)

	// Create daemon without controller
	d := New(cfg, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = d.Start(ctx)
	}()

	// Wait for socket to be ready
	waitForSocket(t, cfg.Paths.Socket, 2*time.Second)

	// Send status request
	conn, err := net.Dial("unix", cfg.Paths.Socket)
	if err != nil {
		t.Fatalf("dial socket: %v", err)
	}
	defer func() { _ = conn.Close() }()

	req := Request{Method: "status", ID: 1}
	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	if err := encoder.Encode(req); err != nil {
		t.Fatalf("encode request: %v", err)
	}

	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error == "" {
		t.Error("expected error when no controller available")
	}
}

func TestDaemon_HandlePause_NoController(t *testing.T) {
	cfg := config.Default()
	cfg.Paths.Socket = shortSocketPath(t)

	d := New(cfg, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = d.Start(ctx)
	}()

	// Wait for socket to be ready
	waitForSocket(t, cfg.Paths.Socket, 2*time.Second)

	conn, err := net.Dial("unix", cfg.Paths.Socket)
	if err != nil {
		t.Fatalf("dial socket: %v", err)
	}
	defer func() { _ = conn.Close() }()

	req := Request{Method: "pause", ID: 1}
	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	if err := encoder.Encode(req); err != nil {
		t.Fatalf("encode request: %v", err)
	}

	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error == "" {
		t.Error("expected error when no controller available")
	}
}

func TestDaemon_HandleResume_NoController(t *testing.T) {
	cfg := config.Default()
	cfg.Paths.Socket = shortSocketPath(t)

	d := New(cfg, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = d.Start(ctx)
	}()

	// Wait for socket to be ready
	waitForSocket(t, cfg.Paths.Socket, 2*time.Second)

	conn, err := net.Dial("unix", cfg.Paths.Socket)
	if err != nil {
		t.Fatalf("dial socket: %v", err)
	}
	defer func() { _ = conn.Close() }()

	req := Request{Method: "resume", ID: 1}
	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	if err := encoder.Encode(req); err != nil {
		t.Fatalf("encode request: %v", err)
	}

	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error == "" {
		t.Error("expected error when no controller available")
	}
}

func TestDaemon_HandleStop_NoController(t *testing.T) {
	cfg := config.Default()
	cfg.Paths.Socket = shortSocketPath(t)

	d := New(cfg, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = d.Start(ctx)
	}()

	// Wait for socket to be ready
	waitForSocket(t, cfg.Paths.Socket, 2*time.Second)

	conn, err := net.Dial("unix", cfg.Paths.Socket)
	if err != nil {
		t.Fatalf("dial socket: %v", err)
	}
	defer func() { _ = conn.Close() }()

	req := Request{Method: "stop", ID: 1}
	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	if err := encoder.Encode(req); err != nil {
		t.Fatalf("encode request: %v", err)
	}

	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Error == "" {
		t.Error("expected error when no controller available")
	}
}

func TestDaemon_StopIdempotent(t *testing.T) {
	tmp := t.TempDir()
	cfg := config.Default()
	cfg.Paths.Socket = filepath.Join(tmp, "test.sock")

	d := New(cfg, nil, nil)

	// Stop on non-running daemon should be safe
	if err := d.Stop(); err != nil {
		t.Errorf("Stop() on non-running daemon returned error: %v", err)
	}

	// Multiple stops should be safe
	if err := d.Stop(); err != nil {
		t.Errorf("second Stop() returned error: %v", err)
	}
}

func TestDaemon_CleanupStaleSocket(t *testing.T) {
	tmp := t.TempDir()
	cfg := config.Default()
	cfg.Paths.Socket = filepath.Join(tmp, "test.sock")

	// Create a stale socket file
	if err := os.WriteFile(cfg.Paths.Socket, []byte("stale"), 0644); err != nil {
		t.Fatalf("create stale socket: %v", err)
	}

	d := New(cfg, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = d.Start(ctx)
	}()

	// Wait for socket to appear - the stale file should be replaced
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		info, err := os.Stat(cfg.Paths.Socket)
		if err == nil && info.Mode().Type() == os.ModeSocket {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Verify it's actually a socket now
	info, err := os.Stat(cfg.Paths.Socket)
	if err != nil {
		t.Fatalf("stat socket: %v", err)
	}
	if info.Mode().Type() != os.ModeSocket {
		t.Error("expected socket file, got regular file")
	}
}
