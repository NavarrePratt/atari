package daemon

import (
	"context"
	"fmt"
	"time"
)

// handleRequest dispatches the request to the appropriate handler.
func (d *Daemon) handleRequest(ctx context.Context, req *Request) Response {
	switch req.Method {
	case "status":
		return d.handleStatus()
	case "pause":
		return d.handlePause()
	case "resume":
		return d.handleResume()
	case "stop":
		return d.handleStop(req)
	default:
		return Response{Error: fmt.Sprintf("unknown method: %s", req.Method)}
	}
}

// handleStatus returns the current daemon status.
func (d *Daemon) handleStatus() Response {
	if d.controller == nil {
		return Response{Error: "no controller available"}
	}

	state := d.controller.State()
	stats := d.controller.Stats()

	d.mu.RLock()
	startTime := d.startTime
	d.mu.RUnlock()

	return Response{
		Result: StatusResponse{
			Status:      string(state),
			CurrentBead: stats.CurrentBead,
			Uptime:      time.Since(startTime).Truncate(time.Second).String(),
			StartTime:   startTime.Format(time.RFC3339),
			Stats: StatusStats{
				Iteration:    stats.Iteration,
				CurrentTurns: stats.CurrentTurns,
				TotalSeen:    stats.QueueStats.TotalSeen,
				Completed:    stats.QueueStats.Completed,
				Failed:       stats.QueueStats.Failed,
				Abandoned:    stats.QueueStats.Abandoned,
				InBackoff:    stats.QueueStats.InBackoff,
			},
		},
	}
}

// handlePause requests the controller to pause.
func (d *Daemon) handlePause() Response {
	if d.controller == nil {
		return Response{Error: "no controller available"}
	}

	d.controller.Pause()
	return Response{Result: "pausing"}
}

// handleResume requests the controller to resume.
func (d *Daemon) handleResume() Response {
	if d.controller == nil {
		return Response{Error: "no controller available"}
	}

	d.controller.Resume()
	return Response{Result: "resuming"}
}

// handleStop requests the controller to stop and schedules daemon shutdown.
func (d *Daemon) handleStop(req *Request) Response {
	if d.controller == nil {
		return Response{Error: "no controller available"}
	}

	// Check for force parameter
	force := false
	if params, ok := req.Params.(map[string]interface{}); ok {
		if f, ok := params["force"].(bool); ok {
			force = f
		}
	}

	d.controller.Stop()

	// Schedule daemon shutdown via stop channel
	go func() {
		if force {
			// Force immediate shutdown
			time.Sleep(50 * time.Millisecond)
		} else {
			// Allow some time for graceful shutdown
			time.Sleep(100 * time.Millisecond)
		}
		// Signal the daemon to stop
		select {
		case d.stopCh <- struct{}{}:
		default:
			// Already signaled
		}
	}()

	return Response{Result: "stopping"}
}
