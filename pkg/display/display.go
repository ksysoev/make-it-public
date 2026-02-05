// Package display provides terminal output formatting for the mit CLI.
// It handles colorized output, spinners, banners, and error formatting
// for interactive terminal sessions.
package display

import (
	"io"
	"os"

	"github.com/fatih/color"
)

// Display handles all terminal presentation for the CLI.
// It adapts output based on whether the terminal is interactive
// and respects the NO_COLOR environment variable.
type Display struct {
	out         io.Writer
	errOut      io.Writer
	interactive bool
	noColor     bool
}

// New creates a new Display instance configured for the given terminal mode.
// When interactive is false or NO_COLOR is set, colored output is disabled.
//
// IMPORTANT: This function modifies the global color.NoColor variable which affects
// all Display instances in the application. This design assumes a single Display
// instance will be created per application lifecycle. If multiple instances with
// different color settings are needed, the Display struct should be refactored to
// avoid relying on global state.
func New(interactive bool) *Display {
	noColor := os.Getenv("NO_COLOR") != "" || !interactive

	if noColor {
		color.NoColor = true
	}

	return &Display{
		out:         os.Stdout,
		errOut:      os.Stderr,
		interactive: interactive,
		noColor:     noColor,
	}
}

// IsInteractive returns true if the display is in interactive mode.
func (d *Display) IsInteractive() bool {
	return d.interactive
}

// Writer returns the output writer used by the display.
func (d *Display) Writer() io.Writer {
	return d.out
}

// ErrorWriter returns the error output writer used by the display.
func (d *Display) ErrorWriter() io.Writer {
	return d.errOut
}
