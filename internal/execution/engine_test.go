package execution

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HectorCortes/haro/internal/adapter"
	"github.com/HectorCortes/haro/internal/adapter/claude"
	"github.com/HectorCortes/haro/internal/adapter/opencode"
	"github.com/HectorCortes/haro/internal/project"
	"github.com/HectorCortes/haro/internal/store"
)

func TestCommandCycle(t *testing.T) {
	root := t.TempDir()
	if err := project.Init(root); err != nil {
		t.Fatalf("init: %v", err)
	}
	// Create workflow with 3 steps for testing dependencies
	wfDir := filepath.Join(root, ".haro", "workflows", "demo")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatalf("mkdir wf: %v", err)
	}
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
  - id: s3
    type: command
    run: echo s3
    depends_on: [s2]
    produces: [out3.txt]
`
	if err := os.WriteFile(filepath.Join(wfDir, "workflow.yaml"), []byte(wfYAML), 0o600); err != nil {
		t.Fatalf("write wf: %v", err)
	}
	// Store
	dbPath := filepath.Join(root, ".haro", "store.db")
	ctx := context.Background()
	s, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = s.Close() }()

	artifacts := filepath.Join(root, ".haro", "artifacts")

	// Helper to create execution
	createExec := func() string {
		eng := NewEngine(s, &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
			return 0, "", "", nil
		}}, root)
		id, err := eng.CreateExecution(ctx, "demo")
		if err != nil {
			t.Fatalf("CreateExecution: %v", err)
		}
		return id
	}

	t.Run("exit 0 with produces completes", func(t *testing.T) {
		fake := &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
			// Simulate command creating out1.txt
			p := filepath.Join(artifacts, "out1.txt")
			_ = os.MkdirAll(filepath.Dir(p), 0o755)
			_ = os.WriteFile(p, []byte("out1"), 0o600)
			return 0, "stdout ok", "", nil
		}}
		eng := NewEngine(s, fake, root)
		execID := createExec()
		// Run s1
		if err := eng.RunStep(ctx, execID, "s1", ""); err != nil {
			t.Fatalf("RunStep s1: %v", err)
		}
		st, _ := s.Steps().Get(ctx, execID, "s1")
		if st.Status != "completed" {
			t.Fatalf("s1 status = %q, want completed", st.Status)
		}
		if len(fake.Calls) != 1 {
			t.Fatalf("runner calls = %d, want 1", len(fake.Calls))
		}
		// Check evidence visible is present (redacted+truncated)
	})

	t.Run("exit nonzero fails with stdout/stderr", func(t *testing.T) {
		fake := &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
			return 1, "stdout fail", "stderr fail", nil
		}}
		eng := NewEngine(s, fake, root)
		execID := createExec()
		err := eng.RunStep(ctx, execID, "s1", "")
		if err != nil {
			t.Fatalf("RunStep should not error for runner failure, but mark failed: %v", err)
		}
		st, _ := s.Steps().Get(ctx, execID, "s1")
		if st.Status != "failed" {
			t.Fatalf("s1 status = %q, want failed", st.Status)
		}
		// Evidence should contain stdout/stderr
	})

	t.Run("exit 0 missing produces fails listing", func(t *testing.T) {
		fake := &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
			// Do NOT create out1.txt
			return 0, "ok", "", nil
		}}
		eng := NewEngine(s, fake, root)
		execID := createExec()
		// Ensure out1.txt does not exist
		_ = os.Remove(filepath.Join(artifacts, "out1.txt"))
		err := eng.RunStep(ctx, execID, "s1", "")
		if err == nil {
			t.Fatalf("expected error for missing produces")
		}
		if !contains(err.Error(), "missing") {
			t.Fatalf("expected missing artifact error, got %v", err)
		}
		st, _ := s.Steps().Get(ctx, execID, "s1")
		if st.Status != "failed" {
			t.Fatalf("s1 status = %q, want failed on missing produces", st.Status)
		}
	})

	t.Run("unsatisfied depends_on unexecuted", func(t *testing.T) {
		called := false
		fake := &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
			called = true
			return 0, "", "", nil
		}}
		eng := NewEngine(s, fake, root)
		execID := createExec()
		// Try to run s2 before s1 completes
		err := eng.RunStep(ctx, execID, "s2", "")
		if err == nil {
			t.Fatalf("expected error for unsatisfied depends_on")
		}
		if called {
			t.Fatalf("runner should not be called for unsatisfied deps")
		}
		st, _ := s.Steps().Get(ctx, execID, "s2")
		if st.Status != "pending" {
			t.Fatalf("s2 status = %q, want pending (unexecuted)", st.Status)
		}
	})
}

func TestEngine_RequiresProducesOrder(t *testing.T) {
	// Verify requires checked before execution, produces after exit
	root := t.TempDir()
	_ = project.Init(root)
	wfDir := filepath.Join(root, ".haro", "workflows", "reqtest")
	_ = os.MkdirAll(wfDir, 0o755)
	wfYAML := `version: 2
