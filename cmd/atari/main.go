package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"golang.org/x/term"

	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/controller"
	"github.com/npratt/atari/internal/daemon"
	"github.com/npratt/atari/internal/events"
	initcmd "github.com/npratt/atari/internal/init"
	"github.com/npratt/atari/internal/observer"
	"github.com/npratt/atari/internal/runner"
	"github.com/npratt/atari/internal/shutdown"
	"github.com/npratt/atari/internal/testutil"
	"github.com/npratt/atari/internal/tui"
	"github.com/npratt/atari/internal/workqueue"
)

var version = "dev"

// getDaemonClient creates a daemon client by finding daemon.json in the project.
func getDaemonClient() (*daemon.Client, error) {
	info, err := daemon.FindDaemonInfo("")
	if err != nil {
		return nil, fmt.Errorf("daemon not running: %w", err)
	}
	return daemon.NewClient(info.SocketPath), nil
}

// tailLast reads and prints the last n lines from the log file.
func tailLast(path string, n int) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No events yet (log file does not exist)")
			return nil
		}
		return fmt.Errorf("open log file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Read all lines into a buffer (simple approach for reasonable file sizes)
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read log file: %w", err)
	}

	if len(lines) == 0 {
		fmt.Println("No events yet")
		return nil
	}

	// Get last n lines
	start := 0
	if len(lines) > n {
		start = len(lines) - n
	}

	for _, line := range lines[start:] {
		printEventLine(line)
	}
	return nil
}

// waitForFile waits for a file to be created and returns the opened file.
func waitForFile(ctx context.Context, path string) (*os.File, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
			file, err := os.Open(path)
			if err == nil {
				return file, nil
			}
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("open file: %w", err)
			}
			// File still doesn't exist, continue waiting
		}
	}
}

// tailFollow follows the log file and prints new lines as they appear.
func tailFollow(ctx context.Context, path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Waiting for log file to be created...")
			// Wait for file to appear
			file, err = waitForFile(ctx, path)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("open log file: %w", err)
		}
	}
	defer func() { _ = file.Close() }()

	// Seek to end
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("seek to end: %w", err)
	}

	fmt.Println("Following events (Ctrl+C to stop)...")
	reader := bufio.NewReader(file)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					// No more data, wait a bit
					time.Sleep(100 * time.Millisecond)
					continue
				}
				return fmt.Errorf("read log: %w", err)
			}
			printEventLine(strings.TrimSuffix(line, "\n"))
		}
	}
}

// printEventLine prints a single event line in a human-readable format.
func printEventLine(line string) {
	// Try to parse as JSON and format nicely
	var event map[string]interface{}
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		// Not JSON, print as-is
		fmt.Println(line)
		return
	}

	// Format: [timestamp] type: message/data
	timestamp := ""
	if ts, ok := event["timestamp"].(string); ok {
		// Parse and format timestamp for readability
		if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			timestamp = t.Format("15:04:05")
		} else {
			timestamp = ts
		}
	}

	eventType := ""
	if t, ok := event["type"].(string); ok {
		eventType = t
	}

	// Build output based on event type
	var detail string
	switch eventType {
	case "session.start", "session.end":
		if id, ok := event["bead_id"].(string); ok {
			detail = fmt.Sprintf("bead=%s", id)
		}
	case "iteration.start", "iteration.end":
		if num, ok := event["iteration"].(float64); ok {
			detail = fmt.Sprintf("iteration=%d", int(num))
		}
	case "error":
		if msg, ok := event["message"].(string); ok {
			detail = msg
		}
	default:
		// Generic handling
		if msg, ok := event["message"].(string); ok {
			detail = msg
		}
	}

	if detail != "" {
		fmt.Printf("[%s] %s: %s\n", timestamp, eventType, detail)
	} else {
		fmt.Printf("[%s] %s\n", timestamp, eventType)
	}
}

