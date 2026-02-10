package dummy

import (
	"fmt"
	"io"
	"net/url"
	"sort"

	"github.com/fatih/color"
)

// FormURLEncodedFormatter formats application/x-www-form-urlencoded data.
type FormURLEncodedFormatter struct {
	keyColor   *color.Color
	valueColor *color.Color
}

// NewFormURLEncodedFormatter creates a new FormURLEncodedFormatter with color settings.
func NewFormURLEncodedFormatter() *FormURLEncodedFormatter {
	return &FormURLEncodedFormatter{
		keyColor:   color.New(color.FgMagenta),
		valueColor: color.New(color.FgYellow),
	}
}

// FormatInteractive parses and displays form data as colorized key-value pairs.
func (f *FormURLEncodedFormatter) FormatInteractive(w io.Writer, data []byte, _ map[string]string) error {
	values, err := url.ParseQuery(string(data))
	if err != nil {
		return fmt.Errorf("failed to parse form data: %w", err)
	}

	// Sort keys for consistent output
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, key := range keys {
		vals := values[key]
		for _, val := range vals {
			_, err := fmt.Fprintf(w, "%s: %s\n",
				f.keyColor.Sprint(key),
				f.valueColor.Sprint(val))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// FormatStructured parses form data into a map for structured logging.
func (f *FormURLEncodedFormatter) FormatStructured(data []byte, _ map[string]string) (key string, val any, err error) {
	values, err := url.ParseQuery(string(data))
	if err != nil {
		return bodyKey, string(data), fmt.Errorf("failed to parse form data: %w", err)
	}

	// Convert url.Values to a simple map for cleaner logging
	result := make(map[string]any)

	for k, v := range values {
		if len(v) == 1 {
			result[k] = v[0]
		} else {
			result[k] = v
		}
	}

	return bodyKey, result, nil
}
