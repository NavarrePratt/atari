package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/npratt/atari/internal/config"
	"github.com/npratt/atari/internal/controller"
	"github.com/npratt/atari/internal/daemon"
	"github.com/npratt/atari/internal/events"
	"github.com/npratt/atari/internal/runner"
	"github.com/npratt/atari/internal/shutdown"
	"github.com/npratt/atari/internal/testutil"
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
	rootCmd.PersistentFlags().String(FlagLogFile, ".atari/atari.log", "Log file path")
	rootCmd.PersistentFlags().String(FlagStateFile, ".atari/state.json", "State file path")
	rootCmd.PersistentFlags().String(FlagSocketPath, ".atari/atari.sock", "Unix socket path for daemon control")

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

The daemon will poll bd ready for available work and spawn Claude Code
sessions to work on each bead until completion.

Use --daemon to run in the background.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if viper.GetBool(FlagVerbose) {
				logLevel.Set(slog.LevelDebug)
				logger.Debug("verbose logging enabled")
			}

			// Load default config and apply CLI flags
			cfg := config.Default()
			cfg.Paths.Log = viper.GetString(FlagLogFile)
			cfg.Paths.State = viper.GetString(FlagStateFile)
			cfg.Paths.Socket = viper.GetString(FlagSocketPath)
			if label := viper.GetString(FlagLabel); label != "" {
				cfg.WorkQueue.Label = label
			}
			cfg.AgentID = viper.GetString(FlagAgentID)
			cfg.BDActivity.Enabled = viper.GetBool(FlagBDActivityEnabled)

			// Find project root for path resolution
			projectRoot := daemon.FindProjectRoot("")

			// Resolve all paths to absolute
			var err error
			cfg.Paths, err = daemon.ResolvePaths(cfg.Paths, projectRoot)
			if err != nil {
				return fmt.Errorf("resolve paths: %w", err)
			}

			// Check if daemon is already running
			if viper.GetBool(FlagDaemon) {
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
				"agent_id", cfg.AgentID,
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

			// Create controller
			ctrl := controller.New(cfg, wq, router, cmdRunner, processRunner, logger)

			// Create daemon for RPC control
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
	startCmd.Flags().Int(FlagMaxTurns, 50, "Max turns per Claude session")
	startCmd.Flags().String(FlagLabel, "", "Filter bd ready by label")
	startCmd.Flags().String(FlagPrompt, "", "Custom prompt template file")
	startCmd.Flags().String(FlagModel, "opus", "Claude model to use")
	startCmd.Flags().String(FlagAgentID, "", "Agent bead ID for state reporting (e.g., bd-xxx)")
	startCmd.Flags().Bool(FlagBDActivityEnabled, true, "Enable BD activity watcher")

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

	// Register all commands
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(pauseCmd)
	rootCmd.AddCommand(resumeCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(eventsCmd)

	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		logger.Error("command failed", "error", err)
		os.Exit(1)
	}
}
