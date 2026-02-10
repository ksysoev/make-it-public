package dummy

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
)

const contentTypeJSON = "application/json"

type Config struct {
	Body        string   `mapstructure:"body"`
	JSON        string   `mapstructure:"json"`
	Headers     []string `mapstructure:"headers"`
	Status      int      `mapstructure:"status"`
	Interactive bool     `mapstructure:"interactive"`
}

type Response struct {
	Headers     http.Header
	Body        string
	ContentType string
	Status      int
}

type Server struct {
	registry    *FormatterRegistry
	isReady     chan struct{}
	addr        string
	resp        Response
	interactive bool
}

// New creates and initializes a new Server instance configured with the provided settings.
// It validates the Config parameters and determines the response type (JSON or plain text).
// Accepts cfg Config containing the response body, JSON string, HTTP status code, and custom headers.
// Returns a pointer to the Server instance and an error if the configuration is invalid (e.g., status code out of range, both body and JSON set, malformed headers).
func New(cfg Config) (*Server, error) {
	if cfg.Status < 200 || cfg.Status >= 600 {
		return nil, fmt.Errorf("invalid status code: %d", cfg.Status)
	}

	resp := Response{
		Status:  cfg.Status,
		Headers: make(http.Header),
	}

	switch {
	case cfg.JSON != "" && cfg.Body != "":
		return nil, fmt.Errorf("cannot specify both body and json responses at the same time")
	case cfg.JSON != "":
		resp.Body = cfg.JSON
		resp.ContentType = contentTypeJSON
	case cfg.Body != "":
		resp.Body = cfg.Body
		resp.ContentType = "text/plain"
	}

	// Parse custom headers
	for _, header := range cfg.Headers {
		parts := strings.SplitN(header, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid header format: %s (expected 'Name:Value')", header)
		}

		headerName := strings.TrimSpace(parts[0])
		headerValue := strings.TrimSpace(parts[1])

		if headerName == "" {
			return nil, fmt.Errorf("header name cannot be empty")
		}

		resp.Headers.Add(headerName, headerValue)
	}

	// Initialize formatter registry
	registry := NewFormatterRegistry()
	registry.Register(contentTypeJSON, NewJSONFormatter())
	registry.Register("application/x-www-form-urlencoded", NewFormURLEncodedFormatter())

	yamlFormatter := NewYAMLFormatter()
	registry.Register("application/yaml", yamlFormatter)
	registry.Register("application/x-yaml", yamlFormatter)
	registry.Register("text/yaml", yamlFormatter)
	registry.Register("multipart/form-data", NewMultipartFormatter())
	registry.RegisterPrefix("text/", NewTextFormatter())

	return &Server{
		isReady:     make(chan struct{}),
		registry:    registry,
		resp:        resp,
		interactive: cfg.Interactive,
	}, nil
}

// Run starts the server and listens for incoming HTTP connections.
// It initializes a TCP listener on a random port, announces readiness by closing the isReady channel,
// and serves HTTP requests using the Server instance.
// Accepts ctx to manage the server lifecycle and handle shutdown signals.
// Returns an error if the listener fails to start or the server encounters issues during execution.
func (s *Server) Run(ctx context.Context) error {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		close(s.isReady)
		return fmt.Errorf("failed to start local server: %w", err)
	}

	s.addr = l.Addr().String()

	srv := http.Server{
		Handler:           s,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      5 * time.Second,
	}

	go func() {
		<-ctx.Done()

		if err := srv.Close(); err != nil {
			fmt.Printf("Error closing server: %v\n", err)
		}
	}()

	close(s.isReady)

	return srv.Serve(l)
}

// Addr waits for the server to be ready and retrieves the bound address as a string.
// It blocks until the readiness signal is received by reading from isReady channel.
// Returns the server's address in "host:port" format.
func (s *Server) Addr() string {
	<-s.isReady
	return s.addr
}

