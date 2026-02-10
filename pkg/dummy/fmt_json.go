package dummy

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/TylerBrock/colorjson"
	"github.com/fatih/color"
)

// JSONFormatter formats JSON data with colorized, indented output.
type JSONFormatter struct {
	colorFmt *colorjson.Formatter
}

// NewJSONFormatter creates and configures a new JSONFormatter with color settings.
func NewJSONFormatter() *JSONFormatter {
	f := colorjson.NewFormatter()
	f.Indent = 2
	f.KeyColor = color.New(color.FgMagenta)
	f.StringColor = color.New(color.FgYellow)
	f.BoolColor = color.New(color.FgBlue)
	f.NumberColor = color.New(color.FgGreen)
	f.NullColor = color.New(color.FgRed)

	return &JSONFormatter{
		colorFmt: f,
	}
}

// FormatInteractive writes colorized, indented JSON to the writer.
func (f *JSONFormatter) FormatInteractive(w io.Writer, data []byte, _ map[string]string) error {
	var parsedData any

	if err := json.Unmarshal(data, &parsedData); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	output, err := f.colorFmt.Marshal(parsedData)
	if err != nil {
		return fmt.Errorf("failed to format JSON: %w", err)
	}

	_, err = fmt.Fprintf(w, "%s\n", output)

	return err
}

// FormatStructured returns parsed JSON as structured data for logging.
func (f *JSONFormatter) FormatStructured(data []byte, _ map[string]string) (key string, val any, err error) {
	var parsedData any

	if err := json.Unmarshal(data, &parsedData); err != nil {
		return bodyKey, string(data), fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return bodyKey, parsedData, nil
}
