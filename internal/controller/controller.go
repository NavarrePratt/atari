// Package controller orchestrates the main drain loop, coordinating
// work queue, session manager, and event router.
package controller

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/events"
	"github.com/npratt/atari/internal/session"
	"github.com/npratt/atari/internal/testutil"
	"github.com/npratt/atari/internal/workqueue"
)

// State represents the controller's current state.
type State string

// Controller states.
const (
	StateIdle     State = "idle"
	StateWorking  State = "working"
	StatePaused   State = "paused"
	StateStopping State = "stopping"
	StateStopped  State = "stopped"
)

// agentStateMap maps controller states to bd agent states.
var agentStateMap = map[State]string{
	StateIdle:     "idle",
	StateWorking:  "running",
	StatePaused:   "idle",
	StateStopping: "stopped",
	StateStopped:  "dead",
}

// Controller orchestrates work queue polling and Claude session execution.
type Controller struct {
	config    *config.Config
	workQueue *workqueue.Manager
	router    *events.Router
	runner    testutil.CommandRunner
	logger    *slog.Logger

	state   State
	stateMu sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Control signals for pause/resume/stop
	pauseSignal  chan struct{}
	resumeSignal chan struct{}
	stopSignal   chan struct{}

	// Statistics
	iteration int
}

// New creates a Controller with the given dependencies.
func New(cfg *config.Config, wq *workqueue.Manager, router *events.Router, runner testutil.CommandRunner, logger *slog.Logger) *Controller {
	if logger == nil {
		logger = slog.Default()
	}
	return &Controller{
		config:       cfg,
		workQueue:    wq,
		router:       router,
		runner:       runner,
		logger:       logger,
		state:        StateIdle,
		pauseSignal:  make(chan struct{}, 1),
		resumeSignal: make(chan struct{}, 1),
		stopSignal:   make(chan struct{}, 1),
	}
}

// Run starts the main drain loop. It blocks until the context is cancelled
// or Stop is called. Returns nil on clean shutdown.
func (c *Controller) Run(ctx context.Context) error {
	c.ctx, c.cancel = context.WithCancel(ctx)

	// Get working directory for DrainStartEvent
	workDir := "."

	c.emit(&events.DrainStartEvent{
		BaseEvent: events.NewInternalEvent(events.EventDrainStart),
		WorkDir:   workDir,
	})
	c.logger.Info("drain started", "state", StateIdle)

	for {
		select {
		case <-c.ctx.Done():
			return c.shutdown("context cancelled")
		case <-c.stopSignal:
			return c.shutdown("stop requested")
		default:
		}

		state := c.getState()
		switch state {
		case StateIdle:
			c.runIdle()
		case StateWorking:
			// This state is handled within runIdle after selecting a bead
		case StatePaused:
			c.runPaused()
		case StateStopping:
			c.runStopping()
		case StateStopped:
			return nil
		}
	}
}

// runIdle polls for work and transitions to working if a bead is available.
func (c *Controller) runIdle() {
	// Check for pause/stop signals first
	select {
	case <-c.pauseSignal:
		c.setState(StatePaused)
		c.logger.Info("paused while idle")
		return
	case <-c.stopSignal:
		c.setState(StateStopping)
		return
	case <-c.ctx.Done():
		return
	default:
	}

	// Poll for work
	bead, err := c.workQueue.Next(c.ctx)
	if err != nil {
		c.logger.Error("work queue poll failed", "error", err)
		c.emit(&events.ErrorEvent{
			BaseEvent: events.NewInternalEvent(events.EventError),
			Message:   fmt.Sprintf("work queue poll failed: %v", err),
			Severity:  events.SeverityWarning,
		})
		c.sleep(c.config.WorkQueue.PollInterval)
		return
	}

	if bead == nil {
		// No work available
		c.sleep(c.config.WorkQueue.PollInterval)
		return
	}

	// Work available - run session
	c.runWorkingOnBead(bead)
}

