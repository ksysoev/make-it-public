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

type Server struct {
	isReady chan struct{}
	jsonFmt *colorjson.Formatter
	addr    string
}

func New() *Server {
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
	}
}

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

func (s *Server) Addr() string {
	<-s.isReady
	return s.addr
}

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

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`ok`))
}

// printBody selects the appropriate formatter based on content type
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

func (s *Server) printText(data []byte) error {
	tx := color.New(color.FgGreen)
	tx.SetWriter(os.Stdout)

	defer tx.UnsetWriter(os.Stdout)

	_, err := tx.Fprintln(os.Stdout, string(data))

	return err
}

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
