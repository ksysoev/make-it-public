package display

import (
	"fmt"

	"github.com/fatih/color"
)

// ShowError displays a formatted error message with an optional hint.
// The error is displayed prominently with color coding to draw attention.
func (d *Display) ShowError(title string, err error, hint string) {
	// Define colors
	errLabelColor := color.New(color.FgRed, color.Bold)
	errMsgColor := color.New(color.FgRed)
	hintLabelColor := color.New(color.FgYellow, color.Bold)
	hintColor := color.New(color.FgWhite)

	fmt.Fprintln(d.errOut)

	// Error header
	errLabelColor.Fprint(d.errOut, "[ERR] ")
	errLabelColor.Fprintln(d.errOut, title)
	fmt.Fprintln(d.errOut)

	// Error details
	if err != nil {
		fmt.Fprint(d.errOut, "  ")
		errMsgColor.Fprintln(d.errOut, err.Error())
		fmt.Fprintln(d.errOut)
	}

	// Hint
	if hint != "" {
		hintLabelColor.Fprintln(d.errOut, "  Hint:")
		fmt.Fprint(d.errOut, "  ")
		hintColor.Fprintln(d.errOut, hint)
		fmt.Fprintln(d.errOut)
	}
}

// ShowErrorSimple displays a simple error message without additional formatting.
// Use this for minor errors that don't need prominent display.
func (d *Display) ShowErrorSimple(message string) {
	errColor := color.New(color.FgRed)
	errColor.Fprintln(d.errOut, message)
}
