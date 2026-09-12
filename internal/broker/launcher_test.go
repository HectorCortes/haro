package broker_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/HectorCortes/haro/internal/broker"
	"github.com/HectorCortes/haro/internal/cmd"
	"github.com/HectorCortes/haro/internal/ipc"
)

// TestHelperProcess is the exec-helper used by process-spawning tests. When
// HARO_TEST_BROKER_DAEMON points at a project root it blocks running the
// broker daemon for that root (exactly like `haro broker --project`).
func TestHelperProcess(t *testing.T) {
	root := os.Getenv("HARO_TEST_BROKER_DAEMON")
	if root == "" {
		return
	}
	code := cmd.Execute(context.Background(), []string{"broker", "--project", root}, root, os.Stdout, os.Stderr)
	os.Exit(code)
}

// processSpawn starts the daemon as a detached helper process (real F-02
// lifecycle: outlives the CLI that launched it).
func processSpawn(t *testing.T) (broker.Spawner, func() int) {
	var mu sync.Mutex
	spawns := 0
	fn := func(ctx context.Context, canon string) error {
		c := exec.Command(os.Args[0], "-test.run=TestHelperProcess$")
		c.Env = append(os.Environ(), "HARO_TEST_BROKER_DAEMON="+canon)
		if err := c.Start(); err != nil {
			return err
		}
		mu.Lock()
		spawns++
		mu.Unlock()
		go func() { _ = c.Wait() }() // reap; helper daemon outlives us
		return nil
	}
	return fn, func() int {
		mu.Lock()
		defer mu.Unlock()
		return spawns
	}
}

// speakHealth dials the endpoint and performs one health JSON-RPC round trip.
func speakHealth(t *testing.T, ep string) {
	t.Helper()
	tr := ipc.DefaultTransport()
	conn, err := tr.Dial(ep)
	if err != nil {
		t.Fatalf("dial %q: %v", ep, err)
	}
	defer func() { _ = conn.Close() }()
	req := `{"jsonrpc":"2.0","method":"health","id":1}` + "\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatalf("write health: %v", err)
	}
	br := bufio.NewReader(conn)
	line, err := br.ReadString('\n')
	if err != nil {
		t.Fatalf("read health response: %v", err)
	}
	var resp struct {
		JSONRPC string          `json:"jsonrpc"`
		Result  json.RawMessage `json:"result"`
		ID      json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		t.Fatalf("decode health response %q: %v", line, err)
	}
	var result struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil || !result.OK {
		t.Fatalf("health result not ok: %q (err=%v)", resp.Result, err)
	}
}