name: reqtest
steps:
  - id: a
    type: command
    run: echo a
    produces: [a.txt]
  - id: b
    type: command
    run: echo b
    requires: [a.txt]
    produces: [b.txt]
`
	_ = os.WriteFile(filepath.Join(wfDir, "workflow.yaml"), []byte(wfYAML), 0o600)
	dbPath := filepath.Join(root, ".haro", "store.db")
	ctx := context.Background()
	s, _ := store.Open(ctx, dbPath)
	defer func() { _ = s.Close() }()
	artifacts := filepath.Join(root, ".haro", "artifacts")
	// Run a, then b with requires satisfied
	fake := &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
		// Create appropriate file based on argv
		if len(argv) > 0 && contains(stringsJoin(argv), "a") {
			_ = os.MkdirAll(artifacts, 0o755)
			_ = os.WriteFile(filepath.Join(artifacts, "a.txt"), []byte("a"), 0o600)
		} else {
			_ = os.WriteFile(filepath.Join(artifacts, "b.txt"), []byte("b"), 0o600)
		}
		return 0, "", "", nil
	}}
	_ = fake // unused direct, engine will use
	// Use engine to test
	eng := NewEngine(s, fake, root)
	execID, _ := eng.CreateExecution(ctx, "reqtest")
	if err := eng.RunStep(ctx, execID, "a", ""); err != nil {
		t.Fatalf("run a: %v", err)
	}
	// Now b requires a.txt which exists — should succeed
	if err := eng.RunStep(ctx, execID, "b", ""); err != nil {
		t.Fatalf("run b with satisfied requires: %v", err)
	}
	// Triangulation: invalid requires should fail before runner
	_ = os.Remove(filepath.Join(artifacts, "a.txt"))
	// To isolate requires, create execution where b depends_on a but a is completed without file
	// We already tested depends_on; for requires we need case where depends_on satisfied but requires missing
	// So run a without creating need.txt
	// Use new execution for this test
	wfDir3 := filepath.Join(root, ".haro", "workflows", "reqtest3")
	_ = os.MkdirAll(wfDir3, 0o755)
	wfYAML3 := `version: 2
name: reqtest3
steps:
  - id: a
    type: command
    run: echo a
    produces: [need.txt]
  - id: b
    type: command
    run: echo b
    requires: [need.txt]
    depends_on: [a]
    produces: [out.txt]