// ServeHTTP handles incoming HTTP requests, logs request details, and optionally formats the request body for output.
// In interactive mode, it logs the HTTP method, URL, protocol, and headers with colors to stdout.
// In non-interactive mode, it uses structured logging (slog) for all request details.
// Responds with configured status, headers, and body.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Read the request body first (needed for both modes)
	var (
		bodyBytes []byte
		bodyErr   error
	)

	if r.Body != nil {
		defer func() { _ = r.Body.Close() }()

		bodyBytes, bodyErr = io.ReadAll(r.Body)
		if bodyErr != nil {
			slog.Error("Error reading request body", "error", bodyErr)
		}
	}

	// Log request based on mode
	if s.interactive {
		s.logInteractive(r, bodyBytes)
	} else {
		s.logStructured(r, bodyBytes)
	}

	// Apply custom headers first
	for name, values := range s.resp.Headers {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Set Content-Type if specified (can be overridden by custom headers)
	if s.resp.ContentType != "" && w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", s.resp.ContentType)
	}

	w.WriteHeader(s.resp.Status)

	if _, err := w.Write([]byte(s.resp.Body)); err != nil {
		slog.Error("Error writing response", "error", err)
	}
}

// logInteractive outputs the request in a colorized, human-readable format to stdout.
func (s *Server) logInteractive(r *http.Request, bodyBytes []byte) {
	tx := color.New(color.FgGreen)
	tx.SetWriter(os.Stdout)

	_, _ = fmt.Fprintf(os.Stdout, "%s %s %s\n", r.Method, r.URL.String(), r.Proto)
	printHeaders(r.Header, os.Stdout)

	_, _ = fmt.Fprintln(os.Stdout)
	tx.UnsetWriter(os.Stdout)

	if len(bodyBytes) > 0 {
		contentType := r.Header.Get("Content-Type")

		if err := s.printBody(bodyBytes, contentType); err != nil {
			fmt.Printf("Error formatting body: %v\n", err)
		}
	}
}

// logStructured outputs the request using structured logging (slog).
func (s *Server) logStructured(r *http.Request, bodyBytes []byte) {
	// Convert headers to a loggable format
	headers := make(map[string]string)
	for name, values := range r.Header {
		headers[name] = strings.Join(values, ", ")
	}

	// Build log attributes
	attrs := []any{
		slog.String("method", r.Method),
		slog.String("url", r.URL.String()),
		slog.String("proto", r.Proto),
		slog.String("remote_addr", r.RemoteAddr),
		slog.Any("headers", headers),
	}

	// Add body if present
	if len(bodyBytes) > 0 {
		contentType := r.Header.Get("Content-Type")

		attrs = append(attrs,
			slog.Int("body_size", len(bodyBytes)),
			slog.String("content_type", contentType))

		// Try to format using registered formatters
		mediaType, params := parseContentType(contentType)
		if formatter, ok := s.registry.Get(mediaType); ok {
			key, val, err := formatter.FormatStructured(bodyBytes, params)
			if err == nil {
				attrs = append(attrs, slog.Any(key, val))
			} else {
				attrs = append(attrs, slog.String("body", string(bodyBytes)))
			}
		}
		// If no formatter found, just log size/content-type (don't add body)
	}

	slog.Info("incoming HTTP request", attrs...)
}

// printBody processes and outputs the given data based on its content type.
// It determines the appropriate formatting method by parsing and evaluating the content type string.
// Accepts data as a byte slice representing the request body and contentType as a string indicating the MIME type.
// Returns an error if the content type is unsupported or if a formatting operation fails.
func (s *Server) printBody(data []byte, contentType string) error {
	mediaType, params := parseContentType(contentType)

	formatter, ok := s.registry.Get(mediaType)
	if !ok {
		return fmt.Errorf("unsupported content type: %s", mediaType)
	}

	return formatter.FormatInteractive(os.Stdout, data, params)
}

// printHeaders formats and writes sorted HTTP headers to the specified output writer.
// It iterates over the provided headers, sorts them alphabetically, and writes each header-value pair to the writer.
// Header names are displayed in cyan color, while values are displayed in the default color.
// Accepts headers as an http.Header object and out as an io.Writer for output.
// Returns no value but may fail silently if there are errors in writing to the output.
func printHeaders(headers http.Header, out io.Writer) {
	headerNames := make([]string, 0, len(headers))
	for header := range headers {
		headerNames = append(headerNames, header)
	}

	sort.Strings(headerNames)

	headerNameColor := color.New(color.FgCyan)
	headerValueColor := color.New(color.FgWhite)

	for _, header := range headerNames {
		values := headers[header]
		for _, value := range values {
			_, _ = fmt.Fprintf(out, "%s: %s\n",
				headerNameColor.Sprint(header),
				headerValueColor.Sprint(value))
		}
	}
}
