package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

const (
	// maxMessageSize is the maximum size of a JSON-RPC message (1MB).
	maxMessageSize = 1024 * 1024
	// readTimeout is the timeout for reading a request from a client.
	readTimeout = 30 * time.Second
	// socketPermissions are the file permissions for the Unix socket.
	socketPermissions = 0600
)

// Start begins listening on the Unix socket and serving requests.
// It blocks until the context is cancelled or an error occurs.
func (d *Daemon) Start(ctx context.Context) error {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return fmt.Errorf("daemon already running")
	}
	d.mu.Unlock()

	// Clean up stale socket if it exists
	_ = os.Remove(d.sockPath)

	// Create listener
	listener, err := net.Listen("unix", d.sockPath)
	if err != nil {
		return fmt.Errorf("listen on socket: %w", err)
	}

	// Set socket permissions
	if err := os.Chmod(d.sockPath, socketPermissions); err != nil {
		_ = listener.Close()
		return fmt.Errorf("set socket permissions: %w", err)
	}

	d.mu.Lock()
	d.listener = listener
	d.running = true
	d.startTime = time.Now()
	d.mu.Unlock()

	d.logger.Info("daemon started", "socket", d.sockPath)

	// Start accept loop
	go d.serve(ctx)

	// Wait for context cancellation
	<-ctx.Done()

	return d.Stop()
}

// Stop closes the listener and cleans up resources.
func (d *Daemon) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return nil
	}

	d.running = false

	if d.listener != nil {
		if err := d.listener.Close(); err != nil {
			d.logger.Error("error closing listener", "error", err)
		}
		d.listener = nil
	}

	// Remove socket file
	_ = os.Remove(d.sockPath)

	d.logger.Info("daemon stopped")
	return nil
}

// serve accepts connections and dispatches them to handlers.
func (d *Daemon) serve(ctx context.Context) {
	for {
		conn, err := d.listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				// Check if we're shutting down
				d.mu.RLock()
				running := d.running
				d.mu.RUnlock()
				if !running {
					return
				}
				d.logger.Error("accept error", "error", err)
				continue
			}
		}

		go d.handleConnection(ctx, conn)
	}
}

// handleConnection reads a request, dispatches it, and writes the response.
func (d *Daemon) handleConnection(ctx context.Context, conn net.Conn) {
	defer func() { _ = conn.Close() }()

	// Set read deadline
	if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
		d.logger.Error("set read deadline error", "error", err)
		return
	}

	// Use limited reader to prevent DoS
	limitedReader := io.LimitReader(conn, maxMessageSize)
	decoder := json.NewDecoder(limitedReader)
	encoder := json.NewEncoder(conn)

	var req Request
	if err := decoder.Decode(&req); err != nil {
		_ = encoder.Encode(Response{Error: fmt.Sprintf("decode error: %v", err)})
		return
	}

	resp := d.handleRequest(ctx, &req)
	resp.ID = req.ID
	_ = encoder.Encode(resp)
}
