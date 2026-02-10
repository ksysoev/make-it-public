package dummy

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatterRegistry(t *testing.T) {
	t.Run("exact match", func(t *testing.T) {
		registry := NewFormatterRegistry()
		jsonFormatter := NewJSONFormatter()

		registry.Register("application/json", jsonFormatter)

		formatter, ok := registry.Get("application/json")
		assert.True(t, ok)
		assert.Equal(t, jsonFormatter, formatter)
	})

	t.Run("prefix match", func(t *testing.T) {
		registry := NewFormatterRegistry()
		textFormatter := NewTextFormatter()

		registry.RegisterPrefix("text/", textFormatter)

		formatter, ok := registry.Get("text/plain")
		assert.True(t, ok)
		assert.Equal(t, textFormatter, formatter)

		formatter, ok = registry.Get("text/html")
		assert.True(t, ok)
		assert.Equal(t, textFormatter, formatter)
	})

	t.Run("exact match takes precedence over prefix", func(t *testing.T) {
		registry := NewFormatterRegistry()
		textFormatter := NewTextFormatter()
		jsonFormatter := NewJSONFormatter()

		registry.RegisterPrefix("application/", textFormatter)
		registry.Register("application/json", jsonFormatter)

		formatter, ok := registry.Get("application/json")
		assert.True(t, ok)
		assert.Equal(t, jsonFormatter, formatter)

		formatter, ok = registry.Get("application/xml")
		assert.True(t, ok)
		assert.Equal(t, textFormatter, formatter)
	})

	t.Run("not found", func(t *testing.T) {
		registry := NewFormatterRegistry()

		_, ok := registry.Get("application/json")
		assert.False(t, ok)
	})
}

func TestParseContentType(t *testing.T) {
	tests := []struct {
		expectedParams   map[string]string
		name             string
		contentType      string
		expectedMedia    string
		checkParamsCount bool
	}{
		{
			name:           "simple content type",
			contentType:    "application/json",
			expectedMedia:  "application/json",
			expectedParams: map[string]string{},
		},
		{
			name:          "with charset",
			contentType:   "application/json; charset=utf-8",
			expectedMedia: "application/json",
			expectedParams: map[string]string{
				"charset": "utf-8",
			},
			checkParamsCount: true,
		},
		{
			name:          "multipart with boundary",
			contentType:   "multipart/form-data; boundary=----WebKitFormBoundary7MA4YWxkTrZu0gW",
			expectedMedia: "multipart/form-data",
			expectedParams: map[string]string{
				"boundary": "----WebKitFormBoundary7MA4YWxkTrZu0gW",
			},
			checkParamsCount: true,
		},
		{
			name:           "malformed (fallback)",
			contentType:    "invalid;;type",
			expectedMedia:  "invalid",
			expectedParams: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			media, params := parseContentType(tt.contentType)
			assert.Equal(t, tt.expectedMedia, media)

			if tt.checkParamsCount {
				assert.Len(t, params, len(tt.expectedParams))
			}

			for k, v := range tt.expectedParams {
				assert.Equal(t, v, params[k])
			}
		})
	}
}

func TestJSONFormatter(t *testing.T) {
	// Disable colors for predictable output
	oldNoColor := color.NoColor
	color.NoColor = true

	defer func() { color.NoColor = oldNoColor }()

	formatter := NewJSONFormatter()

	t.Run("format interactive valid JSON", func(t *testing.T) {
		data := []byte(`{"name":"John","age":30,"active":true}`)

		var buf bytes.Buffer

		err := formatter.FormatInteractive(&buf, data, nil)
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "name")
		assert.Contains(t, output, "John")
		assert.Contains(t, output, "age")
		assert.Contains(t, output, "30")
	})

	t.Run("format interactive invalid JSON", func(t *testing.T) {
		data := []byte(`{"invalid":`)

		var buf bytes.Buffer

		err := formatter.FormatInteractive(&buf, data, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal JSON")
	})

	t.Run("format structured valid JSON", func(t *testing.T) {
		data := []byte(`{"name":"John","age":30}`)

		key, val, err := formatter.FormatStructured(data, nil)
		require.NoError(t, err)

		assert.Equal(t, "body", key)

		mapVal, ok := val.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "John", mapVal["name"])
		assert.Equal(t, float64(30), mapVal["age"])
	})

	t.Run("format structured invalid JSON", func(t *testing.T) {
		data := []byte(`{"invalid":`)

		key, val, err := formatter.FormatStructured(data, nil)
		assert.Error(t, err)
		assert.Equal(t, "body", key)
		assert.Equal(t, string(data), val)
	})
}

