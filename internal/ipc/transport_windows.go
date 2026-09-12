//go:build windows

package ipc

import (
	"fmt"
	"net"
	"time"

	"github.com/Microsoft/go-winio"
)

// windowsTransport implements Transport over pure-Go named pipes.
type windowsTransport struct{}

func defaultTransport() Transport { return windowsTransport{} }

func (windowsTransport) Endpoint(root string) (string, error) { return SocketPath(root) }

func (windowsTransport) Listen(endpoint string) (net.Listener, error) {
	l, err := winio.ListenPipe(endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("listen pipe %q: %w", endpoint, err)
	}
	return l, nil
}

func (windowsTransport) Dial(endpoint string) (net.Conn, error) {
	return winio.DialPipe(endpoint, nil)
}

func (windowsTransport) DialTimeout(endpoint string, d time.Duration) (net.Conn, error) {
	return winio.DialPipe(endpoint, &d)
}

// RemoveStaleEndpoint is a no-op on Windows: named pipes have no filesystem
// pathname to unlink; stale pipes disappear with the owning process.
func RemoveStaleEndpoint(_ Transport, _ string) {}