// runWorkingOnBead executes a Claude session for the given bead.
func (c *Controller) runWorkingOnBead(bead *workqueue.Bead) {
	c.setState(StateWorking)
	c.iteration++

	c.logger.Info("starting iteration",
		"iteration", c.iteration,
		"bead_id", bead.ID,
		"title", bead.Title,
	)

	// Get attempt count from history
	attempt := 1
	history := c.workQueue.History()
	if h, ok := history[bead.ID]; ok {
		attempt = h.Attempts
	}

	c.emit(&events.IterationStartEvent{
		BaseEvent: events.NewInternalEvent(events.EventIterationStart),
		BeadID:    bead.ID,
		Title:     bead.Title,
		Priority:  bead.Priority,
		Attempt:   attempt,
	})

	startTime := time.Now()

	// Run the session
	result, err := c.runSession(bead)

	duration := time.Since(startTime)

	// Record outcome
	if err != nil {
		c.logger.Error("session failed",
			"bead_id", bead.ID,
			"error", err,
			"duration", duration,
		)
		c.workQueue.RecordFailure(bead.ID, err)

		// Check if bead was abandoned
		history := c.workQueue.History()
		if h, ok := history[bead.ID]; ok && h.Status == workqueue.HistoryAbandoned {
			c.emit(&events.BeadAbandonedEvent{
				BaseEvent:   events.NewInternalEvent(events.EventBeadAbandoned),
				BeadID:      bead.ID,
				Attempts:    h.Attempts,
				MaxFailures: c.config.Backoff.MaxFailures,
				LastError:   err.Error(),
			})
		}

		c.emit(&events.IterationEndEvent{
			BaseEvent:  events.NewInternalEvent(events.EventIterationEnd),
			BeadID:     bead.ID,
			Success:    false,
			DurationMs: duration.Milliseconds(),
			Error:      err.Error(),
		})
	} else {
		c.logger.Info("session completed",
			"bead_id", bead.ID,
			"duration", duration,
		)
		c.workQueue.RecordSuccess(bead.ID)

		c.emit(&events.IterationEndEvent{
			BaseEvent:    events.NewInternalEvent(events.EventIterationEnd),
			BeadID:       bead.ID,
			Success:      true,
			NumTurns:     result.NumTurns,
			DurationMs:   duration.Milliseconds(),
			TotalCostUSD: result.TotalCostUSD,
		})
	}

	// Transition based on pending signals
	select {
	case <-c.stopSignal:
		c.setState(StateStopping)
		return
	case <-c.pauseSignal:
		c.setState(StatePaused)
		c.logger.Info("paused after iteration")
		return
	default:
		c.setState(StateIdle)
	}
}

// SessionResult holds the outcome of a Claude session.
type SessionResult struct {
	NumTurns     int
	TotalCostUSD float64
}

// runSession executes a single Claude session for the bead.
func (c *Controller) runSession(bead *workqueue.Bead) (*SessionResult, error) {
	sess := session.New(c.config, c.router)

	// Build prompt with bead context
	prompt := fmt.Sprintf("%s\n\nWork on bead: %s - %s\n\n%s",
		c.config.Prompt, bead.ID, bead.Title, bead.Description)

	c.wg.Add(1)
	defer c.wg.Done()

	c.emit(&events.SessionStartEvent{
		BaseEvent: events.NewInternalEvent(events.EventSessionStart),
		BeadID:    bead.ID,
		Title:     bead.Title,
	})

	if err := sess.Start(c.ctx, prompt); err != nil {
		return nil, fmt.Errorf("start session: %w", err)
	}

	// Parse stream in goroutine
	parser := session.NewParser(sess.Stdout(), c.router, sess)
	parseDone := make(chan error, 1)
	go func() {
		parseDone <- parser.Parse()
	}()

	// Wait for session to complete
	waitErr := sess.Wait()

	// Wait for parser to finish
	<-parseDone

	if waitErr != nil {
		// Include stderr in error message if available
		stderr := sess.Stderr()
		if stderr != "" {
			return nil, fmt.Errorf("session error: %w\nstderr: %s", waitErr, stderr)
		}
		return nil, fmt.Errorf("session error: %w", waitErr)
	}

	// Return result (actual values would come from parsed events)
	return &SessionResult{}, nil
}