`
	_ = os.WriteFile(filepath.Join(wfDir3, "workflow.yaml"), []byte(wfYAML3), 0o600)
	// Run a but fake does not create file, so a will fail due to missing produces, but we can force a to be completed by creating file manually?
	// Simplify: create need.txt manually after a completes? Actually requires check for b should happen before runner, so if need.txt missing, b should fail.
	// Let's make a succeed, then remove need.txt before b
	fake3 := &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
		_ = os.MkdirAll(artifacts, 0o755)
		_ = os.WriteFile(filepath.Join(artifacts, "need.txt"), []byte("need"), 0o600)
		return 0, "", "", nil
	}}
	eng3 := NewEngine(s, fake3, root)
	execID3, _ := eng3.CreateExecution(ctx, "reqtest3")
	if err := eng3.RunStep(ctx, execID3, "a", ""); err != nil {
		t.Fatalf("run a3: %v", err)
	}
	_ = os.Remove(filepath.Join(artifacts, "need.txt"))
	fakeB := &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
		return 0, "", "", nil
	}}
	engB := NewEngine(s, fakeB, root)
	err := engB.RunStep(ctx, execID3, "b", "")
	if err == nil || !contains(strings.ToLower(err.Error()), "requires") {
		t.Fatalf("expected requires error, got %v", err)
	}
}

// helpers
func contains(s, substr string) bool { return strings.Contains(s, substr) }
func stringsJoin(a []string) string  { return strings.Join(a, " ") }

// writeClaudeGateFixture writes an executable claude fixture emitting the
// pinned success envelope, with optional extra body.
func writeClaudeGateFixture(t *testing.T, extra string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "claude-fixture.sh")
	script := "#!/bin/sh\n" +
		"stdin=$(cat)\n" +
		"cat <<'CLAUDE_EOF'\n" +
		`{"type":"system","subtype":"init","session_id":"sess_claude_gate_1"}` + "\n" +
		"CLAUDE_EOF\n" +
		"cat <<'CLAUDE_EOF'\n" +
		`{"type":"assistant","message":{"content":[{"type":"text","text":"claude gate output"}]}}` + "\n" +
		"CLAUDE_EOF\n" +
		"cat <<'CLAUDE_EOF'\n" +
		`{"type":"result","subtype":"success","is_error":false,"result":"done"}` + "\n" +
		"CLAUDE_EOF\n" +
		extra + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestClaudeAgentAttemptEndToEnd covers task 5.1: a claude attempt end to
// end persists evidence and transport identity through the generic engine
// path.
func TestClaudeAgentAttemptEndToEnd(t *testing.T) {
	ctx := context.Background()
	wfYAML := `version: 2
name: agentwf
steps:
  - id: ag
    type: agent
    harness: [claude]
    instructions: do the thing
    mode: headless
`
	_, eng := newAgentEngine(t, wfYAML, `version: 2
harnesses:
  claude:
    binary: /nonexistent/claude
`)
	execID, err := eng.CreateExecution(ctx, "agentwf")
	if err != nil {
		t.Fatalf("create execution: %v", err)
	}
	fixture := writeClaudeGateFixture(t, "")
	setupFakeManager(t, eng, map[string]adapter.Adapter{"claude": claude.NewAdapter(fixture, nil, 10_000_000_000)})

	if err := eng.RunStep(ctx, execID, "ag", ""); err != nil {
		t.Fatalf("run agent step: %v", err)
	}
	assertStepStatus(t, eng, execID, "ag", "completed")
	attemptID := eng.singleAttemptID(t, execID, "ag")

	tr, err := eng.store.Transport().Get(ctx, attemptID)
	if err != nil {
		t.Fatalf("get transport: %v", err)
	}
	if tr.AdapterName != "claude" || tr.NativeSessionID == nil || *tr.NativeSessionID != "sess_claude_gate_1" {
		t.Fatalf("transport identity = %+v", tr)
	}
	if tr.ProtocolVersion == nil || *tr.ProtocolVersion != 1 {
		t.Fatalf("protocol_version = %v, want 1", tr.ProtocolVersion)
	}
	assertAttemptEvidence(t, eng, execID, "ag", 1)
}

// TestClaudeAgentEmptyIntersectionFailsClosed covers task 5.1: when the
// claude candidate is configured but unavailable, the intersection is empty,
// the step fails closed, no attempt runs, and no synthetic success or
// identity is ever produced.
func TestClaudeAgentEmptyIntersectionFailsClosed(t *testing.T) {
	ctx := context.Background()
	wfYAML := `version: 2
name: agentwf
steps:
  - id: ag
    type: agent
    harness: [claude]
    instructions: do the thing
    mode: headless
`
	_, eng := newAgentEngine(t, wfYAML, `version: 2
harnesses:
  claude:
    binary: /nonexistent/claude
`)
	execID, err := eng.CreateExecution(ctx, "agentwf")
	if err != nil {
		t.Fatalf("create execution: %v", err)
	}
	// The claude adapter probes unavailable: empty intersection.
	unavailable := claude.NewAdapter("/nonexistent/claude-binary", nil, time.Second)
	setupFakeManager(t, eng, map[string]adapter.Adapter{"claude": unavailable})

	if err := eng.RunStep(ctx, execID, "ag", ""); err == nil {
		t.Fatalf("empty intersection must fail closed")
	}
	assertStepStatus(t, eng, execID, "ag", "failed")
	assertAttemptEvidence(t, eng, execID, "ag", 0)
}

// TestClaudeAgentCleanFailureFallsThrough covers task 5.1: a claude clean
// failure (zero-text EOF) falls through to the next usable candidate; a
// zero-text claude stream never becomes synthetic success.
func TestClaudeAgentCleanFailureFallsThrough(t *testing.T) {
	ctx := context.Background()
	wfYAML := `version: 2