// checkBrInstalled verifies that the br (beads_rust) binary is available in PATH.
func checkBrInstalled() error {
	if _, err := exec.LookPath("br"); err != nil {
		return fmt.Errorf("br (beads_rust) not found in PATH\n\n" +
			"Atari requires beads_rust for issue tracking.\n\n" +
			"Install with:\n" +
			"  cargo install beads_rust\n\n" +
			"Or see: https://github.com/Dicklesworthstone/beads_rust")
	}
	return nil
}

func main() {
	logLevel := &slog.LevelVar{}
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))

	viper.SetEnvPrefix("ATARI")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	rootCmd := &cobra.Command{
		Use:   "atari",
		Short: "Applied Training: Automatic Research & Implementation",
		Long: `atari (Applied Training: Automatic Research & Implementation) is a daemon
controller that orchestrates Claude Code sessions to automatically work through
beads (bd) issues until all ready work is complete.

It polls for available work, spawns Claude sessions, streams unified events,
and persists state for pause/resume capability.`,
		SilenceUsage: true,
	}

	// Persistent flags available to all commands
	rootCmd.PersistentFlags().Bool(FlagVerbose, false, "Enable verbose (debug) logging")
	rootCmd.PersistentFlags().String(FlagConfig, "", "Config file path (default: .atari/config.yaml)")
	rootCmd.PersistentFlags().String(FlagLogFile, "", "Log file path")
	rootCmd.PersistentFlags().String(FlagStateFile, "", "State file path")
	rootCmd.PersistentFlags().String(FlagSocketPath, "", "Unix socket path for daemon control")

	// Bind all flags to viper
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		_ = viper.BindPFlag(f.Name, f)
	})

	// Version command
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("atari %s\n", version)
		},
	}

	// Start command
	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start the atari daemon",
		Long: `Start the atari daemon to process available beads.

The daemon will poll br ready for available work and spawn Claude Code
sessions to work on each bead until completion.

Use --daemon to run in the background.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Verify br (beads_rust) is installed before proceeding
			if err := checkBrInstalled(); err != nil {
				return err
			}

			daemonMode := viper.GetBool(FlagDaemon)

			// Determine TUI mode: explicit flag > auto-detect from TTY
			tuiEnabled := viper.GetBool(FlagTUI)
			if !cmd.Flags().Changed(FlagTUI) && !daemonMode {
				// Auto-enable TUI when stdout is a TTY and not in daemon mode
				tuiEnabled = term.IsTerminal(int(os.Stdout.Fd()))
			}

			// Check for incompatible flags
			if tuiEnabled && daemonMode {
				return fmt.Errorf("--tui and --daemon flags are incompatible")
			}

			if viper.GetBool(FlagVerbose) {
				logLevel.Set(slog.LevelDebug)
				logger.Debug("verbose logging enabled")
			}

			// Load config from files with defaults
			cfg, err := config.LoadConfig(viper.GetViper())
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			// Apply CLI flag overrides (only if explicitly set)
			if cmd.Flags().Changed(FlagLogFile) {
				cfg.Paths.Log = viper.GetString(FlagLogFile)
			}
			if cmd.Flags().Changed(FlagStateFile) {
				cfg.Paths.State = viper.GetString(FlagStateFile)
			}
			if cmd.Flags().Changed(FlagSocketPath) {
				cfg.Paths.Socket = viper.GetString(FlagSocketPath)
			}
			if cmd.Flags().Changed(FlagLabel) {
				cfg.WorkQueue.Label = viper.GetString(FlagLabel)
			}
			if cmd.Flags().Changed(FlagEpic) {
				cfg.WorkQueue.Epic = viper.GetString(FlagEpic)
			}
			if cmd.Flags().Changed(FlagUnassignedOnly) {
				cfg.WorkQueue.UnassignedOnly = viper.GetBool(FlagUnassignedOnly)
			}
			if cmd.Flags().Changed(FlagExcludeLabels) {
				cfg.WorkQueue.ExcludeLabels = viper.GetStringSlice(FlagExcludeLabels)
			}
			if cmd.Flags().Changed(FlagPrompt) {
				cfg.PromptFile = viper.GetString(FlagPrompt)
			}
			if cmd.Flags().Changed(FlagBDActivityEnabled) {
				cfg.BDActivity.Enabled = viper.GetBool(FlagBDActivityEnabled)
			}

			// Observer flag overrides
			if cmd.Flags().Changed(FlagObserverEnabled) {
				cfg.Observer.Enabled = viper.GetBool(FlagObserverEnabled)
			}
			if cmd.Flags().Changed(FlagObserverModel) {
				cfg.Observer.Model = viper.GetString(FlagObserverModel)
			}
			if cmd.Flags().Changed(FlagObserverLayout) {
				cfg.Observer.Layout = viper.GetString(FlagObserverLayout)
			}
			if cmd.Flags().Changed(FlagObserverRecentEvents) {
				cfg.Observer.RecentEvents = viper.GetInt(FlagObserverRecentEvents)
			}

			// Graph flag overrides
			if cmd.Flags().Changed(FlagGraphEnabled) {
				cfg.Graph.Enabled = viper.GetBool(FlagGraphEnabled)
			}
			if cmd.Flags().Changed(FlagGraphDensity) {
				cfg.Graph.Density = viper.GetString(FlagGraphDensity)
			}

			// Handle max-turns: set config field if flag provided
			if cmd.Flags().Changed(FlagMaxTurns) {
				cfg.Claude.MaxTurns = viper.GetInt(FlagMaxTurns)
			}

			// Find project root for path resolution
			projectRoot := daemon.FindProjectRoot("")

			// Resolve all paths to absolute
			cfg.Paths, err = daemon.ResolvePaths(cfg.Paths, projectRoot)
			if err != nil {
				return fmt.Errorf("resolve paths: %w", err)
			}

			// Check if daemon is already running
			if daemonMode {
				client := daemon.NewClient(cfg.Paths.Socket)
				if client.IsRunning() {
					return fmt.Errorf("daemon already running (socket: %s)", cfg.Paths.Socket)
				}

				// Daemonize: fork and let parent exit
				shouldExit, _, err := daemon.Daemonize(cfg)
				if err != nil {
					return fmt.Errorf("daemonize: %w", err)
				}
				if shouldExit {
					return nil // Parent exits cleanly
				}
				// Child continues below
			}

			// Ensure .atari directory exists
			atariDir := daemon.DaemonInfoPath(projectRoot)
			if err := os.MkdirAll(atariDir[:len(atariDir)-len("/daemon.json")], 0755); err != nil {
				return fmt.Errorf("create .atari directory: %w", err)
			}

			logger.Info("atari starting",
				"version", version,
				"log_file", cfg.Paths.Log,
				"state_file", cfg.Paths.State,
				"label", cfg.WorkQueue.Label,
				"daemon_mode", viper.GetBool(FlagDaemon),
			)

			// Write daemon info for CLI discovery
			daemonInfo := &daemon.DaemonInfo{
				SocketPath: cfg.Paths.Socket,
				PIDPath:    cfg.Paths.PID,
				LogPath:    cfg.Paths.Log,
				StartTime:  time.Now(),
				PID:        os.Getpid(),
			}
			if err := daemon.WriteDaemonInfo(daemon.DaemonInfoPath(projectRoot), daemonInfo); err != nil {
				logger.Warn("failed to write daemon info", "error", err)
			}

			// Create event router
			router := events.NewRouter(events.DefaultBufferSize)

			// Create and start sinks
			logSink := events.NewLogSink(cfg.Paths.Log)
			stateSink := events.NewStateSink(cfg.Paths.State)

			// Create context for sinks
			ctx := cmd.Context()
			sinkCtx, sinkCancel := context.WithCancel(ctx)

			// Subscribe sinks to router and start them
			logEvents := router.Subscribe()
			if err := logSink.Start(sinkCtx, logEvents); err != nil {
				sinkCancel()
				return fmt.Errorf("start log sink: %w", err)
			}

			stateEvents := router.SubscribeBuffered(events.StateBufferSize)
			if err := stateSink.Start(sinkCtx, stateEvents); err != nil {
				sinkCancel()
				_ = logSink.Stop()
				return fmt.Errorf("start state sink: %w", err)
			}

			// Create command runner for real commands
			cmdRunner := testutil.NewExecRunner()

			// Create process runner for bd activity watcher
			processRunner := runner.NewExecProcessRunner()

			// Create work queue
			wq := workqueue.New(cfg, cmdRunner)

			// TUI mode: redirect logger to file before creating controller
			ctrlLogger := logger
			var tuiLogResult *TUILoggerResult
			if tuiEnabled {
				var err error
				tuiLogResult, err = SetupTUILogger(filepath.Dir(cfg.Paths.Log), logLevel, cfg.LogRotation)
				if err != nil {
					sinkCancel()
					router.Close()
					_ = logSink.Stop()
					_ = stateSink.Stop()
					return err
				}
				ctrlLogger = tuiLogResult.Logger
				slog.SetDefault(ctrlLogger)
			}

			// Create session broker for coordinating Claude process access
			broker := observer.NewSessionBroker()

			// Create controller with appropriate logger and broker
			ctrl := controller.New(cfg, wq, router, cmdRunner, processRunner, ctrlLogger,
				controller.WithBroker(broker))

			// TUI mode: run TUI in foreground with controller in background
			if tuiEnabled {
				defer func() { _ = tuiLogResult.Close() }()

				// Subscribe TUI to events with buffering
				tuiEvents := router.SubscribeBuffered(5000)
				defer router.Unsubscribe(tuiEvents)

				// Create observer for interactive Q&A (if enabled)
				var obs *observer.Observer
				if cfg.Observer.Enabled {
					logReader := observer.NewLogReader(cfg.Paths.Log)
					contextBuilder := observer.NewContextBuilder(logReader, &cfg.Observer)
					obs = observer.NewObserver(&cfg.Observer, broker, contextBuilder, ctrl)
				}

				// Create graph fetcher for bead visualization
				graphFetcher := tui.NewBDFetcher(cmdRunner)

				// Create TUI with callbacks and observer
				tuiApp := tui.New(tuiEvents,
					tui.WithOnPause(ctrl.GracefulPause),
					tui.WithOnResume(ctrl.Resume),
					tui.WithOnQuit(ctrl.Stop),
					tui.WithStatsGetter(ctrl),
					tui.WithObserver(obs),
					tui.WithGraphFetcher(graphFetcher),
					tui.WithBeadStateGetter(ctrl),
				)

				// Run controller in background
				ctrlDone := make(chan error, 1)
				go func() {
					ctrlDone <- ctrl.Run(ctx)
				}()

				// Run TUI in foreground (blocks until quit)
				tuiErr := tuiApp.Run()

				// Ensure controller stops when TUI exits
				ctrl.Stop()
				<-ctrlDone

				// Clean up
				sinkCancel()
				router.Close()
				_ = logSink.Stop()
				_ = stateSink.Stop()
				_ = daemon.RemoveDaemonInfo(daemon.DaemonInfoPath(projectRoot))

				return tuiErr
			}

			// Non-TUI mode: create daemon for RPC control
			dmn := daemon.New(cfg, ctrl, logger)

			// Start daemon socket server in background
			daemonCtx, daemonCancel := context.WithCancel(ctx)
			daemonDone := make(chan struct{})
			go func() {
				defer close(daemonDone)
				if err := dmn.Start(daemonCtx); err != nil {
					logger.Error("daemon server error", "error", err)
				}
			}()

			// Run with graceful shutdown handling
			err = shutdown.RunWithGracefulShutdown(
				ctx,
				logger,
				30*time.Second,
				func(runCtx context.Context) error {
					return ctrl.Run(runCtx)
				},
				func(shutdownCtx context.Context) error {
					ctrl.Stop()
					daemonCancel()
					<-daemonDone // Wait for daemon to finish
					return nil
				},
			)

			// Clean up sinks
			sinkCancel()
			router.Close()
			_ = logSink.Stop()
			_ = stateSink.Stop()

			// Remove daemon info on clean exit
			_ = daemon.RemoveDaemonInfo(daemon.DaemonInfoPath(projectRoot))

			return err
		},
	}

	// Start command specific flags
	startCmd.Flags().Bool(FlagDaemon, false, "Run as a background daemon")
	startCmd.Flags().Bool(FlagTUI, false, "Enable terminal UI")
	startCmd.Flags().Int(FlagMaxTurns, 0, "Max turns per Claude session (0 = unlimited)")
	startCmd.Flags().String(FlagLabel, "", "Filter br ready by label")
	startCmd.Flags().String(FlagEpic, "", "Restrict work to beads under this epic (e.g., bd-xxx)")
	startCmd.Flags().Bool(FlagUnassignedOnly, false, "Only claim unassigned beads")
	startCmd.Flags().StringSlice(FlagExcludeLabels, nil, "Labels to exclude from work selection (comma-separated)")
	startCmd.Flags().String(FlagPrompt, "", "Custom prompt template file")
	startCmd.Flags().Bool(FlagBDActivityEnabled, true, "Enable BD activity watcher")

	// Observer flags
	startCmd.Flags().Bool(FlagObserverEnabled, true, "Enable observer mode in TUI")
	startCmd.Flags().String(FlagObserverModel, "haiku", "Claude model for observer queries")
	startCmd.Flags().String(FlagObserverLayout, "horizontal", "Observer pane layout (horizontal/vertical)")
	startCmd.Flags().Int(FlagObserverRecentEvents, 20, "Recent events for observer context")

	// Graph flags
	startCmd.Flags().Bool(FlagGraphEnabled, true, "Enable graph pane in TUI")
	startCmd.Flags().String(FlagGraphDensity, "standard", "Graph node density (compact/standard/detailed)")

	startCmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = viper.BindPFlag(f.Name, f)
	})

	// Status command
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show daemon status",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := getDaemonClient()
			if err != nil {
				return err
			}

			status, err := client.Status()
			if err != nil {
				return err
			}

			if viper.GetBool(FlagJSON) {
				data, err := json.MarshalIndent(status, "", "  ")
				if err != nil {
					return fmt.Errorf("marshal status: %w", err)
				}
				fmt.Println(string(data))
				return nil
			}

			// Human-readable output
			fmt.Printf("Status: %s\n", status.Status)
			if status.CurrentBead != "" {
				fmt.Printf("Current bead: %s\n", status.CurrentBead)
				if status.Stats.CurrentTurns > 0 {
					fmt.Printf("Current turns: %d\n", status.Stats.CurrentTurns)
				}
			}
			fmt.Printf("Uptime: %s\n", status.Uptime)
			fmt.Printf("Started: %s\n", status.StartTime)
			fmt.Printf("Stats:\n")
			fmt.Printf("  Iteration: %d\n", status.Stats.Iteration)
			fmt.Printf("  Total seen: %d\n", status.Stats.TotalSeen)
			fmt.Printf("  Completed: %d\n", status.Stats.Completed)
			fmt.Printf("  Failed: %d\n", status.Stats.Failed)
			fmt.Printf("  Abandoned: %d\n", status.Stats.Abandoned)
			if status.Stats.InBackoff > 0 {
				fmt.Printf("  In backoff: %d\n", status.Stats.InBackoff)
			}
			return nil
		},
	}
	statusCmd.Flags().Bool(FlagJSON, false, "Output status as JSON")
	_ = viper.BindPFlag(FlagJSON, statusCmd.Flags().Lookup(FlagJSON))

	// Pause command
	pauseCmd := &cobra.Command{
		Use:   "pause",
		Short: "Pause the daemon after current bead completes",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := getDaemonClient()
			if err != nil {
				return err
			}

			if err := client.Pause(); err != nil {
				return err
			}

			fmt.Println("Pause requested - daemon will pause after current bead completes")
			return nil
		},
	}

	// Resume command
	resumeCmd := &cobra.Command{
		Use:   "resume",
		Short: "Resume the daemon from paused state",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := getDaemonClient()
			if err != nil {
				return err
			}

			if err := client.Resume(); err != nil {
				return err
			}

			fmt.Println("Resume requested - daemon will continue processing")
			return nil
		},
	}

	// Stop command
	stopCmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := getDaemonClient()
			if err != nil {
				return err
			}

			force := viper.GetBool(FlagForce)
			if err := client.Stop(force); err != nil {
				return err
			}

			if force {
				fmt.Println("Stop requested - daemon stopping immediately")
			} else {
				fmt.Println("Stop requested - daemon will stop after current bead completes")
			}
			return nil
		},
	}

	stopCmd.Flags().Bool(FlagForce, false, "Stop immediately without waiting for current bead")
	stopCmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = viper.BindPFlag(f.Name, f)
	})

	// Events command
	eventsCmd := &cobra.Command{
		Use:   "events",
		Short: "View recent events",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get log file path from daemon.json or use default
			logPath := viper.GetString(FlagLogFile)
			info, err := daemon.FindDaemonInfo("")
			if err == nil {
				logPath = info.LogPath
			} else {
				// Resolve relative path
				projectRoot := daemon.FindProjectRoot("")
				resolved, err := daemon.ResolvePaths(config.PathsConfig{Log: logPath}, projectRoot)
				if err == nil {
					logPath = resolved.Log
				}
			}

			count := viper.GetInt(FlagCount)
			follow := viper.GetBool(FlagFollow)

			if follow {
				return tailFollow(cmd.Context(), logPath)
			}
			return tailLast(logPath, count)
		},
	}

	eventsCmd.Flags().Bool(FlagFollow, false, "Follow event stream (like tail -f)")
	eventsCmd.Flags().Int(FlagCount, 20, "Number of recent events to show")
	eventsCmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = viper.BindPFlag(f.Name, f)
	})

	// Init command
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize Claude Code configuration for atari",
		Long: `Initialize Claude Code with rules, skills, and commands
