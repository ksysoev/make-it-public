package display

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/fatih/color"
)

const (
	bannerWidth = 65
)

// ShowConnected displays a colorful banner with the public URL and forwarding info.
// In interactive mode, shows a colorful banner.
// In non-interactive mode, logs connection details using structured logging.
func (d *Display) ShowConnected(publicURL, localAddr string) {
	if !d.interactive {
		// In non-interactive mode, log connection info using slog
		if localAddr != "" {
			slog.Info("service is now publicly accessible",
				slog.String("public_url", publicURL),
				slog.String("forwarding", localAddr))
		} else {
			slog.Info("service is now publicly accessible",
				slog.String("public_url", publicURL))
		}

		return
	}

	// Define colors
	borderColor := color.New(color.FgCyan, color.Bold)
	titleColor := color.New(color.FgGreen, color.Bold)
	successColor := color.New(color.FgGreen)
	labelColor := color.New(color.FgWhite, color.Bold)
	urlColor := color.New(color.FgHiCyan, color.Bold)
	addrColor := color.New(color.FgYellow)
	hintColor := color.New(color.FgHiBlack)

	// Build the banner
	fmt.Fprintln(d.out)

	// Top border
	borderColor.Fprintln(d.out, "+"+strings.Repeat("=", bannerWidth-2)+"+")

	// Empty line
	d.printBannerEmptyLine(borderColor)

	// Title
	d.printBannerLineColored(borderColor, titleColor, "make-it-public")

	// Empty line
	d.printBannerEmptyLine(borderColor)

	// Success message
	d.printBannerLineWithPrefix(borderColor, successColor, "[OK]", " Your service is now publicly accessible!")

	// Empty line
	d.printBannerEmptyLine(borderColor)

	// Public URL
	d.printBannerKeyValue(borderColor, labelColor, urlColor, "Public URL", publicURL)

	// Forwarding
	if localAddr != "" {
		d.printBannerKeyValue(borderColor, labelColor, addrColor, "Forwarding", localAddr)
	}

	// Empty line
	d.printBannerEmptyLine(borderColor)

	// Hint
	d.printBannerLineColored(borderColor, hintColor, "Press Ctrl+C to disconnect")

	// Empty line
	d.printBannerEmptyLine(borderColor)

	// Bottom border
	borderColor.Fprintln(d.out, "+"+strings.Repeat("=", bannerWidth-2)+"+")

	fmt.Fprintln(d.out)
}

// printBannerEmptyLine prints an empty line within the banner borders.
func (d *Display) printBannerEmptyLine(borderColor *color.Color) {
	borderColor.Fprint(d.out, "|")
	fmt.Fprint(d.out, strings.Repeat(" ", bannerWidth-2))
	borderColor.Fprintln(d.out, "|")
}

// printBannerLineColored prints a colored line within the banner borders.
func (d *Display) printBannerLineColored(borderColor, contentColor *color.Color, content string) {
	borderColor.Fprint(d.out, "|")
	fmt.Fprint(d.out, "  ")
	contentColor.Fprint(d.out, content)

	padding := bannerWidth - 2 - len(content) - 2
	if padding < 0 {
		padding = 0
	}

	fmt.Fprint(d.out, strings.Repeat(" ", padding))
	borderColor.Fprintln(d.out, "|")
}

// printBannerLineWithPrefix prints a line with a colored prefix and content.
func (d *Display) printBannerLineWithPrefix(borderColor, prefixColor *color.Color, prefix, content string) {
	borderColor.Fprint(d.out, "|")
	fmt.Fprint(d.out, "  ")
	prefixColor.Fprint(d.out, prefix)
	fmt.Fprint(d.out, content)

	padding := bannerWidth - 2 - len(prefix) - len(content) - 2
	if padding < 0 {
		padding = 0
	}

	fmt.Fprint(d.out, strings.Repeat(" ", padding))
	borderColor.Fprintln(d.out, "|")
}

// printBannerKeyValue prints a key-value pair within the banner.
func (d *Display) printBannerKeyValue(borderColor, labelColor, valueColor *color.Color, label, value string) {
	borderColor.Fprint(d.out, "|")
	fmt.Fprint(d.out, "  ")
	labelColor.Fprint(d.out, label)
	fmt.Fprint(d.out, "   ")
	valueColor.Fprint(d.out, value)

	// Calculate padding
	contentLen := len(label) + 3 + len(value)
	padding := bannerWidth - 2 - contentLen - 2

	if padding < 0 {
		padding = 0
	}

	fmt.Fprint(d.out, strings.Repeat(" ", padding))
	borderColor.Fprintln(d.out, "|")
}
