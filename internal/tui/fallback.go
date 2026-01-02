package tui

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/term"
)

// isTerminal returns true if both stdout and stdin are TTYs.
func isTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd())) && term.IsTerminal(int(os.Stdin.Fd()))
}

// terminalSize returns the current terminal width and height.
// Returns 0, 0 if the terminal size cannot be determined.
func terminalSize() (width, height int) {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 0, 0
	}
	return width, height
}

// terminalTooSmall returns true if the terminal is below the minimum size.
func terminalTooSmall() bool {
	width, height := terminalSize()
	return width < minWidth || height < minHeight
}

// runSimple provides line-by-line output for non-interactive environments.
// It reads events from the channel, formats them, and prints to stdout.
// Exits when the channel closes or on interrupt signal.
func (t *TUI) runSimple() error {
	// Set up interrupt handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan)

	for {
		select {
		case <-sigChan:
			// Clean exit on interrupt
			return nil
		case event, ok := <-t.eventChan:
			if !ok {
				// Channel closed, exit cleanly
				return nil
			}

			// Format and print the event
			text := Format(event)
			if text == "" {
				continue
			}

			timestamp := time.Now().Format("15:04:05")
			fmt.Printf("%s %s\n", timestamp, text)
		}
	}
}
