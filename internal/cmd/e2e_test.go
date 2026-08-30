package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HectorCortes/haro/internal/execution"
)

func TestE2EWorkflowsDiscovery(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	// init via Execute
	out := &bytes.Buffer{}
	if code := Execute(ctx, []string{"init"}, root, out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("init code = %d", code)
	}
	// create two workflows
	for _, name := range []string{"alpha", "beta"} {
		dir := filepath.Join(root, ".haro", "workflows", name)
		_ = os.MkdirAll(dir, 0o755)
		yaml := "version: 2\nname: " + name + "\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n"
		_ = os.WriteFile(filepath.Join(dir, "workflow.yaml"), []byte(yaml), 0o600)
	}
	// workflows list --json via Execute
	out.Reset()
	if code := Execute(ctx, []string{"workflows", "list", "--json"}, root, out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("list code = %d, out=%q", code, out.String())
	}
	var resp struct {
		Workflows []struct {
			Name string `json:"name"`
		} `json:"workflows"`
	}
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Workflows) != 2 {
		t.Fatalf("workflows len = %d, want 2", len(resp.Workflows))
	}
	// Should be sorted
	if resp.Workflows[0].Name != "alpha" || resp.Workflows[1].Name != "beta" {
		t.Fatalf("order wrong: %v", resp.Workflows)
	}
	// workflows describe
	out.Reset()
	if code := Execute(ctx, []string{"workflows", "describe", "alpha", "--json"}, root, out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("describe code = %d", code)
	}
	var desc map[string]any
	if err := json.Unmarshal(out.Bytes(), &desc); err != nil {
		t.Fatalf("describe json: %v", err)
	}
	if desc["name"] != "alpha" {
		t.Fatalf("describe name = %v", desc["name"])
	}
}

