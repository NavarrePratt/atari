package daemon

import (
	"context"
	"fmt"
	"time"

	"github.com/npratt/atari/internal/controller"
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
	case "retry":
		return d.handleRetry(req)
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

// handlePause requests the controller to pause at the next turn boundary.
func (d *Daemon) handlePause() Response {
	if d.controller == nil {
		return Response{Error: "no controller available"}
	}

	d.controller.GracefulPause()
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

	if force {
		// Force immediate shutdown
		d.controller.ForceStop()
	} else {
		// Graceful shutdown - wait for current bead to complete
		d.controller.Stop()

		// Schedule auto-force after timeout
		gracefulTimeout := d.config.Shutdown.GracefulTimeout
		go func() {
			time.Sleep(gracefulTimeout)
			// Check if we're still stopping (not already stopped)
			state := d.controller.State()
			if state != controller.StateStopped {
				d.logger.Info("graceful timeout reached, forcing stop",
					"timeout", gracefulTimeout,
					"state", state)
				d.controller.ForceStop()
			}
		}()
	}

	// Schedule daemon shutdown via stop channel (gives controller time to stop)
	go func() {
		time.Sleep(100 * time.Millisecond)
		select {
		case d.stopCh <- struct{}{}:
		default:
			// Already signaled
		}
	}()

	return Response{Result: "stopping"}
}

// handleRetry requests the controller to retry a bead.
// If no bead ID is provided, retries the currently stalled bead.
func (d *Daemon) handleRetry(req *Request) Response {
	if d.controller == nil {
		return Response{Error: "no controller available"}
	}

	// Extract bead_id parameter if provided
	beadID := ""
	if params, ok := req.Params.(map[string]interface{}); ok {
		if id, ok := params["bead_id"].(string); ok {
			beadID = id
		}
	}

	if err := d.controller.RetryBead(beadID); err != nil {
		return Response{Error: err.Error()}
	}

	return Response{Result: "retrying"}
}
