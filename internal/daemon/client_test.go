package daemon

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mockServer starts a mock daemon server that returns canned responses.
func mockServer(t *testing.T, sockPath string, handler func(req Request) Response) func() {
	t.Helper()

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	done := make(chan struct{})
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-done:
					return
				default:
					continue
				}
			}

			go func(c net.Conn) {
				defer func() { _ = c.Close() }()

				var req Request
				if err := json.NewDecoder(c).Decode(&req); err != nil {
					return
				}

				resp := handler(req)
				resp.ID = req.ID
				_ = json.NewEncoder(c).Encode(resp)
			}(conn)
		}
	}()

	return func() {
		close(done)
		_ = listener.Close()
		_ = os.Remove(sockPath)
	}
}

func TestClient_Status_Success(t *testing.T) {
	sockPath := shortSocketPath(t)

	cleanup := mockServer(t, sockPath, func(req Request) Response {
		if req.Method != "status" {
			return Response{Error: "unexpected method"}
		}
		return Response{
			Result: StatusResponse{
				Status:      "running",
				CurrentBead: "bd-001",
				Uptime:      "1h30m",
				StartTime:   "2024-01-15T10:00:00Z",
				Stats: StatusStats{
					Iteration: 5,
					TotalSeen: 10,
					Completed: 8,
					Failed:    1,
					Abandoned: 1,
					InBackoff: 0,
				},
			},
		}
	})
	defer cleanup()

	client := NewClient(sockPath)
	status, err := client.Status()
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}

	if status.Status != "running" {
		t.Errorf("expected status 'running', got %q", status.Status)
	}
	if status.CurrentBead != "bd-001" {
		t.Errorf("expected current_bead 'bd-001', got %q", status.CurrentBead)
	}
	if status.Stats.Completed != 8 {
		t.Errorf("expected completed 8, got %d", status.Stats.Completed)
	}
}

func TestClient_Pause_Success(t *testing.T) {
	sockPath := shortSocketPath(t)

	cleanup := mockServer(t, sockPath, func(req Request) Response {
		if req.Method != "pause" {
			return Response{Error: "unexpected method"}
		}
		return Response{Result: "pausing"}
	})
	defer cleanup()

	client := NewClient(sockPath)
	err := client.Pause()
	if err != nil {
		t.Errorf("Pause() error: %v", err)
	}
}

func TestClient_Resume_Success(t *testing.T) {
	sockPath := shortSocketPath(t)

	cleanup := mockServer(t, sockPath, func(req Request) Response {
		if req.Method != "resume" {
			return Response{Error: "unexpected method"}
		}
		return Response{Result: "resuming"}
	})
	defer cleanup()

	client := NewClient(sockPath)
	err := client.Resume()
	if err != nil {
		t.Errorf("Resume() error: %v", err)
	}
}

func TestClient_Stop_Success(t *testing.T) {
	sockPath := shortSocketPath(t)

	cleanup := mockServer(t, sockPath, func(req Request) Response {
		if req.Method != "stop" {
			return Response{Error: "unexpected method"}
		}
		return Response{Result: "stopping"}
	})
	defer cleanup()

	client := NewClient(sockPath)
	err := client.Stop(false)
	if err != nil {
		t.Errorf("Stop() error: %v", err)
	}
}

func TestClient_Stop_Force(t *testing.T) {
	sockPath := shortSocketPath(t)

	var receivedForce bool
	cleanup := mockServer(t, sockPath, func(req Request) Response {
		if req.Method != "stop" {
			return Response{Error: "unexpected method"}
		}
		// Check if force param was received
		if params, ok := req.Params.(map[string]interface{}); ok {
			if f, ok := params["force"].(bool); ok {
				receivedForce = f
			}
		}
		return Response{Result: "stopping"}
	})
	defer cleanup()

	client := NewClient(sockPath)
	err := client.Stop(true)
	if err != nil {
		t.Errorf("Stop(true) error: %v", err)
	}
	if !receivedForce {
		t.Error("expected force=true to be received by server")
	}
}

func TestClient_IsRunning_True(t *testing.T) {
	sockPath := shortSocketPath(t)

	cleanup := mockServer(t, sockPath, func(req Request) Response {
		return Response{Result: "ok"}
	})
	defer cleanup()

	client := NewClient(sockPath)
	if !client.IsRunning() {
		t.Error("expected IsRunning() to return true")
	}
}

func TestClient_IsRunning_False(t *testing.T) {
	client := NewClient("/tmp/nonexistent.sock")
	if client.IsRunning() {
		t.Error("expected IsRunning() to return false for nonexistent socket")
	}
}

func TestClient_SocketNotFound(t *testing.T) {
	client := NewClient("/tmp/nonexistent.sock")
	_, err := client.Status()
	if err == nil {
		t.Fatal("expected error for nonexistent socket")
	}

	expected := "daemon not running (socket not found)"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

func TestClient_DaemonError(t *testing.T) {
	sockPath := shortSocketPath(t)

	cleanup := mockServer(t, sockPath, func(req Request) Response {
		return Response{Error: "no controller available"}
	})
	defer cleanup()

	client := NewClient(sockPath)
	_, err := client.Status()
	if err == nil {
		t.Fatal("expected error for daemon error response")
	}

	expected := "daemon error: no controller available"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

func TestClient_SetTimeout(t *testing.T) {
	client := NewClient("/tmp/test.sock")

	// Check default timeout
	if client.timeout != DefaultClientTimeout {
		t.Errorf("expected default timeout %v, got %v", DefaultClientTimeout, client.timeout)
	}

	// Set new timeout
	client.SetTimeout(10 * time.Second)
	if client.timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", client.timeout)
	}
}

func TestClient_ConnectionRefused(t *testing.T) {
	// Create a socket file but don't listen on it
	tmp := t.TempDir()
	sockPath := filepath.Join(tmp, "test.sock")

	// Create the socket file (not a real socket, just a file)
	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("create socket: %v", err)
	}
	// Close immediately to simulate connection refused
	_ = listener.Close()

	client := NewClient(sockPath)
	_, err = client.Status()
	if err == nil {
		t.Fatal("expected error for closed socket")
	}
	// Should get connection refused error
	if err.Error() != "daemon not running (connection refused)" &&
		err.Error() != "daemon not running (socket not found)" {
		// On some systems, closed socket shows as not found
		t.Logf("got error: %v (acceptable)", err)
	}
}
