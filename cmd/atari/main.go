package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var version = "dev"

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

	// Start command (placeholder)
	startCmd := &cobra.Command{
		Use:   "start",
		Short: "Start the atari daemon",
		Long: `Start the atari daemon to process available beads.

The daemon will poll bd ready for available work and spawn Claude Code
sessions to work on each bead until completion.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if viper.GetBool(FlagVerbose) {
				logLevel.Set(slog.LevelDebug)
				logger.Debug("verbose logging enabled")
			}

			logger.Info("atari starting",
				"version", version,
				"log_file", viper.GetString(FlagLogFile),
				"state_file", viper.GetString(FlagStateFile),
			)

			// TODO: Implement controller start
			fmt.Println("atari start - implementation pending")
			fmt.Println()
			fmt.Println("See docs/IMPLEMENTATION.md for the implementation plan.")
			return nil
		},
	}

	// Start command specific flags
	startCmd.Flags().Bool(FlagTUI, false, "Enable terminal UI")
	startCmd.Flags().Int(FlagMaxTurns, 50, "Max turns per Claude session")
	startCmd.Flags().String(FlagLabel, "", "Filter bd ready by label")
	startCmd.Flags().String(FlagPrompt, "", "Custom prompt template file")
	startCmd.Flags().String(FlagModel, "opus", "Claude model to use")

	startCmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = viper.BindPFlag(f.Name, f)
	})

	// Status command (placeholder)
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show daemon status",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement status via socket
			fmt.Println("atari status - implementation pending")
			return nil
		},
	}

	// Pause command (placeholder)
	pauseCmd := &cobra.Command{
		Use:   "pause",
		Short: "Pause the daemon after current bead completes",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement pause via socket
			fmt.Println("atari pause - implementation pending")
			return nil
		},
	}

	// Resume command (placeholder)
	resumeCmd := &cobra.Command{
		Use:   "resume",
		Short: "Resume the daemon from paused state",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement resume via socket
			fmt.Println("atari resume - implementation pending")
			return nil
		},
	}

	// Stop command (placeholder)
	stopCmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement stop via socket
			fmt.Println("atari stop - implementation pending")
			return nil
		},
	}

	stopCmd.Flags().Bool(FlagGraceful, true, "Wait for current bead to complete")
	stopCmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = viper.BindPFlag(f.Name, f)
	})

	// Events command (placeholder)
	eventsCmd := &cobra.Command{
		Use:   "events",
		Short: "View recent events",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: Implement events tail
			fmt.Println("atari events - implementation pending")
			return nil
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
