package execution

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HectorCortes/haro/internal/project"
	"github.com/HectorCortes/haro/internal/store"
	"github.com/HectorCortes/haro/internal/worktree"
)

func initGitRepo(t *testing.T, dir string) string {
	t.Helper()
	if err := exec.Command("git", "init", dir).Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	_ = exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	_ = exec.Command("git", "-C", dir, "config", "user.name", "test").Run()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("base"), 0o600); err != nil {
		t.Fatalf("write base: %v", err)
	}
	if err := exec.Command("git", "-C", dir, "add", ".").Run(); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if err := exec.Command("git", "-C", dir, "commit", "-m", "init").Run(); err != nil {
		t.Fatalf("git commit: %v", err)
	}
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func TestCreateExecutionCapturesHEAD(t *testing.T) {
	if testing.Short() {
		t.Skip("git required")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	ctx := context.Background()
	dir := t.TempDir()
	base := initGitRepo(t, dir)
	// dirty changes: modify tracked file, add untracked
	_ = os.WriteFile(filepath.Join(dir, "README.md"), []byte("dirty modify"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("untracked content"), 0o600)

	if err := project.Init(dir); err != nil {
		t.Fatalf("project init: %v", err)
	}
	wfDir := filepath.Join(dir, ".haro", "workflows", "demo")
	_ = os.MkdirAll(wfDir, 0o755)
	_ = os.WriteFile(filepath.Join(wfDir, "workflow.yaml"), []byte("version: 2\nname: demo\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n"), 0o600)

	dbPath := filepath.Join(dir, ".haro", "store.db")
	s, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = s.Close() }()

	fake := &worktree.FakeManager{}
	eng := NewEngine(s, &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
		return 0, "", "", nil
	}}, dir)
	eng.SetWorktreeManager(fake)

	id, err := eng.CreateExecution(ctx, "demo")
	if err != nil {
		t.Fatalf("CreateExecution: %v", err)
	}
	got, err := s.Executions().Get(ctx, id)
	if err != nil {
		t.Fatalf("get exec: %v", err)
	}
	if got.BaseCommit == nil {
		t.Fatalf("BaseCommit nil, want %q", base)
	}
	if *got.BaseCommit != base {
		t.Fatalf("BaseCommit = %q want %q (dirty must be ignored)", *got.BaseCommit, base)
	}
	// Ensure worktree was attempted after capture (FakeManager called)
	found := false
	for _, c := range fake.Calls {
		if c.Op == "create" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected worktree create to be called after capture")
	}
}

func TestCreateExecutionCaptureFailsNoHEAD(t *testing.T) {
	if testing.Short() {
		t.Skip("git required")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	ctx := context.Background()
	dir := t.TempDir()
	// git init without commit -> HEAD does not exist, rev-parse fails
	if err := exec.Command("git", "init", dir).Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	_ = exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	_ = exec.Command("git", "-C", dir, "config", "user.name", "test").Run()
	if err := project.Init(dir); err != nil {
		t.Fatalf("project init: %v", err)
	}
	wfDir := filepath.Join(dir, ".haro", "workflows", "demo")
	_ = os.MkdirAll(wfDir, 0o755)
	_ = os.WriteFile(filepath.Join(wfDir, "workflow.yaml"), []byte("version: 2\nname: demo\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n"), 0o600)
	dbPath := filepath.Join(dir, ".haro", "store.db")
	s, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = s.Close() }()
	fake := &worktree.FakeManager{}
	eng := NewEngine(s, &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
		return 0, "", "", nil
	}}, dir)
	eng.SetWorktreeManager(fake)
	id, err := eng.CreateExecution(ctx, "demo")
	if err == nil {
		t.Fatalf("expected failure when HEAD missing, got id %q", id)
	}
	// Ensure no execution row created
	if id != "" {
		if _, gErr := s.Executions().Get(ctx, id); gErr == nil {
			t.Fatalf("execution should not exist after capture failure")
		}
	}
	// Ensure no worktree was created
	for _, c := range fake.Calls {
		if c.Op == "create" {
			t.Fatalf("worktree should not be created when capture fails")
		}
	}
	// Also verify that no rows at all exist for project
}

func TestCreateExecutionAnchorsSymlinkRoot(t *testing.T) {
	if testing.Short() {
		t.Skip("git required")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	ctx := context.Background()
	dir := t.TempDir()
	base := initGitRepo(t, dir)
	if err := project.Init(dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	wfDir := filepath.Join(dir, ".haro", "workflows", "demo")
	_ = os.MkdirAll(wfDir, 0o755)
	_ = os.WriteFile(filepath.Join(wfDir, "workflow.yaml"), []byte("version: 2\nname: demo\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n"), 0o600)
	dbPath := filepath.Join(dir, ".haro", "store.db")
	s, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = s.Close() }()
	sym := filepath.Join(t.TempDir(), "symroot")
	if err := os.Symlink(dir, sym); err != nil {
		t.Skip("symlink not supported")
	}
	fake := &worktree.FakeManager{}
	eng := NewEngine(s, &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
		return 0, "", "", nil
	}}, sym)
	eng.SetWorktreeManager(fake)
	id, err := eng.CreateExecution(ctx, "demo")
	if err != nil {
		t.Fatalf("Create via symlink root: %v", err)
	}
	got, err := s.Executions().Get(ctx, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.BaseCommit == nil || *got.BaseCommit != base {
		t.Fatalf("symlink BaseCommit = %v want %q", got.BaseCommit, base)
	}
}
