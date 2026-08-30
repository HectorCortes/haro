package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI_Routing(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	// init
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	code := Execute(ctx, []string{"init"}, root, out, errOut)
	if code != 0 {
		t.Fatalf("init exit = %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(root, ".haro", "config.yaml")); err != nil {
		t.Fatalf("init didn't create config: %v", err)
	}
	// workflows list empty
	out.Reset()
	code = Execute(ctx, []string{"workflows", "list"}, root, out, errOut)
	if code != 0 {
		t.Fatalf("workflows list exit = %d", code)
	}
	// Create a workflow
	wfDir := filepath.Join(root, ".haro", "workflows", "demo")
	_ = os.MkdirAll(wfDir, 0o755)
	_ = os.WriteFile(filepath.Join(wfDir, "workflow.yaml"), []byte("version: 2\nname: demo\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n"), 0o600)
	// workflows list with --json
	out.Reset()
	code = Execute(ctx, []string{"workflows", "list", "--json"}, root, out, errOut)
	if code != 0 {
		t.Fatalf("list --json exit = %d, errOut=%s", code, errOut.String())
	}
	var listResp map[string]any
	if err := json.Unmarshal(out.Bytes(), &listResp); err != nil {
		t.Fatalf("list json unmarshal: %v, out=%q", err, out.String())
	}
	// Should contain workflows array
	if _, ok := listResp["workflows"]; !ok {
		t.Fatalf("list json missing workflows: %v", listResp)
	}
	// workflows describe
	out.Reset()
	code = Execute(ctx, []string{"workflows", "describe", "demo", "--json"}, root, out, errOut)
	if code != 0 {
		t.Fatalf("describe exit = %d", code)
	}
	var descResp map[string]any
	if err := json.Unmarshal(out.Bytes(), &descResp); err != nil {
		t.Fatalf("describe json: %v", err)
	}
	if descResp["name"] != "demo" {
		t.Fatalf("describe name = %v, want demo", descResp["name"])
	}
}

func TestCLI_FlagSets(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	_ = Execute(ctx, []string{"init"}, root, &bytes.Buffer{}, &bytes.Buffer{})
	// Each leaf should use FlagSet ContinueOnError; test that unknown flag errors with code unexpected_argument? Actually unknown flag should be error with code.
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	code := Execute(ctx, []string{"workflows", "list", "--unknown"}, root, out, errOut)
	if code != 1 {
		t.Fatalf("unknown flag should exit 1, got %d", code)
	}
	var errResp map[string]string
	if err := json.Unmarshal(out.Bytes(), &errResp); err != nil {
		// Might be written to errOut? Check both
		if err2 := json.Unmarshal(errOut.Bytes(), &errResp); err2 != nil {
			t.Fatalf("unknown flag json not found in out or errOut: out=%q errOut=%q", out.String(), errOut.String())
		}
	}
	if errResp["code"] == "" {
		t.Fatalf("error code missing for unknown flag")
	}
}

func TestCLI_DoubleDash(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	_ = Execute(ctx, []string{"init"}, root, &bytes.Buffer{}, &bytes.Buffer{})
	wfDir := filepath.Join(root, ".haro", "workflows", "demo")
	_ = os.MkdirAll(wfDir, 0o755)
	_ = os.WriteFile(filepath.Join(wfDir, "workflow.yaml"), []byte("version: 2\nname: demo\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n"), 0o600)
	// -- should terminate option parsing; remaining positional should be unexpected_argument
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	code := Execute(ctx, []string{"workflows", "list", "--", "--json"}, root, out, errOut)
	if code != 1 {
		t.Fatalf("--json after -- should be unexpected_argument, exit 1, got %d out=%q errOut=%q", code, out.String(), errOut.String())
	}
	var errResp map[string]string
	combined := out.String() + errOut.String()
	if err := json.Unmarshal(out.Bytes(), &errResp); err != nil {
		if err2 := json.Unmarshal(errOut.Bytes(), &errResp); err2 != nil {
			// Try combined?
			t.Fatalf("unexpected_argument json not found: %q", combined)
		}
	}
	if errResp["code"] != "unexpected_argument" {
		t.Fatalf("code = %q, want unexpected_argument", errResp["code"])
	}
	// also test extra positional without --
	out.Reset()
	errOut.Reset()
	code = Execute(ctx, []string{"init", "extra"}, root, out, errOut)
	if code != 1 {
		t.Fatalf("extra positional for init should be unexpected_argument, got %d", code)
	}
}

func TestCLI_Errors(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	// Unknown command should be error
	code := Execute(ctx, []string{"unknown"}, root, out, errOut)
	if code != 1 {
		t.Fatalf("unknown command exit = %d, want 1", code)
	}
	// Check JSON error
	var errResp map[string]string
	if err := json.Unmarshal(out.Bytes(), &errResp); err != nil {
		if err2 := json.Unmarshal(errOut.Bytes(), &errResp); err2 != nil {
			t.Fatalf("error json missing")
		}
	}
	if errResp["error"] == "" || errResp["code"] == "" {
		t.Fatalf("error response missing fields: %v", errResp)
	}
}

func TestCLI_EPIPE(t *testing.T) {
	// syscall.EPIPE while writing should be success/exit 0
	// Simulate by closing pipe writer before Execute writes
	ctx := context.Background()
	root := t.TempDir()
	_ = Execute(ctx, []string{"init"}, root, &bytes.Buffer{}, &bytes.Buffer{})
	wfDir := filepath.Join(root, ".haro", "workflows", "demo")
	_ = os.MkdirAll(wfDir, 0o755)
	_ = os.WriteFile(filepath.Join(wfDir, "workflow.yaml"), []byte("version: 2\nname: demo\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n"), 0o600)
	r, w, _ := os.Pipe()
	_ = r.Close()
	_ = w.Close() // closed pipe
	// Execute with closed writers should handle EPIPE and exit 0
	code := Execute(ctx, []string{"workflows", "list", "--json"}, root, w, w)
	if code != 0 {
		// Our implementation should treat EPIPE as success
		t.Fatalf("EPIPE should exit 0, got %d", code)
	}
	// Triangulation: successful write without EPIPE should still be 0 and contain JSON
	out := &bytes.Buffer{}
	code = Execute(ctx, []string{"workflows", "list", "--json"}, root, out, &bytes.Buffer{})
	if code != 0 || !strings.Contains(out.String(), "workflows") {
		t.Fatalf("normal write should succeed")
	}
	_ = r
}
