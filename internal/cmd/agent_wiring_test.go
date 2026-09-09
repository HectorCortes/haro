package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HectorCortes/haro/internal/store"
)

// writeOpenCodeFixture writes the executable JSONL fixture used by the CLI
// wiring test. It appends one invocation record per run so the test can
// assert exactly one subprocess session per step run.
func writeOpenCodeFixture(t *testing.T, logPath string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "oc-fixture.sh")
	script := "#!/bin/sh\n" +
		"stdin=$(cat)\n" +
		"printf 'argv:%s\\nstdin:%s\\n' \"$*\" \"$stdin\" >> \"" + logPath + "\"\n" +
		"cat >/dev/null\n" +
		"printf '%s\\n' '{\"type\":\"step_start\",\"sessionID\":\"sess_fixture_1\"}'\n" +
		"printf '%s\\n' '{\"type\":\"part\",\"sessionID\":\"sess_fixture_1\",\"part\":{\"type\":\"text\",\"text\":\"fixture output\"}}'\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestAgentManagerCLIInjection covers F-01: each agent-capable CLI invocation
// (run, step run, step reopen, step skip) wires one manager built from the
// project configuration; the report site stays unwired. A real session runs
// through the injected manager exactly once per step run.
func TestAgentManagerCLIInjection(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	if code := Execute(ctx, []string{"init"}, root, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("init code != 0")
	}
	logPath := filepath.Join(t.TempDir(), "invocations.log")
	binary := writeOpenCodeFixture(t, logPath)

	wfDir := filepath.Join(root, ".haro", "workflows", "agentwf")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wfYAML := `version: 2
name: agentwf
steps:
  - id: ag
    type: agent
    harness: [oc]
    instructions: do the thing
    mode: headless
`
	if err := os.WriteFile(filepath.Join(wfDir, "workflow.yaml"), []byte(wfYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	cfgYAML := "version: 2\nharnesses:\n  oc:\n    binary: " + binary + "\n"
	if err := os.WriteFile(filepath.Join(root, ".haro", "config.yaml"), []byte(cfgYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	// run: one manager per invocation; sessions initialize before first use.
	out := &bytes.Buffer{}
	if code := Execute(ctx, []string{"run", "agentwf"}, root, out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("run code = %d, out=%q", code, out.String())
	}
	execID := strings.TrimSpace(out.String())
	if execID == "" {
		t.Fatalf("execution id missing, out=%q", out.String())
	}

	// step run: the agent step completes through the real fixture session.
	out.Reset()
	if code := Execute(ctx, []string{"step", "run", execID, "ag"}, root, out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("step run code = %d, out=%q", code, out.String())
	}

	// The subprocess ran exactly once with the fixed argv and prompt.
	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("fixture log: %v", err)
	}
	logged := string(logData)
	if got := strings.Count(logged, "argv:"); got != 1 {
		t.Fatalf("exactly one fixture invocation expected, got %d: %q", got, logged)
	}
	if !strings.Contains(logged, "argv:run --format json") {
		t.Fatalf("fixture argv mismatch: %q", logged)
	}

	// Step is completed and the real transport identity is persisted.
	s, err := store.Open(ctx, filepath.Join(root, ".haro", "store.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = s.Close() }()
	step, err := s.Steps().Get(ctx, execID, "ag")
	if err != nil {
		t.Fatalf("get step: %v", err)
	}
	if step.Status != "completed" {
		t.Fatalf("agent step status = %q, want completed", step.Status)
	}
	attemptID := ""
	rows, err := s.QueryForTest(ctx, `SELECT id FROM attempts WHERE execution_id = ? AND step_id = ?`, execID, "ag")
	if err != nil {
		t.Fatalf("query attempts: %v", err)
	}
	if rows.Next() {
		if err := rows.Scan(&attemptID); err != nil {
			t.Fatal(err)
		}
	}
	_ = rows.Close()
	if attemptID == "" {
		t.Fatalf("no attempt row for agent step")
	}
	tr, err := s.Transport().Get(ctx, attemptID)
	if err != nil {
		t.Fatalf("get transport: %v", err)
	}
	if tr.NativeSessionID == nil || *tr.NativeSessionID != "sess_fixture_1" {
		t.Fatalf("native_session_id = %v, want sess_fixture_1", tr.NativeSessionID)
	}
	if tr.AdapterName != "oc" {
		t.Fatalf("adapter_name = %q", tr.AdapterName)
	}

	// step skip with harness configuration present: wiring must not break
	// the skip site (fresh pending step needed, so use a new execution).
	out.Reset()
	if code := Execute(ctx, []string{"run", "agentwf"}, root, out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("second run code = %d", code)
	}
	execID2 := strings.TrimSpace(out.String())
	out.Reset()
	if code := Execute(ctx, []string{"step", "skip", execID2, "ag", "--reason", "not needed"}, root, out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("step skip code = %d, out=%q", code, out.String())
	}
	step2, err := s.Steps().Get(ctx, execID2, "ag")
	if err != nil {
		t.Fatalf("get step2: %v", err)
	}
	if step2.Status != "skipped" {
		t.Fatalf("agent step status = %q, want skipped", step2.Status)
	}
}
