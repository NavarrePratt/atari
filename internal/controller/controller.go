// Package controller orchestrates the main drain loop, coordinating
// work queue, session manager, and event router.
package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/npratt/atari/internal/bdactivity"
	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/events"
	"github.com/npratt/atari/internal/observer"
	"github.com/npratt/atari/internal/runner"
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

	// BD activity watcher (optional, started when config.BDActivity.Enabled)
	bdWatcher     *bdactivity.Watcher
	processRunner runner.ProcessRunner

	// Session broker for coordinating Claude process access (optional)
	broker *observer.SessionBroker

	state   State
	stateMu sync.RWMutex

	currentBead      string
	currentBeadTitle string
	currentBeadStart time.Time
	beadMu           sync.RWMutex

	ctx      context.Context
	cancel   context.CancelFunc
	cancelMu sync.Mutex
	wg       sync.WaitGroup

	// Control signals for pause/resume/stop
	pauseSignal         chan struct{}
	gracefulPauseSignal chan struct{}
	resumeSignal        chan struct{}
	stopSignal          chan struct{}

	// Statistics
	iteration    int
	totalCostUSD float64
	startTime    time.Time

	// Session progress tracking
	currentTurnCount int
	currentTurnMu    sync.RWMutex
}

// ControllerOption configures a Controller.
type ControllerOption func(*Controller)

// WithBroker sets the session broker for coordinating Claude process access.
func WithBroker(broker *observer.SessionBroker) ControllerOption {
	return func(c *Controller) {
		c.broker = broker
	}
}

// New creates a Controller with the given dependencies.
// The processRunner parameter is optional - pass nil to disable BD activity watching.
func New(cfg *config.Config, wq *workqueue.Manager, router *events.Router, cmdRunner testutil.CommandRunner, processRunner runner.ProcessRunner, logger *slog.Logger, opts ...ControllerOption) *Controller {
	if logger == nil {
		logger = slog.Default()
	}
	c := &Controller{
		config:              cfg,
		workQueue:           wq,
		router:              router,
		runner:              cmdRunner,
		processRunner:       processRunner,
		logger:              logger,
		state:               StateIdle,
		pauseSignal:         make(chan struct{}, 1),
		gracefulPauseSignal: make(chan struct{}, 1),
		resumeSignal:        make(chan struct{}, 1),
		stopSignal:          make(chan struct{}, 1),
	}

	// Apply options
	for _, opt := range opts {
		opt(c)
	}

	// Build BD activity watcher if enabled and processRunner is available
	if cfg.BDActivity.Enabled && processRunner != nil {
		c.bdWatcher = bdactivity.New(&cfg.BDActivity, router, processRunner, logger)
	}

	return c
}

// Run starts the main drain loop. It blocks until the context is cancelled
// or Stop is called. Returns nil on clean shutdown.
func (c *Controller) Run(ctx context.Context) error {
	c.cancelMu.Lock()
	c.ctx, c.cancel = context.WithCancel(ctx)
	c.cancelMu.Unlock()

	// Record start time for uptime tracking
	c.startTime = time.Now()

	// Start BD activity watcher if configured (best-effort, non-fatal)
	if c.bdWatcher != nil {
		if err := c.bdWatcher.Start(c.ctx); err != nil {
			c.logger.Warn("failed to start bd activity watcher", "error", err)
			// Continue without watcher - it's non-fatal
		} else {
			c.logger.Info("bd activity watcher started")
		}
	}

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
	c.setCurrentBead(bead.ID, bead.Title)
	defer c.clearCurrentBead()
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
	} else if result.GracefulPause {
		// Graceful pause: session was paused mid-work, don't update history
		c.logger.Info("session paused gracefully",
			"bead_id", bead.ID,
			"duration", duration,
		)

		// Accumulate total cost
		c.totalCostUSD += result.TotalCostUSD

		c.emit(&events.IterationEndEvent{
			BaseEvent:    events.NewInternalEvent(events.EventIterationEnd),
			BeadID:       bead.ID,
			Success:      false,
			NumTurns:     result.NumTurns,
			DurationMs:   duration.Milliseconds(),
			TotalCostUSD: result.TotalCostUSD,
			Error:        "session paused gracefully",
		})
	} else {
		// Session completed normally - verify the bead was actually closed
		beadClosed := c.isBeadClosed(bead.ID)

		if beadClosed {
			c.logger.Info("session completed and bead closed",
				"bead_id", bead.ID,
				"duration", duration,
			)
			c.workQueue.RecordSuccess(bead.ID)

			// Accumulate total cost
			c.totalCostUSD += result.TotalCostUSD

			c.emit(&events.IterationEndEvent{
				BaseEvent:    events.NewInternalEvent(events.EventIterationEnd),
				BeadID:       bead.ID,
				Success:      true,
				NumTurns:     result.NumTurns,
				DurationMs:   duration.Milliseconds(),
				TotalCostUSD: result.TotalCostUSD,
			})
		} else {
			// Session completed but bead was not closed - treat as incomplete
			c.logger.Warn("session completed but bead not closed",
				"bead_id", bead.ID,
				"duration", duration,
			)
			incompleteErr := fmt.Errorf("session completed but bead was not closed in bd")
			c.workQueue.RecordFailure(bead.ID, incompleteErr)

			// Accumulate total cost
			c.totalCostUSD += result.TotalCostUSD

			c.emit(&events.IterationEndEvent{
				BaseEvent:    events.NewInternalEvent(events.EventIterationEnd),
				BeadID:       bead.ID,
				Success:      false,
				NumTurns:     result.NumTurns,
				DurationMs:   duration.Milliseconds(),
				TotalCostUSD: result.TotalCostUSD,
				Error:        incompleteErr.Error(),
			})
		}
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
	NumTurns      int
	TotalCostUSD  float64
	GracefulPause bool // true if session was paused gracefully (work not complete)
}

