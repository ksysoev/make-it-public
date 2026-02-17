package dummy

import (
	"fmt"
	"io"

	"github.com/TylerBrock/colorjson"
	"github.com/fatih/color"
	"gopkg.in/yaml.v3"
)

// YAMLFormatter formats YAML data with colorized, indented output.
// It parses YAML and uses the JSON formatter for colorized display.
type YAMLFormatter struct {
	colorFmt *colorjson.Formatter
}

// NewYAMLFormatter creates a new YAMLFormatter with color settings.
func NewYAMLFormatter() *YAMLFormatter {
	f := colorjson.NewFormatter()
	f.Indent = 2
	f.KeyColor = color.New(color.FgMagenta)
	f.StringColor = color.New(color.FgYellow)
	f.BoolColor = color.New(color.FgBlue)
	f.NumberColor = color.New(color.FgGreen)
	f.NullColor = color.New(color.FgRed)

	return &YAMLFormatter{
		colorFmt: f,
	}
}

// FormatInteractive parses YAML and displays it with colorized output.
// It converts YAML to JSON format for display using colorjson.
func (f *YAMLFormatter) FormatInteractive(w io.Writer, data []byte, _ map[string]string) error {
	var parsedData any

	if err := yaml.Unmarshal(data, &parsedData); err != nil {
		return fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	// Convert to JSON-compatible format for colorization
	// yaml.v3 returns map[interface{}]interface{}, but colorjson needs map[string]interface{}
	parsedData = convertYAMLValue(parsedData)

	output, err := f.colorFmt.Marshal(parsedData)
	if err != nil {
		return fmt.Errorf("failed to format YAML: %w", err)
	}

	// #nosec G705 -- This is CLI output formatting, not web output; XSS is not applicable
	_, err = fmt.Fprintf(w, "%s\n", output)

	return err
}

// FormatStructured parses YAML into structured data for logging.
func (f *YAMLFormatter) FormatStructured(data []byte, _ map[string]string) (key string, val any, err error) {
	var parsedData any

	if err := yaml.Unmarshal(data, &parsedData); err != nil {
		return bodyKey, string(data), fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	// Convert to JSON-compatible format for structured logging
	parsedData = convertYAMLValue(parsedData)

	return bodyKey, parsedData, nil
}

// convertYAMLValue recursively converts yaml.v3 types (map[interface{}]interface{})
// to JSON-compatible types (map[string]interface{}) for colorjson and slog.
func convertYAMLValue(val any) any {
	switch v := val.(type) {
	case map[interface{}]interface{}:
		result := make(map[string]interface{}, len(v))
		for key, value := range v {
			keyStr := fmt.Sprintf("%v", key)
			result[keyStr] = convertYAMLValue(value)
		}

		return result
	case map[string]interface{}:
		result := make(map[string]interface{}, len(v))
		for key, value := range v {
			result[key] = convertYAMLValue(value)
		}

		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = convertYAMLValue(item)
		}

		return result
	default:
		return v
	}
}