func TestE2ECommandCycle(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	if code := Execute(ctx, []string{"init"}, root, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("init")
	}
	wfDir := filepath.Join(root, ".haro", "workflows", "demo")
	_ = os.MkdirAll(wfDir, 0o755)
	wfYAML := `version: 2
name: demo
steps:
  - id: s1
    type: command
    run: echo s1
    produces: [out1.txt]
  - id: s2
    type: command
    run: echo s2
    depends_on: [s1]
    requires: [out1.txt]
    produces: [out2.txt]
`
	_ = os.WriteFile(filepath.Join(wfDir, "workflow.yaml"), []byte(wfYAML), 0o600)
	artifacts := filepath.Join(root, ".haro", "artifacts")

	// Helper to run workflow and get execID via Execute
	runWorkflow := func() string {
		out := &bytes.Buffer{}
		if code := Execute(ctx, []string{"run", "demo", "--json"}, root, out, &bytes.Buffer{}); code != 0 {
			t.Fatalf("run code = %d, out=%q", code, out.String())
		}
		var resp map[string]string
		if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
			t.Fatalf("run json: %v", err)
		}
		return resp["execution_id"]
	}

	t.Run("exit 0 with produces completes via Execute", func(t *testing.T) {
		fake := &execution.FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
			p := filepath.Join(artifacts, "out1.txt")
			_ = os.MkdirAll(filepath.Dir(p), 0o755)
			_ = os.WriteFile(p, []byte("out1"), 0o600)
			return 0, "ok", "", nil
		}}
		SetRunnerForTest(fake)
		defer ClearRunnerForTest()
		execID := runWorkflow()
		out := &bytes.Buffer{}
		if code := Execute(ctx, []string{"step", "run", execID, "s1"}, root, out, &bytes.Buffer{}); code != 0 {
			t.Fatalf("step run s1 code = %d, out=%q", code, out.String())
		}
		// Check status
		out.Reset()
		if code := Execute(ctx, []string{"status", execID, "--json"}, root, out, &bytes.Buffer{}); code != 0 {
			t.Fatalf("status code = %d", code)
		}
		var status map[string]any
		_ = json.Unmarshal(out.Bytes(), &status)
		steps := status["steps"].([]any)
		found := false
		for _, s := range steps {
			m := s.(map[string]any)
			if m["id"] == "s1" && m["status"] == "completed" {
				found = true
			}
		}
		if !found {
			t.Fatalf("s1 not completed, status=%v", status)
		}
	})

	t.Run("exit nonzero fails via Execute", func(t *testing.T) {
		fake := &execution.FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
			return 1, "stdout", "stderr", nil
		}}
		SetRunnerForTest(fake)
		defer ClearRunnerForTest()
		execID := runWorkflow()
		out := &bytes.Buffer{}
		// step run should still exit 0? Our Execute returns 1 for run_failed? Actually engine's RunStep for non-zero returns nil (no error) and marks failed, so Execute should return 0 (ok). Check.
		code := Execute(ctx, []string{"step", "run", execID, "s1"}, root, out, &bytes.Buffer{})
		if code != 0 {
			t.Fatalf("nonzero exit should still be ok (failed marked), got %d", code)
		}
		// Verify failed via status
		out.Reset()
		_ = Execute(ctx, []string{"status", execID, "--json"}, root, out, &bytes.Buffer{})
		var status map[string]any
		_ = json.Unmarshal(out.Bytes(), &status)
		steps := status["steps"].([]any)
		for _, s := range steps {
			m := s.(map[string]any)
			if m["id"] == "s1" && m["status"] != "failed" {
				t.Fatalf("s1 should be failed, got %v", m["status"])
			}
		}
	})

	t.Run("exit 0 missing produces fails via Execute", func(t *testing.T) {
		fake := &execution.FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
			// Don't create out1.txt
			return 0, "ok", "", nil
		}}
		SetRunnerForTest(fake)
		defer ClearRunnerForTest()
		execID := runWorkflow()
		_ = os.Remove(filepath.Join(artifacts, "out1.txt"))
		out := &bytes.Buffer{}
		code := Execute(ctx, []string{"step", "run", execID, "s1"}, root, out, &bytes.Buffer{})
		if code == 0 {
			t.Fatalf("missing produces should fail, got 0")
		}
		// Check that code is missing_artifact or run_failed
		var errResp map[string]string
		// Error is written to out (we write error to out)
		if err := json.Unmarshal(out.Bytes(), &errResp); err != nil {
			t.Fatalf("err json: %v, out=%q", err, out.String())
		}
		if !strings.Contains(errResp["error"], "missing") {
			t.Fatalf("error should mention missing, got %v", errResp)
		}
	})

	t.Run("unsatisfied depends_on via Execute", func(t *testing.T) {
		fake := &execution.FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
			return 0, "", "", nil
		}}
		SetRunnerForTest(fake)
		defer ClearRunnerForTest()
		execID := runWorkflow()
		out := &bytes.Buffer{}
		code := Execute(ctx, []string{"step", "run", execID, "s2"}, root, out, &bytes.Buffer{})
		if code == 0 {
			t.Fatalf("unsatisfied deps should fail")
		}
		var errResp map[string]string
		_ = json.Unmarshal(out.Bytes(), &errResp)
		if !strings.Contains(errResp["error"], "unsatisfied") && !strings.Contains(errResp["error"], "dependency") {
			t.Fatalf("should be unsatisfied, got %v", errResp)
		}
	})
}

func TestE2EInitIdempotent(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	out := &bytes.Buffer{}
	if code := Execute(ctx, []string{"init"}, root, out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("first init")
	}
	cfg := filepath.Join(root, ".haro", "config.yaml")
	b1, _ := os.ReadFile(cfg)
	if code := Execute(ctx, []string{"init"}, root, out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("second init")
	}
	b2, _ := os.ReadFile(cfg)
	if string(b1) != string(b2) {
		t.Fatalf("bytes changed")
	}
}