// runSession executes a single Claude session for the bead.
func (c *Controller) runSession(bead *workqueue.Bead) (*SessionResult, error) {
	// Reset turn count at session start
	c.currentTurnMu.Lock()
	c.currentTurnCount = 0
	c.currentTurnMu.Unlock()

	sess := session.New(c.config, c.router)

	// Load prompt template
	promptTemplate, err := c.config.LoadPrompt()
	if err != nil {
		return nil, fmt.Errorf("load prompt: %w", err)
	}

	// Expand template variables
	vars := config.PromptVars{
		BeadID:          bead.ID,
		BeadTitle:       bead.Title,
		BeadDescription: bead.Description,
		Label:           c.config.WorkQueue.Label,
	}
	prompt := config.ExpandPrompt(promptTemplate, vars)

	// For default prompt (no custom file or inline), append bead context
	// Custom prompts should use {{.BeadID}} etc. to include bead info
	if c.config.PromptFile == "" && c.config.Prompt == "" {
		prompt = fmt.Sprintf("%s\n\nWork on bead: %s - %s\n\n%s",
			prompt, bead.ID, bead.Title, bead.Description)
	}

	c.wg.Add(1)
	defer c.wg.Done()

	c.emit(&events.SessionStartEvent{
		BaseEvent: events.NewInternalEvent(events.EventSessionStart),
		BeadID:    bead.ID,
		Title:     bead.Title,
	})

	// Acquire session broker if configured (for observer coordination)
	if c.broker != nil {
		if err := c.broker.Acquire(c.ctx, "drain", 30*time.Second); err != nil {
			return nil, fmt.Errorf("acquire session broker: %w", err)
		}
		defer c.broker.Release()
	}

	if err := sess.Start(c.ctx, prompt); err != nil {
		return nil, fmt.Errorf("start session: %w", err)
	}

	// Check for graceful pause request and wire up turn boundary callback
	select {
	case <-c.gracefulPauseSignal:
		sess.RequestPause()
		c.logger.Info("graceful pause active for session", "bead_id", bead.ID)
	default:
		// No graceful pause requested
	}

	// Parse stream in goroutine
	parser := session.NewParser(sess.Stdout(), c.router, sess)

	// Set up turn boundary callback for graceful pause and turn tracking
	parser.SetOnTurnComplete(func() {
		// Update controller's turn count
		c.currentTurnMu.Lock()
		c.currentTurnCount = parser.TurnCount()
		c.currentTurnMu.Unlock()

		// Check for graceful pause request
		if sess.PauseRequested() {
			c.logger.Info("stopping session at turn boundary", "bead_id", bead.ID)
			sess.Stop()
		}
	})

	parseDone := make(chan error, 1)
	go func() {
		parseDone <- parser.Parse()
	}()

	// Wait for session to complete
	waitErr := sess.Wait()

	// Wait for parser to finish
	<-parseDone

	// If we stopped due to graceful pause, signal the controller to pause
	// and don't treat the process termination as an error
	if sess.PauseRequested() {
		select {
		case c.pauseSignal <- struct{}{}:
		default:
		}
		// Graceful pause stops are not errors, but work is not complete
		result := &SessionResult{GracefulPause: true}
		if parserResult := parser.Result(); parserResult != nil {
			result.NumTurns = parserResult.NumTurns
			result.TotalCostUSD = parserResult.TotalCostUSD
		}
		return result, nil
	}

	if waitErr != nil {
		// Include stderr in error message if available
		stderr := sess.Stderr()
		if stderr != "" {
			return nil, fmt.Errorf("session error: %w\nstderr: %s", waitErr, stderr)
		}
		return nil, fmt.Errorf("session error: %w", waitErr)
	}

	// Retrieve session result from parser
	result := &SessionResult{}
	if parserResult := parser.Result(); parserResult != nil {
		result.NumTurns = parserResult.NumTurns
		result.TotalCostUSD = parserResult.TotalCostUSD
	} else {
		c.logger.Warn("session completed without result event", "bead_id", bead.ID)
	}

	return result, nil
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

	// Stop BD activity watcher if running
	if c.bdWatcher != nil && c.bdWatcher.Running() {
		if err := c.bdWatcher.Stop(); err != nil {
			c.logger.Warn("failed to stop bd activity watcher", "error", err)
		} else {
			c.logger.Info("bd activity watcher stopped")
		}
	}

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
	c.cancelMu.Lock()
	if c.cancel != nil {
		c.cancel()
	}
	c.cancelMu.Unlock()
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

// GracefulPause requests the controller to pause at the next turn boundary.
// This allows Claude to complete its current tool use before stopping.
func (c *Controller) GracefulPause() {
	select {
	case c.gracefulPauseSignal <- struct{}{}:
		c.logger.Info("graceful pause requested (turn boundary)")
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
	Iteration    int
	QueueStats   workqueue.QueueStats
	CurrentBead  string
	CurrentTurns int // turns completed in current session (0 if idle)
}

// Stats returns current statistics.
func (c *Controller) Stats() Stats {
	c.currentTurnMu.RLock()
	turns := c.currentTurnCount
	c.currentTurnMu.RUnlock()

	return Stats{
		Iteration:    c.iteration,
		QueueStats:   c.workQueue.Stats(),
		CurrentBead:  c.CurrentBead(),
		CurrentTurns: turns,
	}
}

// Iteration returns the current iteration count.
func (c *Controller) Iteration() int {
	return c.iteration
}

// Completed returns the number of successfully completed beads.
func (c *Controller) Completed() int {
	return c.workQueue.Stats().Completed
}

// Failed returns the number of failed beads.
func (c *Controller) Failed() int {
	return c.workQueue.Stats().Failed
}

// Abandoned returns the number of abandoned beads.
func (c *Controller) Abandoned() int {
	return c.workQueue.Stats().Abandoned
}

// CurrentTurns returns the number of turns completed in the current session.
// Returns 0 if no session is active.
func (c *Controller) CurrentTurns() int {
	c.currentTurnMu.RLock()
	defer c.currentTurnMu.RUnlock()
	return c.currentTurnCount
}

// getState returns the current state (thread-safe).
func (c *Controller) getState() State {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.state
}

// setState updates the state and reports to bd agent (thread-safe).
// Emits DrainStateChangedEvent when the state actually changes.
func (c *Controller) setState(s State) {
	c.stateMu.Lock()
	oldState := c.state
	c.state = s
	c.stateMu.Unlock()

	// Only emit and report if state actually changed
	if oldState == s {
		return
	}

	// Emit state change event
	c.emit(&events.DrainStateChangedEvent{
		BaseEvent: events.NewInternalEvent(events.EventDrainStateChanged),
		From:      string(oldState),
		To:        string(s),
	})

	// Report state change to bd agent (best effort, outside lock)
	c.reportAgentState(s)
}

// CurrentBead returns the ID of the bead currently being worked on,
// or an empty string if no bead is active (thread-safe).
func (c *Controller) CurrentBead() string {
	c.beadMu.RLock()
	defer c.beadMu.RUnlock()
	return c.currentBead
}

// setCurrentBead updates the current bead info (thread-safe).
func (c *Controller) setCurrentBead(beadID, title string) {
	c.beadMu.Lock()
	c.currentBead = beadID
	c.currentBeadTitle = title
	c.currentBeadStart = time.Now()
	c.beadMu.Unlock()
}

// clearCurrentBead clears the current bead info (thread-safe).
func (c *Controller) clearCurrentBead() {
	c.beadMu.Lock()
	c.currentBead = ""
	c.currentBeadTitle = ""
	c.currentBeadStart = time.Time{}
	c.beadMu.Unlock()
}

// GetDrainState returns the current drain state for observer context.
// Implements observer.DrainStateProvider interface.
func (c *Controller) GetDrainState() observer.DrainState {
	c.beadMu.RLock()
	beadID := c.currentBead
	beadTitle := c.currentBeadTitle
	beadStart := c.currentBeadStart
	c.beadMu.RUnlock()

	c.currentTurnMu.RLock()
	turns := c.currentTurnCount
	c.currentTurnMu.RUnlock()

	state := observer.DrainState{
		Status:       string(c.getState()),
		Uptime:       time.Since(c.startTime),
		TotalCost:    c.totalCostUSD,
		CurrentTurns: turns,
	}

	if beadID != "" {
		state.CurrentBead = &observer.CurrentBeadInfo{
			ID:        beadID,
			Title:     beadTitle,
			StartedAt: beadStart,
		}
	}

	return state
}

// Broker returns the session broker, if configured.
func (c *Controller) Broker() *observer.SessionBroker {
	return c.broker
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

// isBeadClosed checks if a bead has been closed in bd.
// Returns true if the bead status is "closed" or "completed", false otherwise.
func (c *Controller) isBeadClosed(beadID string) bool {
	if c.runner == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	output, err := c.runner.Run(ctx, "bd", "show", beadID, "--json")
	if err != nil {
		c.logger.Warn("failed to check bead status",
			"bead_id", beadID,
			"error", err)
		return false
	}

	// Parse the JSON output to get status
	// bd show --json returns an array with one element
	var beads []struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(output, &beads); err != nil {
		c.logger.Warn("failed to parse bead status",
			"bead_id", beadID,
			"error", err)
		return false
	}

	if len(beads) == 0 {
		return false
	}

	status := beads[0].Status
	return status == "closed" || status == "completed"
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
