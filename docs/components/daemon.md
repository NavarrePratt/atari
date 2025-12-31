# Daemon Mode

Provides background execution with external control via Unix socket RPC.

## Purpose

The Daemon component is responsible for:
- Running atari as a background process
- Providing a Unix socket for IPC
- Implementing JSON-RPC protocol for control commands
- Managing PID file for process tracking
- Enabling pause/resume/stop from CLI

## Interface

```go
type Daemon struct {
    config     *config.Config
    controller *controller.Controller
    listener   net.Listener
    pidFile    string
    sockPath   string
}

// Public API
func New(cfg *config.Config, ctrl *controller.Controller) *Daemon
func (d *Daemon) Start(ctx context.Context) error
func (d *Daemon) Stop() error
func (d *Daemon) Running() bool

// RPC Client (for CLI commands)
type Client struct {
    sockPath string
}

func NewClient(sockPath string) *Client
func (c *Client) Status() (*StatusResponse, error)
func (c *Client) Pause() error
func (c *Client) Resume() error
func (c *Client) Stop() error
```

## Dependencies

| Component | Usage |
|-----------|-------|
| config.Config | Socket path, PID file path |
| controller.Controller | Execute control commands |

## File Locations

Default paths (in project directory):
- Socket: `.atari/atari.sock`
- PID file: `.atari/atari.pid`
- State: `.atari/state.json`
- Log: `.atari/atari.log`

## Implementation

### Starting the Daemon

```go
func (d *Daemon) Start(ctx context.Context) error {
    // Check for existing daemon
    if d.isRunning() {
        return fmt.Errorf("daemon already running (pid %d)", d.readPID())
    }

    // Clean up stale files
    d.cleanup()

    // Create runtime directory
    dir := filepath.Dir(d.sockPath)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return fmt.Errorf("create runtime directory: %w", err)
    }

    // Write PID file
    if err := d.writePID(); err != nil {
        return fmt.Errorf("write pid file: %w", err)
    }

    // Create Unix socket
    listener, err := net.Listen("unix", d.sockPath)
    if err != nil {
        d.removePID()
        return fmt.Errorf("create socket: %w", err)
    }
    d.listener = listener

    // Set socket permissions
    os.Chmod(d.sockPath, 0600)

    // Start RPC handler
    go d.serve(ctx)

    // Start controller
    go func() {
        if err := d.controller.Run(ctx); err != nil {
            fmt.Fprintf(os.Stderr, "controller error: %v\n", err)
        }
    }()

    return nil
}
```

### PID File Management

```go
func (d *Daemon) writePID() error {
    pid := os.Getpid()
    return os.WriteFile(d.pidFile, []byte(strconv.Itoa(pid)), 0644)
}

func (d *Daemon) readPID() int {
    data, err := os.ReadFile(d.pidFile)
    if err != nil {
        return 0
    }
    pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
    return pid
}

func (d *Daemon) removePID() {
    os.Remove(d.pidFile)
}

func (d *Daemon) isRunning() bool {
    pid := d.readPID()
    if pid == 0 {
        return false
    }

    // Check if process exists
    process, err := os.FindProcess(pid)
    if err != nil {
        return false
    }

    // On Unix, FindProcess always succeeds - send signal 0 to check
    err = process.Signal(syscall.Signal(0))
    return err == nil
}

func (d *Daemon) cleanup() {
    // Remove stale socket
    os.Remove(d.sockPath)

    // Remove stale PID file if process not running
    if !d.isRunning() {
        d.removePID()
    }
}
```

### Unix Socket Server

```go
func (d *Daemon) serve(ctx context.Context) {
    for {
        conn, err := d.listener.Accept()
        if err != nil {
            select {
            case <-ctx.Done():
                return
            default:
                fmt.Fprintf(os.Stderr, "accept error: %v\n", err)
                continue
            }
        }

        go d.handleConnection(ctx, conn)
    }
}

func (d *Daemon) handleConnection(ctx context.Context, conn net.Conn) {
    defer conn.Close()

    // Set read timeout
    conn.SetReadDeadline(time.Now().Add(30 * time.Second))

    decoder := json.NewDecoder(conn)
    encoder := json.NewEncoder(conn)

    var req Request
    if err := decoder.Decode(&req); err != nil {
        encoder.Encode(Response{Error: err.Error()})
        return
    }

    resp := d.handleRequest(ctx, &req)
    encoder.Encode(resp)
}
```

### RPC Protocol

```go
type Request struct {
    Method string `json:"method"`
    Params any    `json:"params,omitempty"`
}

type Response struct {
    Result any    `json:"result,omitempty"`
    Error  string `json:"error,omitempty"`
}

type StatusResponse struct {
    Status       string  `json:"status"`
    CurrentBead  string  `json:"current_bead,omitempty"`
    Uptime       string  `json:"uptime"`
    Stats        Stats   `json:"stats"`
}

func (d *Daemon) handleRequest(ctx context.Context, req *Request) Response {
    switch req.Method {
    case "status":
        return d.handleStatus()
    case "pause":
        return d.handlePause()
    case "resume":
        return d.handleResume()
    case "stop":
        return d.handleStop()
    default:
        return Response{Error: fmt.Sprintf("unknown method: %s", req.Method)}
    }
}

func (d *Daemon) handleStatus() Response {
    state := d.controller.State()
    stats := d.controller.Stats()

    return Response{
        Result: StatusResponse{
            Status:      string(state),
            CurrentBead: d.controller.CurrentBead(),
            Uptime:      time.Since(d.startTime).String(),
            Stats:       stats,
        },
    }
}

func (d *Daemon) handlePause() Response {
    if err := d.controller.Pause(); err != nil {
        return Response{Error: err.Error()}
    }
    return Response{Result: "pausing"}
}

func (d *Daemon) handleResume() Response {
    if err := d.controller.Resume(); err != nil {
        return Response{Error: err.Error()}
    }
    return Response{Result: "resuming"}
}

func (d *Daemon) handleStop() Response {
    if err := d.controller.Stop(); err != nil {
        return Response{Error: err.Error()}
    }

    // Schedule daemon shutdown
    go func() {
        time.Sleep(100 * time.Millisecond)
        d.Stop()
    }()

    return Response{Result: "stopping"}
}
```

