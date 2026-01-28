// Package controller orchestrates the main drain loop, coordinating
// work queue, session manager, and event router.
package controller

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/npratt/atari/internal/bdactivity"
	"github.com/npratt/atari/internal/brclient"
	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/events"
	"github.com/npratt/atari/internal/observer"
	"github.com/npratt/atari/internal/runner"
	"github.com/npratt/atari/internal/session"
	"github.com/npratt/atari/internal/viewmodel"
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
	brClient  brclient.Client
	logger    *slog.Logger

	// BD activity watcher (optional, started when config.BDActivity.Enabled)
	bdWatcher     *bdactivity.Watcher
	processRunner runner.ProcessRunner

	// Session broker for coordinating Claude process access (optional)
	broker *observer.SessionBroker

	// State sink for persistence (optional)
	stateSink *events.StateSink

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
	gracefulStopSignal  chan struct{}

	// Statistics (protected by statsMu)
	statsMu      sync.Mutex
	iteration    int
	totalCostUSD float64
	startTime    time.Time

	// Session progress tracking
	currentTurnCount int
	currentTurnMu    sync.RWMutex

	// Validated epic info (populated during startup if epic configured)
	epicID    string
	epicTitle string
}

// ControllerOption configures a Controller.
type ControllerOption func(*Controller)

// WithBroker sets the session broker for coordinating Claude process access.
func WithBroker(broker *observer.SessionBroker) ControllerOption {
	return func(c *Controller) {
		c.broker = broker
	}
}

// WithStateSink sets the state sink for persistence.
func WithStateSink(sink *events.StateSink) ControllerOption {
	return func(c *Controller) {
		c.stateSink = sink
	}
}

// New creates a Controller with the given dependencies.
// The processRunner parameter is optional - pass nil to disable BD activity watching.
func New(cfg *config.Config, wq *workqueue.Manager, router *events.Router, brClient brclient.Client, processRunner runner.ProcessRunner, logger *slog.Logger, opts ...ControllerOption) *Controller {
	if logger == nil {
		logger = slog.Default()
	}
	c := &Controller{
		config:              cfg,
		workQueue:           wq,
		router:              router,
		brClient:            brClient,
		processRunner:       processRunner,
		logger:              logger,
		state:               StateIdle,
		pauseSignal:         make(chan struct{}, 1),
		gracefulPauseSignal: make(chan struct{}, 1),
		resumeSignal:        make(chan struct{}, 1),
		stopSignal:          make(chan struct{}, 1),
		gracefulStopSignal:  make(chan struct{}, 1),
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
	c.setStartTime(time.Now())

	// Validate epic if configured (fail fast with clear error)
	if err := c.validateEpic(c.ctx); err != nil {
		return err
	}

	// Restore active top-level from persisted state (for top-level selection mode)
	c.restoreActiveTopLevel()

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
			return c.shutdown("force stop requested")
		case <-c.gracefulStopSignal:
			// Graceful stop from idle state - can stop immediately
			if c.getState() == StateIdle || c.getState() == StatePaused {
				return c.shutdown("graceful stop requested")
			}
			// Put signal back for after bead completion
			select {
			case c.gracefulStopSignal <- struct{}{}:
			default:
			}
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
	case <-c.gracefulPauseSignal:
		// When idle, graceful pause acts like regular pause
		c.setState(StatePaused)
		c.logger.Info("paused while idle (graceful)")
		return
	case <-c.stopSignal:
		c.setState(StateStopping)
		return
	case <-c.gracefulStopSignal:
		// When idle, graceful stop acts like regular stop
		c.setState(StateStopping)
		c.logger.Info("stopping while idle (graceful)")
		return
	case <-c.ctx.Done():
		return
	default:
	}

	// Poll for work using appropriate selection method.
	// Epic flag takes precedence (handled within workqueue.Next via config).
	// Selection mode only matters when no epic is specified.
	bead, err := c.selectNextBead()
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
		// No work available - try to close any eligible epics
		go c.closeEligibleEpics("")
		c.sleep(c.config.WorkQueue.PollInterval)
		return
	}

	// Work available - run session
	c.runWorkingOnBead(bead)
}

