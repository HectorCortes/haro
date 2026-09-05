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

func TestCompose_SingleWorktree(t *testing.T) {
	root := t.TempDir()
	_ = project.Init(root)
	// Create a composed workflow: main includes lib twice (reuse-twice)
	mainDir := filepath.Join(root, ".haro", "workflows", "main")
	libDir := filepath.Join(root, ".haro", "workflows", "lib")
	_ = os.MkdirAll(mainDir, 0755)
	_ = os.MkdirAll(libDir, 0755)
	_ = os.WriteFile(filepath.Join(libDir, "workflow.yaml"), []byte("version: 2\nname: lib\nsteps:\n  - id: x\n    type: command\n    run: echo x\n  - id: y\n    type: command\n    run: echo y\n"), 0600)
	_ = os.WriteFile(filepath.Join(mainDir, "workflow.yaml"), []byte("version: 2\nname: main\nsteps:\n  - id: a\n    type: workflow\n    source: ../lib/workflow.yaml\n  - id: b\n    type: workflow\n    source: ../lib/workflow.yaml\n"), 0600)
	ctx := context.Background()
	dbPath := filepath.Join(root, ".haro", "store.db")
	s, _ := store.Open(ctx, dbPath)
	defer func() { _ = s.Close() }()
	fakeWT := &worktree.FakeManager{}
	eng := NewEngine(s, &FakeRunner{}, root)
	eng.SetWorktreeManager(fakeWT)
	execID, err := eng.CreateExecution(ctx, "main")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(fakeWT.Calls) == 0 {
		t.Fatalf("worktree not created")
	}
	createCalls := 0
	for _, c := range fakeWT.Calls {
		if c.Op == "create" {
			createCalls++
		}
	}
	if createCalls != 1 {
		t.Fatalf("expected exactly 1 worktree create, got %d: %+v", createCalls, fakeWT.Calls)
	}
	// Verify execution has 4 steps a.x, a.y, b.x, b.y
	steps, _ := s.Steps().List(ctx, execID)
	if len(steps) != 4 {
		t.Fatalf("steps = %d want 4, got %+v", len(steps), steps)
	}
	// Verify workspace root is stored
	exec, _ := s.Executions().Get(ctx, execID)
	if exec.WorkspaceRoot == "" {
		t.Fatalf("workspace root empty")
	}
	// Ensure no second worktree per step
	_ = execID
}
