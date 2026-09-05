package execution

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HectorCortes/haro/internal/store"
	"github.com/HectorCortes/haro/internal/workflow"
	"github.com/HectorCortes/haro/internal/worktree"
)

func TestCreateExecutionWorkspace(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	// init project and git
	if err := os.MkdirAll(filepath.Join(dir, ".haro", "workflows", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	// workflow with shared at workflow level
	yamlShared := "version: 2\nname: demo\nworkspace:\n  mode: shared\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n  - id: s2\n    type: command\n    run: echo hi\n    workspace:\n      mode: isolated\n"
	if err := os.WriteFile(filepath.Join(dir, ".haro", "workflows", "demo", "workflow.yaml"), []byte(yamlShared), 0o600); err != nil {
		t.Fatal(err)
	}
	// store
	s, err := store.Open(ctx, filepath.Join(dir, ".haro", "store.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func(){ _ = s.Close() }()
	fakeWT := &worktree.FakeManager{}
	eng := NewEngine(s, &FakeRunner{}, dir)
	eng.SetWorktreeManager(fakeWT)

	id, err := eng.CreateExecution(ctx, "demo")
	if err != nil {
		t.Fatalf("create execution: %v", err)
	}
	exec, err := s.Executions().Get(ctx, id)
	if err != nil {
		t.Fatalf("get exec: %v", err)
	}
	// workflow shared => execution should be shared, workspace_root == dir, no worktree created for execution shared?
	// But design says worktree per execution when isolated. Since workflow shared, execution shared => root.
	if exec.WorkspaceMode != "shared" {
		t.Fatalf("expected execution shared, got %q", exec.WorkspaceMode)
	}
	if exec.WorkspaceRoot != dir {
		t.Fatalf("shared root should be dir, got %q", exec.WorkspaceRoot)
	}
	steps, _ := s.Steps().List(ctx, id)
	var s1Mode, s2Mode string
	for _, st := range steps {
		if st.StepID == "s1" {
			s1Mode = st.WorkspaceMode
		}
		if st.StepID == "s2" {
			s2Mode = st.WorkspaceMode
		}
	}
	if s1Mode != "shared" {
		t.Fatalf("s1 should inherit shared, got %q", s1Mode)
	}
	if s2Mode != "isolated" {
		t.Fatalf("s2 should override isolated, got %q", s2Mode)
	}
	// fake should not have created worktree for shared execution? Check
	hasCreate := false
	for _, c := range fakeWT.Calls {
		if c.Op == "create" {
			hasCreate = true
		}
	}
	if hasCreate {
		t.Fatalf("shared execution should not create worktree, but fake recorded create")
	}

	// now test isolated default creates worktree
	dir2 := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir2, ".haro", "workflows", "iso"), 0o755); err != nil {
		t.Fatal(err)
	}
	yamlIso := "version: 2\nname: iso\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n"
	if err := os.WriteFile(filepath.Join(dir2, ".haro", "workflows", "iso", "workflow.yaml"), []byte(yamlIso), 0o600); err != nil {
		t.Fatal(err)
	}
	s2, err := store.Open(ctx, filepath.Join(dir2, ".haro", "store.db"))
	if err != nil {
		t.Fatalf("open s2: %v", err)
	}
	defer func(){ _ = s2.Close() }()
	fake2 := &worktree.FakeManager{}
	eng2 := NewEngine(s2, &FakeRunner{}, dir2)
	eng2.SetWorktreeManager(fake2)
	id2, err := eng2.CreateExecution(ctx, "iso")
	if err != nil {
		t.Fatalf("create iso: %v", err)
	}
	exec2, _ := s2.Executions().Get(ctx, id2)
	if exec2.WorkspaceMode != "isolated" {
		t.Fatalf("iso exec mode = %q want isolated", exec2.WorkspaceMode)
	}
	if !strings.Contains(exec2.WorkspaceRoot, ".haro/worktrees") {
		t.Fatalf("isolated root should be worktree path, got %q", exec2.WorkspaceRoot)
	}
	hasCreate2 := false
	for _, c := range fake2.Calls {
		if c.Op == "create" {
			hasCreate2 = true
			break
		}
	}
	if !hasCreate2 {
		t.Fatalf("isolated should create worktree, calls %v", fake2.Calls)
	}
	// also test workflow import: need to verify that workflow.Parse handles workspace for execution creation
	_ = workflow.Validate
}

func TestRunStepClaim(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".haro", "workflows", "demo"), 0o755); err != nil {
		t.Fatal(err)
	}
	// workflow with two steps claiming same path via produces
	yaml := "version: 2\nname: demo\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n    produces: [src/foo.ts]\n  - id: s2\n    type: command\n    run: echo hi\n    produces: [src/foo.ts]\n"
	if err := os.WriteFile(filepath.Join(dir, ".haro", "workflows", "demo", "workflow.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	// ensure src dir exists for canonicalization (EvalSymlinks needs root existence)
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(ctx, filepath.Join(dir, ".haro", "store.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func(){ _ = s.Close() }()
	// Need two executions with shared mode to test conflict
	// First execution shared
	// Create shared workflow for conflict test: need workspace shared
	yamlShared := "version: 2\nname: demo\nworkspace:\n  mode: shared\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n    produces: [src/foo.ts]\n"
	if err := os.WriteFile(filepath.Join(dir, ".haro", "workflows", "demo", "workflow.yaml"), []byte(yamlShared), 0o600); err != nil {
		t.Fatal(err)
	}
	eng := NewEngine(s, &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
		// create artifacts file expected by produces
		_ = os.MkdirAll(filepath.Join(dir, ".haro", "artifacts", "src"), 0o755)
		_ = os.WriteFile(filepath.Join(dir, ".haro", "artifacts", "src", "foo.ts"), []byte("content"), 0o600)
		_ = os.MkdirAll(filepath.Join(dir, ".haro", "artifacts", "src"), 0o755)
		_ = os.WriteFile(filepath.Join(dir, ".haro", "artifacts", "src", "blocked"), []byte("content"), 0o600)
		return 0, "ok", "", nil
	}}, dir)
	eng.SetWorktreeManager(&worktree.FakeManager{})
	id1, err := eng.CreateExecution(ctx, "demo")
	if err != nil {
		t.Fatalf("create exec1: %v", err)
	}
	id2, err := eng.CreateExecution(ctx, "demo")
	if err != nil {
		t.Fatalf("create exec2: %v", err)
	}
	// Need to ensure artifacts dir for runner? Not needed for fake
	// Run first step: should acquire claim on src/foo.ts
	if err := eng.RunStep(ctx, id1, "s1", ""); err != nil {
		t.Fatalf("run s1 exec1: %v", err)
	}
	// Check claim active for exec1 - should be released after success (defer)
	active, _ := s.PathClaims().ListActive(ctx, dir)
	if len(active) != 0 {
		t.Fatalf("expected 0 active after s1 released, got %d %v", len(active), active)
	}
	// Second exec's same path should be blocked while first holds? But first's claim was released after RunStep (defer Release). So we need test with concurrent holding.
	// Instead test logical_conflict via holding claim manually
	// Clean and create manual claim
	_, _ = s.PathClaims().ListActive(ctx, dir)
	// Simulate long-running claim: directly acquire via store before RunStep
	// Create execution that holds claim without releasing
	// For test, we will run step with slow runner and attempt concurrent second step
	// Simplify: test that after failed acquire, error is logical_conflict with owner info
	// Create isolated steps that conflict when external? Use shared mode for both.

	// Direct test for logical_conflict error type
	// Acquire a claim manually to block second
	blockClaim := store.PathClaim{
		ProjectID:        dir,
		LogicalPath:      "src/blocked",
		Mode:             "shared",
		OwnerExecutionID: id1,
		OwnerStepID:      "s1",
	}
	ok, _, err := s.PathClaims().Acquire(ctx, blockClaim)
	if err != nil || !ok {
		t.Fatalf("manual block acquire: %v ok %v", err, ok)
	}
	// Create workflow that claims src/blocked
	yamlBlock := "version: 2\nname: demo\nworkspace:\n  mode: shared\nsteps:\n  - id: s2\n    type: command\n    run: echo hi\n    produces: [src/blocked]\n"
	if err := os.WriteFile(filepath.Join(dir, ".haro", "workflows", "demo", "workflow.yaml"), []byte(yamlBlock), 0o600); err != nil {
		t.Fatal(err)
	}
	// Need new execution with step s2 claiming same path
	// Create fresh exec for s2's workflow - but we already have id2 with old workflow snapshot (s1 only). Instead create new exec after updating workflow file
	id3, err := eng.CreateExecution(ctx, "demo")
	if err != nil {
		t.Fatalf("create exec3: %v", err)
	}
	err = eng.RunStep(ctx, id3, "s2", "")
	if err == nil {
		t.Fatalf("expected logical_conflict for blocked path")
	}
	if !strings.Contains(err.Error(), "logical_conflict") {
		t.Fatalf("expected logical_conflict error, got %v", err)
	}
	if !strings.Contains(err.Error(), id1) && !strings.Contains(err.Error(), "s1") {
		t.Fatalf("logical_conflict should contain owner, got %v", err)
	}
	// Verify wrong-owner release rejected: try releasing blockClaim with wrong owner via engine's defer? Already tested store level
	// Test that after release, blocked path succeeds
	if err := s.PathClaims().Release(ctx, dir, "src/blocked", id1, "s1"); err != nil {
		t.Fatalf("release: %v", err)
	}
	// Now s2 should succeed
	if err := eng.RunStep(ctx, id3, "s2", ""); err != nil {
		// Might need to handle pending state already? After previous failed attempt, step remains pending, so retry should work
		t.Fatalf("run s2 after release should succeed: %v", err)
	}
	// Check that claims are released after success (ListActive should not contain s2's claim)
	active2, _ := s.PathClaims().ListActive(ctx, dir)
	for _, c := range active2 {
		if c.LogicalPath == "src/blocked" && c.OwnerExecutionID == id3 {
			t.Fatalf("claim should be released after success, still active %v", c)
		}
	}
	// also ensure s1 from earlier still not active (released via defer) - already released
	_ = id2
}
