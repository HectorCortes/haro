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

func TestReportWorkspaceSelection(t *testing.T) {
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
	dbPath := filepath.Join(dir, ".haro", "store.db")
	s, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = s.Close() }()
	_ = s.Projects().Create(ctx, dir, dir)
	eng := NewEngine(s, &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
		return 0, "", "", nil
	}}, dir)

	// Create file with newline in name to test NUL parsing (git can handle via -z)
	// This is tricky: create file with newline using os.WriteFile with path containing \n
	newlineName := "file\nwith\nnewline.txt"
	_ = os.WriteFile(filepath.Join(dir, newlineName), []byte("content"), 0o600)
	// Need to add to git diff: create new file, diff vs base should include it
	// For shared mode, diff base vs working tree includes new file as untracked or added? Actually added files are tracked as untracked via ls-files, or diff shows added?
	// We test ls-files NUL parsing: file with newline is untracked, ls-files -z should emit it correctly
	// To make diff handle it, we also need to stage? But we will have it untracked
	execID := "workspace-select-newline"
	_ = s.Executions().Create(ctx, &store.Execution{ID: execID, ProjectID: dir, WorkflowSource: "/tmp/wf.yaml", Status: "completed", WorkspaceMode: "shared", WorkspaceRoot: dir, StartedAt: "2025-01-01T00:00:00Z", BaseCommit: &base})
	rpt, err := eng.Report(ctx, execID)
	if err != nil {
		t.Fatalf("report with newline file: %v", err)
	}
	// Ensure newline file appears exactly as one entry with newlines preserved
	found := false
	for _, f := range rpt.ChangedFiles {
		if f == newlineName {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("NUL parsing failed for newline filename, got %q", rpt.ChangedFiles)
	}
	// Also test ChangedFiles delegates same result
	cf, err := eng.ChangedFiles(ctx, execID)
	if err != nil {
		t.Fatalf("ChangedFiles: %v", err)
	}
	if len(cf) != len(rpt.ChangedFiles) {
		t.Fatalf("ChangedFiles len %d != Report len %d", len(cf), len(rpt.ChangedFiles))
	}
	// Cleanup
	_ = os.Remove(filepath.Join(dir, newlineName))
	_ = exec.Command("git", "-C", dir, "clean", "-fd").Run()
	// Removed fallback: verify diff with HEAD includes committed file but not untracked
	// Create a committed file for fallback test already covered in state test, here just ensure fallback works with base..HEAD argv
	execRemoved := &store.Execution{ID: "ws-removed", ProjectID: dir, WorkspaceMode: "isolated", WorkspaceRoot: filepath.Join(dir, ".haro", "worktrees", "missing"), Status: "completed", StartedAt: "2025-01-01T00:00:00Z", BaseCommit: &base}
	_ = s.Executions().Create(ctx, execRemoved)
	_ = os.WriteFile(filepath.Join(dir, "fallback_committed.txt"), []byte("x"), 0o600)
	_ = exec.Command("git", "-C", dir, "add", "fallback_committed.txt").Run()
	_ = exec.Command("git", "-C", dir, "commit", "-m", "fallback").Run()
	rpt2, err := eng.Report(ctx, "ws-removed")
	if err != nil {
		t.Fatalf("fallback report: %v", err)
	}
	has := false
	for _, f := range rpt2.ChangedFiles {
		if f == "fallback_committed.txt" {
			has = true
			break
		}
	}
	if !has {
		t.Fatalf("fallback diff should contain fallback_committed.txt, got %v", rpt2.ChangedFiles)
	}
	// Ensure ls-files not included (untracked files not in fallback)
	_ = os.WriteFile(filepath.Join(dir, "untracked_fallback.txt"), []byte("y"), 0o600)
	rpt3, err := eng.Report(ctx, "ws-removed")
	if err != nil {
		t.Fatalf("fallback second: %v", err)
	}
	for _, f := range rpt3.ChangedFiles {
		if f == "untracked_fallback.txt" {
			t.Fatalf("fallback must not include untracked, got %v", rpt3.ChangedFiles)
		}
	}
	// Cleanup fallback
	_ = exec.Command("git", "-C", dir, "reset", "--hard", base).Run()
	_ = os.Remove(filepath.Join(dir, "fallback_committed.txt"))
	_ = os.Remove(filepath.Join(dir, "untracked_fallback.txt"))
	_ = exec.Command("git", "-C", dir, "clean", "-fd").Run()
}