// selectNextBead uses the appropriate selection method based on configuration.
// Epic flag takes precedence over selection mode.
func (c *Controller) selectNextBead() (*workqueue.Bead, error) {
	// If epic is configured, use global selection (workqueue.Next already filters by epic)
	if c.config.WorkQueue.Epic != "" {
		return c.workQueue.Next(c.ctx)
	}

	// Check selection mode
	if c.config.WorkQueue.SelectionMode == "top-level" {
		// Track the active top-level before selection
		beforeTopLevel := c.workQueue.ActiveTopLevel()

		bead, err := c.workQueue.NextTopLevel(c.ctx)
		if err != nil {
			return nil, err
		}

		// Sync active top-level changes to state sink
		afterTopLevel := c.workQueue.ActiveTopLevel()
		if afterTopLevel != beforeTopLevel {
			c.syncActiveTopLevel()
		}

		return bead, nil
	}

	// Default to global selection
	return c.workQueue.Next(c.ctx)
}

// syncActiveTopLevel persists the current active top-level to the state sink.
func (c *Controller) syncActiveTopLevel() {
	if c.stateSink == nil {
		return
	}

	activeID := c.workQueue.ActiveTopLevel()
	activeTitle := c.getActiveTopLevelTitle(activeID)

	c.stateSink.SetActiveTopLevel(activeID, activeTitle)
	c.logger.Debug("synced active top-level to state",
		"id", activeID,
		"title", activeTitle)
}