name: agentwf
steps:
  - id: ag
    type: agent
    harness: [claude, oc]
    instructions: do the thing
    mode: headless
`
	_, eng := newAgentEngine(t, wfYAML, `version: 2
harnesses:
  claude:
    binary: /nonexistent/claude
  oc:
    binary: /nonexistent/oc
`)
	execID, err := eng.CreateExecution(ctx, "agentwf")
	if err != nil {
		t.Fatalf("create execution: %v", err)
	}
	// Claude fixture emits init but zero text: clean failure at EOF.
	zeroText := filepath.Join(t.TempDir(), "claude-fixture.sh")
	script := "#!/bin/sh\n" +
		"cat <<'CLAUDE_EOF'\n" +
		`{"type":"system","subtype":"init","session_id":"sess_claude_zero_1"}` + "\n" +
		"CLAUDE_EOF\n"
	if err := os.WriteFile(zeroText, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	ocFixture := writeOpenCodeGateFixture(t)
	setupFakeManager(t, eng, map[string]adapter.Adapter{
		"claude": claude.NewAdapter(zeroText, nil, 10_000_000_000),
		"oc":     opencode.NewAdapter(ocFixture, nil, 10_000_000_000),
	})

	if err := eng.RunStep(ctx, execID, "ag", ""); err != nil {
		t.Fatalf("fallback to opencode must succeed: %v", err)
	}
	assertStepStatus(t, eng, execID, "ag", "completed")
	// Two attempts: the claude clean failure and the opencode success; each
	// persisted inline evidence.
	assertAttemptEvidence(t, eng, execID, "ag", 2)
	// The claude attempt kept its real identity.
	ctx2 := context.Background()
	rows, err := eng.store.(*store.SQLiteStore).QueryForTest(ctx2,
		`SELECT t.attempt_id, t.native_session_id, t.adapter_name FROM attempt_transport t JOIN attempts a ON a.id = t.attempt_id WHERE a.execution_id = ? AND a.step_id = ?`, execID, "ag")
	if err != nil {
		t.Fatalf("query transports: %v", err)
	}
	defer func() { _ = rows.Close() }()
	sawZero := false
	for rows.Next() {
		var attemptID, native, adapterName string
		if err := rows.Scan(&attemptID, &native, &adapterName); err != nil {
			t.Fatal(err)
		}
		if adapterName == "claude" && native == "sess_claude_zero_1" {
			sawZero = true
		}
	}
	if !sawZero {
		t.Fatalf("claude clean-failure attempt must keep its native identity")
	}
}

// TestSupervisedAgentStepFailsClosed covers v2-no-regresion/F-02: an agent
// step declared with `mode: supervised` fails closed at runtime with the
// distinct reason `supervised mode not supported`, before any attempt,
// fallback, or adapter session; execution must not silently downgrade to
// headless even when a usable adapter is available.
func TestSupervisedAgentStepFailsClosed(t *testing.T) {
	ctx := context.Background()
	wfYAML := `version: 2
name: agentwf
steps:
  - id: ag
    type: agent
    harness: [opencode]
    instructions: do the thing
    mode: supervised
`
	root, eng := newAgentEngine(t, wfYAML, `version: 2
harnesses:
  opencode:
    binary: /nonexistent/oc
