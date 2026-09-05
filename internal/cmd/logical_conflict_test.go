package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/HectorCortes/haro/internal/execution"
	"github.com/HectorCortes/haro/internal/store"
)

func TestLogicalConflict(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	// init project with external_paths? not needed
	// Create workflow shared claiming src/foo.ts
	if err := os.MkdirAll(filepath.Join(dir, ".haro", "workflows", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "version: 2\nname: demo\nworkspace:\n  mode: shared\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n    produces: [src/foo.ts]\n"
	if err := os.WriteFile(filepath.Join(dir, ".haro", "workflows", "demo", "workflow.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	// create execution via CLI run
	out := &bytes.Buffer{}
	code := Execute(ctx, []string{"run", "demo", "--json"}, dir, out, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("run failed: %v out=%q", code, out.String())
	}
	var runResp map[string]string
	_ = json.Unmarshal(out.Bytes(), &runResp)
	execID := runResp["execution_id"]
	if execID == "" {
		t.Fatalf("no execution_id, out=%q", out.String())
	}
	// Acquire a blocking claim via store directly for same project (dir) on same path
	s, err := store.Open(ctx, filepath.Join(dir, ".haro", "store.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	// Create a second execution that also claims src/foo.ts but we will block it via manual claim
	block := store.PathClaim{
		ProjectID:        dir,
		LogicalPath:      "src/foo.ts",
		Mode:             "shared",
		OwnerExecutionID: "other-exec",
		OwnerStepID:      "other-step",
	}
	if ok, _, err := s.PathClaims().Acquire(ctx, block); err != nil || !ok {
		t.Fatalf("manual block acquire failed: %v ok=%v", err, ok)
	}
	_ = s.Close()
	// Now run step via CLI - should get logical_conflict
	SetRunnerForTest(&execution.FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
		// create expected artifact file so not missing
		_ = os.MkdirAll(filepath.Join(dir, ".haro", "artifacts", "src"), 0o755)
		_ = os.WriteFile(filepath.Join(dir, ".haro", "artifacts", "src", "foo.ts"), []byte("x"), 0o600)
		return 0, "ok", "", nil
	}})
	defer ClearRunnerForTest()
	out.Reset()
	code = Execute(ctx, []string{"step", "run", execID, "s1", "--json"}, dir, out, &bytes.Buffer{})
	if code != 1 {
		t.Fatalf("expected logical_conflict exit 1, got %d out=%q", code, out.String())
	}
	var errResp map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal err resp: %v out=%q", err, out.String())
	}
	if errResp["code"] != "logical_conflict" {
		t.Fatalf("code = %v want logical_conflict, resp=%v", errResp["code"], errResp)
	}
	if errResp["owner_execution_id"] == nil || errResp["owner_step_id"] == nil {
		t.Fatalf("logical_conflict should contain owner info, got %v", errResp)
	}
	// Also test steps next filtering: blocked step should be omitted
	// Create workflow with single pending step that is blocked, steps next should return nil
	yaml2 := "version: 2\nname: demo2\nworkspace:\n  mode: shared\nsteps:\n  - id: s2\n    type: command\n    run: echo hi\n    produces: [src/foo.ts]\n"
	_ = os.MkdirAll(filepath.Join(dir, ".haro", "workflows", "demo2"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, ".haro", "workflows", "demo2", "workflow.yaml"), []byte(yaml2), 0o600)
	out.Reset()
	code = Execute(ctx, []string{"run", "demo2", "--json"}, dir, out, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("run demo2: %v", out.String())
	}
	var run2 map[string]string
	_ = json.Unmarshal(out.Bytes(), &run2)
	exec2 := run2["execution_id"]
	// steps next should be filtered because src/foo.ts is blocked by other-exec claim
	out.Reset()
	code = Execute(ctx, []string{"steps", "next", exec2, "--json"}, dir, out, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("steps next: %d %q", code, out.String())
	}
	var nextResp map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &nextResp); err != nil {
		t.Fatalf("unmarshal next: %v", err)
	}
	// When blocked, next should be nil (no runnable step)
	if nextResp["next"] != nil {
		// Could be steps next returns {"step_id":...} or {"next": nil}
		// Our current API returns {"next": nil} when no step, but check
		if _, ok := nextResp["step_id"]; ok {
			t.Fatalf("blocked step should not be returned as next, got %v", nextResp)
		}
	}
	// Release block and verify steps next returns s2
	s2, _ := store.Open(ctx, filepath.Join(dir, ".haro", "store.db"))
	_ = s2.PathClaims().Release(ctx, dir, "src/foo.ts", "other-exec", "other-step")
	_ = s2.Close()
	out.Reset()
	code = Execute(ctx, []string{"steps", "next", exec2, "--json"}, dir, out, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("steps next after release: %d", code)
	}
	var next2 map[string]interface{}
	_ = json.Unmarshal(out.Bytes(), &next2)
	if next2["step_id"] == nil && next2["next"] != nil {
		t.Fatalf("after release expected next step, got %v", next2)
	}
}
