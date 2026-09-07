package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/HectorCortes/haro/internal/store"
)

func TestReportParityE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("git required")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	ctx := context.Background()
	repo := t.TempDir()
	base := initGitRepoForCLI(t, repo)
	_ = Execute(ctx, []string{"init"}, repo, &bytes.Buffer{}, &bytes.Buffer{})
	// Create terminal execution via store
	storePath := filepath.Join(repo, ".haro", "store.db")
	s, err := store.Open(ctx, storePath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	_ = s.Projects().Create(ctx, repo, repo)
	termID := "parity-test"
	_ = s.Executions().Create(ctx, &store.Execution{ID: termID, ProjectID: repo, WorkflowSource: "/tmp/wf.yaml", Status: "completed", WorkspaceMode: "shared", WorkspaceRoot: repo, StartedAt: "2025-01-01T00:00:00Z", BaseCommit: &base})
	_ = s.Close()
	// Create files to ensure ordered output
	_ = os.WriteFile(filepath.Join(repo, "z_parity.txt"), []byte("z"), 0o600)
	_ = os.WriteFile(filepath.Join(repo, "a_parity.txt"), []byte("a"), 0o600)
	_ = os.WriteFile(filepath.Join(repo, "m_parity.txt"), []byte("m"), 0o600)

	// Plain
	outPlain := &bytes.Buffer{}
	if code := Execute(ctx, []string{"report", termID}, repo, outPlain, &bytes.Buffer{}); code != 0 {
		t.Fatalf("plain exit %d out=%q", code, outPlain.String())
	}
	plainLines := strings.Split(strings.TrimSpace(outPlain.String()), "\n")
	// filter empty
	var plainFiltered []string
	for _, l := range plainLines {
		if strings.TrimSpace(l) != "" {
			plainFiltered = append(plainFiltered, strings.TrimSpace(l))
		}
	}
	// JSON
	outJSON := &bytes.Buffer{}
	if code := Execute(ctx, []string{"report", termID, "--json"}, repo, outJSON, &bytes.Buffer{}); code != 0 {
		t.Fatalf("json exit %d", code)
	}
	var resp map[string]any
	if err := json.Unmarshal(outJSON.Bytes(), &resp); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if resp["execution_id"] != termID {
		t.Fatalf("execution_id mismatch")
	}
	if resp["base_commit"] != base {
		t.Fatalf("base_commit mismatch got %v want %q", resp["base_commit"], base)
	}
	cfAny, ok := resp["changed_files"].([]any)
	if !ok {
		t.Fatalf("changed_files not array")
	}
	var jsonFiles []string
	for _, v := range cfAny {
		if s, ok := v.(string); ok {
			jsonFiles = append(jsonFiles, s)
		}
	}
	// Ordered equality: plain and json should be same ordered list (lexicographically sorted)
	if !reflect.DeepEqual(plainFiltered, jsonFiles) {
		t.Fatalf("plain vs json mismatch\nplain=%v\njson=%v", plainFiltered, jsonFiles)
	}
	// Ensure sorted
	for i := 1; i < len(jsonFiles); i++ {
		if jsonFiles[i-1] > jsonFiles[i] {
			t.Fatalf("not sorted: %v", jsonFiles)
		}
	}
	// Test nullable base_commit inclusion: create execution with null base (terminal legacy)
	s2, _ := store.Open(ctx, storePath)
	_ = s2.Executions().Create(ctx, &store.Execution{ID: "parity-null", ProjectID: repo, WorkflowSource: "/tmp/wf.yaml", Status: "completed", WorkspaceMode: "shared", WorkspaceRoot: repo, StartedAt: "2025-01-01T00:00:00Z", BaseCommit: nil})
	_ = s2.Close()
	out := &bytes.Buffer{}
	code := Execute(ctx, []string{"report", "parity-null", "--json"}, repo, out, &bytes.Buffer{})
	if code != 1 {
		t.Fatalf("null base should be not_available, got %d", code)
	}
	var errResp map[string]string
	if err := json.Unmarshal(out.Bytes(), &errResp); err != nil {
		t.Fatalf("err json: %v", err)
	}
	if errResp["field"] != "base_commit" || errResp["code"] != "not_available" {
		t.Fatalf("null field/code mismatch: %v", errResp)
	}
	// Cleanup
	_ = os.Remove(filepath.Join(repo, "z_parity.txt"))
	_ = os.Remove(filepath.Join(repo, "a_parity.txt"))
	_ = os.Remove(filepath.Join(repo, "m_parity.txt"))
	_ = exec.Command("git", "-C", repo, "clean", "-fd").Run()
	_ = exec.Command("git", "-C", repo, "reset", "--hard", base).Run()
}
