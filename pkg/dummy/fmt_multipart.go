package dummy

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"

	"github.com/fatih/color"
)

// MultipartFormatter formats multipart/form-data content.
type MultipartFormatter struct {
	fieldColor *color.Color
	valueColor *color.Color
	metaColor  *color.Color
}

// NewMultipartFormatter creates a new MultipartFormatter with color settings.
func NewMultipartFormatter() *MultipartFormatter {
	return &MultipartFormatter{
		fieldColor: color.New(color.FgCyan),
		valueColor: color.New(color.FgYellow),
		metaColor:  color.New(color.FgGreen),
	}
}

// FormatInteractive parses and displays multipart form data with field names, values, and file metadata.
func (f *MultipartFormatter) FormatInteractive(w io.Writer, data []byte, params map[string]string) error {
	boundary := params["boundary"]
	if boundary == "" {
		return fmt.Errorf("missing boundary parameter for multipart/form-data")
	}

	reader := multipart.NewReader(bytes.NewReader(data), boundary)

	partNum := 0

	for {
		part, err := reader.NextPart()

		if err == io.EOF {
			break
		}

		if err != nil {
			return fmt.Errorf("failed to read multipart part: %w", err)
		}

		partNum++

		fieldName := part.FormName()
		fileName := part.FileName()

		if partNum > 1 {
			_, _ = fmt.Fprintln(w)
		}

		if fileName != "" {
			// File upload
			contentType := part.Header.Get("Content-Type")
			if contentType == "" {
				contentType = "application/octet-stream"
			}

			fileData, err := io.ReadAll(part)
			if err != nil {
				return fmt.Errorf("failed to read file content: %w", err)
			}

			_, _ = fmt.Fprintf(w, "%s: [file: %s, %s, %s]\n",
				f.fieldColor.Sprint(fieldName),
				f.valueColor.Sprint(fileName),
				f.metaColor.Sprintf("%d bytes", len(fileData)),
				f.metaColor.Sprint(contentType))
		} else {
			// Text field
			value, err := io.ReadAll(part)
			if err != nil {
				return fmt.Errorf("failed to read field content: %w", err)
			}

			_, _ = fmt.Fprintf(w, "%s: %s\n",
				f.fieldColor.Sprint(fieldName),
				f.valueColor.Sprint(string(value)))
		}
	}

	return nil
}

// FormatStructured parses multipart data into structured format for logging.
func (f *MultipartFormatter) FormatStructured(data []byte, params map[string]string) (key string, val any, err error) {
	boundary := params["boundary"]
	if boundary == "" {
		return bodyKey, nil, fmt.Errorf("missing boundary parameter for multipart/form-data")
	}

	reader := multipart.NewReader(bytes.NewReader(data), boundary)

	result := make(map[string]any)

	for {
		part, err := reader.NextPart()

		if err == io.EOF {
			break
		}

		if err != nil {
			return bodyKey, string(data), fmt.Errorf("failed to read multipart part: %w", err)
		}

		fieldName := part.FormName()
		fileName := part.FileName()

		if fileName != "" {
			// File upload
			contentType := part.Header.Get("Content-Type")

			fileData, err := io.ReadAll(part)
			if err != nil {
				return bodyKey, string(data), fmt.Errorf("failed to read file content: %w", err)
			}

			result[fieldName] = map[string]any{
				"filename":     fileName,
				"size":         len(fileData),
				"content_type": contentType,
			}
		} else {
			// Text field
			value, err := io.ReadAll(part)
			if err != nil {
				return bodyKey, string(data), fmt.Errorf("failed to read field content: %w", err)
			}

			result[fieldName] = string(value)
		}
	}

	return bodyKey, result, nil
}

// extractBoundary extracts the boundary parameter from a Content-Type header.
// This is a helper function for parsing multipart content types.
func extractBoundary(contentType string) (string, error) {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "", fmt.Errorf("failed to parse Content-Type: %w", err)
	}

	boundary := params["boundary"]
	if boundary == "" {
		return "", fmt.Errorf("missing boundary parameter")
	}

	return boundary, nil
}
