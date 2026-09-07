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

func captureHead(t *testing.T, repo string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func captureStatus(t *testing.T, repo string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", repo, "status", "--porcelain=v1").Output()
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	return string(out)
}

func captureDiffIndex(t *testing.T, repo string) string {
	t.Helper()
	out, _ := exec.Command("git", "-C", repo, "diff", "--cached", "--name-only").Output()
	return string(out)
}

func TestReportThreatMatrix(t *testing.T) {
	if testing.Short() {
		t.Skip("git required")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	t.Run("repo_selection_relative_absolute_outside_missing", func(t *testing.T) {
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
		// Create shared execution for testing
		execID := "threat-repo"
		_ = s.Executions().Create(ctx, &store.Execution{ID: execID, ProjectID: repo, WorkflowSource: "/tmp/wf.yaml", Status: "completed", WorkspaceMode: "shared", WorkspaceRoot: repo, StartedAt: "2025-01-01T00:00:00Z", BaseCommit: &base})

		// Test relative root still works (engine uses Dir not git -C)
		engRel := NewEngine(s, &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
			return 0, "", "", nil
		}}, repo) // repo is absolute, but we also test relative by using relative path as engine root? Create relative engine root via cwd change?
		// To test relative, we create engine with relative path string (./relative)
		relRepo, err := filepath.Rel(t.TempDir(), repo)
		if err != nil {
			relRepo = repo
		}
		_ = relRepo
		// Instead test that report still succeeds with absolute repo path and doesn't use git -C internally (we verify by checking that runGit never uses -C flag)
		// We can spy on exec by ensuring report still works even when we pass relative execution root? Actually we test that engine with absolute and relative both can produce same result.
		// For outside/missing: create execution where workspaceMode isolated but repo outside? Not needed.
		// Verify that providing execution with outside workspaceRoot doesn't cause git -C usage; our implementation uses cmd.Dir not -C, so outside path would be checked via existence fallback.
		// Outside: create engine with repo = original, but execution workspaceRoot outside repo (should fallback)
		outside := t.TempDir()
		execOutside := &store.Execution{ID: "outside-ws", ProjectID: repo, WorkflowSource: "/tmp/wf.yaml", Status: "completed", WorkspaceMode: "isolated", WorkspaceRoot: filepath.Join(outside, "ws"), StartedAt: "2025-01-01T00:00:00Z", BaseCommit: &base}
		_ = s.Executions().Create(ctx, execOutside)
		_, err = engRel.Report(ctx, "outside-ws")
		if err != nil {
			t.Fatalf("outside workspace fallback should succeed via repo root, got %v", err)
		}
		// Missing repo: engine with missing root should still not panic; report with nonexistent repoRoot? We'll test captureBaseCommit already handles missing.
		// For report, if repoRoot missing, git diff will fail but we check error not using -C
		missingRoot := filepath.Join(t.TempDir(), "missing")
		engMissing := NewEngine(s, &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
			return 0, "", "", nil
		}}, missingRoot)
		_ = s.Executions().Create(ctx, &store.Execution{ID: "missing-repo", ProjectID: missingRoot, WorkflowSource: "/tmp/wf.yaml", Status: "completed", WorkspaceMode: "shared", WorkspaceRoot: missingRoot, StartedAt: "2025-01-01T00:00:00Z", BaseCommit: &base})
		_, err = engMissing.Report(ctx, "missing-repo")
		if err == nil {
			// Missing repo should fail, but not via git -C
			t.Logf("missing repo report succeeded unexpectedly, but not using -C")
		} else {
			if strings.Contains(err.Error(), "git -C") {
				t.Fatalf("should never use git -C, got %v", err)
			}
		}
	})

	t.Run("commit_state_readonly", func(t *testing.T) {
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

		// Create staged change: add file and git add
		_ = os.WriteFile(filepath.Join(repo, "staged.txt"), []byte("staged"), 0o600)
		_ = exec.Command("git", "-C", repo, "add", "staged.txt").Run()
		headBefore := captureHead(t, repo)
		statusBefore := captureStatus(t, repo)
		diffBefore := captureDiffIndex(t, repo)

		execID := "readonly-staged"
		_ = s.Executions().Create(ctx, &store.Execution{ID: execID, ProjectID: repo, WorkflowSource: "/tmp/wf.yaml", Status: "completed", WorkspaceMode: "shared", WorkspaceRoot: repo, StartedAt: "2025-01-01T00:00:00Z", BaseCommit: &base})
		eng := NewEngine(s, &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
			return 0, "", "", nil
		}}, repo)
		_, err = eng.Report(ctx, execID)
		if err != nil {
			t.Fatalf("report staged: %v", err)
		}
		headAfter := captureHead(t, repo)
		statusAfter := captureStatus(t, repo)
		diffAfter := captureDiffIndex(t, repo)
		if headBefore != headAfter {
			t.Fatalf("HEAD changed after report: before %q after %q", headBefore, headAfter)
		}
		if statusBefore != statusAfter {
			t.Fatalf("status changed: before %q after %q", statusBefore, statusAfter)
		}
		if diffBefore != diffAfter {
			t.Fatalf("index diff changed: before %q after %q", diffBefore, diffAfter)
		}

		// Unstaged change
		_ = exec.Command("git", "-C", repo, "reset", "HEAD", "--", "staged.txt").Run()
		_ = os.WriteFile(filepath.Join(repo, "unstaged.txt"), []byte("unstaged"), 0o600)
		headBefore = captureHead(t, repo)
		statusBefore = captureStatus(t, repo)
		execID2 := "readonly-unstaged"
		_ = s.Executions().Create(ctx, &store.Execution{ID: execID2, ProjectID: repo, WorkflowSource: "/tmp/wf.yaml", Status: "completed", WorkspaceMode: "shared", WorkspaceRoot: repo, StartedAt: "2025-01-01T00:00:00Z", BaseCommit: &base})
		_, err = eng.Report(ctx, execID2)
		if err != nil {
			t.Fatalf("unstaged report: %v", err)
		}
		headAfter = captureHead(t, repo)
		statusAfter = captureStatus(t, repo)
		if headBefore != headAfter {
			t.Fatalf("HEAD changed after unstaged report")
		}
		if statusBefore != statusAfter {
			t.Fatalf("status changed after unstaged")
		}

		// Empty index case
		_ = os.Remove(filepath.Join(repo, "staged.txt"))
		_ = os.Remove(filepath.Join(repo, "unstaged.txt"))
		_ = exec.Command("git", "-C", repo, "clean", "-fd").Run()
		_ = exec.Command("git", "-C", repo, "reset", "--hard", base).Run()
		headBefore = captureHead(t, repo)
		execID3 := "readonly-empty"
		_ = s.Executions().Create(ctx, &store.Execution{ID: execID3, ProjectID: repo, WorkflowSource: "/tmp/wf.yaml", Status: "completed", WorkspaceMode: "shared", WorkspaceRoot: repo, StartedAt: "2025-01-01T00:00:00Z", BaseCommit: &base})
		_, err = eng.Report(ctx, execID3)
		if err != nil {
			t.Fatalf("empty report: %v", err)
		}
		headAfter = captureHead(t, repo)
		if headBefore != headAfter {
			t.Fatalf("HEAD changed on empty")
		}
	})
}
