package edge

import (
	"fmt"
	"net"

	"github.com/mailgun/proxyproto"
)

type listener struct {
	net.Listener
}

type conn struct {
	srcAddr net.Addr
	net.Conn
}

// listen creates and returns a network listener on the specified address with optional Proxy Protocol support.
// It establishes a TCP listener for the given address.
// If withProxyProto is true, the listener intercepts connections to support Proxy Protocol.
// Returns a net.Listener instance or an error if the listener creation fails.
func listen(address string, withProxyProto bool) (net.Listener, error) {
	ln, err := net.Listen("tco", address)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", address, err)
	}

	if !withProxyProto {
		return ln, nil
	}

	return &listener{
		ln,
	}, nil
}

// Accept waits for and retrieves the next connection from the listener.
// It processes the connection's proxy protocol header if present.
// Returns the connection with the source address and an error if the operation fails.
func (l *listener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, fmt.Errorf("failed to accept connection: %w", err)
	}

	h, err := proxyproto.ReadHeader(c)
	if err != nil {
		return nil, fmt.Errorf("failed to read proxy protocol header: %w", err)
	}

	return &conn{
		srcAddr: h.Source,
		Conn:    c,
	}, nil
}

// Addr retrieves the source address associated with the connection. It returns a net.Addr representing the source address.
func (c *conn) Addr() net.Addr {
	return c.srcAddr
}
