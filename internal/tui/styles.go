package tui

import "github.com/charmbracelet/lipgloss"

// styles contains all lipgloss styles used by the TUI.
var styles = struct {
	// Layout styles
	Container lipgloss.Style
	Divider   lipgloss.Style
	Spacer    lipgloss.Style

	// Header styles
	Status   lipgloss.Style
	Cost     lipgloss.Style
	Duration lipgloss.Style
	Bead     lipgloss.Style
	Turns    lipgloss.Style

	// Footer style
	Footer lipgloss.Style

	// Event styles
	Tool       lipgloss.Style
	BeadStatus lipgloss.Style
	Session    lipgloss.Style
	Error      lipgloss.Style

	// Status colors
	StatusIdle    lipgloss.Style
	StatusWorking lipgloss.Style
	StatusPaused  lipgloss.Style
	StatusStopped lipgloss.Style
	StatusStalled lipgloss.Style

	// Focus indicators
	FocusedBorder   lipgloss.Style
	UnfocusedBorder lipgloss.Style
}{
	// Layout styles
	Container: lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")),

	Divider: lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")),

	Spacer: lipgloss.NewStyle(),

	// Header styles
	Status: lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212")),

	Cost: lipgloss.NewStyle().
		Foreground(lipgloss.Color("220")),

	Duration: lipgloss.NewStyle().
		Foreground(lipgloss.Color("220")),

	Bead: lipgloss.NewStyle().
		Foreground(lipgloss.Color("39")),

	Turns: lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")),

	// Footer style
	Footer: lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")),

	// Event styles
	Tool: lipgloss.NewStyle().
		Foreground(lipgloss.Color("250")),

	BeadStatus: lipgloss.NewStyle().
		Foreground(lipgloss.Color("114")),

	Session: lipgloss.NewStyle().
		Foreground(lipgloss.Color("177")),

	Error: lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")),

	// Status colors
	StatusIdle: lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")),

	StatusWorking: lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("82")),

	StatusPaused: lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")),

	StatusStopped: lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")),

	StatusStalled: lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196")), // Red, bold for attention

	// Focus indicators
	FocusedBorder: lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")), // Bright blue for focused

	UnfocusedBorder: lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")), // Dimmed gray for unfocused
}

// graphStyles contains styles specific to graph rendering.
var graphStyles = struct {
	// Node styles
	Node         lipgloss.Style // Default node style
	NodeSelected lipgloss.Style // Selected/focused node
	NodeCurrent  lipgloss.Style // Currently processing bead
	NodeDimmed   lipgloss.Style // Out-of-view node (dependency from different view)
	NodeFailed   lipgloss.Style // Bead in backoff (failed but will retry)
	NodeAbandoned lipgloss.Style // Bead exceeded max failures

	// Glyph styles
	GlyphDimmed lipgloss.Style // Dimmed tree glyphs for out-of-view connections
}{
	Node: lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")),

	NodeSelected: lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")). // Bright cyan for selection
		Background(lipgloss.Color("236")), // Subtle background

	NodeCurrent: lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("82")). // Bright green for current bead
		Background(lipgloss.Color("22")), // Green-tinted background

	NodeDimmed: lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")), // Subtle gray - visible but obviously different

	NodeFailed: lipgloss.NewStyle().
		Foreground(lipgloss.Color("208")), // Orange for beads in backoff

	NodeAbandoned: lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")), // Red for beads that exceeded max failures

	GlyphDimmed: lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")), // Dark gray for tree lines to dimmed nodes
}