func TestE2EStaleProducesInvalid(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	_ = Execute(ctx, []string{"init"}, root, &bytes.Buffer{}, &bytes.Buffer{})
	wfDir := filepath.Join(root, ".haro", "workflows", "stale")
	_ = os.MkdirAll(wfDir, 0o755)
	wfYAML := `version: 2
name: stale
steps:
  - id: s1
    type: command
    run: echo s1
    produces: [s1.txt]
  - id: s2
    type: command
    run: echo s2
    depends_on: [s1]
    requires: [s1.txt]
    produces: [s2.txt]
  - id: s3
    type: command
    run: echo s3
    depends_on: [s2]
    requires: [s2.txt]
    produces: [s3.txt]
`
	_ = os.WriteFile(filepath.Join(wfDir, "workflow.yaml"), []byte(wfYAML), 0o600)
	artifacts := filepath.Join(root, ".haro", "artifacts")
	// Run all three to completed
	fake := &execution.FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
		for _, prod := range []string{"s1.txt", "s2.txt", "s3.txt"} {
			if len(argv) > 1 && strings.Contains(argv[1], strings.TrimSuffix(prod, ".txt")) {
				p := filepath.Join(artifacts, prod)
				_ = os.MkdirAll(filepath.Dir(p), 0o755)
				_ = os.WriteFile(p, []byte(prod), 0o600)
			}
		}
		return 0, "", "", nil
	}}
	SetRunnerForTest(fake)
	// run workflow
	out := &bytes.Buffer{}
	if code := Execute(ctx, []string{"run", "stale", "--json"}, root, out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("run")
	}
	var resp map[string]string
	_ = json.Unmarshal(out.Bytes(), &resp)
	execID := resp["execution_id"]
	for _, sid := range []string{"s1", "s2", "s3"} {
		out.Reset()
		if code := Execute(ctx, []string{"step", "run", execID, sid}, root, out, &bytes.Buffer{}); code != 0 {
			t.Fatalf("run %s: %d %q", sid, code, out.String())
		}
	}
	// Verify files exist
	for _, f := range []string{"s1.txt", "s2.txt", "s3.txt"} {
		if _, err := os.Stat(filepath.Join(artifacts, f)); err != nil {
			t.Fatalf("file %s missing", f)
		}
	}
	// Reopen s1 cascade via Execute
	out.Reset()
	if code := Execute(ctx, []string{"step", "reopen", execID, "s1", "--cascade"}, root, out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("reopen s1")
	}
	// Files should still exist but requires should now be invalid
	for _, f := range []string{"s1.txt", "s2.txt", "s3.txt"} {
		if _, err := os.Stat(filepath.Join(artifacts, f)); err != nil {
			t.Fatalf("file %s should remain after reopen", f)
		}
	}
	// Try to run s2 - should fail due to invalidated s1.txt
	ClearRunnerForTest()
	fake2 := &execution.FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
		_ = os.WriteFile(filepath.Join(artifacts, "s2.txt"), []byte("s2"), 0o600)
		return 0, "", "", nil
	}}
	SetRunnerForTest(fake2)
	defer ClearRunnerForTest()
	out.Reset()
	code := Execute(ctx, []string{"step", "run", execID, "s2"}, root, out, &bytes.Buffer{})
	if code == 0 {
		t.Fatalf("s2 should fail due to stale requires/unsatisfied after reopen")
	}
	var errResp map[string]string
	_ = json.Unmarshal(out.Bytes(), &errResp)
	if !strings.Contains(errResp["error"], "invalidated") && !strings.Contains(errResp["error"], "requires") && !strings.Contains(errResp["error"], "unsatisfied") {
		t.Fatalf("stale error wrong: %v", errResp)
	}
	// Status should show s1,s2,s3 pending after reopen
	out.Reset()
	_ = Execute(ctx, []string{"status", execID, "--json"}, root, out, &bytes.Buffer{})
	var status map[string]any
	_ = json.Unmarshal(out.Bytes(), &status)
	steps := status["steps"].([]any)
	for _, s := range steps {
		m := s.(map[string]any)
		if m["status"] != "pending" {
			t.Fatalf("after reopen status should be pending, got %v", m)
		}
	}
}