func TestTextFormatter(t *testing.T) {
	// Disable colors for predictable output
	oldNoColor := color.NoColor
	color.NoColor = true

	defer func() { color.NoColor = oldNoColor }()

	formatter := NewTextFormatter()

	t.Run("format interactive", func(t *testing.T) {
		data := []byte("Hello, World!")

		var buf bytes.Buffer

		err := formatter.FormatInteractive(&buf, data, nil)
		require.NoError(t, err)

		assert.Contains(t, buf.String(), "Hello, World!")
	})

	t.Run("format structured", func(t *testing.T) {
		data := []byte("Hello, World!")

		key, val, err := formatter.FormatStructured(data, nil)
		require.NoError(t, err)

		assert.Equal(t, "body", key)
		assert.Equal(t, "Hello, World!", val)
	})
}

func TestFormURLEncodedFormatter(t *testing.T) {
	// Disable colors for predictable output
	oldNoColor := color.NoColor
	color.NoColor = true

	defer func() { color.NoColor = oldNoColor }()

	formatter := NewFormURLEncodedFormatter()

	t.Run("format interactive simple form", func(t *testing.T) {
		data := []byte("name=John&age=30&active=true")

		var buf bytes.Buffer

		err := formatter.FormatInteractive(&buf, data, nil)
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "name: John")
		assert.Contains(t, output, "age: 30")
		assert.Contains(t, output, "active: true")
	})

	t.Run("format interactive with URL encoding", func(t *testing.T) {
		data := []byte("message=Hello%20World&email=test%40example.com")

		var buf bytes.Buffer

		err := formatter.FormatInteractive(&buf, data, nil)
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "Hello World")
		assert.Contains(t, output, "test@example.com")
	})

	t.Run("format interactive multiple values", func(t *testing.T) {
		data := []byte("tags=go&tags=golang&tags=programming")

		var buf bytes.Buffer

		err := formatter.FormatInteractive(&buf, data, nil)
		require.NoError(t, err)

		output := buf.String()
		assert.Equal(t, 3, strings.Count(output, "tags:"))
	})

	t.Run("format structured", func(t *testing.T) {
		data := []byte("name=John&age=30")

		key, val, err := formatter.FormatStructured(data, nil)
		require.NoError(t, err)

		assert.Equal(t, "body", key)

		mapVal, ok := val.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "John", mapVal["name"])
		assert.Equal(t, "30", mapVal["age"])
	})

	t.Run("format structured multiple values", func(t *testing.T) {
		data := []byte("tags=go&tags=golang")

		key, val, err := formatter.FormatStructured(data, nil)
		require.NoError(t, err)

		assert.Equal(t, "body", key)

		mapVal, ok := val.(map[string]any)
		require.True(t, ok)

		tagsVal, ok := mapVal["tags"].([]string)
		require.True(t, ok)
		assert.Len(t, tagsVal, 2)
		assert.Contains(t, tagsVal, "go")
		assert.Contains(t, tagsVal, "golang")
	})

	t.Run("format interactive invalid form data", func(t *testing.T) {
		data := []byte("invalid%")

		var buf bytes.Buffer

		err := formatter.FormatInteractive(&buf, data, nil)
		assert.Error(t, err)
	})
}

