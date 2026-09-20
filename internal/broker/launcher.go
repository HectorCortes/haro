// Package broker implements the per-project broker daemon and its lazy
// launcher: socket identity via internal/ipc, flock-guarded single-daemon
// startup, zombie-socket recovery, and the JSON-RPC runtime.
package broker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/HectorCortes/haro/internal/ipc"
)

// Spawner launches a broker daemon process for canonicalRoot. It must start
// the daemon detached (it must outlive the CLI) and return without waiting.
type Spawner func(ctx context.Context, canonicalRoot string) error

// Launcher ensures a responsive per-project broker exists (F-02, F-05).
type Launcher struct {
	tr    ipc.Transport
	spawn Spawner
	// dialBudget caps the liveness re-dial loop after launch.
	dialBudget time.Duration
}

// NewLauncher builds a launcher. A nil spawn uses the production detached
// launch of os.Executable() with `broker --project <canonical root>`.
func NewLauncher(tr ipc.Transport, spawn Spawner) *Launcher {
	return &Launcher{tr: tr, spawn: spawn, dialBudget: 5 * time.Second}
}

// brokerLockPath is the flock-guarded lockfile under the project root.
func brokerLockPath(canonicalRoot string) string {
	return filepath.Join(canonicalRoot, ".haro", "broker.lock")
}

func brokerStartingPath(canonicalRoot string) string {
	return filepath.Join(canonicalRoot, ".haro", "broker.starting")
}

// acquireBrokerLock opens (creating .haro when needed) and flock-locks the
// broker lockfile non-blockingly. ErrLockHeld when another launcher runs.
func acquireBrokerLock(canonicalRoot string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Join(canonicalRoot, ".haro"), 0o755); err != nil {
		return nil, fmt.Errorf("create .haro: %w", err)
	}
	f, err := os.OpenFile(brokerLockPath(canonicalRoot), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lockfile: %w", err)
	}
	if err := flockExclusive(f); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}

func releaseBrokerLock(f *os.File) error {
	if f == nil {
		return nil
	}
	_ = flockUnlock(f)
	return f.Close()
}

// Ensure idempotently returns the live endpoint for root: Dial -> flock ->
// double-check Dial -> remove only a refused socket -> detached launch ->
// backoff Dial. When the lockfile is held by another launcher (ErrLockHeld),
// Ensure only re-dials for liveness and never spawns.
func (l *Launcher) Ensure(ctx context.Context, root string) (string, error) {
	canon, err := ipc.CanonicalRoot(root)
	if err != nil {
		return "", err
	}
	ep, err := l.tr.Endpoint(canon)
	if err != nil {
		return "", err
	}
	if l.dial(ep, 500*time.Millisecond) {
		return ep, nil
	}
	lock, err := acquireBrokerLock(canon)
	if err != nil {
		// Another launcher holds the flock: liveness re-dial only.
		return ep, l.redial(ctx, ep)
	}
	defer func() { _ = releaseBrokerLock(lock) }()
	// Double-check under the lock: a broker may have come up meanwhile.
	if l.dial(ep, 500*time.Millisecond) {
		return ep, nil
	}
	// A previous launcher may have released the flock while its detached
	// daemon is still binding. The marker closes that small hand-off race: a
	// fresh marker means another launcher owns startup and this caller only
	// waits for liveness. Stale markers are safe to remove after a crash.
	if starting, err := claimBrokerStart(canon, l.dialBudget); err != nil {
		_ = releaseBrokerLock(lock)
		return ep, fmt.Errorf("claim broker startup: %w", err)
	} else if !starting {
		_ = releaseBrokerLock(lock)
		return ep, l.redial(ctx, ep)
	}
	markerOwned := true
	defer func() {
		if markerOwned {
			_ = os.Remove(brokerStartingPath(canon))
		}
	}()
	// Zombie socket: exists but refused -> remove (safe-metadata checked).
	ipc.RemoveStaleEndpoint(l.tr, ep)
	// Launch while still holding the lock: another Ensure that arrives in
	// this window fails its non-blocking lock attempt and re-dials only,
	// so exactly one launcher spawns the daemon.
	if err := l.launch(ctx, canon); err != nil {
		return ep, fmt.Errorf("launch broker: %w", err)
	}
	// Release before redialing so the daemon can acquire the lock.
	_ = releaseBrokerLock(lock)
	err = l.redial(ctx, ep)
	if err == nil {
		markerOwned = false
		_ = os.Remove(brokerStartingPath(canon))
	}
	return ep, err
}

// claimBrokerStart claims the single launcher hand-off marker. The returned
// boolean is true when this caller owns startup and false when another fresh
// launcher already owns it.
func claimBrokerStart(canonicalRoot string, staleAfter time.Duration) (bool, error) {
	path := brokerStartingPath(canonicalRoot)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err == nil {
		_, _ = f.WriteString(strings.TrimSpace(canonicalRoot) + "\n")
		_ = f.Close()
		return true, nil
	}
	if !os.IsExist(err) {
		return false, err
	}
	info, statErr := os.Stat(path)
	if statErr == nil && time.Since(info.ModTime()) <= staleAfter && info.Mode().Perm() == 0o600 {
		return false, nil
	}
	if statErr == nil {
		if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
			return false, removeErr
		}
	}
	f, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if os.IsExist(err) {
			return false, nil
		}
		return false, err
	}
	_, _ = f.WriteString(strings.TrimSpace(canonicalRoot) + "\n")
	_ = f.Close()
	return true, nil
}

func (l *Launcher) dial(ep string, d time.Duration) bool {
	conn, err := l.tr.DialTimeout(ep, d)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func (l *Launcher) redial(ctx context.Context, ep string) error {
	deadline := time.Now().Add(l.dialBudget)
	for {
		if l.dial(ep, 500*time.Millisecond) {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("broker endpoint %q did not become responsive", ep)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (l *Launcher) launch(ctx context.Context, canon string) error {
	if l.spawn != nil {
		// Detached daemons must outlive the CLI context.
		return l.spawn(context.WithoutCancel(ctx), canon)
	}
	return productionSpawn(canon)
}

// productionSpawn launches this binary in broker mode, detached.
func productionSpawn(canonicalRoot string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	base := strings.ToLower(filepath.Base(exe))
	if strings.HasSuffix(base, ".test") || strings.HasSuffix(base, ".test.exe") {
		return fmt.Errorf("refusing to spawn test binary %q as broker daemon", exe)
	}
	c := exec.Command(exe, "broker", "--project", canonicalRoot)
	detachAttrs(c)
	if err := c.Start(); err != nil {
		return fmt.Errorf("start broker process: %w", err)
	}
	go func() { _ = c.Wait() }() // reap; daemon outlives the CLI
	return nil
}