// runPaused waits for resume or stop signal.
func (c *Controller) runPaused() {
	select {
	case <-c.resumeSignal:
		c.setState(StateIdle)
		c.logger.Info("resumed")
	case <-c.stopSignal:
		c.setState(StateStopping)
	case <-c.ctx.Done():
		c.setState(StateStopping)
	}
}

// runStopping waits for any active session to complete.
func (c *Controller) runStopping() {
	// Wait for any in-flight work
	c.wg.Wait()
	c.setState(StateStopped)
}

// shutdown performs graceful shutdown.
func (c *Controller) shutdown(reason string) error {
	c.logger.Info("shutting down", "reason", reason)

	c.setState(StateStopping)
	c.wg.Wait()
	c.setState(StateStopped)

	c.emit(&events.DrainStopEvent{
		BaseEvent: events.NewInternalEvent(events.EventDrainStop),
		Reason:    reason,
	})

	c.logger.Info("shutdown complete")
	return nil
}

// Stop requests graceful shutdown. It returns immediately; use Run's
// return to wait for shutdown completion.
func (c *Controller) Stop() {
	select {
	case c.stopSignal <- struct{}{}:
	default:
		// Signal already pending
	}
	if c.cancel != nil {
		c.cancel()
	}
}

// Pause requests the controller to pause after the current iteration.
func (c *Controller) Pause() {
	select {
	case c.pauseSignal <- struct{}{}:
		c.logger.Info("pause requested")
	default:
		// Signal already pending
	}
}

// Resume requests the controller to resume from paused state.
func (c *Controller) Resume() {
	select {
	case c.resumeSignal <- struct{}{}:
		c.logger.Info("resume requested")
	default:
		// Signal already pending
	}
}

// State returns the current controller state.
func (c *Controller) State() State {
	return c.getState()
}

// Stats returns current queue statistics.
type Stats struct {
	Iteration   int
	QueueStats  workqueue.QueueStats
	CurrentBead string
}

// Stats returns current statistics.
func (c *Controller) Stats() Stats {
	return Stats{
		Iteration:  c.iteration,
		QueueStats: c.workQueue.Stats(),
	}
}

// getState returns the current state (thread-safe).
func (c *Controller) getState() State {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.state
}

// setState updates the state and reports to bd agent (thread-safe).
func (c *Controller) setState(s State) {
	c.stateMu.Lock()
	c.state = s
	c.stateMu.Unlock()

	// Report state change to bd agent (best effort, outside lock)
	c.reportAgentState(s)
}

// reportAgentState reports the controller state to beads via bd agent state command.
// Errors are logged but do not affect controller operation.
// If config.AgentID is empty, agent state reporting is disabled.
func (c *Controller) reportAgentState(state State) {
	if c.runner == nil || c.config.AgentID == "" {
		return
	}

	agentState, ok := agentStateMap[state]
	if !ok {
		agentState = "idle"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := c.runner.Run(ctx, "bd", "agent", "state", c.config.AgentID, agentState)
	if err != nil {
		c.logger.Warn("failed to report agent state",
			"state", agentState,
			"error", err)
	}
}

// emit sends an event to the router if available.
func (c *Controller) emit(event events.Event) {
	if c.router != nil {
		c.router.Emit(event)
	}
}

// sleep waits for the given duration, respecting context cancellation.
func (c *Controller) sleep(d time.Duration) {
	select {
	case <-time.After(d):
	case <-c.ctx.Done():
	}
}
