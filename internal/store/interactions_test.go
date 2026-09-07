package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestInteractionCASIdempotent(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "inter.db")
	s, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = s.Close() }()

	// Need parent rows for FK
	if err := s.Projects().Create(ctx, "proj-i", dir); err != nil {
		t.Fatalf("create proj: %v", err)
	}
	exec := &Execution{ID: "exec-i", ProjectID: "proj-i", WorkflowSource: "wf.yaml", Status: "pending", WorkspaceMode: "isolated", WorkspaceRoot: dir, StartedAt: "2025-01-01T00:00:00Z"}
	if err := s.Executions().Create(ctx, exec); err != nil {
		t.Fatalf("create exec: %v", err)
	}
	step := &ExecutionStep{ExecutionID: "exec-i", StepID: "step-i", Type: "command", Status: "pending", DependsOn: "[]", Requires: "[]", Produces: "[]", WorkspaceMode: "isolated", CurrentGeneration: 0}
	if err := s.Steps().Create(ctx, step); err != nil {
		t.Fatalf("create step: %v", err)
	}
	gen := &Generation{ID: "gen-i", ExecutionID: "exec-i", StepID: "step-i", Number: 1, CreatedAt: "2025-01-01T00:00:00Z"}
	if err := s.Generations().Create(ctx, gen); err != nil {
		t.Fatalf("create gen: %v", err)
	}
	attempt := &Attempt{ID: "attempt-i", ExecutionID: "exec-i", StepID: "step-i", GenerationID: "gen-i", Status: "running", StartedAt: "2025-01-01T00:00:00Z"}
	if err := s.Attempts().Create(ctx, attempt); err != nil {
		t.Fatalf("create attempt: %v", err)
	}

	// Create interaction
	inter := &Interaction{ID: "inter-1", AttemptID: "attempt-i", Type: "permission", Status: "pending", IdempotencyKey: "key-1"}
	if err := s.Interactions().Create(ctx, inter); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := s.Interactions().Get(ctx, "inter-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != "pending" || got.IdempotencyKey != "key-1" {
		t.Fatalf("got %+v", got)
	}

	// Resolve with correct key
	dec := "allow"
	resolved, err := s.Interactions().Resolve(ctx, "inter-1", "key-1", dec)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.Status != "resolved" || resolved.Decision == nil || *resolved.Decision != dec {
		t.Fatalf("resolved %+v", resolved)
	}

	// Idempotent same key should return same result, not error
	resolved2, err := s.Interactions().Resolve(ctx, "inter-1", "key-1", dec)
	if err != nil {
		t.Fatalf("Resolve idempotent: %v", err)
	}
	if resolved2.Status != "resolved" || *resolved2.Decision != dec {
		t.Fatalf("idempotent resolved %+v", resolved2)
	}

	// Duplicate idempotency_key for same attempt with different id should fail ErrUniqueViolation
	inter2 := &Interaction{ID: "inter-2", AttemptID: "attempt-i", Type: "question", Status: "pending", IdempotencyKey: "key-1"}
	if err := s.Interactions().Create(ctx, inter2); !errors.Is(err, ErrUniqueViolation) {
		t.Fatalf("duplicate key expected ErrUniqueViolation got %v", err)
	}

	// Wrong key on Resolve should fail ErrUniqueViolation (CAS mismatch)
	inter3 := &Interaction{ID: "inter-3", AttemptID: "attempt-i", Type: "permission", Status: "pending", IdempotencyKey: "key-3"}
	if err := s.Interactions().Create(ctx, inter3); err != nil {
		t.Fatalf("Create inter3: %v", err)
	}
	if _, err := s.Interactions().Resolve(ctx, "inter-3", "wrong-key", "deny"); !errors.Is(err, ErrUniqueViolation) {
		t.Fatalf("wrong key Resolve expected ErrUniqueViolation got %v", err)
	}

	// Get non-existent should be ErrNotFound
	if _, err := s.Interactions().Get(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get missing expected ErrNotFound got %v", err)
	}
}
