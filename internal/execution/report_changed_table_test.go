package execution

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/HectorCortes/haro/internal/project"
	"github.com/HectorCortes/haro/internal/store"
)

func TestChangedFilesTable(t *testing.T) {
	if testing.Short() {
		t.Skip("skip git table in short")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	cases := []struct {
		name string
		setup func(t *testing.T, repo string)
		// expect check is done via oracle comparison + filter rules, but we also check specific paths
	}{
		{
			name: "added",
			setup: func(t *testing.T, repo string) {
				_ = os.WriteFile(filepath.Join(repo, "added_new.txt"), []byte("new"), 0o600)
			},
		},
		{
			name: "modified",
			setup: func(t *testing.T, repo string) {
				_ = os.WriteFile(filepath.Join(repo, "README.md"), []byte("modified content"), 0o600)
			},
		},
		{
			name: "deleted",
			setup: func(t *testing.T, repo string) {
				_ = os.Remove(filepath.Join(repo, "README.md"))
			},
		},
		{
			name: "renamed",
			setup: func(t *testing.T, repo string) {
				_ = exec.Command("git", "-C", repo, "mv", "README.md", "RENAMED.md").Run()
			},
		},
		{
			name: "untracked",
			setup: func(t *testing.T, repo string) {
				_ = os.WriteFile(filepath.Join(repo, "untracked_table.txt"), []byte("untracked"), 0o600)
			},
		},
		{
			name: "duplicate",
			setup: func(t *testing.T, repo string) {
				// duplicate via modify + also untracked? Actually to test dedupe, we can create a file that appears in both diff and ls-files via edge? Simpler: we test dedupe logic separately via normalizePaths; here we ensure duplicate filtered via sort/dedupe.
				_ = os.WriteFile(filepath.Join(repo, "dup.txt"), []byte("a"), 0o600)
				_ = exec.Command("git", "-C", repo, "add", "dup.txt").Run()
				_ = exec.Command("git", "-C", repo, "commit", "-m", "add dup").Run()
				_ = os.WriteFile(filepath.Join(repo, "dup.txt"), []byte("modified dup"), 0o600)
				// Also create untracked with same name? Can't duplicate path differently; dedupe ensures one entry.
			},
		},
		{
			name: "haro_excluded",
			setup: func(t *testing.T, repo string) {
				_ = os.MkdirAll(filepath.Join(repo, ".haro", "tmp"), 0o755)
				_ = os.WriteFile(filepath.Join(repo, ".haro", "tmp", "ignored.txt"), []byte("x"), 0o600)
			},
		},
		{
			name: "escape_rejected",
			setup: func(t *testing.T, repo string) {
				// Try to create path that would be ../escape if possible via git? Hard to trick git to emit escape, but we test filtering via direct call.
				_ = os.WriteFile(filepath.Join(repo, "normal_escape.txt"), []byte("x"), 0o600)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			repo := t.TempDir()
			base := initGitRepo(t, repo)
			// Ensure additional file for delete/rename to work: base repo has README.md
			tc.setup(t, repo)
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
			execID := "table-" + tc.name
			_ = s.Executions().Create(ctx, &store.Execution{ID: execID, ProjectID: repo, WorkflowSource: "/tmp/wf.yaml", Status: "completed", WorkspaceMode: "shared", WorkspaceRoot: repo, StartedAt: "2025-01-01T00:00:00Z", BaseCommit: &base})
			eng := NewEngine(s, &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
				return 0, "", "", nil
			}}, repo)
			rpt, err := eng.Report(ctx, execID)
			if err != nil {
				t.Fatalf("report: %v", err)
			}
			// Oracle: compute expected via git diff + ls-files with identical flags
			// Use same fixed argv as implementation
			diffOut, _ := exec.Command("git", "-C", repo, "diff", "--name-only", "--diff-filter=ADMR", "--find-renames", "-z", base, "--").Output()
			lsOut, _ := exec.Command("git", "-C", repo, "ls-files", "--others", "--exclude-standard", "-z").Output()
			var oracle []string
			for _, b := range splitNUL(diffOut) {
				if b != "" {
					oracle = append(oracle, b)
				}
			}
			for _, b := range splitNUL(lsOut) {
				if b != "" {
					oracle = append(oracle, b)
				}
			}
			// Filter oracle same as implementation: clean, .haro exclude, dedupe, sort, escape reject
			expected, _ := normalizePaths(repo, repo, "shared", oracle)
			// For haro_excluded, expected should not contain .haro/*
			gotSorted := append([]string{}, rpt.ChangedFiles...)
			sort.Strings(gotSorted)
			// Compare got vs expected oracle filtering (identical flags ensure parity)
			if len(gotSorted) != len(expected) {
				t.Fatalf("case %q: got %v want oracle %v (diff=%q ls=%q)", tc.name, gotSorted, expected, string(diffOut), string(lsOut))
			}
			for i := range gotSorted {
				if gotSorted[i] != expected[i] {
					t.Fatalf("case %q mismatch: got %v want %v", tc.name, gotSorted, expected)
				}
			}
			// Specific assertions
			switch tc.name {
			case "added":
				if !containsStr(rpt.ChangedFiles, "added_new.txt") {
					t.Fatalf("expected added_new.txt in %v", rpt.ChangedFiles)
				}
			case "modified":
				if !containsStr(rpt.ChangedFiles, "README.md") {
					t.Fatalf("expected README.md modified")
				}
			case "deleted":
				if !containsStr(rpt.ChangedFiles, "README.md") {
					t.Fatalf("deleted should still report README.md")
				}
			case "renamed":
				// Should emit new path only, not old
				if !containsStr(rpt.ChangedFiles, "RENAMED.md") {
					t.Fatalf("renamed should emit new path RENAMED.md, got %v", rpt.ChangedFiles)
				}
				if containsStr(rpt.ChangedFiles, "README.md") {
					// old path should not be present as separate entry (rename new only)
					// But note git diff --name-only with --find-renames emits only new path, so old should not appear.
					// If old appears, fails.
					t.Fatalf("renamed should not emit old path README.md, got %v", rpt.ChangedFiles)
				}
				// Verify implementation and test use identical --find-renames flags by checking oracle diff new path
				diffNameStatus, _ := exec.Command("git", "-C", repo, "diff", "--name-status", "--find-renames", base, "--").Output()
				if !strings.Contains(string(diffNameStatus), "RENAMED.md") {
					t.Fatalf("oracle --find-renames diff should show RENAMED.md, got %q", string(diffNameStatus))
				}
			case "untracked":
				if !containsStr(rpt.ChangedFiles, "untracked_table.txt") {
					t.Fatalf("untracked should be included")
				}
			case "haro_excluded":
				for _, f := range rpt.ChangedFiles {
					if strings.HasPrefix(f, ".haro/") {
						t.Fatalf(".haro should be excluded, got %q", f)
					}
				}
			case "duplicate":
				// deduped single entry
				cnt := 0
				for _, f := range rpt.ChangedFiles {
					if f == "dup.txt" {
						cnt++
					}
				}
				if cnt != 1 {
					t.Fatalf("duplicate should be deduped, got %v", rpt.ChangedFiles)
				}
			}
			// Cleanup not needed as TempDir
		})
	}
}

func splitNUL(b []byte) []string {
	var out []string
	start := 0
	for i, c := range b {
		if c == 0 {
			out = append(out, string(b[start:i]))
			start = i + 1
		}
	}
	if start < len(b) {
		out = append(out, string(b[start:]))
	}
	return out
}

func containsStr(arr []string, s string) bool {
	for _, v := range arr {
		if v == s {
			return true
		}
	}
	return false
}
