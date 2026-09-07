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

func TestReportGitOracle(t *testing.T) {
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
	_ = os.WriteFile(filepath.Join(repo, "oracle_added.txt"), []byte("added"), 0o600)
	_ = exec.Command("git", "-C", repo, "mv", "README.md", "oracle_renamed.txt").Run()
	_ = os.WriteFile(filepath.Join(repo, "oracle_untracked.txt"), []byte("untracked"), 0o600)

	dbPath := filepath.Join(repo, ".haro", "store.db")
	s, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = s.Close() }()
	_ = s.Projects().Create(ctx, repo, repo)
	execID := "oracle-test"
	_ = s.Executions().Create(ctx, &store.Execution{ID: execID, ProjectID: repo, WorkflowSource: "/tmp/wf.yaml", Status: "completed", WorkspaceMode: "shared", WorkspaceRoot: repo, StartedAt: "2025-01-01T00:00:00Z", BaseCommit: &base})
	eng := NewEngine(s, &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
		return 0, "", "", nil
	}}, repo)
	rpt, err := eng.Report(ctx, execID)
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	// Oracle: git diff --name-status --find-renames + status
	diffStatusOut, _ := exec.Command("git", "-C", repo, "diff", "--name-status", "--find-renames", base, "--").Output()
	statusOut, _ := exec.Command("git", "-C", repo, "status", "--porcelain=v1").Output()
	// Parse oracle: diff --name-status yields lines "M\tpath" or "D\tpath" or "R100\told\tnew"
	var oracle []string
	for _, line := range strings.Split(string(diffStatusOut), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) == 0 {
			continue
		}
		status := strings.TrimSpace(parts[0])
		if strings.HasPrefix(status, "R") {
			// rename: take new path only (parts[2])
			if len(parts) >= 3 {
				oracle = append(oracle, strings.TrimSpace(parts[2]))
			}
		} else {
			if len(parts) >= 2 {
				oracle = append(oracle, strings.TrimSpace(parts[1]))
			}
		}
	}
	// status porcelain: "?? path" is untracked
	for _, line := range strings.Split(string(statusOut), "\n") {
		if strings.HasPrefix(line, "??") {
			p := strings.TrimSpace(line[2:])
			if p != "" {
				// Remove quotes if any? porcelain quotes handling ignore for now
				oracle = append(oracle, p)
			}
		}
	}
	// Filter oracle same as implementation: .haro excluded, dedupe, sorted, cleaned
	filteredOracle, _ := normalizePaths(repo, repo, "shared", oracle)
	sort.Strings(filteredOracle)
	sort.Strings(rpt.ChangedFiles)
	if len(rpt.ChangedFiles) != len(filteredOracle) {
		t.Fatalf("oracle mismatch len got %d (%v) want %d (%v)\n diffStatus=%q\n status=%q", len(rpt.ChangedFiles), rpt.ChangedFiles, len(filteredOracle), filteredOracle, string(diffStatusOut), string(statusOut))
	}
	for i := range filteredOracle {
		if rpt.ChangedFiles[i] != filteredOracle[i] {
			t.Fatalf("oracle mismatch at %d: got %q want %q\n got %v\n want %v", i, rpt.ChangedFiles[i], filteredOracle[i], rpt.ChangedFiles, filteredOracle)
		}
	}
	// Verify rename flags identical: implementation uses --find-renames, test oracle also uses --find-renames, ensure both agree on rename new path
	foundRenamed := false
	for _, f := range rpt.ChangedFiles {
		if f == "oracle_renamed.txt" {
			foundRenamed = true
			break
		}
	}
	if !foundRenamed {
		t.Fatalf("oracle should contain renamed new path oracle_renamed.txt, got %v", rpt.ChangedFiles)
	}
	// Ensure old path not present
	for _, f := range rpt.ChangedFiles {
		if f == "README.md" {
			t.Fatalf("rename old path README.md should not appear when rename detected, got %v", rpt.ChangedFiles)
		}
	}
	// Cleanup
	_ = exec.Command("git", "-C", repo, "reset", "--hard", base).Run()
	_ = exec.Command("git", "-C", repo, "clean", "-fd").Run()
}
