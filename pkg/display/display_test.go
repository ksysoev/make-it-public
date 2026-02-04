package display

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// Disable colors for consistent test output
	color.NoColor = true
}

func TestNew(t *testing.T) {
	t.Run("interactive mode", func(t *testing.T) {
		disp := New(true)
		assert.True(t, disp.IsInteractive())
		assert.NotNil(t, disp.Writer())
		assert.NotNil(t, disp.ErrorWriter())
	})

	t.Run("non-interactive mode", func(t *testing.T) {
		disp := New(false)
		assert.False(t, disp.IsInteractive())
	})

	t.Run("respects NO_COLOR env", func(t *testing.T) {
		os.Setenv("NO_COLOR", "1")

		defer os.Unsetenv("NO_COLOR")

		disp := New(true)
		assert.True(t, disp.noColor)
	})
}

func TestDisplay_ShowConnected(t *testing.T) {
	t.Run("interactive mode shows banner", func(t *testing.T) {
		var buf bytes.Buffer

		disp := &Display{
			out:         &buf,
			errOut:      &buf,
			interactive: true,
			noColor:     true,
		}

		disp.ShowConnected("https://test.example.com", "localhost:8080")

		output := buf.String()
		assert.Contains(t, output, "make-it-public")
		assert.Contains(t, output, "[OK]")
		assert.Contains(t, output, "https://test.example.com")
		assert.Contains(t, output, "localhost:8080")
		assert.Contains(t, output, "Ctrl+C")
	})

	t.Run("non-interactive mode logs to slog", func(t *testing.T) {
		var buf bytes.Buffer

		disp := &Display{
			out:         &buf,
			errOut:      &buf,
			interactive: false,
			noColor:     true,
		}

		disp.ShowConnected("https://test.example.com", "localhost:8080")

		// In non-interactive mode, output goes to slog, not to the buffer
		// The buffer should be empty
		assert.Empty(t, buf.String(), "stdout should be empty in non-interactive mode")

		// Note: We can't easily test slog output here without setting up a custom handler
		// The important thing is that stdout is empty and the function doesn't panic
	})

	t.Run("works without local address", func(t *testing.T) {
		var buf bytes.Buffer

		disp := &Display{
			out:         &buf,
			errOut:      &buf,
			interactive: true,
			noColor:     true,
		}

		disp.ShowConnected("https://test.example.com", "")

		output := buf.String()
		assert.Contains(t, output, "https://test.example.com")
		assert.NotContains(t, output, "Forwarding")
	})
}

func TestDisplay_ShowError(t *testing.T) {
	t.Run("shows error with hint", func(t *testing.T) {
		var buf bytes.Buffer

		disp := &Display{
			out:         &buf,
			errOut:      &buf,
			interactive: true,
			noColor:     true,
		}

		testErr := errors.New("something went wrong")
		disp.ShowError("Test Error", testErr, "Try doing something else")

		output := buf.String()
		assert.Contains(t, output, "[ERR]")
		assert.Contains(t, output, "Test Error")
		assert.Contains(t, output, "something went wrong")
		assert.Contains(t, output, "Hint:")
		assert.Contains(t, output, "Try doing something else")
	})

	t.Run("shows error without hint", func(t *testing.T) {
		var buf bytes.Buffer

		disp := &Display{
			out:         &buf,
			errOut:      &buf,
			interactive: true,
			noColor:     true,
		}

		testErr := errors.New("something went wrong")
		disp.ShowError("Test Error", testErr, "")

		output := buf.String()
		assert.Contains(t, output, "[ERR]")
		assert.Contains(t, output, "Test Error")
		assert.Contains(t, output, "something went wrong")
		assert.NotContains(t, output, "Hint:")
	})

	t.Run("shows error without error details", func(t *testing.T) {
		var buf bytes.Buffer

		disp := &Display{
			out:         &buf,
			errOut:      &buf,
			interactive: true,
			noColor:     true,
		}

		disp.ShowError("Test Error", nil, "Some hint")

		output := buf.String()
		assert.Contains(t, output, "[ERR]")
		assert.Contains(t, output, "Test Error")
		assert.Contains(t, output, "Hint:")
	})
}

func TestDisplay_ShowErrorSimple(t *testing.T) {
	var buf bytes.Buffer

	disp := &Display{
		out:         &buf,
		errOut:      &buf,
		interactive: true,
		noColor:     true,
	}

	disp.ShowErrorSimple("simple error message")

	output := buf.String()
	assert.Contains(t, output, "simple error message")
}

func TestDisplay_ShowConnecting(t *testing.T) {
	t.Run("interactive mode returns spinner", func(t *testing.T) {
		var buf bytes.Buffer

		disp := &Display{
			out:         &buf,
			errOut:      &buf,
			interactive: true,
			noColor:     true,
		}

		spinner := disp.ShowConnecting("example.com:8080")
		require.NotNil(t, spinner)

		// Give spinner time to write at least one frame
		time.Sleep(150 * time.Millisecond)
		spinner.Stop()

		output := buf.String()
		assert.Contains(t, output, "Connecting to example.com:8080")
	})

	t.Run("non-interactive mode returns nil", func(t *testing.T) {
		var buf bytes.Buffer

		disp := &Display{
			out:         &buf,
			errOut:      &buf,
			interactive: false,
			noColor:     true,
		}

		spinner := disp.ShowConnecting("example.com:8080")
		assert.Nil(t, spinner)
	})
}

