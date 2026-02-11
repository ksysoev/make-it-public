package dummy

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/coder/websocket"
	"github.com/fatih/color"
)

// WSConfig holds configuration for the WebSocket echo server.
type WSConfig struct {
	Interactive bool `mapstructure:"interactive"`
}

// WSEchoServer is a WebSocket echo server that logs incoming connections and messages.
type WSEchoServer struct {
	isReady     chan struct{}
	addr        string
	interactive bool
}

// NewWSEchoServer creates and initializes a new WebSocket echo server instance.
// It validates the configuration and prepares the server for starting.
// Accepts cfg WSConfig containing the interactive mode flag.
// Returns a pointer to the WSEchoServer instance and an error if the configuration is invalid.
func NewWSEchoServer(cfg WSConfig) (*WSEchoServer, error) {
	return &WSEchoServer{
		isReady:     make(chan struct{}),
		interactive: cfg.Interactive,
	}, nil
}

// Run starts the WebSocket echo server and listens for incoming connections.
// It initializes a TCP listener on a random port, announces readiness by closing the isReady channel,
// and serves WebSocket connections using the server instance.
// Accepts ctx to manage the server lifecycle and handle shutdown signals.
// Returns an error if the listener fails to start or the server encounters issues during execution.
func (s *WSEchoServer) Run(ctx context.Context) error {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		close(s.isReady)
		return fmt.Errorf("failed to start websocket echo server: %w", err)
	}

	s.addr = l.Addr().String()

	srv := http.Server{
		Handler:           s,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      0, // No timeout for WebSocket connections
	}

	go func() {
		<-ctx.Done()

		if err := srv.Close(); err != nil {
			fmt.Printf("Error closing websocket server: %v\n", err)
		}
	}()

	close(s.isReady)

	return srv.Serve(l)
}

// Addr waits for the server to be ready and retrieves the bound address as a string.
// It blocks until the readiness signal is received by reading from isReady channel.
// Returns the server's address in "host:port" format.
func (s *WSEchoServer) Addr() string {
	<-s.isReady
	return s.addr
}

// ServeHTTP handles incoming HTTP requests and upgrades them to WebSocket connections.
// It logs the handshake request, upgrades the connection, and enters a message echo loop.
func (s *WSEchoServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Log the WebSocket handshake request
	if s.interactive {
		s.logHandshakeInteractive(r)
	} else {
		s.logHandshakeStructured(r)
	}

	// Upgrade to WebSocket with increased message size limit (10 MB)
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // Allow connections from any origin for testing
	})
	if err != nil {
		slog.Error("Failed to upgrade to WebSocket", "error", err)
		return
	}

	// Set message read limit to 10 MB for large messages
	conn.SetReadLimit(10 * 1024 * 1024)

	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	// Log connection establishment
	if s.interactive {
		fmt.Fprintln(os.Stdout, color.GreenString("✓ WebSocket connection established"))
		fmt.Fprintln(os.Stdout)
	} else {
		slog.Info("websocket connection established", "remote_addr", r.RemoteAddr)
	}

	// Echo loop
	s.echoLoop(r.Context(), conn, r.RemoteAddr)

	// Log disconnection
	if s.interactive {
		fmt.Fprintln(os.Stdout, color.RedString("✕ WebSocket connection closed"))
		fmt.Fprintln(os.Stdout)
	} else {
		slog.Info("websocket connection closed", "remote_addr", r.RemoteAddr)
	}
}

// logHandshakeInteractive outputs the handshake request in a colorized, human-readable format to stdout.
func (s *WSEchoServer) logHandshakeInteractive(r *http.Request) {
	tx := color.New(color.FgCyan)
	tx.SetWriter(os.Stdout)

	_, _ = fmt.Fprintf(os.Stdout, "── WebSocket connection from %s ──\n", r.RemoteAddr)
	_, _ = fmt.Fprintf(os.Stdout, "%s %s %s\n", r.Method, r.URL.String(), r.Proto)
	printHeaders(r.Header, os.Stdout)

	_, _ = fmt.Fprintln(os.Stdout)
	tx.UnsetWriter(os.Stdout)
}