// TestEnsureLaunchesDaemonWhenAbsent covers F-02: absent broker -> launch +
// retry, and the daemon outlives the CLI that started it.
func TestEnsureLaunchesDaemonWhenAbsent(t *testing.T) {
	spawn, count := processSpawn(t)
	root := t.TempDir()
	l := broker.NewLauncher(ipc.DefaultTransport(), spawn)
	ep, err := l.Ensure(context.Background(), root)
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	// The launcher itself is the "CLI": it returned, the daemon must remain.
	deadline := time.Now().Add(5 * time.Second)
	for {
		err := speakHealthOnce(ep)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("daemon did not outlive CLI: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	// One launch exactly.
	if n := count(); n != 1 {
		t.Fatalf("expected exactly 1 spawn, got %d", n)
	}
	// Idempotent Ensure reuses the live broker without spawning again.
	ep2, err := l.Ensure(context.Background(), root)
	if err != nil || ep2 != ep {
		t.Fatalf("idempotent Ensure: %q %v", ep2, err)
	}
	if n := count(); n != 1 {
		t.Fatalf("idempotent Ensure spawned again: %d", n)
	}
}

func speakHealthOnce(ep string) error {
	tr := ipc.DefaultTransport()
	conn, err := tr.Dial(ep)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.Write([]byte(`{"jsonrpc":"2.0","method":"health","id":1}` + "\n")); err != nil {
		return err
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	br := bufio.NewReader(conn)
	line, err := br.ReadString('\n')
	if err != nil {
		return err
	}
	var resp struct {
		Result struct {
			OK bool `json:"ok"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		return err
	}
	if !resp.Result.OK {
		return fmt.Errorf("health not ok: %s", line)
	}
	return nil
}

// TestEnsureHerdExactlyOneDaemon covers F-05: N concurrent Ensure calls with
// no broker produce exactly one daemon (flock + double-check Dial).
func TestEnsureHerdExactlyOneDaemon(t *testing.T) {
	var mu sync.Mutex
	spawns := 0
	root := t.TempDir()
	l := broker.NewLauncher(ipc.DefaultTransport(), func(ctx context.Context, canon string) error {
		mu.Lock()
		spawns++
		mu.Unlock()
		// Detached daemon: background context so it outlives Ensure.
		go func() {
			_ = broker.RunDaemon(context.Background(), canon)
		}()
		return nil
	})
	const n = 8
	errs := make([]error, n)
	eps := make([]string, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ep, err := l.Ensure(context.Background(), root)
			eps[i], errs[i] = ep, err
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("Ensure[%d]: %v", i, err)
		}
	}
	for i := 1; i < n; i++ {
		if eps[i] != eps[0] {
			t.Fatalf("herd endpoints differ: %q vs %q", eps[0], eps[i])
		}
	}
	if n2 := muLockInt(&mu, &spawns); n2 != 1 {
		t.Fatalf("herd spawned %d daemons, want 1", n2)
	}
	speakHealth(t, eps[0])
}

func muLockInt(mu *sync.Mutex, v *int) int {
	mu.Lock()
	defer mu.Unlock()
	return *v
}

// TestEnsureRemovesZombieSocket covers zombie recovery: a socket path that
// exists but refuses connections is removed and launch proceeds.
func TestEnsureRemovesZombieSocket(t *testing.T) {
	root := t.TempDir()
	ep, err := ipc.SocketPath(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ep, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	l := broker.NewLauncher(ipc.DefaultTransport(), func(ctx context.Context, canon string) error {
		go func() {
			_ = broker.RunDaemon(context.Background(), canon)
		}()
		return nil
	})
	got, err := l.Ensure(context.Background(), root)
	if err != nil {
		t.Fatalf("Ensure with zombie socket: %v", err)
	}
	if got != ep {
		t.Fatalf("endpoint %q != %q", got, ep)
	}
	speakHealth(t, ep)
}

// TestEnsureLockHeldDialsOnly covers the flock EWOULDBLOCK path: when another
// launcher holds the lock, Ensure must NOT spawn; it re-dials for liveness
// until the racing launcher's daemon appears.
func TestEnsureLockHeldDialsOnly(t *testing.T) {
	root := t.TempDir()
	lockPath := filepath.Join(root, ".haro", "broker.lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		t.Fatal(err)
	}
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lock.Close() }()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatalf("test could not hold lock: %v", err)
	}

	var mu sync.Mutex
	spawns := 0
	l := broker.NewLauncher(ipc.DefaultTransport(), func(ctx context.Context, canon string) error {
		mu.Lock()
		spawns++
		mu.Unlock()
		return nil
	})

	// Racer: after we release, a daemon starts (it acquires its own lock).
	go func() {
		time.Sleep(150 * time.Millisecond)
		_ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
		_ = broker.RunDaemon(context.Background(), root)
	}()

	ep, err := l.Ensure(context.Background(), root)
	if err != nil {
		t.Fatalf("Ensure under held lock: %v", err)
	}
	speakHealth(t, ep)
	mu.Lock()
	n := spawns
	mu.Unlock()
	if n != 0 {
		t.Fatalf("Ensure spawned while flock held: %d", n)
	}
}
