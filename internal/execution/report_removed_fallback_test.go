package execution

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/HectorCortes/haro/internal/project"
	"github.com/HectorCortes/haro/internal/store"
)

func TestReportRemovedFallback(t *testing.T) {
	if testing.Short() {
		t.Skip("git required")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	ctx := context.Background()
	repo := t.TempDir()
	base := initGitRepo(t, repo)
	_ = project.Init(repo)
	dbPath := filepath.Join(repo, ".haro", "store.db")
	s, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = s.Close() }()
	_ = s.Projects().Create(ctx, repo, repo)
	eng := NewEngine(s, &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
		return 0, "", "", nil
	}}, repo)
	// Create isolated execution with existing workspace
	ws := filepath.Join(repo, ".haro", "worktrees", "fallback-test")
	_ = os.MkdirAll(ws, 0o755)
	// Create a real worktree via execution's manager? Use NewEngine with real manager
	engReal := NewEngine(s, &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
		return 0, "", "", nil
	}}, repo)
	// Use worktree manager directly
	// Create worktree via git
	_ = exec.Command("git", "-C", repo, "worktree", "add", "--detach", ws, "HEAD").Run()
	defer func() { _ = exec.Command("git", "-C", repo, "worktree", "remove", "--force", ws).Run(); _ = exec.Command("git", "-C", repo, "worktree", "prune").Run() }()

	execID := "removed-fallback"
	_ = s.Executions().Create(ctx, &store.Execution{ID: execID, ProjectID: repo, WorkflowSource: "/tmp/wf.yaml", Status: "completed", WorkspaceMode: "isolated", WorkspaceRoot: ws, StartedAt: "2025-01-01T00:00:00Z", BaseCommit: &base})
	// Create a committed change after base (to be visible in fallback base..HEAD) and an untracked file in workspace
	// First, create workspace untracked
	_ = os.WriteFile(filepath.Join(ws, "workspace_untracked.txt"), []byte("wstrack"), 0o600)
	// Then commit a file in repo root (visible via fallback)
	_ = os.WriteFile(filepath.Join(repo, "committed_fallback.txt"), []byte("committed"), 0o600)
	_ = exec.Command("git", "-C", repo, "add", "committed_fallback.txt").Run()
	_ = exec.Command("git", "-C", repo, "commit", "-m", "fallback commit").Run()
	// Verify existing workspace report includes untracked
	rptExisting, err := engReal.Report(ctx, execID)
	if err != nil {
		t.Fatalf("existing report: %v", err)
	}
	hasUntrackedExisting := false
	for _, f := range rptExisting.ChangedFiles {
		if f == "workspace_untracked.txt" {
			hasUntrackedExisting = true
			break
		}
	}
	if !hasUntrackedExisting {
		t.Fatalf("existing should include workspace_untracked.txt, got %v", rptExisting.ChangedFiles)
	}
	// Now remove workspace -> fallback
	_ = exec.Command("git", "-C", repo, "worktree", "remove", "--force", ws).Run()
	_ = os.RemoveAll(ws)
	rptFallback, err := eng.Report(ctx, execID) // eng uses repo root fallback when ws missing
	if err != nil {
		t.Fatalf("fallback report: %v", err)
	}
	hasCommitted := false
	hasUntrackedFallback := false
	for _, f := range rptFallback.ChangedFiles {
		if f == "committed_fallback.txt" {
			hasCommitted = true
		}
		if f == "workspace_untracked.txt" {
			hasUntrackedFallback = true
		}
	}
	if !hasCommitted {
		t.Fatalf("fallback should contain committed_fallback.txt via base..HEAD, got %v", rptFallback.ChangedFiles)
	}
	if hasUntrackedFallback {
		t.Fatalf("fallback must NOT include untracked workspace_untracked.txt (lossy), got %v", rptFallback.ChangedFiles)
	}
	// Also create untracked in repo root that should NOT appear in fallback (since ls-files omitted)
	_ = os.WriteFile(filepath.Join(repo, "repo_untracked_fallback.txt"), []byte("x"), 0o600)
	rptFallback2, _ := eng.Report(ctx, execID)
	for _, f := range rptFallback2.ChangedFiles {
		if f == "repo_untracked_fallback.txt" {
			t.Fatalf("fallback should not include repo untracked, got %v", rptFallback2.ChangedFiles)
		}
	}
	// Cleanup
	_ = exec.Command("git", "-C", repo, "reset", "--hard", base).Run()
	_ = exec.Command("git", "-C", repo, "clean", "-fd").Run()
	_ = os.Remove(filepath.Join(repo, "committed_fallback.txt"))
	_ = exec.Command("git", "-C", repo, "worktree", "prune").Run()
}

// Helpers for test to avoid import cycle
type RealManagerShim struct{}

func NewWorktreeManagerForTest() *RealManagerShim { return &RealManagerShim{} }
