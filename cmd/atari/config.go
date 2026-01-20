package main

// Flag names for Viper binding
const (
	// Global flags
	FlagVerbose    = "verbose"
	FlagConfig     = "config"
	FlagLogFile    = "log-file"
	FlagStateFile  = "state-file"
	FlagSocketPath = "socket-path"

	// Start command flags
	FlagTUI               = "tui"
	FlagMaxTurns          = "max-turns"
	FlagLabel             = "label"
	FlagEpic              = "epic"
	FlagUnassignedOnly    = "unassigned-only"
	FlagExcludeLabels     = "exclude-labels"
	FlagSelectionMode     = "selection-mode"
	FlagEagerSwitch       = "eager-switch"
	FlagPrompt            = "prompt"
	FlagBDActivityEnabled = "bd-activity-enabled"

	// Start command daemon mode flags
	FlagDaemon = "daemon"

	// Stop command flags
	FlagGraceful = "graceful"
	FlagForce    = "force"

	// Events command flags
	FlagFollow = "follow"
	FlagCount  = "count"

	// Output format flags
	FlagJSON = "json"

	// Init command flags
	FlagDryRun  = "dry-run"
	FlagMinimal = "minimal"
	FlagGlobal  = "global"

	// Observer flags
	FlagObserverEnabled      = "observer-enabled"
	FlagObserverModel        = "observer-model"
	FlagObserverLayout       = "observer-layout"
	FlagObserverRecentEvents = "observer-recent-events"

	// Graph flags
	FlagGraphEnabled = "graph-enabled"
	FlagGraphDensity = "graph-density"
)
