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

func TestReportStateAndWorkspace(t *testing.T) {
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
	// Need workflow for creation? For this test we insert executions directly via store to control state/mode.
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

	// unknown -> not_found
	_, err = eng.Report(ctx, "nonexistent-id")
	if err == nil {
		t.Fatalf("expected not_found for unknown")
	}
	if re, ok := err.(*ReportError); !ok || re.Code != "not_found" {
		t.Fatalf("unknown code = %v want not_found, err %v", re, err)
	}

	// pending -> not_completed
	execPending := &store.Execution{ID: "pending-id", ProjectID: dir, WorkflowSource: "/tmp/wf.yaml", Status: "pending", WorkspaceMode: "shared", WorkspaceRoot: dir, StartedAt: "2025-01-01T00:00:00Z", BaseCommit: &base}
	_ = s.Executions().Create(ctx, execPending)
	_, err = eng.Report(ctx, "pending-id")
	if err == nil {
		t.Fatalf("pending should be not_completed")
	}
	if re, ok := err.(*ReportError); !ok || re.Code != "not_completed" {
		t.Fatalf("pending code = %v", err)
	}

	// running -> not_completed
	execRunning := &store.Execution{ID: "running-id", ProjectID: dir, WorkflowSource: "/tmp/wf.yaml", Status: "running", WorkspaceMode: "shared", WorkspaceRoot: dir, StartedAt: "2025-01-01T00:00:00Z", BaseCommit: &base}
	_ = s.Executions().Create(ctx, execRunning)
	_, err = eng.Report(ctx, "running-id")
	if err == nil {
		t.Fatalf("running should be not_completed")
	}
	if re, ok := err.(*ReportError); !ok || re.Code != "not_completed" {
		t.Fatalf("running code %v", err)
	}

	// shared -> root: make a shared terminal execution and mutate a file, ensure report reflects diff at repo root
	execShared := &store.Execution{ID: "shared-terminal", ProjectID: dir, WorkflowSource: "/tmp/wf.yaml", Status: "completed", WorkspaceMode: "shared", WorkspaceRoot: dir, StartedAt: "2025-01-01T00:00:00Z", BaseCommit: &base}
	_ = s.Executions().Create(ctx, execShared)
	// mutate
	_ = os.WriteFile(filepath.Join(dir, "shared_new.txt"), []byte("shared"), 0o600)
	rpt, err := eng.Report(ctx, "shared-terminal")
	if err != nil {
		t.Fatalf("shared report: %v", err)
	}
	found := false
	for _, f := range rpt.ChangedFiles {
		if f == "shared_new.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("shared report should contain shared_new.txt, got %v", rpt.ChangedFiles)
	}

	// isolated existing -> workspace: create worktree fake existing
	execIsoExisting := &store.Execution{ID: "iso-existing", ProjectID: dir, WorkflowSource: "/tmp/wf.yaml", Status: "completed", WorkspaceMode: "isolated", WorkspaceRoot: filepath.Join(dir, ".haro", "worktrees", "iso-existing"), StartedAt: "2025-01-01T00:00:00Z", BaseCommit: &base}
	_ = s.Executions().Create(ctx, execIsoExisting)
	// Create the workspace directory with .git indicator and a file
	ws := execIsoExisting.WorkspaceRoot
	_ = os.MkdirAll(ws, 0o755)
	// Simulate worktree: copy git dir linkage? For test we actually use git worktree for realistic, but for isolated existing we use FakeManager existence check via os.Stat
	// To make diff work, we need real worktree with git diff capability. Use real manager to create worktree
	realMgr := worktree.NewManager()
	realWs, err := realMgr.Create(ctx, "iso-existing-real", dir)
	if err != nil {
		t.Fatalf("real worktree create: %v", err)
	}
	defer func() { _ = realMgr.Remove(ctx, realWs) }()
	// Update execution to point to realWs with existing workspace
	execIsoReal := &store.Execution{ID: "iso-real", ProjectID: dir, WorkflowSource: "/tmp/wf.yaml", Status: "completed", WorkspaceMode: "isolated", WorkspaceRoot: realWs, StartedAt: "2025-01-01T00:00:00Z", BaseCommit: &base}
	_ = s.Executions().Create(ctx, execIsoReal)
	// Create untracked file in worktree
	_ = os.WriteFile(filepath.Join(realWs, "iso_untracked.txt"), []byte("iso"), 0o600)
	// Also modify tracked file in worktree
	if data, err := os.ReadFile(filepath.Join(realWs, "README.md")); err == nil {
		_ = os.WriteFile(filepath.Join(realWs, "README.md"), append(data, []byte("modified")...), 0o600)
	}
	rpt2, err := eng.Report(ctx, "iso-real")
	if err != nil {
		t.Fatalf("iso existing report: %v", err)
	}
	// Should include both untracked and modified
	hasUntracked := false
	hasModified := false
	for _, f := range rpt2.ChangedFiles {
		if f == "iso_untracked.txt" {
			hasUntracked = true
		}
		if f == "README.md" {
			hasModified = true
		}
	}
	if !hasUntracked {
		t.Fatalf("isolated existing should include untracked iso_untracked.txt, got %v", rpt2.ChangedFiles)
	}
	if !hasModified {
		t.Fatalf("isolated existing should include modified README.md, got %v", rpt2.ChangedFiles)
	}

	// isolated removed fallback -> base..HEAD no ls-files
	execIsoRemoved := &store.Execution{ID: "iso-removed", ProjectID: dir, WorkflowSource: "/tmp/wf.yaml", Status: "completed", WorkspaceMode: "isolated", WorkspaceRoot: filepath.Join(dir, ".haro", "worktrees", "nonexistent"), StartedAt: "2025-01-01T00:00:00Z", BaseCommit: &base}
	_ = s.Executions().Create(ctx, execIsoRemoved)
	// Create a commit after base to simulate committed change that fallback should detect
	_ = os.WriteFile(filepath.Join(dir, "committed_after.txt"), []byte("committed"), 0o600)
	_ = exec.Command("git", "-C", dir, "add", "committed_after.txt").Run()
	_ = exec.Command("git", "-C", dir, "commit", "-m", "after").Run()
	// Create untracked file in repo root that should NOT appear in removed fallback (lossy)
	_ = os.WriteFile(filepath.Join(dir, "untracked_removed.txt"), []byte("should not appear"), 0o600)
	rpt3, err := eng.Report(ctx, "iso-removed")
	if err != nil {
		t.Fatalf("iso removed fallback report: %v", err)
	}
	hasCommitted := false
	hasUntrackedRemoved := false
	for _, f := range rpt3.ChangedFiles {
		if f == "committed_after.txt" {
			hasCommitted = true
		}
		if f == "untracked_removed.txt" {
			hasUntrackedRemoved = true
		}
		if strings.HasPrefix(f, ".haro/") {
			t.Fatalf("should not contain .haro files, got %q", f)
		}
	}
	if !hasCommitted {
		t.Fatalf("removed fallback should contain committed_after.txt, got %v", rpt3.ChangedFiles)
	}
	if hasUntrackedRemoved {
		t.Fatalf("removed fallback must NOT include untracked (lossy), got %v", rpt3.ChangedFiles)
	}
	// Cleanup
	_ = exec.Command("git", "-C", dir, "reset", "--hard", base).Run()
	_ = os.Remove(filepath.Join(dir, "shared_new.txt"))
	_ = os.Remove(filepath.Join(dir, "untracked_removed.txt"))
	_ = exec.Command("git", "-C", dir, "clean", "-fd").Run()
}
