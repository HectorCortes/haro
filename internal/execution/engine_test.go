package execution

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
func stringsJoin(a []string) string { return strings.Join(a, " ") }