// getActiveTopLevelTitle fetches the title for an active top-level item.
func (c *Controller) getActiveTopLevelTitle(id string) string {
	if id == "" || c.brClient == nil {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	bead, err := c.brClient.Show(ctx, id)
	if err != nil || bead == nil {
		return ""
	}

	return bead.Title
}

// restoreActiveTopLevel restores the active top-level from persisted state.
// It verifies the restored top-level still has ready descendants before using it.
func (c *Controller) restoreActiveTopLevel() {
	if c.stateSink == nil {
		return
	}

	// Only relevant for top-level selection mode (without epic override)
	if c.config.WorkQueue.Epic != "" || c.config.WorkQueue.SelectionMode != "top-level" {
		return
	}

	state := c.stateSink.State()
	if state.ActiveTopLevel == "" {
		return
	}

	// Verify the restored top-level still has ready descendants
	if c.verifyTopLevelHasReadyWork(state.ActiveTopLevel) {
		c.workQueue.SetActiveTopLevel(state.ActiveTopLevel)
		c.logger.Info("restored active top-level from state",
			"id", state.ActiveTopLevel,
			"title", state.ActiveTopLevelTitle)
	} else {
		// Clear stale active top-level from state
		c.stateSink.SetActiveTopLevel("", "")
		c.logger.Info("cleared stale active top-level from state",
			"id", state.ActiveTopLevel)
	}
}

// verifyTopLevelHasReadyWork checks if a top-level item still has ready descendants.
func (c *Controller) verifyTopLevelHasReadyWork(topLevelID string) bool {
	if c.brClient == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get all beads for hierarchy traversal
	allBeads, err := c.brClient.List(ctx, nil)
	if err != nil || len(allBeads) == 0 {
		return false
	}

	// Build descendant set
	descendants := map[string]bool{topLevelID: true}
	for {
		added := false
		for _, bead := range allBeads {
			if bead.Parent != "" && descendants[bead.Parent] && !descendants[bead.ID] {
				descendants[bead.ID] = true
				added = true
			}
		}
		if !added {
			break
		}
	}

	// Get ready beads
	readyBeads, err := c.brClient.Ready(ctx, nil)
	if err != nil || len(readyBeads) == 0 {
		return false
	}

	// Check if any ready bead is a descendant (excluding epics)
	for _, bead := range readyBeads {
		if bead.IssueType == "epic" {
			continue
		}
		if descendants[bead.ID] {
			return true
		}
	}

	return false
}

// runWorkingOnBead executes a Claude session for the given bead.
func (c *Controller) runWorkingOnBead(bead *workqueue.Bead) {
	c.setState(StateWorking)
	c.setCurrentBead(bead.ID, bead.Title)
	defer c.clearCurrentBead()
	iteration := c.incrementIteration()

	c.logger.Info("starting iteration",
		"iteration", iteration,
		"bead_id", bead.ID,
		"title", bead.Title,
	)

	// Get attempt count from history
	attempt := 1
	history := c.workQueue.History()
	if h, ok := history[bead.ID]; ok {
		attempt = h.Attempts
	}

	// Get active top-level context for selection mode
	topLevelID := c.workQueue.ActiveTopLevel()
	topLevelTitle := c.getActiveTopLevelTitle(topLevelID)

	c.emit(&events.IterationStartEvent{
		BaseEvent:     events.NewInternalEvent(events.EventIterationStart),
		BeadID:        bead.ID,
		Title:         bead.Title,
		Priority:      bead.Priority,
		Attempt:       attempt,
		TopLevelID:    topLevelID,
		TopLevelTitle: topLevelTitle,
	})

	startTime := time.Now()

	// Run the session
	result, err := c.runSession(bead)

	duration := time.Since(startTime)

	// Handle session outcome using extracted helpers
	if err != nil {
		c.handleSessionError(bead, err, duration)
	} else if result.GracefulPause {
		c.handleGracefulPause(bead, result, duration)
	} else if c.isBeadClosed(bead.ID) {
		c.handleBeadClosed(bead, result, duration)
	} else {
		c.handleFollowUp(bead, result, duration)
	}

	// Check for eager switching to higher priority work (after successful completion)
	c.checkEagerSwitch()

	// Transition based on pending signals
	select {
	case <-c.stopSignal:
		c.setState(StateStopping)
		return
	case <-c.gracefulStopSignal:
		c.setState(StateStopping)
		c.logger.Info("stopping after iteration (graceful)")
		return
	case <-c.pauseSignal:
		c.setState(StatePaused)
		c.logger.Info("paused after iteration")
		return
	case <-c.gracefulPauseSignal:
		// Graceful pause was requested but session ended naturally
		c.setState(StatePaused)
		c.logger.Info("paused after iteration (graceful)")
		return
	default:
		c.setState(StateIdle)
	}
}

// handleSessionError records failure outcome when a session encounters an error.
func (c *Controller) handleSessionError(bead *workqueue.Bead, err error, duration time.Duration) {
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
}

// handleGracefulPause records outcome when a session was paused mid-work.
func (c *Controller) handleGracefulPause(bead *workqueue.Bead, result *SessionResult, duration time.Duration) {
	c.logger.Info("session paused gracefully",
		"bead_id", bead.ID,
		"duration", duration,
	)

	c.accumulateCost(result.TotalCostUSD)

	c.emit(&events.IterationEndEvent{
		BaseEvent:    events.NewInternalEvent(events.EventIterationEnd),
		BeadID:       bead.ID,
		Success:      false,
		NumTurns:     result.NumTurns,
		DurationMs:   duration.Milliseconds(),
		TotalCostUSD: result.TotalCostUSD,
		Error:        "session paused gracefully",
		SessionID:    result.SessionID,
	})
}

// handleBeadClosed records success when the main session closed the bead.
func (c *Controller) handleBeadClosed(bead *workqueue.Bead, result *SessionResult, duration time.Duration) {
	c.logger.Info("session completed and bead closed",
		"bead_id", bead.ID,
		"duration", duration,
	)
	c.workQueue.RecordSuccess(bead.ID)

	go c.closeEligibleEpics(bead.ID)

	c.accumulateCost(result.TotalCostUSD)

	c.emit(&events.IterationEndEvent{
		BaseEvent:    events.NewInternalEvent(events.EventIterationEnd),
		BeadID:       bead.ID,
		Success:      true,
		NumTurns:     result.NumTurns,
		DurationMs:   duration.Milliseconds(),
		TotalCostUSD: result.TotalCostUSD,
		SessionID:    result.SessionID,
	})
}

// handleFollowUp manages the follow-up session when the main session didn't close the bead.
func (c *Controller) handleFollowUp(bead *workqueue.Bead, mainResult *SessionResult, duration time.Duration) {
	c.logger.Warn("session completed but bead not closed, attempting follow-up",
		"bead_id", bead.ID,
		"duration", duration,
	)

	totalCost := mainResult.TotalCostUSD

	followUpClosed, followUpResult, followUpErr := c.runFollowUpSession(bead)

	if followUpResult != nil {
		totalCost += followUpResult.TotalCostUSD
	}
	c.accumulateCost(totalCost)

	if followUpClosed {
		c.handleFollowUpSuccess(bead, mainResult, followUpResult, totalCost, duration)
	} else if followUpErr == nil && c.getBeadStatus(bead.ID) == "open" {
		c.handleFollowUpResetToOpen(bead, mainResult, followUpResult, totalCost, duration)
	} else {
		c.handleFollowUpFailure(bead, mainResult, followUpErr, totalCost, duration)
	}
}

// handleFollowUpSuccess records success when the follow-up session closed the bead.
func (c *Controller) handleFollowUpSuccess(bead *workqueue.Bead, mainResult, followUpResult *SessionResult, totalCost float64, duration time.Duration) {
	c.logger.Info("follow-up session closed bead",
		"bead_id", bead.ID,
		"total_duration", duration,
	)
	c.workQueue.RecordSuccess(bead.ID)

	go c.closeEligibleEpics(bead.ID)

	c.emit(&events.IterationEndEvent{
		BaseEvent:    events.NewInternalEvent(events.EventIterationEnd),
		BeadID:       bead.ID,
		Success:      true,
		NumTurns:     mainResult.NumTurns + followUpResult.NumTurns,
		DurationMs:   duration.Milliseconds(),
		TotalCostUSD: totalCost,
	})
}

// handleFollowUpResetToOpen records outcome when follow-up reset the bead to open.
func (c *Controller) handleFollowUpResetToOpen(bead *workqueue.Bead, mainResult, followUpResult *SessionResult, totalCost float64, duration time.Duration) {
	c.logger.Info("follow-up reset bead to open for retry",
		"bead_id", bead.ID,
	)
	incompleteErr := fmt.Errorf("bead reset to open for retry")
	c.workQueue.RecordFailure(bead.ID, incompleteErr)

	c.emit(&events.IterationEndEvent{
		BaseEvent:    events.NewInternalEvent(events.EventIterationEnd),
		BeadID:       bead.ID,
		Success:      false,
		NumTurns:     mainResult.NumTurns + followUpResult.NumTurns,
		DurationMs:   duration.Milliseconds(),
		TotalCostUSD: totalCost,
		Error:        incompleteErr.Error(),
	})
}

// handleFollowUpFailure handles the case when both main and follow-up sessions failed to close the bead.
func (c *Controller) handleFollowUpFailure(bead *workqueue.Bead, mainResult *SessionResult, followUpErr error, totalCost float64, duration time.Duration) {
	resetNotes := "Atari: main session and follow-up both failed to close bead. Resetting to open for manual review or retry."
	if followUpErr != nil {
		resetNotes = fmt.Sprintf("Atari: follow-up session error: %v. Resetting to open.", followUpErr)
	}

	if resetErr := c.resetBeadToOpen(bead.ID, resetNotes); resetErr != nil {
		c.logger.Error("failed to reset bead to open",
			"bead_id", bead.ID,
			"error", resetErr,
		)
	}

	incompleteErr := fmt.Errorf("session and follow-up both failed to close bead")
	c.workQueue.RecordFailure(bead.ID, incompleteErr)

	c.emit(&events.IterationEndEvent{
		BaseEvent:    events.NewInternalEvent(events.EventIterationEnd),
		BeadID:       bead.ID,
		Success:      false,
		NumTurns:     mainResult.NumTurns,
		DurationMs:   duration.Milliseconds(),
		TotalCostUSD: totalCost,
		Error:        incompleteErr.Error(),
	})
}

// checkEagerSwitch checks if a higher priority top-level item is available.
// If eager_switch is enabled and a higher priority item exists, clears the
// active top-level to force re-selection on the next iteration.
func (c *Controller) checkEagerSwitch() {
	// Only applies to top-level mode with eager_switch enabled
	if c.config.WorkQueue.Epic != "" {
		return
	}
	if c.config.WorkQueue.SelectionMode != "top-level" {
		return
	}
	if !c.config.WorkQueue.EagerSwitch {
		return
	}

	activeID := c.workQueue.ActiveTopLevel()
	if activeID == "" {
		return
	}

	// Check if there's a higher priority top-level available
	higherPriorityID := c.findHigherPriorityTopLevel(activeID)
	if higherPriorityID != "" {
		c.logger.Info("eager switch: clearing active top-level for higher priority work",
			"current", activeID,
			"higher_priority", higherPriorityID)
		c.workQueue.ClearActiveTopLevel()
		c.syncActiveTopLevel()
	}
}

// findHigherPriorityTopLevel checks if there's a top-level item with higher
// priority (lower number) than the current one that has ready work.
// Returns the ID of the higher priority item, or empty string if none.
func (c *Controller) findHigherPriorityTopLevel(currentID string) string {
	if c.brClient == nil {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get current item's priority
	currentBead, err := c.brClient.Show(ctx, currentID)
	if err != nil || currentBead == nil {
		return ""
	}
	currentPriority := currentBead.Priority

	// Get all beads for hierarchy analysis
	allBeads, err := c.brClient.List(ctx, nil)
	if err != nil || len(allBeads) == 0 {
		return ""
	}

	// Get ready beads
	readyBeads, err := c.brClient.Ready(ctx, nil)
	if err != nil || len(readyBeads) == 0 {
		return ""
	}

	// Build map of ready bead IDs (excluding epics)
	readySet := make(map[string]bool)
	for _, b := range readyBeads {
		if b.IssueType != "epic" {
			readySet[b.ID] = true
		}
	}

	// Identify top-level items
	topLevelItems := make([]struct {
		ID       string
		Priority int
	}, 0)
	for _, b := range allBeads {
		if b.IssueType == "epic" || b.Parent == "" {
			topLevelItems = append(topLevelItems, struct {
				ID       string
				Priority int
			}{b.ID, b.Priority})
		}
	}

	// Check each top-level item with higher priority than current
	for _, item := range topLevelItems {
		if item.ID == currentID {
			continue
		}
		if item.Priority >= currentPriority {
			continue
		}

		// Check if this top-level has ready descendants
		descendants := map[string]bool{item.ID: true}
		for {
			added := false
			for _, b := range allBeads {
				if b.Parent != "" && descendants[b.Parent] && !descendants[b.ID] {
					descendants[b.ID] = true
					added = true
				}
			}
			if !added {
				break
			}
		}

		for id := range descendants {
			if readySet[id] {
				return item.ID
			}
		}
	}

	return ""
}

// SessionResult holds the outcome of a Claude session.
type SessionResult struct {
	NumTurns      int
	TotalCostUSD  float64
	GracefulPause bool   // true if session was paused gracefully (work not complete)
	SessionID     string // Claude session ID for resume capability
}

// runSession executes a single Claude session for the bead.
func (c *Controller) runSession(bead *workqueue.Bead) (*SessionResult, error) {
	// Reset turn count at session start
	c.currentTurnMu.Lock()
	c.currentTurnCount = 0
	c.currentTurnMu.Unlock()

	sess := session.New(c.config, c.router)

	// Check if this bead has a stored session ID for resume
	if resumeID := c.getStoredSessionID(bead.ID); resumeID != "" {
		c.logger.Info("resuming previous session",
			"bead_id", bead.ID,
			"session_id", resumeID)
		sess.SetResumeID(resumeID)
	}

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

	// Track if we're attempting to resume
	attemptingResume := c.getStoredSessionID(bead.ID) != ""

	if err := sess.Start(c.ctx, prompt); err != nil {
		// If resume failed, try again with fresh session
		if attemptingResume {
			c.logger.Warn("resume failed, starting fresh session",
				"bead_id", bead.ID,
				"error", err)

			// Create new session without resume ID
			sess = session.New(c.config, c.router)
			if err := sess.Start(c.ctx, prompt); err != nil {
				return nil, fmt.Errorf("start session: %w", err)
			}
		} else {
			return nil, fmt.Errorf("start session: %w", err)
		}
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
			result.SessionID = parserResult.SessionID
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
		result.SessionID = parserResult.SessionID
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
	case <-c.gracefulStopSignal:
		c.setState(StateStopping)
		c.logger.Info("stopping from paused (graceful)")
	case <-c.ctx.Done():
		c.setState(StateStopping)
	}
}

// runStopping waits for any active session to complete and emits the stop event.
func (c *Controller) runStopping() {
	// Wait for any in-flight work
	c.wg.Wait()

	// Stop BD activity watcher if running
	if c.bdWatcher != nil && c.bdWatcher.Running() {
		if err := c.bdWatcher.Stop(); err != nil {
			c.logger.Warn("failed to stop bd activity watcher", "error", err)
		} else {
			c.logger.Info("bd activity watcher stopped")
		}
	}

	// Emit stop event
	c.emit(&events.DrainStopEvent{
		BaseEvent: events.NewInternalEvent(events.EventDrainStop),
		Reason:    "graceful stop completed",
	})

	c.setState(StateStopped)
	c.logger.Info("shutdown complete")
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

// Stop requests graceful shutdown. The controller will wait for the current
// bead to complete before stopping. It returns immediately; use Run's return
// to wait for shutdown completion.
func (c *Controller) Stop() {
	select {
	case c.gracefulStopSignal <- struct{}{}:
		c.logger.Info("graceful stop requested (will wait for current bead)")
	default:
		// Signal already pending
	}
}

// ForceStop requests immediate shutdown, cancelling the current session.
// It returns immediately; use Run's return to wait for shutdown completion.
func (c *Controller) ForceStop() {
	select {
	case c.stopSignal <- struct{}{}:
		c.logger.Info("force stop requested (immediate)")
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

	statsSnap := c.getStatsSnapshot()

	return Stats{
		Iteration:    statsSnap.Iteration,
		QueueStats:   c.workQueue.Stats(),
		CurrentBead:  c.CurrentBead(),
		CurrentTurns: turns,
	}
}

// Iteration returns the current iteration count.
func (c *Controller) Iteration() int {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()
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

// incrementIteration increments and returns the new iteration count (thread-safe).
func (c *Controller) incrementIteration() int {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()
	c.iteration++
	return c.iteration
}

// accumulateCost adds the given cost to totalCostUSD (thread-safe).
func (c *Controller) accumulateCost(cost float64) {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()
	c.totalCostUSD += cost
}

// setStartTime sets the start time for uptime tracking (thread-safe).
func (c *Controller) setStartTime(t time.Time) {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()
	c.startTime = t
}

// StatsSnapshot returns an atomic snapshot of statistics (thread-safe).
type StatsSnapshot struct {
	Iteration    int
	TotalCostUSD float64
	StartTime    time.Time
	Uptime       time.Duration
}

// getStatsSnapshot returns an atomic snapshot of all stats fields.
func (c *Controller) getStatsSnapshot() StatsSnapshot {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()
	return StatsSnapshot{
		Iteration:    c.iteration,
		TotalCostUSD: c.totalCostUSD,
		StartTime:    c.startTime,
		Uptime:       time.Since(c.startTime),
	}
}

// GetStats returns an atomic snapshot of all TUI statistics.
func (c *Controller) GetStats() viewmodel.TUIStats {
	queueStats := c.workQueue.Stats()
	blockedBeads := c.workQueue.GetBlockedBeads()

	c.currentTurnMu.RLock()
	turns := c.currentTurnCount
	c.currentTurnMu.RUnlock()

	stats := viewmodel.TUIStats{
		Completed:    queueStats.Completed,
		Failed:       queueStats.Failed,
		Abandoned:    queueStats.Abandoned,
		InBackoff:    queueStats.InBackoff,
		CurrentBead:  c.CurrentBead(),
		CurrentTurns: turns,
	}

	// Set TopBlockedBead if there are any blocked beads
	if len(blockedBeads) > 0 {
		stats.TopBlockedBead = &blockedBeads[0]
	}

	return stats
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

	statsSnap := c.getStatsSnapshot()

	state := observer.DrainState{
		Status:       string(c.getState()),
		Uptime:       statsSnap.Uptime,
		TotalCost:    statsSnap.TotalCostUSD,
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

// GetBeadState returns the workqueue state for a bead.
// Implements tui.BeadStateGetter interface.
// Returns:
//   - status: "", "failed", or "abandoned"
//   - attempts: number of attempts (0 if never tried)
//   - inBackoff: true if bead is currently in backoff period
func (c *Controller) GetBeadState(beadID string) (status string, attempts int, inBackoff bool) {
	return c.workQueue.GetBeadState(beadID)
}

// reportAgentState logs the controller state change.
func (c *Controller) reportAgentState(state State) {
	agentState, ok := agentStateMap[state]
	if !ok {
		agentState = "idle"
	}
	c.logger.Info("agent state changed",
		"state", agentState,
		"controller_state", state)
}

// isBeadClosed checks if a bead has been closed in bd.
// Returns true if the bead status is "closed" or "completed", false otherwise.
func (c *Controller) isBeadClosed(beadID string) bool {
	if c.brClient == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	bead, err := c.brClient.Show(ctx, beadID)
	if err != nil {
		c.logger.Warn("failed to check bead status",
			"bead_id", beadID,
			"error", err)
		return false
	}

	if bead == nil {
		return false
	}

	return bead.Status == "closed" || bead.Status == "completed"
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

// runFollowUpSession runs a minimal session to verify and close an unclosed bead.
// Returns true if the bead was closed, false otherwise.
func (c *Controller) runFollowUpSession(bead *workqueue.Bead) (bool, *SessionResult, error) {
	if !c.config.FollowUp.Enabled {
		return false, nil, nil
	}

	c.logger.Info("running follow-up session",
		"bead_id", bead.ID,
		"max_turns", c.config.FollowUp.MaxTurns,
	)

	// Create follow-up session with reduced max turns
	followUpConfig := *c.config
	followUpConfig.Claude.MaxTurns = c.config.FollowUp.MaxTurns

	sess := session.New(&followUpConfig, c.router)

	// Expand follow-up prompt with bead context
	vars := config.PromptVars{
		BeadID:          bead.ID,
		BeadTitle:       bead.Title,
		BeadDescription: bead.Description,
		Label:           c.config.WorkQueue.Label,
	}
	prompt := config.ExpandPrompt(config.DefaultFollowUpPrompt, vars)

	c.emit(&events.SessionStartEvent{
		BaseEvent: events.NewInternalEvent(events.EventSessionStart),
		BeadID:    bead.ID,
		Title:     bead.Title + " (follow-up)",
	})

	// Acquire session broker if configured
	if c.broker != nil {
		if err := c.broker.Acquire(c.ctx, "follow-up", 30*time.Second); err != nil {
			return false, nil, fmt.Errorf("acquire session broker: %w", err)
		}
		defer c.broker.Release()
	}

	if err := sess.Start(c.ctx, prompt); err != nil {
		return false, nil, fmt.Errorf("start follow-up session: %w", err)
	}

	// Parse stream
	parser := session.NewParser(sess.Stdout(), c.router, sess)

	parseDone := make(chan error, 1)
	go func() {
		parseDone <- parser.Parse()
	}()

	// Wait for session to complete
	waitErr := sess.Wait()
	<-parseDone

	if waitErr != nil {
		stderr := sess.Stderr()
		if stderr != "" {
			return false, nil, fmt.Errorf("follow-up session error: %w\nstderr: %s", waitErr, stderr)
		}
		return false, nil, fmt.Errorf("follow-up session error: %w", waitErr)
	}

	// Get session result
	result := &SessionResult{}
	if parserResult := parser.Result(); parserResult != nil {
		result.NumTurns = parserResult.NumTurns
		result.TotalCostUSD = parserResult.TotalCostUSD
	}

	// Check if follow-up closed the bead
	closed := c.isBeadClosed(bead.ID)

	// Also check if it was reset to open (which is acceptable)
	if !closed {
		status := c.getBeadStatus(bead.ID)
		if status == "open" {
			c.logger.Info("follow-up reset bead to open", "bead_id", bead.ID)
			// Bead reset to open is success for follow-up (not stuck anymore)
			return false, result, nil
		}
	}

	return closed, result, nil
}

// resetBeadToOpen resets a stuck bead from in_progress to open status.
func (c *Controller) resetBeadToOpen(beadID, notes string) error {
	if c.brClient == nil {
		return fmt.Errorf("no br client available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := c.brClient.UpdateStatus(ctx, beadID, "open", notes)
	if err != nil {
		return fmt.Errorf("reset bead status: %w", err)
	}

	c.logger.Info("reset bead to open", "bead_id", beadID, "notes", notes)
	return nil
}

// getBeadStatus returns the current status of a bead.
func (c *Controller) getBeadStatus(beadID string) string {
	if c.brClient == nil {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	bead, err := c.brClient.Show(ctx, beadID)
	if err != nil || bead == nil {
		return ""
	}

	return bead.Status
}

// getStoredSessionID retrieves the stored session ID for a bead from history.
// Returns empty string if no session ID is stored.
func (c *Controller) getStoredSessionID(beadID string) string {
	history := c.workQueue.History()
	if h, ok := history[beadID]; ok && h.LastSessionID != "" {
		return h.LastSessionID
	}
	return ""
}

// ValidatedEpic returns the validated epic info, if any epic was configured and validated.
// Returns empty strings if no epic was configured. Thread-safe.
func (c *Controller) ValidatedEpic() (id, title string) {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()
	return c.epicID, c.epicTitle
}

// validateEpic checks that the configured epic ID exists and is of type "epic".
// Returns nil if no epic is configured, or if validation succeeds.
// Returns an error if the epic doesn't exist or is not of type "epic".
func (c *Controller) validateEpic(ctx context.Context) error {
	epicID := c.config.WorkQueue.Epic
	if epicID == "" {
		return nil
	}

	if c.brClient == nil {
		return fmt.Errorf("cannot validate epic: no br client available")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	bead, err := c.brClient.Show(ctx, epicID)
	if err != nil {
		return fmt.Errorf("epic not found: %s", epicID)
	}

	if bead == nil {
		return fmt.Errorf("epic not found: %s", epicID)
	}

	if bead.IssueType != "epic" {
		return fmt.Errorf("%s is not an epic (type: %s)", epicID, bead.IssueType)
	}

	// Store validated epic info (protected by statsMu)
	c.statsMu.Lock()
	c.epicID = bead.ID
	c.epicTitle = bead.Title
	c.statsMu.Unlock()

	c.logger.Info("validated epic",
		"epic_id", bead.ID,
		"epic_title", bead.Title)

	return nil
}

// closeEligibleEpics closes epics where all children are completed.
// This is called asynchronously after a successful bead completion or when idle.
// Errors are logged but not propagated - this is a best-effort operation.
func (c *Controller) closeEligibleEpics(triggeringBeadID string) {
	if c.brClient == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	closedEpics, err := c.brClient.CloseEligibleEpics(ctx)
	if err != nil {
		c.logger.Warn("failed to close eligible epics",
			"triggering_bead_id", triggeringBeadID,
			"error", err)
		return
	}

	if len(closedEpics) == 0 {
		c.logger.Debug("no epics closed", "triggering_bead_id", triggeringBeadID)
		return
	}

	// Emit events for each closed epic
	for _, epic := range closedEpics {
		c.logger.Info("epic auto-closed",
			"epic_id", epic.ID,
			"title", epic.Title,
			"total_children", epic.DependentCount,
			"triggering_bead_id", triggeringBeadID)

		c.emit(&events.EpicClosedEvent{
			BaseEvent:        events.NewInternalEvent(events.EventEpicClosed),
			EpicID:           epic.ID,
			Title:            epic.Title,
			TotalChildren:    epic.DependentCount,
			TriggeringBeadID: triggeringBeadID,
			CloseReason:      "All child issues completed",
		})
	}
}
