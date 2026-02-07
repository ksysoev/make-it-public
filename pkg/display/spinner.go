package display

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/fatih/color"
)

// Spinner displays an animated spinner in the terminal during long-running operations.
type Spinner struct {
	out       io.Writer
	done      chan struct{}
	message   string
	mu        sync.Mutex
	running   bool
	closeOnce sync.Once
	wg        sync.WaitGroup
}

// spinnerFrames defines the ASCII animation frames for the spinner.
var spinnerFrames = []string{"|", "/", "-", "\\"}

// NewSpinner creates a new spinner with the given message.
// The spinner will display in the provided writer.
func NewSpinner(message string, out io.Writer) *Spinner {
	return &Spinner{
		message: message,
		out:     out,
		done:    make(chan struct{}),
	}
}

// Start begins the spinner animation in a goroutine.
// Call Stop() or one of the completion methods to stop the spinner.
func (s *Spinner) Start() {
	s.mu.Lock()

	if s.running {
		s.mu.Unlock()
		return
	}

	s.running = true
	s.wg.Add(1)
	s.mu.Unlock()

	go s.animate()
}

// closeChannel safely closes the done channel, preventing double-close panics.
func (s *Spinner) closeChannel() {
	s.closeOnce.Do(func() {
		close(s.done)
	})
}

// Stop halts the spinner animation without displaying a final message.
func (s *Spinner) Stop() {
	s.mu.Lock()

	if !s.running {
		s.mu.Unlock()
		return
	}

	s.running = false
	s.closeChannel()
	s.mu.Unlock()

	// Wait for animation goroutine to finish
	s.wg.Wait()

	// Clear the current line
	fmt.Fprint(s.out, "\r\033[K")
}

// Success stops the spinner and displays a success message.
func (s *Spinner) Success(message string) {
	s.mu.Lock()

	if !s.running {
		s.mu.Unlock()
		// Still print success even if spinner wasn't running
		successColor := color.New(color.FgGreen, color.Bold)

		fmt.Fprint(s.out, "\r\033[K")
		successColor.Fprint(s.out, "[OK] ")
		fmt.Fprintln(s.out, message)

		return
	}

	s.running = false
	s.closeChannel()
	s.mu.Unlock()

	// Wait for animation goroutine to finish
	s.wg.Wait()

	successColor := color.New(color.FgGreen, color.Bold)

	fmt.Fprint(s.out, "\r\033[K")
	successColor.Fprint(s.out, "[OK] ")
	fmt.Fprintln(s.out, message)
}

// Fail stops the spinner and displays a failure message.
func (s *Spinner) Fail(message string) {
	s.mu.Lock()

	if !s.running {
		s.mu.Unlock()
		// Still print failure even if spinner wasn't running
		failColor := color.New(color.FgRed, color.Bold)

		fmt.Fprint(s.out, "\r\033[K")
		failColor.Fprint(s.out, "[ERR] ")
		fmt.Fprintln(s.out, message)

		return
	}

	s.running = false
	s.closeChannel()
	s.mu.Unlock()

	// Wait for animation goroutine to finish
	s.wg.Wait()

	failColor := color.New(color.FgRed, color.Bold)

	fmt.Fprint(s.out, "\r\033[K")
	failColor.Fprint(s.out, "[ERR] ")
	fmt.Fprintln(s.out, message)
}

// animate runs the spinner animation loop.
func (s *Spinner) animate() {
	defer s.wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	spinnerColor := color.New(color.FgCyan, color.Bold)
	frameIdx := 0

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			frame := spinnerFrames[frameIdx%len(spinnerFrames)]

			fmt.Fprint(s.out, "\r\033[K")
			spinnerColor.Fprint(s.out, frame)
			fmt.Fprintf(s.out, " %s", s.message)

			frameIdx++
		}
	}
}

// ShowConnecting creates and starts a spinner for the connection phase.
// Returns the spinner so it can be stopped when connection completes.
func (d *Display) ShowConnecting(server string) *Spinner {
	if !d.interactive {
		return nil
	}

	message := fmt.Sprintf("Connecting to %s...", server)
	spinner := NewSpinner(message, d.out)
	spinner.Start()

	return spinner
}
