package execution

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/HectorCortes/haro/internal/adapter"
	"github.com/HectorCortes/haro/internal/project"
	"github.com/HectorCortes/haro/internal/store"
)

func TestReopenSkip(t *testing.T) {
	root := t.TempDir()
	_ = project.Init(root)
	wfDir := filepath.Join(root, ".haro", "workflows", "cascade")
	_ = os.MkdirAll(wfDir, 0o755)
	wfYAML := `version: 2
name: cascade
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
	ctx := context.Background()
	dbPath := filepath.Join(root, ".haro", "store.db")
	s, _ := store.Open(ctx, dbPath)
	defer func() { _ = s.Close() }()
	artifacts := filepath.Join(root, ".haro", "artifacts")
	// Run all three steps to completed
	eng := NewEngine(s, &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
		for _, prod := range []string{"s1.txt", "s2.txt", "s3.txt"} {
			p := filepath.Join(artifacts, prod)
			_ = os.MkdirAll(filepath.Dir(p), 0o755)
			// Only create the file corresponding to current step? For simplicity create all? But we need to create correct one.
			// We'll infer from argv: echo s1 -> s1.txt, etc.
			if len(argv) > 1 && argv[1] == "s1" && prod == "s1.txt" {
				_ = os.WriteFile(p, []byte("s1"), 0o600)
			}
			if len(argv) > 1 && argv[1] == "s2" && prod == "s2.txt" {
				_ = os.WriteFile(p, []byte("s2"), 0o600)
			}
			if len(argv) > 1 && argv[1] == "s3" && prod == "s3.txt" {
				_ = os.WriteFile(p, []byte("s3"), 0o600)
			}
		}
		return 0, "", "", nil
	}}, root)
	execID, _ := eng.CreateExecution(ctx, "cascade")
	if err := eng.RunStep(ctx, execID, "s1", ""); err != nil {
		t.Fatalf("run s1: %v", err)
	}
	if err := eng.RunStep(ctx, execID, "s2", ""); err != nil {
		t.Fatalf("run s2: %v", err)
	}
	if err := eng.RunStep(ctx, execID, "s3", ""); err != nil {
		t.Fatalf("run s3: %v", err)
	}
	// Verify all completed
	for _, sid := range []string{"s1", "s2", "s3"} {
		st, _ := s.Steps().Get(ctx, execID, sid)
		if st.Status != "completed" {
			t.Fatalf("%s status = %q, want completed", sid, st.Status)
		}
	}
	// Verify files exist
	for _, f := range []string{"s1.txt", "s2.txt", "s3.txt"} {
		if _, err := os.Stat(filepath.Join(artifacts, f)); err != nil {
			t.Fatalf("file %s should exist: %v", f, err)
		}
	}
	// Capture generation counts before reopen
	gensBefore, _ := s.Generations().ListByStep(ctx, execID, "s1")
	if len(gensBefore) != 1 {
		t.Fatalf("gens before = %d", len(gensBefore))
	}
	// Reopen first with cascade
	if err := eng.ReopenStep(ctx, execID, "s1", true, ""); err != nil {
		t.Fatalf("reopen s1 cascade: %v", err)
	}
	// Check generations invalidated for s1, files remain but invalidated
	gensAfter, _ := s.Generations().ListByStep(ctx, execID, "s1")
	if len(gensAfter) != 1 || gensAfter[0].InvalidatedAt == nil {
		t.Fatalf("s1 generation should be invalidated")
	}
	if gensAfter[0].InvalidatedByStep == nil || *gensAfter[0].InvalidatedByStep != "s1" {
		t.Fatalf("invalidated_by_step wrong: %v", gensAfter[0].InvalidatedByStep)
	}
	// Files should still exist
	for _, f := range []string{"s1.txt", "s2.txt", "s3.txt"} {
		if _, err := os.Stat(filepath.Join(artifacts, f)); err != nil {
			t.Fatalf("file %s should remain after reopen: %v", f, err)
		}
	}
	// Descendants should be pending
	for _, sid := range []string{"s1", "s2", "s3"} {
		st, _ := s.Steps().Get(ctx, execID, sid)
		if st.Status != "pending" {
			t.Fatalf("%s status after reopen = %q, want pending", sid, st.Status)
		}
	}
	// Generations for s2,s3 should be invalidated as well (descendants)
	gensS2, _ := s.Generations().ListByStep(ctx, execID, "s2")
	if len(gensS2) != 1 || gensS2[0].InvalidatedAt == nil {
		t.Fatalf("s2 generation should be invalidated after cascade")
	}
	// Now requires for s2 should fail because s1.txt invalidated
	fakeFail := &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
		_ = os.WriteFile(filepath.Join(artifacts, "s2.txt"), []byte("s2"), 0o600)
		return 0, "", "", nil
	}}
	eng2 := NewEngine(s, fakeFail, root)
	err := eng2.RunStep(ctx, execID, "s2", "")
	if err == nil {
		t.Fatalf("expected requires invalid after reopen, got nil")
	}
	// Audit: step_transition_events should have entries for reopen
	trans, _ := s.Events().ListTransitions(ctx, execID, "s1")
	found := false
	for _, tr := range trans {
		if tr.ToStatus == "pending" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no transition to pending audited for s1")
	}

	// Skip tests
	t.Run("skip virgin", func(t *testing.T) {
		root2 := t.TempDir()
		_ = project.Init(root2)
		wfDir2 := filepath.Join(root2, ".haro", "workflows", "skiptest")
		_ = os.MkdirAll(wfDir2, 0o755)
		wfYAML2 := `version: 2
name: skiptest
steps:
  - id: a
    type: command
    run: echo a
  - id: b
    type: command
    run: echo b
`
		_ = os.WriteFile(filepath.Join(wfDir2, "workflow.yaml"), []byte(wfYAML2), 0o600)
		dbPath2 := filepath.Join(root2, ".haro", "store.db")
		s2, _ := store.Open(ctx, dbPath2)
		defer func() { _ = s2.Close() }()
		eng2 := NewEngine(s2, &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
			return 0, "", "", nil
		}}, root2)
		execID2, _ := eng2.CreateExecution(ctx, "skiptest")
		// Skip virgin pending step a with reason
		if err := eng2.SkipStep(ctx, execID2, "a", "not needed"); err != nil {
			t.Fatalf("skip virgin a: %v", err)
		}
		st, _ := s2.Steps().Get(ctx, execID2, "a")
		if st.Status != "skipped" {
			t.Fatalf("a status = %q, want skipped", st.Status)
		}
		// Audit skip
		trans, _ := s2.Events().ListTransitions(ctx, execID2, "a")
		foundSkip := false
		for _, tr := range trans {
			if tr.ToStatus == "skipped" {
				foundSkip = true
			}
		}
		if !foundSkip {
			t.Fatalf("skip not audited")
		}
		// Try to skip already skipped -> should be idempotent, no extra event
		beforeCount := len(trans)
		_ = eng2.SkipStep(ctx, execID2, "a", "again")
		after, _ := s2.Events().ListTransitions(ctx, execID2, "a")
		if len(after) != beforeCount {
			t.Fatalf("skip idempotent failed: before %d after %d", beforeCount, len(after))
		}
		// Try to skip without reason -> should fail
		if err := eng2.SkipStep(ctx, execID2, "b", ""); err == nil {
			t.Fatalf("skip without reason should fail")
		}
		// Try to skip after attempt -> create attempt for b then try skip
		artifacts2 := filepath.Join(root2, ".haro", "artifacts")
		fakeB := &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
			return 0, "", "", nil
		}}
		engB := NewEngine(s2, fakeB, root2)
		_ = engB.RunStep(ctx, execID2, "b", "")
		// Now b is completed, skip should fail (not pending, has attempts)
		if err := engB.SkipStep(ctx, execID2, "b", "reason"); err == nil {
			t.Fatalf("skip after attempt should fail")
		}
		_ = artifacts2
	})
}

// TestReopenRequiresCurrentGenerationRecovery proves v2-no-regresion/F-07:
// a requires check considers only each producer step's latest current
// generation. Right after reopen (before the producer reruns) the
// downstream step stays blocked; once the producer reruns and its latest
// generation is valid, the older invalidated generation no longer blocks
// and the downstream step succeeds. The agent path uses the same predicate.
func TestReopenRequiresCurrentGenerationRecovery(t *testing.T) {
	root := t.TempDir()
	_ = project.Init(root)
	wfDir := filepath.Join(root, ".haro", "workflows", "recovery")
	_ = os.MkdirAll(wfDir, 0o755)
	wfYAML := `version: 2
name: recovery
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
	ctx := context.Background()
	s, _ := store.Open(ctx, filepath.Join(root, ".haro", "store.db"))
	defer func() { _ = s.Close() }()
	artifacts := filepath.Join(root, ".haro", "artifacts")
	eng := NewEngine(s, &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
		_ = os.MkdirAll(artifacts, 0o755)
		if len(argv) > 1 {
			_ = os.WriteFile(filepath.Join(artifacts, argv[1]+".txt"), []byte(argv[1]), 0o600)
		}
		return 0, "", "", nil
	}}, root)
	execID, _ := eng.CreateExecution(ctx, "recovery")
	for _, sid := range []string{"s1", "s2", "s3"} {
		if err := eng.RunStep(ctx, execID, sid, ""); err != nil {
			t.Fatalf("run %s: %v", sid, err)
		}
	}
	// Reopen the producer with cascade: generations invalidated, descendants pending.
	if err := eng.ReopenStep(ctx, execID, "s1", true, ""); err != nil {
		t.Fatalf("reopen s1: %v", err)
	}
	// Negative path: right after reopen, before the producer reruns, the
	// downstream step must stay blocked.
	if err := eng.RunStep(ctx, execID, "s2", ""); err == nil {
		t.Fatalf("downstream must stay blocked right after reopen (before producer rerun)")
	}
	// Producer rerun: creates a new valid generation (N+1); the old one stays
	// invalidated in history.
	if err := eng.RunStep(ctx, execID, "s1", ""); err != nil {
		t.Fatalf("producer rerun: %v", err)
	}
	gens, _ := s.Generations().ListByStep(ctx, execID, "s1")
	if len(gens) != 2 {
		t.Fatalf("producer generations = %d, want 2", len(gens))
	}
	if gens[0].InvalidatedAt == nil || gens[1].InvalidatedAt != nil {
		t.Fatalf("latest generation must be valid with old one invalidated: %+v", gens)
	}
	// Downstream must now run: only the latest valid generation gates requires.
	if err := eng.RunStep(ctx, execID, "s2", ""); err != nil {
		t.Fatalf("downstream must succeed after producer rerun, got: %v", err)
	}
	if err := eng.RunStep(ctx, execID, "s3", ""); err != nil {
		t.Fatalf("transitive downstream must succeed, got: %v", err)
	}

	t.Run("agent requires uses the same latest-generation predicate", func(t *testing.T) {
		root2 := t.TempDir()
		_ = project.Init(root2)
		wfDir2 := filepath.Join(root2, ".haro", "workflows", "agentrec")
		_ = os.MkdirAll(wfDir2, 0o755)
		wfYAML2 := `version: 2
name: agentrec
steps:
  - id: p
    type: command
    run: echo p
    produces: [p.txt]
  - id: a
    type: agent
    harness: [opencode, claudecode]
    instructions: consume
    mode: headless
    requires: [p.txt]
`
		_ = os.WriteFile(filepath.Join(wfDir2, "workflow.yaml"), []byte(wfYAML2), 0o600)
		s2, _ := store.Open(ctx, filepath.Join(root2, ".haro", "store.db"))
		defer func() { _ = s2.Close() }()
		artifacts2 := filepath.Join(root2, ".haro", "artifacts")
		eng2 := NewEngine(s2, &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
			_ = os.MkdirAll(artifacts2, 0o755)
			_ = os.WriteFile(filepath.Join(artifacts2, "p.txt"), []byte("p"), 0o600)
			return 0, "", "", nil
		}}, root2)
		// Inject a fake manager: production no longer simulates agent runs.
		cfgYAML2 := "version: 2\nharnesses:\n  opencode:\n    binary: /nonexistent/first\n  claudecode:\n    binary: /nonexistent/second\n"
		if err := os.WriteFile(filepath.Join(root2, ".haro", "config.yaml"), []byte(cfgYAML2), 0o600); err != nil {
			t.Fatalf("write config: %v", err)
		}
		setupFakeManager(t, eng2, map[string]adapter.Adapter{
			"opencode":   newFakeHarnessAdapter(true, "completed", "consume output"),
			"claudecode": newFakeHarnessAdapter(true, "completed", "consume output"),
		})
		execID2, _ := eng2.CreateExecution(ctx, "agentrec")
		if err := eng2.RunStep(ctx, execID2, "p", ""); err != nil {
			t.Fatalf("run producer: %v", err)
		}
		if err := eng2.RunStep(ctx, execID2, "a", ""); err != nil {
			t.Fatalf("run agent: %v", err)
		}
		// Reopen the producer without cascade: its latest generation becomes
		// invalid while the agent step is completed.
		if err := eng2.ReopenStep(ctx, execID2, "p", false, ""); err != nil {
			t.Fatalf("reopen producer: %v", err)
		}
		// Agent feedback reconstruction must be blocked while the producer's
		// latest generation is invalid.
		if err := eng2.RunStep(ctx, execID2, "a", "retry"); err == nil {
			t.Fatalf("agent requires must block while producer latest generation is invalid")
		}
		// Producer rerun recovers: latest generation valid.
		if err := eng2.RunStep(ctx, execID2, "p", ""); err != nil {
			t.Fatalf("producer rerun: %v", err)
		}
		if err := eng2.RunStep(ctx, execID2, "a", "retry"); err != nil {
			t.Fatalf("agent must succeed after producer rerun, got: %v", err)
		}
	})
}

