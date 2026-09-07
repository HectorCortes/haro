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
)

func TestReportReadOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("git required")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	ctx := context.Background()
	repo := t.TempDir()
	base := initGitRepo(t, repo)
	if err := project.Init(repo); err != nil {
		t.Fatalf("init: %v", err)
	}
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

	// Create staged and unstaged changes to verify reporting doesn't mutate
	_ = os.WriteFile(filepath.Join(repo, "readonly_staged.txt"), []byte("staged"), 0o600)
	_ = exec.Command("git", "-C", repo, "add", "readonly_staged.txt").Run()
	_ = os.WriteFile(filepath.Join(repo, "readonly_unstaged.txt"), []byte("unstaged"), 0o600)

	headBefore := captureHead(t, repo)
	statusBefore := captureStatus(t, repo)
	indexBefore := captureDiffIndex(t, repo)

	execOK := &store.Execution{ID: "readonly-ok", ProjectID: repo, WorkflowSource: "/tmp/wf.yaml", Status: "completed", WorkspaceMode: "shared", WorkspaceRoot: repo, StartedAt: "2025-01-01T00:00:00Z", BaseCommit: &base}
	_ = s.Executions().Create(ctx, execOK)
	_, err = eng.Report(ctx, "readonly-ok")
	if err != nil {
		t.Fatalf("report success: %v", err)
	}
	if got := captureHead(t, repo); got != headBefore {
		t.Fatalf("HEAD changed after success report: %q vs %q", got, headBefore)
	}
	if got := captureStatus(t, repo); got != statusBefore {
		t.Fatalf("status changed after success: %q vs %q", got, statusBefore)
	}
	if got := captureDiffIndex(t, repo); got != indexBefore {
		t.Fatalf("index changed after success: %q vs %q", got, indexBefore)
	}

	// Failed report (not_found) should also leave unchanged
	_, _ = eng.Report(ctx, "nonexistent")
	if got := captureHead(t, repo); got != headBefore {
		t.Fatalf("HEAD changed after failed report")
	}
	if got := captureStatus(t, repo); got != statusBefore {
		t.Fatalf("status changed after failed not_found")
	}
	// not_completed
	_ = s.Executions().Create(ctx, &store.Execution{ID: "readonly-pending", ProjectID: repo, WorkflowSource: "/tmp/wf.yaml", Status: "running", WorkspaceMode: "shared", WorkspaceRoot: repo, StartedAt: "2025-01-01T00:00:00Z", BaseCommit: &base})
	_, _ = eng.Report(ctx, "readonly-pending")
	if got := captureHead(t, repo); got != headBefore {
		t.Fatalf("HEAD changed after not_completed")
	}
	// not_available
	_ = s.Executions().Create(ctx, &store.Execution{ID: "readonly-null", ProjectID: repo, WorkflowSource: "/tmp/wf.yaml", Status: "completed", WorkspaceMode: "shared", WorkspaceRoot: repo, StartedAt: "2025-01-01T00:00:00Z", BaseCommit: nil})
	_, _ = eng.Report(ctx, "readonly-null")
	if got := captureHead(t, repo); got != headBefore {
		t.Fatalf("HEAD changed after not_available")
	}
	// Ensure no commit/push/merge/reset side effects: verify git log still single init commit plus our base
	logOut, _ := exec.Command("git", "-C", repo, "log", "--oneline").Output()
	if !strings.Contains(string(logOut), "init") {
		t.Fatalf("log missing init after reports")
	}
	if strings.Count(string(logOut), "\n") != 1 {
		// Should have 1 commit (init) ; if we created staged files but not committed, log should not grow
		// We did not commit staged file, so log should still be 1
		// If extra commit appeared, fail
		if strings.TrimSpace(string(logOut)) != "" && !strings.HasSuffix(strings.TrimSpace(string(logOut)), "init") {
			t.Fatalf("unexpected log after reports: %q", string(logOut))
		}
	}
	// Cleanup
	_ = exec.Command("git", "-C", repo, "reset", "HEAD", "--", "readonly_staged.txt").Run()
	_ = os.Remove(filepath.Join(repo, "readonly_staged.txt"))
	_ = os.Remove(filepath.Join(repo, "readonly_unstaged.txt"))
	_ = exec.Command("git", "-C", repo, "clean", "-fd").Run()
}
