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

func TestReportNormalization(t *testing.T) {
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

	// Create conditions for duplicate, .haro, and escape via diff+ls-files
	// Duplicate: create file, commit, then modify same file -> diff should dedupe? Actually duplicate can come from both diff and ls-files union if file is both tracked modified and untracked? But we can test dedupe via normalization directly calling helper?
	// Simpler: test through end-to-end report: create duplicate scenario by having same file appear in both diff and ls-files? For untracked files, diff not includes them, ls-files does. For modified tracked file, diff includes, ls-files not. So duplicate not trivial via real git, but we can test dedupe via direct normalizePaths call
	// Test .haro exclusion: create file inside .haro that is untracked (should be ignored by ls-files via .gitignore? Actually .haro is maybe ignored via .gitignore, but ls-files --others --exclude-standard will exclude ignored files, so .haro should not appear anyway. However we create .haro/artifacts/inner.txt tracked? Exclude via filter.

	// Create .haro file that is tracked modified to ensure filter removes it
	// First ensure .haro is inside repo root
	_ = os.MkdirAll(filepath.Join(dir, ".haro", "artifacts"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".haro", "artifacts", "should_exclude.txt"), []byte("inside haro"), 0o600)
	// Create normal file
	_ = os.WriteFile(filepath.Join(dir, "normal.txt"), []byte("normal"), 0o600)
	// Create another file that will be sorted check
	_ = os.WriteFile(filepath.Join(dir, "a_sort.txt"), []byte("a"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "z_sort.txt"), []byte("z"), 0o600)
	// Create symlink attempt? For symlink escape test, create symlink inside repo pointing outside?
	// Create outside temp file and symlink to it
	outside := filepath.Join(t.TempDir(), "outside.txt")
	_ = os.WriteFile(outside, []byte("outside"), 0o600)
	_ = os.Symlink(outside, filepath.Join(dir, "escape_link.txt"))

	execID := "norm-test"
	_ = s.Executions().Create(ctx, &store.Execution{ID: execID, ProjectID: dir, WorkflowSource: "/tmp/wf.yaml", Status: "completed", WorkspaceMode: "shared", WorkspaceRoot: dir, StartedAt: "2025-01-01T00:00:00Z", BaseCommit: &base})
	rpt, err := eng.Report(ctx, execID)
	if err != nil {
		t.Fatalf("norm report: %v", err)
	}
	// Check .haro excluded
	for _, f := range rpt.ChangedFiles {
		if f == ".haro/artifacts/should_exclude.txt" || f == ".haro" {
			t.Fatalf(".haro should be excluded, got %v", rpt.ChangedFiles)
		}
	}
	// Check sorted
	for i := 1; i < len(rpt.ChangedFiles); i++ {
		if rpt.ChangedFiles[i-1] > rpt.ChangedFiles[i] {
			t.Fatalf("not sorted: %v", rpt.ChangedFiles)
		}
	}
	// Check absolute rejected: we didn't create absolute path via git, so just check normalization helper directly
	paths := []string{"a.txt", "a.txt", "b.txt", ".haro/c.txt", "../escape.txt", "/abs.txt", "z_sort.txt"}
	normalized, _ := normalizePaths(dir, dir, "shared", paths)
	// Expect deduped, filtered, sorted, no escape/abs/.haro
	expected := map[string]bool{"a.txt": true, "b.txt": true, "z_sort.txt": true}
	if len(normalized) != len(expected) {
		t.Fatalf("normalize got %v want %v", normalized, expected)
	}
	for _, p := range normalized {
		if !expected[p] {
			t.Fatalf("unexpected normalized path %q in %v", p, normalized)
		}
	}
	// Cleanup
	_ = os.Remove(filepath.Join(dir, "normal.txt"))
	_ = os.Remove(filepath.Join(dir, "a_sort.txt"))
	_ = os.Remove(filepath.Join(dir, "z_sort.txt"))
	_ = os.Remove(filepath.Join(dir, "escape_link.txt"))
	_ = os.RemoveAll(filepath.Join(dir, ".haro", "artifacts", "should_exclude.txt"))
	_ = exec.Command("git", "-C", dir, "clean", "-fd").Run()
	_ = exec.Command("git", "-C", dir, "reset", "--hard", base).Run()
}
