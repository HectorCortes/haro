package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HectorCortes/haro/internal/store"
)

func initGitRepoForCLI(t *testing.T, dir string) string {
	t.Helper()
	if err := exec.Command("git", "init", dir).Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	_ = exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	_ = exec.Command("git", "-C", dir, "config", "user.name", "test").Run()
	_ = os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base"), 0o600)
	_ = exec.Command("git", "-C", dir, "add", ".").Run()
	_ = exec.Command("git", "-C", dir, "commit", "-m", "init").Run()
	out, _ := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	return strings.TrimSpace(string(out))
}

func TestReportCLI(t *testing.T) {
	if testing.Short() {
		t.Skip("git required")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	ctx := context.Background()
	repo := t.TempDir()
	base := initGitRepoForCLI(t, repo)
	if code := Execute(ctx, []string{"init"}, repo, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("init failed")
	}
	wfDir := filepath.Join(repo, ".haro", "workflows", "demo")
	_ = os.MkdirAll(wfDir, 0o755)
	_ = os.WriteFile(filepath.Join(wfDir, "workflow.yaml"), []byte("version: 2\nname: demo\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n"), 0o600)
	out := &bytes.Buffer{}
	if code := Execute(ctx, []string{"run", "demo", "--json"}, repo, out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("run failed: %s", out.String())
	}
	var runResp map[string]string
	if err := json.Unmarshal(out.Bytes(), &runResp); err != nil {
		t.Fatalf("run json: %v out=%q", err, out.String())
	}
	execID := runResp["execution_id"]
	if execID == "" {
		t.Fatalf("no execution_id")
	}
	// Test not_found
	out.Reset()
	code := Execute(ctx, []string{"report", "nonexistent-id"}, repo, out, &bytes.Buffer{})
	if code != 1 {
		t.Fatalf("not_found should exit 1, got %d out=%q", code, out.String())
	}
	var errResp map[string]string
	if err := json.Unmarshal(out.Bytes(), &errResp); err != nil {
		t.Fatalf("not_found json: %v out=%q", err, out.String())
	}
	if errResp["code"] != "not_found" {
		t.Fatalf("not_found code = %q want not_found", errResp["code"])
	}
	// Test not_completed (pending/running)
	out.Reset()
	code = Execute(ctx, []string{"report", execID}, repo, out, &bytes.Buffer{})
	if code != 1 {
		t.Fatalf("running should be not_completed, got %d out=%q", code, out.String())
	}
	if err := json.Unmarshal(out.Bytes(), &errResp); err != nil {
		t.Fatalf("not_completed json: %v", err)
	}
	if errResp["code"] != "not_completed" {
		t.Fatalf("not_completed code = %q want not_completed", errResp["code"])
	}
	// Test FlagSet parsing: unknown flag should be invalid_argument
	out.Reset()
	code = Execute(ctx, []string{"report", execID, "--unknown"}, repo, out, &bytes.Buffer{})
	if code != 1 {
		t.Fatalf("unknown flag should exit 1, got %d", code)
	}
	if err := json.Unmarshal(out.Bytes(), &errResp); err != nil {
		t.Fatalf("unknown flag json: %v", err)
	}
	if errResp["code"] == "" {
		t.Fatalf("unknown flag should have code")
	}
	// Test missing execution id
	out.Reset()
	code = Execute(ctx, []string{"report"}, repo, out, &bytes.Buffer{})
	if code != 1 {
		t.Fatalf("missing id should exit 1, got %d", code)
	}
	// Test -- terminator unexpected
	out.Reset()
	code = Execute(ctx, []string{"report", execID, "--", "extra"}, repo, out, &bytes.Buffer{})
	if code != 1 {
		t.Fatalf("-- extra should be unexpected_argument, got %d out=%q", code, out.String())
	}
	if err := json.Unmarshal(out.Bytes(), &errResp); err == nil {
		if errResp["code"] != "unexpected_argument" {
			t.Fatalf("code = %q want unexpected_argument", errResp["code"])
		}
	}
	// Create terminal execution for plain/json/EPIPE tests via store
	storePath := filepath.Join(repo, ".haro", "store.db")
	s, err := store.Open(ctx, storePath)
	if err != nil {
		t.Fatalf("store open: %v", err)
	}
	_ = s.Projects().Create(ctx, repo, repo)
	termID := "cli-terminal"
	_ = s.Executions().Create(ctx, &store.Execution{ID: termID, ProjectID: repo, WorkflowSource: "/tmp/wf.yaml", Status: "completed", WorkspaceMode: "shared", WorkspaceRoot: repo, StartedAt: "2025-01-01T00:00:00Z", BaseCommit: &base})
	_ = s.Close()
	// Create file to be reported
	_ = os.WriteFile(filepath.Join(repo, "cli_new.txt"), []byte("cli"), 0o600)

	// Plain output
	out.Reset()
	code = Execute(ctx, []string{"report", termID}, repo, out, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("plain report should exit 0, got %d out=%q", code, out.String())
	}
	if !strings.Contains(out.String(), "cli_new.txt") {
		t.Fatalf("plain should contain cli_new.txt, got %q", out.String())
	}
	// Ensure plain is one per line
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("plain lines empty")
	}

	// JSON output
	out.Reset()
	code = Execute(ctx, []string{"report", termID, "--json"}, repo, out, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("json report exit %d out=%q", code, out.String())
	}
	var jsonResp map[string]any
	if err := json.Unmarshal(out.Bytes(), &jsonResp); err != nil {
		t.Fatalf("json unmarshal: %v out=%q", err, out.String())
	}
	if jsonResp["execution_id"] != termID {
		t.Fatalf("json execution_id = %v want %q", jsonResp["execution_id"], termID)
	}
	if jsonResp["base_commit"] != base {
		t.Fatalf("json base_commit = %v want %q", jsonResp["base_commit"], base)
	}
	cf, ok := jsonResp["changed_files"].([]any)
	if !ok {
		t.Fatalf("changed_files not array: %v", jsonResp["changed_files"])
	}
	found := false
	for _, v := range cf {
		if s, _ := v.(string); s == "cli_new.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("json should contain cli_new.txt, got %v", cf)
	}

	// EPIPE: use closed pipe
	r, w, _ := os.Pipe()
	_ = r.Close()
	_ = w.Close()
	code = Execute(ctx, []string{"report", termID, "--json"}, repo, w, w)
	if code != 0 {
		t.Fatalf("EPIPE json should exit 0, got %d", code)
	}
	r2, w2, _ := os.Pipe()
	_ = r2.Close()
	_ = w2.Close()
	code = Execute(ctx, []string{"report", termID}, repo, w2, w2)
	if code != 0 {
		t.Fatalf("EPIPE plain should exit 0, got %d", code)
	}
	// not_available case: null base_commit
	s2, _ := store.Open(ctx, storePath)
	_ = s2.Executions().Create(ctx, &store.Execution{ID: "null-cli", ProjectID: repo, WorkflowSource: "/tmp/wf.yaml", Status: "completed", WorkspaceMode: "shared", WorkspaceRoot: repo, StartedAt: "2025-01-01T00:00:00Z", BaseCommit: nil})
	_ = s2.Close()
	out.Reset()
	code = Execute(ctx, []string{"report", "null-cli", "--json"}, repo, out, &bytes.Buffer{})
	if code != 1 {
		t.Fatalf("null base should be not_available exit 1, got %d out=%q", code, out.String())
	}
	if err := json.Unmarshal(out.Bytes(), &errResp); err != nil {
		t.Fatalf("null json: %v", err)
	}
	if errResp["code"] != "not_available" || errResp["field"] != "base_commit" {
		t.Fatalf("null code/field = %v want not_available/base_commit", errResp)
	}
	// Cleanup
	_ = os.Remove(filepath.Join(repo, "cli_new.txt"))
	_ = exec.Command("git", "-C", repo, "clean", "-fd").Run()
	_ = exec.Command("git", "-C", repo, "reset", "--hard", base).Run()
}
