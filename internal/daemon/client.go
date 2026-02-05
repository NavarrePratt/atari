package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
	"time"
)

const (
	// DefaultClientTimeout is the default timeout for client operations.
	DefaultClientTimeout = 5 * time.Second
)

// Client connects to the daemon via Unix socket.
type Client struct {
	sockPath string
	timeout  time.Duration
}

// NewClient creates a new daemon client.
func NewClient(sockPath string) *Client {
	return &Client{
		sockPath: sockPath,
		timeout:  DefaultClientTimeout,
	}
}

// SetTimeout sets the timeout for client operations.
func (c *Client) SetTimeout(d time.Duration) {
	c.timeout = d
}

// call sends a JSON-RPC request to the daemon and returns the response.
func (c *Client) call(method string, params any) (*Response, error) {
	conn, err := net.DialTimeout("unix", c.sockPath, c.timeout)
	if err != nil {
		return nil, c.wrapConnError(err)
	}
	defer func() { _ = conn.Close() }()

	if err := conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		return nil, fmt.Errorf("set deadline: %w", err)
	}

	req := Request{Method: method, Params: params}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("daemon error: %s", resp.Error)
	}

	return &resp, nil
}

// wrapConnError converts connection errors to user-friendly messages.
func (c *Client) wrapConnError(err error) error {
	// Check for syscall errors that indicate specific conditions
	var sysErr syscall.Errno
	if errors.As(err, &sysErr) {
		switch sysErr {
		case syscall.ENOENT:
			return errors.New("daemon not running (socket not found)")
		case syscall.ECONNREFUSED:
			return errors.New("daemon not running (connection refused)")
		}
	}

	// Fallback check for os.IsNotExist
	if os.IsNotExist(err) {
		return errors.New("daemon not running (socket not found)")
	}

	if errors.Is(err, os.ErrDeadlineExceeded) {
		return errors.New("daemon request timed out")
	}

	return fmt.Errorf("connect to daemon: %w", err)
}

// Status returns the current daemon status.
func (c *Client) Status() (*StatusResponse, error) {
	resp, err := c.call("status", nil)
	if err != nil {
		return nil, err
	}

	// Re-marshal and unmarshal to convert the result to StatusResponse
	data, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}

	var status StatusResponse
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, fmt.Errorf("unmarshal status: %w", err)
	}

	return &status, nil
}

// Pause requests the daemon to pause processing.
func (c *Client) Pause() error {
	_, err := c.call("pause", nil)
	return err
}

// Resume requests the daemon to resume processing.
func (c *Client) Resume() error {
	_, err := c.call("resume", nil)
	return err
}

// Stop requests the daemon to stop. If force is true, stops immediately.
func (c *Client) Stop(force bool) error {
	params := StopParams{Force: force}
	_, err := c.call("stop", params)
	return err
}

// Retry requests the daemon to retry a bead.
// If beadID is empty, retries the currently stalled bead.
func (c *Client) Retry(beadID string) error {
	params := RetryParams{BeadID: beadID}
	_, err := c.call("retry", params)
	return err
}

// IsRunning checks if the daemon is running by attempting to connect.
func (c *Client) IsRunning() bool {
	conn, err := net.DialTimeout("unix", c.sockPath, time.Second)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
