package execution

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/HectorCortes/haro/internal/project"
	"github.com/HectorCortes/haro/internal/store"
	"github.com/HectorCortes/haro/internal/worktree"
)

func TestReportComposedE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("git required")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	ctx := context.Background()
	repo := t.TempDir()
	base := initGitRepo(t, repo)
	// Use composed fixture: create main with workflow referencing lib
	_ = project.Init(repo)
	// Create lib workflow
	libDir := filepath.Join(repo, ".haro", "workflows", "lib")
	_ = os.MkdirAll(libDir, 0o755)
	_ = os.WriteFile(filepath.Join(libDir, "workflow.yaml"), []byte("version: 2\nname: lib\nsteps:\n  - id: inner\n    type: command\n    run: echo inner\n    produces: [lib_out.txt]\n"), 0o600)
	mainDir := filepath.Join(repo, ".haro", "workflows", "main")
	_ = os.MkdirAll(mainDir, 0o755)
	_ = os.WriteFile(filepath.Join(mainDir, "workflow.yaml"), []byte("version: 2\nname: main\nsteps:\n  - id: wf\n    type: workflow\n    source: ../lib/workflow.yaml\n  - id: direct\n    type: command\n    run: echo direct\n    produces: [direct_out.txt]\n"), 0o600)

	dbPath := filepath.Join(repo, ".haro", "store.db")
	s, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = s.Close() }()
	eng := NewEngine(s, &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
		return 0, "", "", nil
	}}, repo)
	// Use real worktree for report accuracy
	eng.SetWorktreeManager(worktree.NewManager())
	execID, err := eng.CreateExecution(ctx, "main")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Verify flattened: single execution with namespaced steps
	steps, _ := s.Steps().List(ctx, execID)
	if len(steps) != 2 {
		t.Fatalf("composed should have 2 steps wf.inner + direct, got %d %v", len(steps), steps)
	}
	hasInner := false
	hasDirect := false
	for _, st := range steps {
		if st.StepID == "wf.inner" {
			hasInner = true
		}
		if st.StepID == "direct" {
			hasDirect = true
		}
	}
	if !hasInner || !hasDirect {
		t.Fatalf("missing expected steps inner=%v direct=%v", hasInner, hasDirect)
	}
	// Simulate execution completion: mark steps completed and execution completed via store
	for _, st := range steps {
		_ = s.Steps().UpdateStatus(ctx, execID, st.StepID, "completed")
	}
	_ = s.Executions().UpdateStatus(ctx, execID, "completed")
	// Create a file change in repo (shared? but main is default isolated, so workspace is worktree)
	// For isolated, report should use workspace; let's create file in worktree
	execRec, _ := s.Executions().Get(ctx, execID)
	ws := execRec.WorkspaceRoot
	// workspace exists (isolated), create untracked file inside it
	_ = os.WriteFile(filepath.Join(ws, "composed_untracked.txt"), []byte("untracked"), 0o600)
	// Also modify tracked file in workspace
	_ = os.WriteFile(filepath.Join(ws, "base.txt"), []byte("modified base"), 0o600)
	rpt, err := eng.Report(ctx, execID)
	if err != nil {
		t.Fatalf("composed report: %v", err)
	}
	// Must be one report, flattened composition retains one execution_id
	if rpt.ExecutionID != execID {
		t.Fatalf("report execution_id %q != %q", rpt.ExecutionID, execID)
	}
	// No per-step report exists (we don't have per-step report API, so trivially satisfied)
	// Verify combined report covers complete execution (includes change from workspace)
	foundUntracked := false
	foundModified := false
	for _, f := range rpt.ChangedFiles {
		if f == "composed_untracked.txt" {
			foundUntracked = true
		}
		if f == "base.txt" {
			foundModified = true
		}
	}
	if !foundUntracked {
		t.Fatalf("composed report missing untracked composed_untracked.txt, got %v", rpt.ChangedFiles)
	}
	if !foundModified {
		t.Fatalf("composed report missing modified base.txt")
	}
	// Untracked only on existing workspace: now simulate removal fallback and ensure untracked not included
	_ = exec.Command("git", "-C", repo, "worktree", "remove", "--force", ws).Run()
	_ = os.RemoveAll(ws)
	// Ensure workspace considered removed
	rpt2, err := eng.Report(ctx, execID)
	if err != nil {
		t.Fatalf("fallback report: %v", err)
	}
	for _, f := range rpt2.ChangedFiles {
		if f == "composed_untracked.txt" {
			t.Fatalf("fallback should not include untracked composed_untracked.txt (lossy), got %v", rpt2.ChangedFiles)
		}
	}
	// Cleanup
	_ = exec.Command("git", "-C", repo, "worktree", "prune").Run()
	_ = exec.Command("git", "-C", repo, "reset", "--hard", base).Run()
	_ = exec.Command("git", "-C", repo, "clean", "-fd").Run()
	_ = exec.Command("git", "-C", repo, "worktree", "prune").Run()
}
