package execution

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/HectorCortes/haro/internal/project"
	"github.com/HectorCortes/haro/internal/store"
	"github.com/HectorCortes/haro/internal/worktree"
)

func TestReopen_RetainedDescendants(t *testing.T) {
	// Must preserve s1->s2->s3 behavior pinned by state_test.go
	root := t.TempDir()
	_ = project.Init(root)
	wfDir := filepath.Join(root, ".haro", "workflows", "cascade")
	_ = os.MkdirAll(wfDir, 0755)
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
	_ = os.WriteFile(filepath.Join(wfDir, "workflow.yaml"), []byte(wfYAML), 0600)
	ctx := context.Background()
	dbPath := filepath.Join(root, ".haro", "store.db")
	s, _ := store.Open(ctx, dbPath)
	defer func() { _ = s.Close() }()
	artifacts := filepath.Join(root, ".haro", "artifacts")
	eng := NewEngine(s, &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
		for _, prod := range []string{"s1.txt", "s2.txt", "s3.txt"} {
			p := filepath.Join(artifacts, prod)
			_ = os.MkdirAll(filepath.Dir(p), 0755)
			if len(argv) > 1 && argv[1] == "s1" && prod == "s1.txt" {
				_ = os.WriteFile(p, []byte("s1"), 0600)
			}
			if len(argv) > 1 && argv[1] == "s2" && prod == "s2.txt" {
				_ = os.WriteFile(p, []byte("s2"), 0600)
			}
			if len(argv) > 1 && argv[1] == "s3" && prod == "s3.txt" {
				_ = os.WriteFile(p, []byte("s3"), 0600)
			}
		}
		return 0, "", "", nil
	}}, root)
	eng.SetWorktreeManager(&worktree.FakeManager{})
	execID, _ := eng.CreateExecution(ctx, "cascade")
	_ = eng.RunStep(ctx, execID, "s1", "")
	_ = eng.RunStep(ctx, execID, "s2", "")
	_ = eng.RunStep(ctx, execID, "s3", "")
	// reopen s1 cascade
	if err := eng.ReopenStep(ctx, execID, "s1", true, ""); err != nil {
		t.Fatalf("reopen s1: %v", err)
	}
	for _, sid := range []string{"s1", "s2", "s3"} {
		st, _ := s.Steps().Get(ctx, execID, sid)
		if st.Status != "pending" {
			t.Fatalf("%s status after reopen = %q want pending", sid, st.Status)
		}
	}
}

