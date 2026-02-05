package display

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/fatih/color"
)

const separatorWidth = 65

// ShowRequestSeparator displays a visual separator for incoming HTTP requests.
// It shows the client IP address in a styled separator line to distinguish
// between different requests in the terminal output.
func (d *Display) ShowRequestSeparator(clientIP string) {
	if !d.interactive {
		return
	}

	// Create colors for the separator
	separatorColor := color.New(color.FgHiBlack)
	ipColor := color.New(color.FgCyan)

	// Calculate padding to fill the line
	// Format: ──── <clientIP> ───────────────────────────────────
	prefixLen := 4 // "──── "
	suffixStart := prefixLen + 1 + utf8.RuneCountInString(clientIP) + 1
	suffixLen := separatorWidth - suffixStart

	if suffixLen < 0 {
		suffixLen = 10 // minimum suffix
	}

	// Print the separator line
	fmt.Fprintln(d.out)
	separatorColor.Fprint(d.out, strings.Repeat("─", prefixLen))
	fmt.Fprint(d.out, " ")
	ipColor.Fprint(d.out, clientIP)
	fmt.Fprint(d.out, " ")
	separatorColor.Fprintln(d.out, strings.Repeat("─", suffixLen))
}
