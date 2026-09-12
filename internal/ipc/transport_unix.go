//go:build !windows

package ipc

import (
	"fmt"
	"net"
	"os"
	"syscall"
	"time"
)

// unixTransport implements Transport over Unix domain sockets.
type unixTransport struct{}

func defaultTransport() Transport { return unixTransport{} }

func (unixTransport) Endpoint(root string) (string, error) { return SocketPath(root) }

func (unixTransport) Listen(endpoint string) (net.Listener, error) {
	l, err := net.Listen("unix", endpoint)
	if err != nil {
		return nil, fmt.Errorf("listen %q: %w", endpoint, err)
	}
	// Broker endpoints are private to the user: enforce 0600 on the socket.
	if err := os.Chmod(endpoint, 0o600); err != nil {
		_ = l.Close()
		return nil, fmt.Errorf("chmod socket %q: %w", endpoint, err)
	}
	return l, nil
}

func (unixTransport) Dial(endpoint string) (net.Conn, error) {
	return net.Dial("unix", endpoint)
}

func (unixTransport) DialTimeout(endpoint string, d time.Duration) (net.Conn, error) {
	return net.DialTimeout("unix", endpoint, d)
}

// SocketMetadataSafe reports whether an existing endpoint file is a usable
// stale socket: regular or socket file with 0600 metadata. Foreign or
// world-readable files are never removed or reused as broker endpoints.
func socketMetadataSafe(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		if st.Mode&0o777 != 0o600 {
			return false
		}
	} else if info.Mode().Perm() != 0o600 {
		return false
	}
	return info.Mode().IsRegular() || info.Mode()&os.ModeSocket != 0
}

// RemoveStaleEndpoint removes endpoint only when it exists, has safe 0600
// metadata, and refuses connections (zombie socket recovery).
func RemoveStaleEndpoint(tr Transport, endpoint string) {
	if conn, err := tr.DialTimeout(endpoint, 250*time.Millisecond); err == nil {
		_ = conn.Close()
		return // live endpoint: never remove
	}
	if !socketMetadataSafe(endpoint) {
		return
	}
	_ = os.Remove(endpoint)
}