### Stopping the Daemon

```go
func (d *Daemon) Stop() error {
    if d.listener != nil {
        d.listener.Close()
    }

    d.removePID()
    os.Remove(d.sockPath)

    return nil
}
```

## RPC Client

Used by CLI commands to communicate with the daemon:

```go
type Client struct {
    sockPath string
    timeout  time.Duration
}

func NewClient(sockPath string) *Client {
    return &Client{
        sockPath: sockPath,
        timeout:  5 * time.Second,
    }
}

func (c *Client) call(method string, params any) (*Response, error) {
    conn, err := net.DialTimeout("unix", c.sockPath, c.timeout)
    if err != nil {
        return nil, fmt.Errorf("connect to daemon: %w", err)
    }
    defer conn.Close()

    conn.SetDeadline(time.Now().Add(c.timeout))

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

func (c *Client) Status() (*StatusResponse, error) {
    resp, err := c.call("status", nil)
    if err != nil {
        return nil, err
    }

    // Type assert the result
    data, _ := json.Marshal(resp.Result)
    var status StatusResponse
    json.Unmarshal(data, &status)

    return &status, nil
}

func (c *Client) Pause() error {
    _, err := c.call("pause", nil)
    return err
}

func (c *Client) Resume() error {
    _, err := c.call("resume", nil)
    return err
}

func (c *Client) Stop() error {
    _, err := c.call("stop", nil)
    return err
}
```

### Detecting Running Daemon

```go
func (c *Client) IsRunning() bool {
    conn, err := net.DialTimeout("unix", c.sockPath, time.Second)
    if err != nil {
        return false
    }
    conn.Close()
    return true
}
```

## Foreground vs Background

Atari can run in two modes:

### Foreground (Default for Phase 1)

```bash
atari start
```

- Runs in current terminal
- Ctrl+C for graceful shutdown
- Output visible in terminal
- No RPC needed for control

### Background (Daemon Mode)

```bash
atari start --daemon
```

- Detaches from terminal
- Control via `atari pause/resume/stop`
- Logs to file
- PID file for tracking

### Daemonization

```go
func daemonize() error {
    // Fork and exit parent
    if os.Getenv("ATARI_DAEMON") != "1" {
        cmd := exec.Command(os.Args[0], os.Args[1:]...)
        cmd.Env = append(os.Environ(), "ATARI_DAEMON=1")
        cmd.Stdin = nil
        cmd.Stdout = nil
        cmd.Stderr = nil
        cmd.SysProcAttr = &syscall.SysProcAttr{
            Setsid: true,
        }

        if err := cmd.Start(); err != nil {
            return fmt.Errorf("fork daemon: %w", err)
        }

        fmt.Printf("Started daemon (pid %d)\n", cmd.Process.Pid)
        os.Exit(0)
    }

    // Child process continues as daemon
    return nil
}
```

## Testing

### Unit Tests

- PID file: create, read, remove, stale detection
- RPC protocol: request/response encoding
- Client: all methods work correctly

### Integration Tests

- Start daemon, send commands, stop
- Detect already running daemon
- Clean up after crash

### Test Fixtures

```go
func TestDaemonLifecycle(t *testing.T) {
    tmp := t.TempDir()
    sockPath := filepath.Join(tmp, "test.sock")
    pidPath := filepath.Join(tmp, "test.pid")

    cfg := &config.Config{
        SocketPath: sockPath,
        PIDFile:    pidPath,
    }

    ctrl := &mockController{}
    d := New(cfg, ctrl)

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Start
    if err := d.Start(ctx); err != nil {
        t.Fatal(err)
    }

    // Verify running
    if !d.Running() {
        t.Error("daemon should be running")
    }

    // Test client
    client := NewClient(sockPath)
    status, err := client.Status()
    if err != nil {
        t.Fatal(err)
    }
    if status.Status == "" {
        t.Error("expected status")
    }

    // Stop
    if err := client.Stop(); err != nil {
        t.Fatal(err)
    }

    time.Sleep(200 * time.Millisecond)

    if d.Running() {
        t.Error("daemon should be stopped")
    }
}
```

## Error Handling

| Error | Action |
|-------|--------|
| Socket already exists | Check if daemon running, error or cleanup |
| PID file stale | Remove and continue |
| Connection refused | Daemon not running |
| Request timeout | Return timeout error |
| Invalid method | Return error response |

## Security Considerations

- Socket has 0600 permissions (owner only)
- PID file has 0644 permissions
- No authentication (relies on filesystem permissions)
- Consider adding token-based auth for shared systems

## Future Considerations

- **TCP socket**: Allow remote control
- **TLS**: Encrypted connections
- **Authentication**: Token or certificate-based
- **Multiple daemons**: Per-project daemons with different sockets
