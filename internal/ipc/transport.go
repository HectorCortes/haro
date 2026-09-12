// Package ipc carries the CLI<->broker transport boundary: endpoint identity
// (socket.go), platform transports (transport_unix/transport_windows.go),
// and the JSON-RPC client multiplexer (client.go).
package ipc

import (
	"net"
	"time"
)

// Transport abstracts the per-project broker endpoint: Unix domain sockets on
// Unix-like systems, pure-Go named pipes on Windows. Implementations are
// selected by build tags; behavior parity is required.
type Transport interface {
	// Endpoint derives the platform-specific endpoint for a canonical project
	// root. Keeping identity on the transport lets platform implementations
	// own their endpoint shape while callers retain one seam.
	Endpoint(root string) (string, error)
	// Listen binds endpoint and starts accepting connections.
	Listen(endpoint string) (net.Listener, error)
	// Dial connects to endpoint.
	Dial(endpoint string) (net.Conn, error)
	// DialTimeout connects to endpoint with a bounded wait.
	DialTimeout(endpoint string, d time.Duration) (net.Conn, error)
}

// DefaultTransport returns the platform transport.
func DefaultTransport() Transport { return defaultTransport() }
