package main

// Flag names for Viper binding
const (
	// Global flags
	FlagVerbose    = "verbose"
	FlagLogFile    = "log-file"
	FlagStateFile  = "state-file"
	FlagSocketPath = "socket-path"

	// Start command flags
	FlagTUI      = "tui"
	FlagMaxTurns = "max-turns"
	FlagLabel    = "label"
	FlagPrompt   = "prompt"
	FlagModel    = "model"

	// Stop command flags
	FlagGraceful = "graceful"

	// Events command flags
	FlagFollow = "follow"
	FlagCount  = "count"
)
