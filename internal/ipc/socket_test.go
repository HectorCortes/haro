package ipc

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestSocketPathCanonicalEquivalents verifies the canonical table: symlink
// and trailing-slash equivalents of the same root share one endpoint.
func TestSocketPathCanonicalEquivalents(t *testing.T) {
	tmp := t.TempDir()
	real := filepath.Join(tmp, "proj")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(tmp, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	a, err := SocketPath(real)
	if err != nil {
		t.Fatalf("SocketPath(real): %v", err)
	}
	b, err := SocketPath(real + string(filepath.Separator))
	if err != nil {
		t.Fatalf("SocketPath(trailing slash): %v", err)
	}
	c, err := SocketPath(link)
	if err != nil {
		t.Fatalf("SocketPath(link): %v", err)
	}
	if a != b {
		t.Fatalf("trailing slash equivalent differs:\n  %q\n  %q", a, b)
	}
	if a != c {
		t.Fatalf("symlink equivalent differs:\n  %q\n  %q", a, c)
	}
	// Deterministic repeat.
	d, err := SocketPath(link)
	if err != nil {
		t.Fatalf("SocketPath repeat: %v", err)
	}
	if d != c {
		t.Fatalf("repeat call differs: %q vs %q", d, c)
	}
}

// TestSocketPathDistinctRoots verifies distinct roots derive distinct
// endpoints.
func TestSocketPathDistinctRoots(t *testing.T) {
	a, err := SocketPath(filepath.Join(t.TempDir(), "alpha"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := SocketPath(filepath.Join(t.TempDir(), "beta"))
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatalf("distinct roots share endpoint %q", a)
	}
}

// TestSocketPathUnixLength verifies the Linux UDS path is at most 108 bytes
// including the trailing NUL.
func TestSocketPathUnixLength(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix length bound applies to UDS paths")
	}
	p, err := SocketPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(p)+1 > 108 {
		t.Fatalf("socket path %q is %d bytes (+NUL=%d) > 108", p, len(p), len(p)+1)
	}
}

// TestSocketPathLongRootSafe verifies a very long canonical root still yields
// a bounded endpoint (hash is fixed width, independent of root length).
func TestSocketPathLongRootSafe(t *testing.T) {
	base := t.TempDir()
	deep := base
	name := strings.Repeat("d", 40)
	for i := 0; i < 6; i++ {
		deep = filepath.Join(deep, name)
	}
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	p, err := SocketPath(deep)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && len(p)+1 > 108 {
		t.Fatalf("long root produced unbounded endpoint %q (%d bytes)", p, len(p))
	}
	// Endpoint must remain hash-bounded, not embed the raw path.
	if strings.Contains(p, name) {
		t.Fatalf("endpoint %q embeds raw path component", p)
	}
}

// TestSocketPathWorktreeCoherence verifies the endpoint is keyed by the
// passed canonical project root, never by an execution worktree directory
// under .haro/worktrees/<id>.
func TestSocketPathWorktreeCoherence(t *testing.T) {
	root := t.TempDir()
	wt := filepath.Join(root, ".haro", "worktrees", "exec-1")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	a, err := SocketPath(root)
	if err != nil {
		t.Fatal(err)
	}
	b, err := SocketPath(wt)
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatalf("worktree path must not collapse to project endpoint: %q", a)
	}
	// The launcher contract keeps worktrees coherent: the passed root wins.
	// It is enforced by Ensure/daemon carrying --project <canonical root>.
}

// TestSocketPathWindowsShape verifies the Windows named-pipe shape (pure
// path-table unit; runs on every platform).
func TestSocketPathWindowsShape(t *testing.T) {
	p, err := SocketPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" && !strings.HasPrefix(p, `\\.\pipe\haro-`) {
		t.Fatalf("windows endpoint %q lacks pipe prefix", p)
	}
	if runtime.GOOS != "windows" && !strings.HasSuffix(p, ".sock") {
		t.Fatalf("unix endpoint %q lacks .sock suffix", p)
	}
}
