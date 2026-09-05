package execution

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/HectorCortes/haro/internal/project"
	"github.com/HectorCortes/haro/internal/store"
	"github.com/HectorCortes/haro/internal/worktree"
)

func TestCreateExecution_NoRow(t *testing.T) {
	t.Run("cycle no row", func(t *testing.T) {
		root := t.TempDir()
		_ = project.Init(root)
		wfDir := filepath.Join(root, ".haro", "workflows", "main")
		_ = os.MkdirAll(wfDir, 0755)
		_ = os.WriteFile(filepath.Join(wfDir, "workflow.yaml"), []byte("version: 2\nname: main\nsteps:\n  - id: self\n    type: workflow\n    source: ./workflow.yaml\n"), 0600)
		ctx := context.Background()
		dbPath := filepath.Join(root, ".haro", "store.db")
		s, _ := store.Open(ctx, dbPath)
		defer func() { _ = s.Close() }()
		fakeWT := &worktree.FakeManager{}
		eng := NewEngine(s, &FakeRunner{}, root)
		eng.SetWorktreeManager(fakeWT)
		_, err := eng.CreateExecution(ctx, "main")
		if err == nil {
			t.Fatalf("expected error for cycle")
		}
		rows, _ := s.QueryForTest(ctx, "SELECT COUNT(*) FROM executions")
		if rows != nil {
			var n int
			if rows.Next() {
				_ = rows.Scan(&n)
			}
			_ = rows.Close()
			if n != 0 {
				t.Fatalf("expected 0 executions after cycle, got %d", n)
			}
		}
		for _, c := range fakeWT.Calls {
			if c.Op == "create" {
				t.Fatalf("worktree create should not have been called on cycle")
			}
		}
	})

	t.Run("contract no row", func(t *testing.T) {
		root := t.TempDir()
		_ = project.Init(root)
		wfDir := filepath.Join(root, ".haro", "workflows", "bad")
		_ = os.MkdirAll(wfDir, 0755)
		_ = os.WriteFile(filepath.Join(wfDir, "workflow.yaml"), []byte("version: 2\nname: bad\ninputs:\n  - name: inp\n    satisfied_by: s1\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n"), 0600)
		ctx := context.Background()
		dbPath := filepath.Join(root, ".haro", "store.db")
		s, _ := store.Open(ctx, dbPath)
		defer func() { _ = s.Close() }()
		fakeWT := &worktree.FakeManager{}
		eng := NewEngine(s, &FakeRunner{}, root)
		eng.SetWorktreeManager(fakeWT)
		_, err := eng.CreateExecution(ctx, "bad")
		if err == nil {
			t.Fatalf("expected contract violation")
		}
		rows, _ := s.QueryForTest(ctx, "SELECT COUNT(*) FROM executions")
		if rows != nil {
			var n int
			if rows.Next() {
				_ = rows.Scan(&n)
			}
			_ = rows.Close()
			if n != 0 {
				t.Fatalf("expected 0 executions after contract violation, got %d", n)
			}
		}
		for _, c := range fakeWT.Calls {
			if c.Op == "create" {
				t.Fatalf("worktree should not be created on contract violation")
			}
		}
	})

	t.Run("guard no row", func(t *testing.T) {
		root := t.TempDir()
		_ = project.Init(root)
		wfDir := filepath.Join(root, ".haro", "workflows", "big")
		_ = os.MkdirAll(wfDir, 0755)
		libDir := filepath.Join(root, ".haro", "workflows", "lib")
		_ = os.MkdirAll(libDir, 0755)
		yaml := "version: 2\nname: big\nsteps:\n"
		for i := 0; i < 257; i++ {
			yaml += "  - id: w" + fmt.Sprintf("%d", i) + "\n    type: workflow\n    source: ../lib/workflow.yaml\n"
		}
		_ = os.WriteFile(filepath.Join(wfDir, "workflow.yaml"), []byte(yaml), 0600)
		_ = os.WriteFile(filepath.Join(libDir, "workflow.yaml"), []byte("version: 2\nname: lib\nsteps:\n  - id: s\n    type: command\n    run: echo hi\n"), 0600)
		ctx := context.Background()
		dbPath := filepath.Join(root, ".haro", "store.db")
		s, _ := store.Open(ctx, dbPath)
		defer func() { _ = s.Close() }()
		fakeWT := &worktree.FakeManager{}
		eng := NewEngine(s, &FakeRunner{}, root)
		eng.SetWorktreeManager(fakeWT)
		_, err := eng.CreateExecution(ctx, "big")
		if err == nil {
			t.Fatalf("expected guard_exceeded")
		}
		rows, _ := s.QueryForTest(ctx, "SELECT COUNT(*) FROM executions")
		if rows != nil {
			var n int
			if rows.Next() {
				_ = rows.Scan(&n)
			}
			_ = rows.Close()
			if n != 0 {
				t.Fatalf("expected 0 executions after guard, got %d", n)
			}
		}
		for _, c := range fakeWT.Calls {
			if c.Op == "create" {
				t.Fatalf("worktree should not be created on guard")
			}
		}
	})
}
