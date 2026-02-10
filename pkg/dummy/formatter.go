package dummy

import (
	"io"
	"mime"
	"strings"
)

const bodyKey = "body"

// ContentFormatter formats request body data for display.
// Implementations handle specific content types (JSON, YAML, form data, etc.)
type ContentFormatter interface {
	// FormatInteractive writes a colorized, human-readable representation to the writer.
	// Accepts w as the output writer, data as raw bytes, and params containing MIME type parameters.
	// Returns an error if formatting fails.
	FormatInteractive(w io.Writer, data []byte, params map[string]string) error

	// FormatStructured returns structured data suitable for slog logging.
	// Accepts data as raw bytes and params containing MIME type parameters.
	// Returns the attribute key name, the structured value for logging, and an error if parsing fails.
	FormatStructured(data []byte, params map[string]string) (string, any, error)
}

// FormatterRegistry maps content types to their formatters.
// It supports both exact matches and prefix matches (e.g., "text/*").
type FormatterRegistry struct {
	exact  map[string]ContentFormatter
	prefix []prefixMatcher
}

type prefixMatcher struct {
	formatter ContentFormatter
	prefix    string
}

// NewFormatterRegistry creates and initializes a new empty FormatterRegistry.
func NewFormatterRegistry() *FormatterRegistry {
	return &FormatterRegistry{
		exact:  make(map[string]ContentFormatter),
		prefix: make([]prefixMatcher, 0),
	}
}

// Register adds a formatter for an exact content type match.
// Accepts contentType as the MIME type string and formatter as the ContentFormatter implementation.
func (r *FormatterRegistry) Register(contentType string, formatter ContentFormatter) {
	r.exact[contentType] = formatter
}

// RegisterPrefix adds a formatter for content types matching a prefix.
// Useful for handling patterns like "text/*".
// Accepts prefix as the content type prefix and formatter as the ContentFormatter implementation.
func (r *FormatterRegistry) RegisterPrefix(prefix string, formatter ContentFormatter) {
	r.prefix = append(r.prefix, prefixMatcher{
		prefix:    prefix,
		formatter: formatter,
	})
}

// Get retrieves the formatter for the given content type.
// First checks for exact matches, then tries prefix matches.
// Accepts contentType as the MIME type string.
// Returns the ContentFormatter and true if found, or nil and false if not found.
func (r *FormatterRegistry) Get(contentType string) (ContentFormatter, bool) {
	// Try exact match first
	if formatter, ok := r.exact[contentType]; ok {
		return formatter, true
	}

	// Try prefix matches
	for _, pm := range r.prefix {
		if strings.HasPrefix(contentType, pm.prefix) {
			return pm.formatter, true
		}
	}

	return nil, false
}

// parseContentType extracts the MIME type and parameters from a Content-Type header.
// Accepts contentType as the full Content-Type header value.
// Returns the base MIME type and a map of parameters (e.g., charset, boundary).
func parseContentType(contentType string) (mediaType string, params map[string]string) {
	mt, pm, err := mime.ParseMediaType(contentType)
	if err != nil {
		// Fallback: just strip parameters manually
		mt = strings.TrimSpace(strings.Split(contentType, ";")[0])
		pm = make(map[string]string)
	}

	return mt, pm
}
