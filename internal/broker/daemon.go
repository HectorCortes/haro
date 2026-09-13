package broker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/HectorCortes/haro/internal/execution"
	"github.com/HectorCortes/haro/internal/ipc"
	"github.com/HectorCortes/haro/internal/ipc/jsonrpc"
	"github.com/HectorCortes/haro/internal/store"
	"github.com/google/uuid"
)

// Daemon lifecycle states.
const (
	StateStarting = "starting"
	StateRunning  = "running"
	StateDraining = "draining"
	StateStopped  = "stopped"
)

// flockAcquireWindow bounds how long a daemon waits for the flock before
// concluding another daemon serves the project.
const flockAcquireWindow = 3 * time.Second

// Daemon is the per-project broker process: it holds the project flock, one
// WAL store and one engine, and accepts concurrent connections.
type Daemon struct {
	root      string // canonical project root
	endpoint  string
	transport ipc.Transport
	listener  net.Listener
	st        store.Store
	engine    *execution.Engine
	ownerID   string
	lock      *os.File
	rpc       *Server

	state atomic.Value // string
	wg    sync.WaitGroup
}

// NewDaemon builds a daemon for the project root (canonicalized).
func NewDaemon(root string, tr ipc.Transport) (*Daemon, error) {
	if tr == nil {
		tr = ipc.DefaultTransport()
	}
	canon, err := ipc.CanonicalRoot(root)
	if err != nil {
		return nil, err
	}
	ep, err := tr.Endpoint(canon)
	if err != nil {
		return nil, err
	}
	dispatcher := NewDispatcher()
	dispatcher.Register("health", func(context.Context, json.RawMessage) (any, *jsonrpc.RPCError) {
		return map[string]bool{"ok": true}, nil
	})
	return &Daemon{root: canon, endpoint: ep, transport: tr, ownerID: uuid.NewString(), rpc: NewServer(dispatcher)}, nil
}

// State reports the current lifecycle state.
func (d *Daemon) State() string {
	if v, ok := d.state.Load().(string); ok {
		return v
	}
	return StateStarting
}

// Endpoint reports the bound endpoint.
func (d *Daemon) Endpoint() string { return d.endpoint }

// RunDaemon is the blocking entry used by `haro broker --project <root>`.
func RunDaemon(ctx context.Context, root string) error {
	d, err := NewDaemon(root, ipc.DefaultTransport())
	if err != nil {
		return err
	}
	return d.Run(ctx)
}

// Run binds the endpoint, acquires the project flock, opens the WAL store
// and engine, and serves concurrent connections until ctx is done. The
// listener binds first (binding is exclusive, so at most one daemon serves
// the endpoint); the flock serializes launch bookkeeping and marks daemon
// ownership for shutdown.
func (d *Daemon) Run(ctx context.Context) error {
	d.state.Store(StateStarting)
	// Zombie socket recovery: remove a refused endpoint before binding.
	ipc.RemoveStaleEndpoint(d.transport, d.endpoint)
	listener, err := d.transport.Listen(d.endpoint)
	if err != nil {
		// Another daemon may have bound the endpoint: liveness check and
		// exit quietly if a broker is responsive.
		if livenessProbe(d.transport, d.endpoint) {
			return nil
		}
		fmt.Fprintf(os.Stderr, "debug daemon listen %s: %v\n", d.root, err)
		return fmt.Errorf("listen %q: %w", d.endpoint, err)
	}
	d.listener = listener
	d.lock, err = acquireBrokerLockWait(d.root, flockAcquireWindow)
	if err != nil {
		// Another launcher/daemon owns the lock; our bind failed the
		// exclusivity race only if a broker is live. Otherwise serve.
		if !errors.Is(err, ErrLockHeld) {
			_ = listener.Close()
			return err
		}
		d.lock = nil
	}
	defer d.releaseLock()

	if err := d.openStore(); err != nil {
		_ = listener.Close()
		_ = os.Remove(d.endpoint)
		return err
	}
	d.registerExecutionHandlers()
	d.registerStepHandlers()
	d.state.Store(StateRunning)

	stop := make(chan struct{})
	go func() {
		_ = d.acceptLoop(ctx)
		close(stop)
	}()

	select {
	case <-ctx.Done():
	case <-stop:
	}
	// TODO(U7): full draining/leases/shutdown orchestration lands with the
	// Runtime; the basic sequence (listener, sessions, socket, store, lock)
	// is already ordered here.
	d.shutdown()
	return nil
}

func (d *Daemon) releaseLock() {
	if d.lock != nil {
		_ = releaseBrokerLock(d.lock)
		d.lock = nil
	}
}

// acquireBrokerLockWait retries a non-blocking flock acquisition for up to
// window; ErrLockHeld when the lock stays held.
func acquireBrokerLockWait(root string, window time.Duration) (*os.File, error) {
	deadline := time.Now().Add(window)
	for {
		f, err := acquireBrokerLock(root)
		if err == nil {
			return f, nil
		}
		if err != ErrLockHeld {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, ErrLockHeld
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func livenessProbe(tr ipc.Transport, ep string) bool {
	conn, err := tr.DialTimeout(ep, 250*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// openStore opens the single WAL store and builds the engine.
func (d *Daemon) openStore() error {
	if err := os.MkdirAll(filepath.Join(d.root, ".haro"), 0o755); err != nil {
		return fmt.Errorf("create .haro: %w", err)
	}
	dbPath := filepath.Join(d.root, ".haro", "store.db")
	s, err := store.Open(context.Background(), dbPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	d.st = s
	eng := execution.NewEngine(s, execution.NewRunner(), d.root)
	d.engine = eng
	return nil
}

// acceptLoop accepts connections concurrently.
func (d *Daemon) acceptLoop(ctx context.Context) error {
	for {
		conn, err := d.listener.Accept()
		if err != nil {
			if ctx.Err() != nil || d.State() == StateStopped {
				return nil
			}
			return err
		}
		d.wg.Add(1)
		go func() {
			defer d.wg.Done()
			d.serveConn(ctx, conn)
		}()
	}
}

// serveConn is the U1 minimal loop; U2 replaces it with the full JSON-RPC
// server (Dispatcher, strict validation, notifications).
func (d *Daemon) serveConn(ctx context.Context, c net.Conn) {
	if d.rpc != nil {
		_ = d.rpc.ServeConn(ctx, c)
	}
}

// shutdown closes the listener, waits for connections, removes the socket,
// and closes the store (flock released by Run's defer).
func (d *Daemon) shutdown() {
	d.state.Store(StateDraining)
	if d.listener != nil {
		_ = d.listener.Close()
	}
	d.wg.Wait()
	d.state.Store(StateStopped)
	if d.st != nil {
		_ = d.st.Close()
		d.st = nil
	}
	// No zombie socket: remove the endpoint we own (safe metadata only).
	if isStaleOwnSocket(d.endpoint) {
		_ = os.Remove(d.endpoint)
	}
}

// isStaleOwnSocket checks the endpoint still exists with 0600 metadata
// before unlinking it during shutdown.
func isStaleOwnSocket(ep string) bool {
	info, err := os.Stat(ep)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSocket != 0 && info.Mode().Perm() == 0o600
}