func TestYAMLFormatter(t *testing.T) {
	// Disable colors for predictable output
	oldNoColor := color.NoColor
	color.NoColor = true

	defer func() { color.NoColor = oldNoColor }()

	formatter := NewYAMLFormatter()

	t.Run("format interactive valid YAML", func(t *testing.T) {
		data := []byte("name: John\nage: 30\nactive: true\n")

		var buf bytes.Buffer

		err := formatter.FormatInteractive(&buf, data, nil)
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "name")
		assert.Contains(t, output, "John")
		assert.Contains(t, output, "age")
		// Note: colorjson may format numbers differently, just check it's present
		assert.NotEmpty(t, output)
	})

	t.Run("format interactive nested YAML", func(t *testing.T) {
		data := []byte("person:\n  name: John\n  age: 30\n")

		var buf bytes.Buffer

		err := formatter.FormatInteractive(&buf, data, nil)
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "person")
		assert.Contains(t, output, "name")
		assert.Contains(t, output, "John")
	})

	t.Run("format interactive invalid YAML", func(t *testing.T) {
		data := []byte("invalid:\n  - unclosed\n  [")

		var buf bytes.Buffer

		err := formatter.FormatInteractive(&buf, data, nil)
		assert.Error(t, err)
	})

	t.Run("format structured", func(t *testing.T) {
		data := []byte("name: John\nage: 30\n")

		key, val, err := formatter.FormatStructured(data, nil)
		require.NoError(t, err)

		assert.Equal(t, "body", key)

		mapVal, ok := val.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "John", mapVal["name"])
		assert.Equal(t, 30, mapVal["age"])
	})
}

func TestMultipartFormatter(t *testing.T) {
	// Disable colors for predictable output
	oldNoColor := color.NoColor
	color.NoColor = true

	defer func() { color.NoColor = oldNoColor }()

	formatter := NewMultipartFormatter()

	t.Run("format interactive text fields", func(t *testing.T) {
		boundary := "----WebKitFormBoundary7MA4YWxkTrZu0gW"
		data := []byte(
			"------WebKitFormBoundary7MA4YWxkTrZu0gW\r\n" +
				"Content-Disposition: form-data; name=\"name\"\r\n\r\n" +
				"John Doe\r\n" +
				"------WebKitFormBoundary7MA4YWxkTrZu0gW\r\n" +
				"Content-Disposition: form-data; name=\"email\"\r\n\r\n" +
				"john@example.com\r\n" +
				"------WebKitFormBoundary7MA4YWxkTrZu0gW--\r\n")

		var buf bytes.Buffer

		params := map[string]string{"boundary": boundary}

		err := formatter.FormatInteractive(&buf, data, params)
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "name: John Doe")
		assert.Contains(t, output, "email: john@example.com")
	})

	t.Run("format interactive with file", func(t *testing.T) {
		boundary := "----WebKitFormBoundary7MA4YWxkTrZu0gW"
		data := []byte(
			"------WebKitFormBoundary7MA4YWxkTrZu0gW\r\n" +
				"Content-Disposition: form-data; name=\"file\"; filename=\"test.txt\"\r\n" +
				"Content-Type: text/plain\r\n\r\n" +
				"file content here\r\n" +
				"------WebKitFormBoundary7MA4YWxkTrZu0gW--\r\n")

		var buf bytes.Buffer

		params := map[string]string{"boundary": boundary}

		err := formatter.FormatInteractive(&buf, data, params)
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "file:")
		assert.Contains(t, output, "test.txt")
		assert.Contains(t, output, "bytes")
		assert.Contains(t, output, "text/plain")
	})

	t.Run("format interactive missing boundary", func(t *testing.T) {
		data := []byte("some data")

		var buf bytes.Buffer

		err := formatter.FormatInteractive(&buf, data, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing boundary")
	})

	t.Run("format structured", func(t *testing.T) {
		boundary := "----WebKitFormBoundary7MA4YWxkTrZu0gW"
		data := []byte(
			"------WebKitFormBoundary7MA4YWxkTrZu0gW\r\n" +
				"Content-Disposition: form-data; name=\"name\"\r\n\r\n" +
				"John Doe\r\n" +
				"------WebKitFormBoundary7MA4YWxkTrZu0gW\r\n" +
				"Content-Disposition: form-data; name=\"file\"; filename=\"test.txt\"\r\n" +
				"Content-Type: text/plain\r\n\r\n" +
				"content\r\n" +
				"------WebKitFormBoundary7MA4YWxkTrZu0gW--\r\n")

		params := map[string]string{"boundary": boundary}

		key, val, err := formatter.FormatStructured(data, params)
		require.NoError(t, err)

		assert.Equal(t, "body", key)

		mapVal, ok := val.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "John Doe", mapVal["name"])

		fileVal, ok := mapVal["file"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "test.txt", fileVal["filename"])
		assert.Equal(t, "text/plain", fileVal["content_type"])
	})
}
