package dummy

import (
	"io"

	"github.com/fatih/color"
)

// TextFormatter formats plain text data with green color.
type TextFormatter struct {
	textColor *color.Color
}

// NewTextFormatter creates a new TextFormatter with green text color.
func NewTextFormatter() *TextFormatter {
	return &TextFormatter{
		textColor: color.New(color.FgGreen),
	}
}

// FormatInteractive writes green-colored text to the writer.
func (f *TextFormatter) FormatInteractive(w io.Writer, data []byte, _ map[string]string) error {
	f.textColor.SetWriter(w)
	defer f.textColor.UnsetWriter(w)

	_, err := f.textColor.Fprintln(w, string(data))

	return err
}

// FormatStructured returns the text as a string for logging.
func (f *TextFormatter) FormatStructured(data []byte, _ map[string]string) (key string, val any, err error) {
	return bodyKey, string(data), nil
}
