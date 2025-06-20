package dummy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/TylerBrock/colorjson"
	"github.com/fatih/color"
)

type Response struct {
	Body        string
	ContentType string
	Status      int
}

type Server struct {
	isReady chan struct{}
	jsonFmt *colorjson.Formatter
	addr    string
	resp    Response
}

// New creates and initializes a new Server instance.
// It sets up a custom JSON formatter with specific colors for formatting JSON data during HTTP request handling.
// Returns a pointer to the newly created Server with an initialized readiness channel and JSON formatter.
func New(resp Response) *Server {
	f := colorjson.NewFormatter()
	f.Indent = 2
	f.KeyColor = color.New(color.FgMagenta)
	f.StringColor = color.New(color.FgYellow)
	f.BoolColor = color.New(color.FgBlue)
	f.NumberColor = color.New(color.FgGreen)
	f.NullColor = color.New(color.FgRed)

	return &Server{
		isReady: make(chan struct{}),
		jsonFmt: f,
		resp:    resp,
	}
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
// It logs the HTTP method, URL, protocol, and headers to the standard output. If a request body exists, it formats
// and logs the content based on the "Content-Type" header. Responds with an HTTP 200 status and "ok" message.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tx := color.New(color.FgGreen)
	tx.SetWriter(os.Stdout)

	_, _ = fmt.Fprintf(os.Stdout, "%s %s %s\n", r.Method, r.URL.String(), r.Proto)
	printHeaders(r.Header, os.Stdout)

	_, _ = fmt.Fprintln(os.Stdout)
	tx.UnsetWriter(os.Stdout)

	// Read the request body
	if r.Body != nil {
		defer func() { _ = r.Body.Close() }()

		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			fmt.Printf("Error reading body: %v\n", err)
		} else if len(bodyBytes) > 0 {
			// Format the body based on content type
			contentType := r.Header.Get("Content-Type")

			if err := s.printBody(bodyBytes, contentType); err != nil {
				fmt.Printf("Error formatting body: %v\n", err)
			}
		}
	}

	w.WriteHeader(s.resp.Status)

	if s.resp.ContentType != "" {
		w.Header().Set("Content-Type", s.resp.ContentType)
	}

	if _, err := w.Write([]byte(s.resp.Body)); err != nil {
		fmt.Printf("Error writing response: %v\n", err)
	}
}

// printBody processes and outputs the given data based on its content type.
// It determines the appropriate formatting method by parsing and evaluating the content type string.
// Accepts data as a byte slice representing the request body and contentType as a string indicating the MIME type.
// Returns an error if the content type is unsupported or if a formatting operation fails.
func (s *Server) printBody(data []byte, contentType string) error {
	// Parse the content type, ignoring parameters like charset
	contentType = strings.TrimSpace(strings.Split(contentType, ";")[0])

	switch {
	case contentType == "application/json":
		return s.printJSON(data)
	case strings.HasPrefix(contentType, "text/"):
		return s.printText(data)
	default:
		return fmt.Errorf("unsupported content type: %s", contentType)
	}
}

// printText outputs the given byte slice as a green-colored string to the standard output.
// It sets the writer to apply the green foreground color, writes the string representation of data,
// and resets the writer after execution.
// Returns an error if the writing operation fails.
func (s *Server) printText(data []byte) error {
	tx := color.New(color.FgGreen)
	tx.SetWriter(os.Stdout)

	defer tx.UnsetWriter(os.Stdout)

	_, err := tx.Fprintln(os.Stdout, string(data))

	return err
}

// printJSON unmarshals raw JSON data, reformats it using the server's formatter, and writes it to the standard output.
// It returns an error if JSON unmarshalling or formatting fails, or if there is an issue writing to output.
func (s *Server) printJSON(data []byte) error {
	var parsedData any

	if err := json.Unmarshal(data, &parsedData); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	output, err := s.jsonFmt.Marshal(parsedData)
	if err != nil {
		return fmt.Errorf("failed to format JSON: %w", err)
	}

	_, err = fmt.Fprintf(os.Stdout, "%s\n", output)

	return err
}

// printHeaders formats and writes sorted HTTP headers to the specified output writer.
// It iterates over the provided headers, sorts them alphabetically, and writes each header-value pair to the writer.
// Accepts headers as an http.Header object and out as an io.Writer for output.
// Returns no value but may fail silently if there are errors in writing to the output.
func printHeaders(headers http.Header, out io.Writer) {
	headerNames := make([]string, 0, len(headers))
	for header := range headers {
		headerNames = append(headerNames, header)
	}

	sort.Strings(headerNames)

	for _, header := range headerNames {
		values := headers[header]
		for _, value := range values {
			_, _ = fmt.Fprintf(out, "%s: %s\n", header, value)
		}
	}
}