func TestStateMachine(t *testing.T) {
	root := t.TempDir()
	_ = project.Init(root)
	wfDir := filepath.Join(root, ".haro", "workflows", "smt")
	_ = os.MkdirAll(wfDir, 0o755)
	wfYAML := `version: 2
name: smt
steps:
  - id: s1
    type: command
    run: echo s1
`
	_ = os.WriteFile(filepath.Join(wfDir, "workflow.yaml"), []byte(wfYAML), 0o600)
	ctx := context.Background()
	dbPath := filepath.Join(root, ".haro", "store.db")
	s, _ := store.Open(ctx, dbPath)
	defer func() { _ = s.Close() }()
	eng := NewEngine(s, &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
		return 0, "", "", nil
	}}, root)
	execID, _ := eng.CreateExecution(ctx, "smt")
	// Helper to count transitions
	countTrans := func(step string) int {
		tr, _ := s.Events().ListTransitions(ctx, execID, step)
		return len(tr)
	}
	// pending -> running -> completed
	st, _ := s.Steps().Get(ctx, execID, "s1")
	if st.Status != "pending" {
		t.Fatalf("initial pending")
	}
	if err := eng.TransitionForTest(ctx, execID, "s1", "running"); err != nil {
		t.Fatalf("pending->running: %v", err)
	}
	if err := eng.TransitionForTest(ctx, execID, "s1", "running"); err != nil {
		t.Fatalf("repeat pending->running should be idempotent (no error)")
	}
	if countTrans("s1") != 1 {
		t.Fatalf("repeat should add no event, got %d", countTrans("s1"))
	}
	if err := eng.TransitionForTest(ctx, execID, "s1", "completed"); err != nil {
		t.Fatalf("running->completed: %v", err)
	}
	if err := eng.TransitionForTest(ctx, execID, "s1", "completed"); err != nil {
		t.Fatalf("repeat should be idempotent")
	}
	if countTrans("s1") != 2 {
		t.Fatalf("trans count after completed = %d, want 2", countTrans("s1"))
	}
	// completed -> pending (reopen)
	if err := eng.TransitionForTest(ctx, execID, "s1", "pending"); err != nil {
		t.Fatalf("completed->pending: %v", err)
	}
	// pending -> skipped
	if err := eng.TransitionForTest(ctx, execID, "s1", "skipped"); err != nil {
		t.Fatalf("pending->skipped: %v", err)
	}
	// repeat skipped -> no extra event
	before := countTrans("s1")
	_ = eng.TransitionForTest(ctx, execID, "s1", "skipped")
	if countTrans("s1") != before {
		t.Fatalf("skipped repeat should be idempotent")
	}
	// forbidden: skipped -> running should fail
	if err := eng.TransitionForTest(ctx, execID, "s1", "running"); err == nil {
		t.Fatalf("skipped->running should be forbidden")
	}
	// Verify forbidden state remains skipped
	st2, _ := s.Steps().Get(ctx, execID, "s1")
	if st2.Status != "skipped" {
		t.Fatalf("status after forbidden = %q, want skipped", st2.Status)
	}
	// pending -> running -> failed -> pending
	root2 := t.TempDir()
	_ = project.Init(root2)
	wfDir2 := filepath.Join(root2, ".haro", "workflows", "smt2")
	_ = os.MkdirAll(wfDir2, 0o755)
	_ = os.WriteFile(filepath.Join(wfDir2, "workflow.yaml"), []byte(wfYAML), 0o600)
	dbPath2 := filepath.Join(root2, ".haro", "store.db")
	s2, _ := store.Open(ctx, dbPath2)
	defer func() { _ = s2.Close() }()
	eng2 := NewEngine(s2, &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
		return 0, "", "", nil
	}}, root2)
	execID2, _ := eng2.CreateExecution(ctx, "smt2")
	_ = eng2.TransitionForTest(ctx, execID2, "s1", "running")
	_ = eng2.TransitionForTest(ctx, execID2, "s1", "failed")
	_ = eng2.TransitionForTest(ctx, execID2, "s1", "pending")
	st3, _ := s2.Steps().Get(ctx, execID2, "s1")
	if st3.Status != "pending" {
		t.Fatalf("failed->pending status = %q", st3.Status)
	}
}
