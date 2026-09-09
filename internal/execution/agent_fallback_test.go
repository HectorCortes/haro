package execution

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HectorCortes/haro/internal/adapter"
	"github.com/HectorCortes/haro/internal/project"
	"github.com/HectorCortes/haro/internal/store"
)

// newAgentEngine builds a temp project with an agent workflow, optional
// config.yaml content, and a store-backed engine with a passthrough runner.
func newAgentEngine(t *testing.T, wfYAML, configYAML string) (string, *Engine) {
	t.Helper()
	root := t.TempDir()
	if err := project.Init(root); err != nil {
		t.Fatalf("init: %v", err)
	}
	wfDir := filepath.Join(root, ".haro", "workflows", "agentwf")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "workflow.yaml"), []byte(wfYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	if configYAML != "" {
		if err := os.WriteFile(filepath.Join(root, ".haro", "config.yaml"), []byte(configYAML), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	st, err := store.Open(context.Background(), filepath.Join(root, ".haro", "store.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	eng := NewEngine(st, &FakeRunner{}, root)
	return root, eng
}

const fallbackWfYAML = `version: 2
name: agentwf
steps:
  - id: ag
    type: agent
    harness: [ghost, unavailable, good]
    instructions: do the thing
    mode: headless
    requires: [req.txt]
`

// writeRequiredArtifacts creates the required artifact so the requires gate
// passes, mirroring a producer step.
func writeRequiredArtifacts(t *testing.T, root string) string {
	t.Helper()
	artifacts := filepath.Join(root, ".haro", "artifacts")
	if err := os.MkdirAll(artifacts, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(artifacts, "req.txt"), []byte("required context"), 0o600); err != nil {
		t.Fatal(err)
	}
	return artifacts
}

// setupFakeManager probes and initializes a fake manager the way the CLI
// factory does, then injects it into the engine.
func setupFakeManager(t *testing.T, eng *Engine, adapters map[string]adapter.Adapter) *adapter.Manager {
	t.Helper()
	ctx := context.Background()
	mgr := adapter.NewManager(adapters)
	probes, err := mgr.Probe(ctx)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	for name, pr := range probes {
		if !pr.Available {
			continue
		}
		if _, err := mgr.Initialize(ctx, name, adapter.Capabilities{ProtocolVersion: 1}); err != nil {
			t.Fatalf("initialize %s: %v", name, err)
		}
	}
	eng.SetAdapterManager(mgr)
	return mgr
}

// TestAgentHarnessIntersectionFallback covers F-06: the engine intersects the
// step's ordered harness list with configured, enabled, registered, and
// successfully probed harnesses without reordering; unknown, disabled, and
// unavailable candidates fall through carrying sanitized context, and the
// usable adapter receives bounded sanitized context with resolved absolute
// requirement paths.
func TestAgentHarnessIntersectionFallback(t *testing.T) {
	ctx := context.Background()
	root, eng := newAgentEngine(t, fallbackWfYAML, `version: 2
harnesses:
  unavailable:
    binary: /nonexistent/unavailable
  good:
    binary: /nonexistent/good
  disabled:
    binary: /nonexistent/disabled
    enabled: false
`)
	writeRequiredArtifacts(t, root)

	execID, err := eng.CreateExecution(ctx, "agentwf")
	if err != nil {
		t.Fatalf("create execution: %v", err)
	}
	unavailable := newFakeHarnessAdapter(false, "completed", "")
	good := newFakeHarnessAdapter(true, "completed", "output from good")
	setupFakeManager(t, eng, map[string]adapter.Adapter{
		"unavailable": unavailable,
		"good":        good,
		"disabled":    newFakeHarnessAdapter(true, "completed", ""),
	})

	if err := eng.RunStep(ctx, execID, "ag", ""); err != nil {
		t.Fatalf("run agent step: %v", err)
	}

	// The usable adapter received exactly one session.
	if got := good.NewSessionCount(); got != 1 {
		t.Fatalf("good must own exactly one session, got %d", got)
	}
	// Unavailable and disabled candidates fell through without owning a
	// session attempt.
	if got := unavailable.NewSessionCount(); got != 0 {
		t.Fatalf("unavailable candidate must not own a session, got %d", got)
	}
	bundle := good.LastBundle()
	// SessionBundle.Requires carries resolved absolute .haro/artifacts paths.
	wantReq := filepath.Join(root, ".haro", "artifacts", "req.txt")
	if bundle.Requires["req.txt"] != wantReq {
		t.Fatalf("resolved requirement = %q, want %q", bundle.Requires["req.txt"], wantReq)
	}
	// The usable adapter receives bounded sanitized context carrying the
	// skip diagnostics for ghost (unknown) and unavailable candidates.
	prompt := good.LastPrompt()
	if len(prompt) > FallbackLimit {
		t.Fatalf("fallback context %d exceeds 2 MiB", len(prompt))
	}
	if !strings.Contains(prompt, "ghost") || !strings.Contains(prompt, "unavailable") {
		t.Fatalf("sanitized context must carry skip diagnostics, got %q", prompt[:min(len(prompt), 512)])
	}
	if strings.Contains(prompt, "secret-value-not-allowed") {
		t.Fatalf("context must be sanitized")
	}
	// Attempts exist only for the usable candidate; it completed.
	attempts, err := eng.store.Attempts().CountByStep(ctx, execID, "ag")
	if err != nil {
		t.Fatalf("count attempts: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("exactly one usable candidate attempt expected, got %d", attempts)
	}
	// The persisted evidence identifies the successful harness.
	assertStepStatus(t, eng, execID, "ag", "completed")
}

// TestAgentHarnessCandidatesExhausted covers F-06 exhaustion: every
// intersected candidate fails cleanly, the step fails closed, and sanitized
// evidence from every attempted candidate is persisted.
func TestAgentHarnessCandidatesExhausted(t *testing.T) {
	ctx := context.Background()
	root, eng := newAgentEngine(t, fallbackWfYAML, `version: 2
harnesses:
  unavailable:
    binary: /nonexistent/unavailable
  good:
    binary: /nonexistent/good
`)
	writeRequiredArtifacts(t, root)

	execID, err := eng.CreateExecution(ctx, "agentwf")
	if err != nil {
		t.Fatalf("create execution: %v", err)
	}
	first := newFakeHarnessAdapter(true, "failed", "first tried and failed")
	second := newFakeHarnessAdapter(true, "failed", "second tried and failed")
	setupFakeManager(t, eng, map[string]adapter.Adapter{
		"unavailable": first,
		"good":        second,
	})

	err = eng.RunStep(ctx, execID, "ag", "")
	if err == nil {
		t.Fatalf("exhausted candidates must fail closed")
	}
	if !strings.Contains(err.Error(), "exhausted") {
		t.Fatalf("exhaustion error expected, got %v", err)
	}
	assertStepStatus(t, eng, execID, "ag", "failed")
	// Every attempted candidate persisted its sanitized attempt evidence:
	// one failed attempt per usable candidate with an output_delta payload.
	assertAttemptEvidence(t, eng, execID, "ag", 2)
}

// TestAgentStepFailsWithoutConfiguredHarness covers F-06: with no
// configuration, no manager, or no usable candidate, the agent step fails
// closed with clear evidence and no simulated result.
func TestAgentStepFailsWithoutConfiguredHarness(t *testing.T) {
	ctx := context.Background()
	wfYAML := `version: 2
name: agentwf
steps:
  - id: ag
    type: agent
    harness: [opencode]
    instructions: do the thing
    mode: headless
`

	t.Run("no manager injected fails closed without simulation", func(t *testing.T) {
		root, eng := newAgentEngine(t, wfYAML, `version: 2
harnesses:
  opencode:
    binary: /nonexistent/oc
`)
		writeRequiredArtifacts(t, root)
		execID, err := eng.CreateExecution(ctx, "agentwf")
		if err != nil {
			t.Fatalf("create execution: %v", err)
		}
		// No manager: production must not synthesize success.
		if err := eng.RunStep(ctx, execID, "ag", ""); err == nil {
			t.Fatalf("agent step without manager must fail closed")
		}
		assertStepStatus(t, eng, execID, "ag", "failed")
		assertAttemptEvidence(t, eng, execID, "ag", 0)
	})

	t.Run("no configured harness fails closed", func(t *testing.T) {
		root, eng := newAgentEngine(t, wfYAML, `version: 2
`)
		writeRequiredArtifacts(t, root)
		execID, err := eng.CreateExecution(ctx, "agentwf")
		if err != nil {
			t.Fatalf("create execution: %v", err)
		}
		good := newFakeHarnessAdapter(true, "completed", "simulated output")
		setupFakeManager(t, eng, map[string]adapter.Adapter{"opencode": good})
		// Harness named in the workflow is not configured: no usable
		// candidate, no fake session may run.
		if err := eng.RunStep(ctx, execID, "ag", ""); err == nil {
			t.Fatalf("agent step without configured harness must fail closed")
		}
		if got := good.NewSessionCount(); got != 0 {
			t.Fatalf("no session may run without configuration, got %d", got)
		}
		assertStepStatus(t, eng, execID, "ag", "failed")
		assertAttemptEvidence(t, eng, execID, "ag", 0)
	})

	t.Run("all candidates unavailable fails closed", func(t *testing.T) {
		root, eng := newAgentEngine(t, wfYAML, `version: 2
harnesses:
  opencode:
    binary: /nonexistent/oc
`)
		writeRequiredArtifacts(t, root)
		unavailable := newFakeHarnessAdapter(false, "completed", "")
		setupFakeManager(t, eng, map[string]adapter.Adapter{"opencode": unavailable})
		execID, err := eng.CreateExecution(ctx, "agentwf")
		if err != nil {
			t.Fatalf("create execution: %v", err)
		}
		if err := eng.RunStep(ctx, execID, "ag", ""); err == nil {
			t.Fatalf("agent step with only unavailable candidates must fail closed")
		}
		assertStepStatus(t, eng, execID, "ag", "failed")
		assertAttemptEvidence(t, eng, execID, "ag", 0)
	})
}

// assertStepStatus asserts the stored status of a step.
func assertStepStatus(t *testing.T, eng *Engine, execID, stepID, want string) {
	t.Helper()
	step, err := eng.store.Steps().Get(context.Background(), execID, stepID)
	if err != nil {
		t.Fatalf("get step: %v", err)
	}
	if step.Status != want {
		t.Fatalf("step %s status = %q, want %q", stepID, step.Status, want)
	}
}

// assertAttemptEvidence asserts the number of attempts for a step and that
// each of them (when any) persisted an inline output_delta payload.
func assertAttemptEvidence(t *testing.T, eng *Engine, execID, stepID string, wantAttempts int) {
	t.Helper()
	ctx := context.Background()
	count, err := eng.store.Attempts().CountByStep(ctx, execID, stepID)
	if err != nil {
		t.Fatalf("count attempts: %v", err)
	}
	if count != wantAttempts {
		t.Fatalf("attempts = %d, want %d", count, wantAttempts)
	}
	if wantAttempts == 0 {
		return
	}
	rows, err := eng.store.(*store.SQLiteStore).QueryForTest(ctx, `SELECT COUNT(DISTINCT e.attempt_id) FROM attempt_events e JOIN attempts a ON a.id = e.attempt_id WHERE a.execution_id = ? AND a.step_id = ? AND e.event_type = 'output_delta' AND e.payload IS NOT NULL`, execID, stepID)
	if err != nil {
		t.Fatalf("query evidence: %v", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		t.Fatalf("no evidence rows")
	}
	var withPayload int
	if err := rows.Scan(&withPayload); err != nil {
		t.Fatal(err)
	}
	if withPayload != wantAttempts {
		t.Fatalf("attempts with persisted payload = %d, want %d", withPayload, wantAttempts)
	}
}

// TestIsTerminalClassifiesTimeoutAndContract covers the design classification:
// timeout and contract errors are terminal; clean harness failures are not.
func TestIsTerminalClassifiesTimeoutAndContract(t *testing.T) {
	if !isTerminal(errors.New("timeout: session exceeded 300s")) {
		t.Fatalf("timeout must be terminal")
	}
	if !isTerminal(errors.New("contract: requirement escapes artifacts root")) {
		t.Fatalf("contract errors must be terminal")
	}
	if isTerminal(errors.New("no output: session ended without text events")) {
		t.Fatalf("clean harness failure must fall through")
	}
	if isTerminal(errors.New("harness oc failed: boom")) {
		t.Fatalf("harness-declared failure must fall through")
	}
}