// logHandshakeStructured outputs the handshake request using structured logging (slog).
func (s *WSEchoServer) logHandshakeStructured(r *http.Request) {
	// Convert headers to a loggable format
	headers := make(map[string]string)
	for name, values := range r.Header {
		headers[name] = values[0]
		if len(values) > 1 {
			for i := 1; i < len(values); i++ {
				headers[name] += ", " + values[i]
			}
		}
	}

	slog.Info("websocket handshake request",
		slog.String("method", r.Method),
		slog.String("url", r.URL.String()),
		slog.String("proto", r.Proto),
		slog.String("remote_addr", r.RemoteAddr),
		slog.Any("headers", headers))
}

// echoLoop continuously reads messages from the WebSocket connection and echoes them back.
func (s *WSEchoServer) echoLoop(ctx context.Context, conn *websocket.Conn, remoteAddr string) {
	for {
		msgType, data, err := conn.Read(ctx)
		if err != nil {
			if websocket.CloseStatus(err) != websocket.StatusNormalClosure &&
				websocket.CloseStatus(err) != websocket.StatusGoingAway {
				slog.Debug("error reading websocket message", "error", err, "remote_addr", remoteAddr)
			}

			return
		}

		// Log the received message
		if s.interactive {
			s.logMessageInteractive(msgType, data)
		} else {
			s.logMessageStructured(msgType, data, remoteAddr)
		}

		// Echo the message back
		err = conn.Write(ctx, msgType, data)
		if err != nil {
			slog.Error("error writing websocket message", "error", err, "remote_addr", remoteAddr)
			return
		}
	}
}

// logMessageInteractive outputs the received message in a colorized, human-readable format to stdout.
func (s *WSEchoServer) logMessageInteractive(msgType websocket.MessageType, data []byte) {
	arrow := color.New(color.FgYellow).Sprint("◀")
	typeColor := color.New(color.FgCyan)
	contentColor := color.New(color.FgGreen)

	var msgTypeStr string

	switch msgType {
	case websocket.MessageText:
		msgTypeStr = "text"
	case websocket.MessageBinary:
		msgTypeStr = "binary"
	}

	_, _ = fmt.Fprintf(os.Stdout, "%s %s (%d bytes)\n",
		arrow,
		typeColor.Sprint(msgTypeStr),
		len(data))

	// Display content
	if msgType == websocket.MessageText {
		// Text message: display as-is
		_, _ = fmt.Fprintln(os.Stdout, contentColor.Sprint(string(data)))
	} else {
		// Binary message: show hex preview (first 256 bytes)
		previewLen := len(data)
		if previewLen > 256 {
			previewLen = 256
		}

		hexDump := hex.Dump(data[:previewLen])
		_, _ = fmt.Fprint(os.Stdout, contentColor.Sprint(hexDump))

		if len(data) > 256 {
			_, _ = fmt.Fprintf(os.Stdout, "... (%s)\n", color.New(color.FgYellow).Sprintf("%d more bytes", len(data)-256))
		}
	}

	_, _ = fmt.Fprintln(os.Stdout)
}

// logMessageStructured outputs the received message using structured logging (slog).
func (s *WSEchoServer) logMessageStructured(msgType websocket.MessageType, data []byte, remoteAddr string) {
	var msgTypeStr string

	switch msgType {
	case websocket.MessageText:
		msgTypeStr = "text"
	case websocket.MessageBinary:
		msgTypeStr = "binary"
	}

	attrs := []any{
		slog.String("type", msgTypeStr),
		slog.Int("size", len(data)),
		slog.String("remote_addr", remoteAddr),
	}

	// Include content for text messages, hex preview for binary
	if msgType == websocket.MessageText {
		attrs = append(attrs, slog.String("content", string(data)))
	} else {
		previewLen := len(data)
		if previewLen > 64 {
			previewLen = 64
		}

		attrs = append(attrs, slog.String("hex_preview", hex.EncodeToString(data[:previewLen])))
	}

	slog.Info("websocket message received", attrs...)
}