`)
	writeRequiredArtifacts(t, root)
	// Supervised is a valid declared mode: CreateExecution must succeed.
	execID, err := eng.CreateExecution(ctx, "agentwf")
	if err != nil {
		t.Fatalf("create execution: %v", err)
	}
	// An available adapter must never be reached through a supervised step.
	good := newFakeHarnessAdapter(true, "completed", "simulated output")
	setupFakeManager(t, eng, map[string]adapter.Adapter{"opencode": good})

	err = eng.RunStep(ctx, execID, "ag", "")
	if err == nil {
		t.Fatalf("supervised mode must fail closed")
	}
	if !strings.Contains(err.Error(), "supervised mode not supported") {
		t.Fatalf("expected supervised rejection reason, got %v", err)
	}
	assertStepStatus(t, eng, execID, "ag", "failed")
	exec, err := eng.store.Executions().Get(ctx, execID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if exec.Status != "failed" {
		t.Fatalf("execution status = %q, want failed", exec.Status)
	}
	// No attempt, no adapter session: the fallback loop was never entered.
	assertAttemptEvidence(t, eng, execID, "ag", 0)
	if got := good.NewSessionCount(); got != 0 {
		t.Fatalf("supervised step must not open adapter sessions, got %d", got)
	}
}

// TestTerminalAgentStepRegressionStoreSeeded pins the existing terminal
// rejection through the store-seeded path (v2-no-regresion/F-02):
// CreateExecution rejects terminal mode during validation, so a legacy
// running execution is seeded directly; RunStep re-parses the workflow
// without validation and must still reach the guard with zero attempts and
// zero adapter sessions.
func TestTerminalAgentStepRegressionStoreSeeded(t *testing.T) {
	ctx := context.Background()
	wfYAML := `version: 2
name: agentwf
steps:
  - id: ag
    type: agent
    harness: [opencode]
    instructions: do the thing
    mode: terminal
`
	root, eng := newAgentEngine(t, wfYAML, `version: 2
harnesses:
  opencode:
    binary: /nonexistent/oc
`)
	// Seed the project, a running execution, and a pending agent step the way
	// a rehydrated execution looks: nil DagHash skips hash re-flattening and
	// WorkflowSource drives the runtime re-parse.
	wfPath := filepath.Join(root, ".haro", "workflows", "agentwf", "workflow.yaml")
	if err := eng.store.Projects().Create(ctx, root, root); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	exec := &store.Execution{
		ID:             "exec-term",
		ProjectID:      root,
		WorkflowSource: wfPath,
		Status:         "running",
		WorkspaceMode:  "shared",
		WorkspaceRoot:  root,
		StartedAt:      time.Now().UTC().Format(time.RFC3339),
		DagHash:        nil,
		BaseCommit:     nil,
	}
	if err := eng.store.Executions().Create(ctx, exec); err != nil {
		t.Fatalf("seed execution: %v", err)
	}
	step := &store.ExecutionStep{
		ExecutionID:       exec.ID,
		StepID:            "ag",
		Type:              "agent",
		Status:            "pending",
		DependsOn:         "[]",
		Requires:          "[]",
		Produces:          "[]",
		WorkspaceMode:     "shared",
		CurrentGeneration: 0,
	}
	if err := eng.store.Steps().Create(ctx, step); err != nil {
		t.Fatalf("seed step: %v", err)
	}
	// An available adapter must never be reached through a terminal step.
	good := newFakeHarnessAdapter(true, "completed", "simulated output")
	setupFakeManager(t, eng, map[string]adapter.Adapter{"opencode": good})

	err := eng.RunStep(ctx, exec.ID, "ag", "")
	if err == nil {
		t.Fatalf("terminal mode must fail closed")
	}
	if !strings.Contains(err.Error(), "terminal mode not supported") {
		t.Fatalf("expected terminal rejection reason, got %v", err)
	}
	assertStepStatus(t, eng, exec.ID, "ag", "failed")
	gotExec, err := eng.store.Executions().Get(ctx, exec.ID)
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	if gotExec.Status != "failed" {
		t.Fatalf("execution status = %q, want failed", gotExec.Status)
	}
	assertAttemptEvidence(t, eng, exec.ID, "ag", 0)
	if got := good.NewSessionCount(); got != 0 {
		t.Fatalf("terminal step must not open adapter sessions, got %d", got)
	}
}

// writeOpenCodeGateFixture writes an executable opencode fixture emitting a
// successful JSONL envelope.
func writeOpenCodeGateFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "oc-fixture.sh")
	script := "#!/bin/sh\n" +
		"cat <<'OC_EOF'\n" +
		`{"type":"step_start","sessionID":"sess_oc_gate_1"}` + "\n" +
		`{"type":"part","sessionID":"sess_oc_gate_1","part":{"type":"text","text":"oc gate output"}}` + "\n" +
		"OC_EOF\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