for use with atari and the bd issue tracking system.

Creates the following structure:
  .claude/
    rules/
      issue-tracking.md
      session-protocol.md (unless --minimal)
    skills/
      bd-issue-tracking.md (unless --minimal)
    commands/
      bd-create.md (unless --minimal)
      bd-plan.md (unless --minimal)
      bd-plan-ultra.md (unless --minimal)
    CLAUDE.md (appended, not overwritten)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := initcmd.Options{
				DryRun:  viper.GetBool(FlagDryRun),
				Force:   viper.GetBool(FlagForce),
				Minimal: viper.GetBool(FlagMinimal),
				Global:  viper.GetBool(FlagGlobal),
			}

			_, err := initcmd.Run(opts)
			return err
		},
	}

	initCmd.Flags().Bool(FlagDryRun, false, "Show what would be changed without making changes")
	initCmd.Flags().Bool(FlagForce, false, "Overwrite existing files (creates timestamped backups)")
	initCmd.Flags().Bool(FlagMinimal, false, "Install only essential rules")
	initCmd.Flags().Bool(FlagGlobal, false, "Install to ~/.claude/ instead of ./.claude/")
	initCmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = viper.BindPFlag(f.Name, f)
	})

	// Register all commands
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(pauseCmd)
	rootCmd.AddCommand(resumeCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(eventsCmd)
	rootCmd.AddCommand(initCmd)

	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		logger.Error("command failed", "error", err)
		os.Exit(1)
	}
}