func TestReopen_NamespacedF06(t *testing.T) {
	root := t.TempDir()
	_ = project.Init(root)
	wfDir := filepath.Join(root, ".haro", "workflows", "main")
	_ = os.MkdirAll(wfDir, 0755)
	libDir := filepath.Join(root, ".haro", "workflows", "lib")
	_ = os.MkdirAll(libDir, 0755)
	// lib declares two producers: p1->artX, p2->artY with inputs/outputs contract
	libYAML := `version: 2
name: lib
inputs: []
outputs:
  - name: artX
    produced_by: p1
  - name: artY
    produced_by: p2
steps:
  - id: p1
    type: command
    run: echo p1
    produces: [artX]
  - id: p2
    type: command
    run: echo p2
    produces: [artY]
`
	_ = os.WriteFile(filepath.Join(libDir, "workflow.yaml"), []byte(libYAML), 0600)
	// main has two workflow nodes a and b, plus b.q requires artX
	mainYAML := `version: 2
name: main
steps:
  - id: a
    type: workflow
    source: ../lib/workflow.yaml
    bindings:
      artX: artX
      artY: artY
  - id: b
    type: workflow
    source: ../lib/workflow.yaml
    bindings:
      artX: artX
      artY: artY
  - id: bq
    type: command
    run: echo bq
    requires: [artX]
`
	// Actually b.q should be inside? Simpler: main has steps a (workflow) and bq that requires artX, but we need to model b.q as flattened id "b.q"?? Instead create separate test using direct execution of flattened steps without composition.
	// For F-06 we need: a.p1 produces artX, a.p2 produces artY, b.q requires artX. Reopening b.q should only invalidate a.p1, not a.p2.
	// We will simulate by creating an execution with steps a.p1, a.p2, b.q directly (bypass compose for unit), then test ReopenStep.
	_ = libYAML
	_ = mainYAML
	// Instead create execution directly via store to setup scenario
	ctx := context.Background()
	dbPath := filepath.Join(root, ".haro", "store.db")
	s, _ := store.Open(ctx, dbPath)
	defer func() { _ = s.Close() }()
	// Manually create execution with dag_hash placeholder
	eng := NewEngine(s, &FakeRunner{}, root)
	eng.SetWorktreeManager(&worktree.FakeManager{})
	// Create project and execution manually with store to avoid needing workflow parsing for this unit test
	projectID := root
	_ = s.Projects().Create(ctx, projectID, root)
	execID := "test-exec"
	exec := &store.Execution{ID: execID, ProjectID: projectID, WorkflowSource: filepath.Join(wfDir, "workflow.yaml"), Status: "running", WorkspaceMode: "isolated", WorkspaceRoot: root, StartedAt: "now"}
	_ = s.Executions().Create(ctx, exec)
	// Create steps: a.p1 produces artX, a.p2 produces artY, b.q requires artX
	steps := []*store.ExecutionStep{
		{ExecutionID: execID, StepID: "a.p1", Type: "command", Status: "pending", DependsOn: "[]", Requires: "[]", Produces: `["artX"]`, WorkspaceMode: "isolated"},
		{ExecutionID: execID, StepID: "a.p2", Type: "command", Status: "pending", DependsOn: "[]", Requires: "[]", Produces: `["artY"]`, WorkspaceMode: "isolated"},
		{ExecutionID: execID, StepID: "b.q", Type: "command", Status: "pending", DependsOn: "[]", Requires: `["artX"]`, Produces: "[]", WorkspaceMode: "isolated"},
	}
	for _, st := range steps {
		_ = s.Steps().Create(ctx, st)
	}
	// Manually set steps to completed with generations to simulate runs
	for _, sid := range []string{"a.p1", "a.p2", "b.q"} {
		_ = s.Steps().UpdateStatus(ctx, execID, sid, "completed")
		_ = s.Steps().UpdateGeneration(ctx, execID, sid, 1)
		gen := &store.Generation{ID: sid + "-gen1", ExecutionID: execID, StepID: sid, Number: 1, CreatedAt: "now"}
		_ = s.Generations().Create(ctx, gen)
		// create attempt
		att := &store.Attempt{ID: sid + "-att1", ExecutionID: execID, StepID: sid, GenerationID: gen.ID, Status: "completed", StartedAt: "now"}
		_ = s.Attempts().Create(ctx, att)
	}
	// Verify completed
	for _, sid := range []string{"a.p1", "a.p2", "b.q"} {
		st, _ := s.Steps().Get(ctx, execID, sid)
		if st.Status != "completed" {
			t.Fatalf("%s not completed before reopen: %q", sid, st.Status)
		}
	}
	eng2 := NewEngine(s, &FakeRunner{}, root)
	eng2.SetWorktreeManager(&worktree.FakeManager{})
	// Reopen b.q with cascade - should invalidate only a.p1 (feeder for artX), not a.p2
	if err := eng2.ReopenStep(ctx, execID, "b.q", true, ""); err != nil {
		t.Fatalf("reopen b.q: %v", err)
	}
	stP1, _ := s.Steps().Get(ctx, execID, "a.p1")
	stP2, _ := s.Steps().Get(ctx, execID, "a.p2")
	stBQ, _ := s.Steps().Get(ctx, execID, "b.q")
	if stP1.Status != "pending" {
		t.Fatalf("a.p1 should be pending after reopen b.q (feeder), got %q", stP1.Status)
	}
	if stP2.Status != "completed" {
		t.Fatalf("a.p2 should remain completed (not feeder), got %q", stP2.Status)
	}
	if stBQ.Status != "pending" {
		t.Fatalf("b.q should be pending, got %q", stBQ.Status)
	}
	// Check generations invalidated for p1 but not p2
	gensP1, _ := s.Generations().ListByStep(ctx, execID, "a.p1")
	if len(gensP1) == 0 || gensP1[0].InvalidatedAt == nil {
		t.Fatalf("a.p1 generation should be invalidated")
	}
	gensP2, _ := s.Generations().ListByStep(ctx, execID, "a.p2")
	if len(gensP2) > 0 && gensP2[0].InvalidatedAt != nil {
		t.Fatalf("a.p2 generation should NOT be invalidated")
	}
}