func TestSpinner(t *testing.T) {
	t.Run("start and stop", func(t *testing.T) {
		var buf bytes.Buffer

		spinner := NewSpinner("testing...", &buf)

		spinner.Start()
		time.Sleep(150 * time.Millisecond)
		spinner.Stop()

		// Should have written something
		assert.NotEmpty(t, buf.String())
	})

	t.Run("success message", func(t *testing.T) {
		var buf bytes.Buffer

		spinner := NewSpinner("testing...", &buf)

		spinner.Start()
		time.Sleep(50 * time.Millisecond)
		spinner.Success("Done!")

		output := buf.String()
		assert.Contains(t, output, "[OK]")
		assert.Contains(t, output, "Done!")
	})

	t.Run("fail message", func(t *testing.T) {
		var buf bytes.Buffer

		spinner := NewSpinner("testing...", &buf)

		spinner.Start()
		time.Sleep(50 * time.Millisecond)
		spinner.Fail("Failed!")

		output := buf.String()
		assert.Contains(t, output, "[ERR]")
		assert.Contains(t, output, "Failed!")
	})

	t.Run("success without start", func(t *testing.T) {
		var buf bytes.Buffer

		spinner := NewSpinner("testing...", &buf)

		spinner.Success("Done!")

		output := buf.String()
		assert.Contains(t, output, "[OK]")
		assert.Contains(t, output, "Done!")
	})

	t.Run("double start is safe", func(t *testing.T) {
		var buf bytes.Buffer

		spinner := NewSpinner("testing...", &buf)

		spinner.Start()
		spinner.Start() // Should not panic or cause issues
		time.Sleep(50 * time.Millisecond)
		spinner.Stop()
	})

	t.Run("double stop is safe", func(t *testing.T) {
		var buf bytes.Buffer

		spinner := NewSpinner("testing...", &buf)

		spinner.Start()
		time.Sleep(50 * time.Millisecond)
		spinner.Stop()
		spinner.Stop() // Should not panic
	})
}

func TestBannerFormatting(t *testing.T) {
	t.Run("banner has proper box structure", func(t *testing.T) {
		var buf bytes.Buffer

		disp := &Display{
			out:         &buf,
			errOut:      &buf,
			interactive: true,
			noColor:     true,
		}

		disp.ShowConnected("https://test.example.com", "localhost:8080")

		output := buf.String()
		lines := strings.Split(output, "\n")

		// Check for box borders
		hasTopBorder := false
		hasBottomBorder := false

		for _, line := range lines {
			if strings.HasPrefix(line, "+") && strings.HasSuffix(line, "+") && strings.Contains(line, "=") {
				if !hasTopBorder {
					hasTopBorder = true
				} else {
					hasBottomBorder = true
				}
			}
		}

		assert.True(t, hasTopBorder, "should have top border")
		assert.True(t, hasBottomBorder, "should have bottom border")
	})
}

func TestDisplay_ShowRequestSeparator(t *testing.T) {
	t.Run("interactive mode shows separator with client IP", func(t *testing.T) {
		var buf bytes.Buffer

		disp := &Display{
			out:         &buf,
			errOut:      &buf,
			interactive: true,
			noColor:     true,
		}

		disp.ShowRequestSeparator("192.168.1.100")

		output := buf.String()
		assert.Contains(t, output, "192.168.1.100")
		assert.Contains(t, output, "────")
	})

	t.Run("non-interactive mode shows nothing", func(t *testing.T) {
		var buf bytes.Buffer

		disp := &Display{
			out:         &buf,
			errOut:      &buf,
			interactive: false,
			noColor:     true,
		}

		disp.ShowRequestSeparator("192.168.1.100")

		assert.Empty(t, buf.String())
	})

	t.Run("handles IPv6 addresses", func(t *testing.T) {
		var buf bytes.Buffer

		disp := &Display{
			out:         &buf,
			errOut:      &buf,
			interactive: true,
			noColor:     true,
		}

		disp.ShowRequestSeparator("2001:0db8:85a3::8a2e:0370:7334")

		output := buf.String()
		assert.Contains(t, output, "2001:0db8:85a3::8a2e:0370:7334")
		assert.Contains(t, output, "────")
	})

	t.Run("handles empty client IP", func(t *testing.T) {
		var buf bytes.Buffer

		disp := &Display{
			out:         &buf,
			errOut:      &buf,
			interactive: true,
			noColor:     true,
		}

		disp.ShowRequestSeparator("")

		output := buf.String()
		assert.Contains(t, output, "────")
	})
}
