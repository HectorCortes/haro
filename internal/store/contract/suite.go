package contract

import (
	"context"
	"errors"
	"testing"

	"github.com/HectorCortes/haro/internal/store"
)

// Factory creates a fresh Store for a test.
type Factory func(t *testing.T) store.Store

// Run executes the shared contract suite against a factory.
func Run(t *testing.T, factory Factory) {
	t.Helper()
	t.Run("StateAndRollback", func(t *testing.T) { testStateAndRollback(t, factory) })
	t.Run("Leases", func(t *testing.T) { testLeases(t, factory) })
	t.Run("Claims", func(t *testing.T) { testClaims(t, factory) })
	t.Run("EventsAndCursors", func(t *testing.T) { testEventsAndCursors(t, factory) })
	t.Run("EventsPayload", func(t *testing.T) { testEventsPayload(t, factory) })
	t.Run("Interactions", func(t *testing.T) { testInteractions(t, factory) })
	t.Run("Constraints", func(t *testing.T) { testConstraints(t, factory) })
}

func testStateAndRollback(t *testing.T, factory Factory) {
	ctx := context.Background()
	s := factory(t)
	defer func() { _ = s.Close() }()
	// Basic project/execution/step lifecycle via Store
	if err := s.Projects().Create(ctx, "proj-state", "/tmp/proj"); err != nil {
		t.Fatalf("create project: %v", err)
	}
	exec := &store.Execution{ID: "exec-state", ProjectID: "proj-state", WorkflowSource: "wf.yaml", Status: "pending", WorkspaceMode: "isolated", WorkspaceRoot: "/tmp/ws", StartedAt: "2025-01-01T00:00:00Z"}
	if err := s.Executions().Create(ctx, exec); err != nil {
		t.Fatalf("create exec: %v", err)
	}
	step := &store.ExecutionStep{ExecutionID: "exec-state", StepID: "step-1", Type: "command", Status: "pending", DependsOn: "[]", Requires: "[]", Produces: "[]", WorkspaceMode: "isolated", CurrentGeneration: 0}
	if err := s.Steps().Create(ctx, step); err != nil {
		t.Fatalf("create step: %v", err)
	}
	gen := &store.Generation{ID: "gen-state-1", ExecutionID: "exec-state", StepID: "step-1", Number: 1, CreatedAt: "2025-01-01T00:00:00Z"}
	if err := s.Generations().Create(ctx, gen); err != nil {
		t.Fatalf("create gen: %v", err)
	}
	attempt := &store.Attempt{ID: "att-state-1", ExecutionID: "exec-state", StepID: "step-1", GenerationID: "gen-state-1", Status: "running", StartedAt: "2025-01-01T00:00:00Z"}
	if err := s.Attempts().Create(ctx, attempt); err != nil {
		t.Fatalf("create attempt: %v", err)
	}
	// Rollback test via WithTx
	err := s.WithTx(ctx, func(tx store.Store) error {
		if err := tx.Projects().Create(ctx, "proj-rollback", "/tmp/rollback"); err != nil {
			return err
		}
		return errors.New("force rollback")
	})
	if err == nil {
		t.Fatalf("expected rollback error")
	}
	if _, err := s.Projects().Get(ctx, "proj-rollback"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("rollback should discard proj-rollback, got %v", err)
	}
	// Commit test
	if err := s.WithTx(ctx, func(tx store.Store) error {
		return tx.Projects().Create(ctx, "proj-commit", "/tmp/commit")
	}); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := s.Projects().Get(ctx, "proj-commit"); err != nil {
		t.Fatalf("commit should persist, got %v", err)
	}
	// Event cursor monotonic
	ev1 := &store.AttemptEvent{AttemptID: "att-state-1", Cursor: 0, EventType: "log", OccurredAt: "2025-01-01T00:00:01Z"}
	if err := s.Events().CreateAttemptEvent(ctx, ev1); err != nil {
		t.Fatalf("create event 0: %v", err)
	}
	n, err := s.Events().NextAttemptCursor(ctx, "att-state-1")
	if err != nil {
		t.Fatalf("next cursor: %v", err)
	}
	if n != 1 {
		t.Fatalf("next cursor = %d want 1", n)
	}
	ev2 := &store.AttemptEvent{AttemptID: "att-state-1", Cursor: n, EventType: "log", OccurredAt: "2025-01-01T00:00:02Z"}
	if err := s.Events().CreateAttemptEvent(ctx, ev2); err != nil {
		t.Fatalf("create event 1: %v", err)
	}
	n2, _ := s.Events().NextAttemptCursor(ctx, "att-state-1")
	if n2 != 2 {
		t.Fatalf("next cursor2 = %d want 2", n2)
	}
}

func testLeases(t *testing.T, factory Factory) {
	ctx := context.Background()
	s := factory(t)
	defer func() { _ = s.Close() }()
	execID := "exec-lease-contract"
	stepID := "step-lease"
	tok1, err := s.Leases().Acquire(ctx, execID, stepID, "holder-A")
	if err != nil {
		t.Fatalf("Acquire 1: %v", err)
	}
	if tok1 != 1 {
		t.Fatalf("tok1 = %d want 1", tok1)
	}
	if err := s.Leases().Release(ctx, execID, stepID, "holder-A", tok1); err != nil {
		t.Fatalf("Release 1: %v", err)
	}
	tok2, err := s.Leases().Acquire(ctx, execID, stepID, "holder-B")
	if err != nil {
		t.Fatalf("Acquire 2: %v", err)
	}
	if tok2 != 2 {
		t.Fatalf("tok2 = %d want 2", tok2)
	}
	if err := s.Leases().Renew(ctx, execID, stepID, "holder-B", tok2); err != nil {
		t.Fatalf("Renew 2: %v", err)
	}
	if err := s.Leases().Renew(ctx, execID, stepID, "holder-B", tok1); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("stale Renew expected ErrNotFound got %v", err)
	}
	if err := s.Leases().Release(ctx, execID, stepID, "holder-B", tok1); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("stale Release expected ErrNotFound got %v", err)
	}
	if err := s.Leases().Release(ctx, execID, stepID, "holder-B", tok2); err != nil {
		t.Fatalf("Release 2: %v", err)
	}
	tok3, err := s.Leases().Acquire(ctx, execID, stepID, "holder-A")
	if err != nil {
		t.Fatalf("Acquire 3: %v", err)
	}
	if tok3 != 3 {
		t.Fatalf("tok3 = %d want 3", tok3)
	}
	if err := s.Leases().Release(ctx, execID, stepID, "holder-A", tok3); err != nil {
		t.Fatalf("Release 3: %v", err)
	}
}

func testClaims(t *testing.T, factory Factory) {
	ctx := context.Background()
	s := factory(t)
	defer func() { _ = s.Close() }()
	if err := s.Projects().Create(ctx, "proj-claim-contract", "/tmp/proj"); err != nil {
		t.Fatalf("create proj: %v", err)
	}
	claimA := store.PathClaim{ProjectID: "proj-claim-contract", LogicalPath: "src", Mode: "shared", OwnerExecutionID: "execA", OwnerStepID: "step1"}
	ok, _, err := s.PathClaims().Acquire(ctx, claimA)
	if err != nil || !ok {
		t.Fatalf("acquire A: %v ok %v", err, ok)
	}
	claimB := store.PathClaim{ProjectID: "proj-claim-contract", LogicalPath: "src/foo.ts", Mode: "shared", OwnerExecutionID: "execB", OwnerStepID: "step2"}
	ok, conf, err := s.PathClaims().Acquire(ctx, claimB)
	if err != nil {
		t.Fatalf("acquire B: %v", err)
	}
	if ok {
		t.Fatalf("expected B blocked")
	}
	if conf == nil || conf.OwnerExecutionID != "execA" {
		t.Fatalf("conflict wrong: %+v", conf)
	}
	if err := s.PathClaims().Release(ctx, "proj-claim-contract", "src", "execA", "step1"); err != nil {
		t.Fatalf("release: %v", err)
	}
	ok, _, err = s.PathClaims().Acquire(ctx, claimB)
	if err != nil || !ok {
		t.Fatalf("acquire B after release: %v ok %v", err, ok)
	}
	active, err := s.PathClaims().ListActive(ctx, "proj-claim-contract")
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(active) != 1 || active[0].LogicalPath != "src/foo.ts" {
		t.Fatalf("list active unexpected: %+v", active)
	}
}

func testEventsAndCursors(t *testing.T, factory Factory) {
	ctx := context.Background()
	s := factory(t)
	defer func() { _ = s.Close() }()
	if err := s.Projects().Create(ctx, "proj-evt", "/tmp/proj"); err != nil {
		t.Fatalf("create proj: %v", err)
	}
	exec := &store.Execution{ID: "exec-evt", ProjectID: "proj-evt", WorkflowSource: "wf.yaml", Status: "pending", WorkspaceMode: "isolated", WorkspaceRoot: "/tmp/ws", StartedAt: "2025-01-01T00:00:00Z"}
	if err := s.Executions().Create(ctx, exec); err != nil {
		t.Fatalf("create exec: %v", err)
	}
	step := &store.ExecutionStep{ExecutionID: "exec-evt", StepID: "step-evt", Type: "command", Status: "pending", DependsOn: "[]", Requires: "[]", Produces: "[]", WorkspaceMode: "isolated", CurrentGeneration: 0}
	if err := s.Steps().Create(ctx, step); err != nil {
		t.Fatalf("create step: %v", err)
	}
	gen := &store.Generation{ID: "gen-evt", ExecutionID: "exec-evt", StepID: "step-evt", Number: 1, CreatedAt: "2025-01-01T00:00:00Z"}
	if err := s.Generations().Create(ctx, gen); err != nil {
		t.Fatalf("create gen: %v", err)
	}
	att := &store.Attempt{ID: "att-evt", ExecutionID: "exec-evt", StepID: "step-evt", GenerationID: "gen-evt", Status: "running", StartedAt: "2025-01-01T00:00:00Z"}
	if err := s.Attempts().Create(ctx, att); err != nil {
		t.Fatalf("create att: %v", err)
	}
	// Transition cursors
	n, err := s.Events().NextTransitionCursor(ctx, "exec-evt", "step-evt")
	if err != nil {
		t.Fatalf("next trans cursor: %v", err)
	}
	if n != 0 {
		t.Fatalf("initial trans cursor = %d want 0", n)
	}
	tr := &store.StepTransitionEvent{ExecutionID: "exec-evt", StepID: "step-evt", Cursor: n, FromStatus: nil, ToStatus: "running", OccurredAt: "2025-01-01T00:00:00Z"}
	if err := s.Events().CreateTransition(ctx, tr); err != nil {
		t.Fatalf("create transition: %v", err)
	}
	n2, _ := s.Events().NextTransitionCursor(ctx, "exec-evt", "step-evt")
	if n2 != 1 {
		t.Fatalf("next trans cursor2 = %d want 1", n2)
	}
	// Duplicate cursor should fail unique
	dup := &store.AttemptEvent{AttemptID: "att-evt", Cursor: 0, EventType: "log", OccurredAt: "2025-01-01T00:00:00Z"}
	if err := s.Events().CreateAttemptEvent(ctx, dup); err != nil {
		t.Fatalf("first attempt event: %v", err)
	}
	dup2 := &store.AttemptEvent{AttemptID: "att-evt", Cursor: 0, EventType: "log", OccurredAt: "2025-01-01T00:00:01Z"}
	if err := s.Events().CreateAttemptEvent(ctx, dup2); !errors.Is(err, store.ErrUniqueViolation) {
		t.Fatalf("duplicate cursor expected ErrUniqueViolation got %v", err)
	}
}

// testEventsPayload proves nullable attempt_events payload behavior on both
// backends: null and non-null payloads round-trip, and PriorOutputDelta picks
// the latest prior attempt's output_delta with non-nil payload winning over
// the legacy payload_ref even when empty.
func testEventsPayload(t *testing.T, factory Factory) {
	ctx := context.Background()
	s := factory(t)
	defer func() { _ = s.Close() }()
	if err := s.Projects().Create(ctx, "proj-evt-payload", "/tmp/proj"); err != nil {
		t.Fatalf("create proj: %v", err)
	}
	exec := &store.Execution{ID: "exec-evt-payload", ProjectID: "proj-evt-payload", WorkflowSource: "wf.yaml", Status: "pending", WorkspaceMode: "isolated", WorkspaceRoot: "/tmp/ws", StartedAt: "2025-01-01T00:00:00Z"}
	if err := s.Executions().Create(ctx, exec); err != nil {
		t.Fatalf("create exec: %v", err)
	}
	step := &store.ExecutionStep{ExecutionID: "exec-evt-payload", StepID: "step-payload", Type: "command", Status: "pending", DependsOn: "[]", Requires: "[]", Produces: "[]", WorkspaceMode: "isolated", CurrentGeneration: 0}
	if err := s.Steps().Create(ctx, step); err != nil {
		t.Fatalf("create step: %v", err)
	}
	gen := &store.Generation{ID: "gen-evt-payload-1", ExecutionID: "exec-evt-payload", StepID: "step-payload", Number: 1, CreatedAt: "2025-01-01T00:00:00Z"}
	if err := s.Generations().Create(ctx, gen); err != nil {
		t.Fatalf("create gen: %v", err)
	}
	gen2 := &store.Generation{ID: "gen-evt-payload-2", ExecutionID: "exec-evt-payload", StepID: "step-payload", Number: 2, CreatedAt: "2025-01-01T00:01:00Z"}
	if err := s.Generations().Create(ctx, gen2); err != nil {
		t.Fatalf("create gen2: %v", err)
	}
	// Legacy prior attempt: payload NULL, payload_ref set.
	if err := s.Attempts().Create(ctx, &store.Attempt{ID: "att-payload-legacy", ExecutionID: "exec-evt-payload", StepID: "step-payload", GenerationID: "gen-evt-payload-1", Status: "completed", StartedAt: "2025-01-01T00:00:00Z"}); err != nil {
		t.Fatalf("create legacy attempt: %v", err)
	}
	legacyRef := "evidence/legacy.txt"
	if err := s.Events().CreateAttemptEvent(ctx, &store.AttemptEvent{AttemptID: "att-payload-legacy", Cursor: 0, EventType: "output_delta", PayloadRef: &legacyRef, OccurredAt: "2025-01-01T00:00:01Z"}); err != nil {
		t.Fatalf("create legacy event: %v", err)
	}
	// Inline prior attempt: payload set, payload_ref NULL.
	if err := s.Attempts().Create(ctx, &store.Attempt{ID: "att-payload-inline", ExecutionID: "exec-evt-payload", StepID: "step-payload", GenerationID: "gen-evt-payload-2", Status: "completed", StartedAt: "2025-01-01T00:01:00Z"}); err != nil {
		t.Fatalf("create inline attempt: %v", err)
	}
	inline := "$ echo hi\nsanitized delta"
	if err := s.Events().CreateAttemptEvent(ctx, &store.AttemptEvent{AttemptID: "att-payload-inline", Cursor: 0, EventType: "output_delta", Payload: &inline, OccurredAt: "2025-01-01T00:01:01Z"}); err != nil {
		t.Fatalf("create inline event: %v", err)
	}
	// Round-trip via the repository read: latest prior attempt wins.
	ev, err := s.Events().PriorOutputDelta(ctx, "att-payload-inline")
	if err != nil {
		t.Fatalf("PriorOutputDelta: %v", err)
	}
	if ev.AttemptID != "att-payload-legacy" {
		t.Fatalf("prior attempt = %q, want att-payload-legacy", ev.AttemptID)
	}
	if ev.Payload != nil || ev.PayloadRef == nil || *ev.PayloadRef != legacyRef {
		t.Fatalf("legacy event = payload %v ref %v, want nil payload and %q ref", ev.Payload, ev.PayloadRef, legacyRef)
	}
	// From a fresh (no prior) attempt of another step there is nothing to read.
	if err := s.Steps().Create(ctx, &store.ExecutionStep{ExecutionID: "exec-evt-payload", StepID: "step-payload-2", Type: "command", Status: "pending", DependsOn: "[]", Requires: "[]", Produces: "[]", WorkspaceMode: "isolated", CurrentGeneration: 0}); err != nil {
		t.Fatalf("create step 2: %v", err)
	}
	genB := &store.Generation{ID: "gen-evt-payload-b", ExecutionID: "exec-evt-payload", StepID: "step-payload-2", Number: 1, CreatedAt: "2025-01-01T00:02:00Z"}
	if err := s.Generations().Create(ctx, genB); err != nil {
		t.Fatalf("create genB: %v", err)
	}
	if err := s.Attempts().Create(ctx, &store.Attempt{ID: "att-payload-other", ExecutionID: "exec-evt-payload", StepID: "step-payload-2", GenerationID: "gen-evt-payload-b", Status: "running", StartedAt: "2025-01-01T00:02:00Z"}); err != nil {
		t.Fatalf("create other attempt: %v", err)
	}
	if _, err := s.Events().PriorOutputDelta(ctx, "att-payload-other"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("PriorOutputDelta cross-step = %v, want ErrNotFound", err)
	}
	// Non-nil empty payload wins over the legacy reference (never fall back).
	empty := ""
	if err := s.Events().CreateAttemptEvent(ctx, &store.AttemptEvent{AttemptID: "att-payload-inline", Cursor: 1, EventType: "output_delta", Payload: &empty, OccurredAt: "2025-01-01T00:01:02Z"}); err != nil {
		t.Fatalf("create empty event: %v", err)
	}
	gen3 := &store.Generation{ID: "gen-evt-payload-3", ExecutionID: "exec-evt-payload", StepID: "step-payload", Number: 3, CreatedAt: "2025-01-01T00:03:00Z"}
	if err := s.Generations().Create(ctx, gen3); err != nil {
		t.Fatalf("create gen3: %v", err)
	}
	if err := s.Attempts().Create(ctx, &store.Attempt{ID: "att-payload-cur", ExecutionID: "exec-evt-payload", StepID: "step-payload", GenerationID: "gen-evt-payload-3", Status: "running", StartedAt: "2025-01-01T00:03:00Z"}); err != nil {
		t.Fatalf("create cur attempt: %v", err)
	}
	evEmpty, err := s.Events().PriorOutputDelta(ctx, "att-payload-cur")
	if err != nil {
		t.Fatalf("PriorOutputDelta cur: %v", err)
	}
	if evEmpty.Payload == nil || *evEmpty.Payload != "" {
		t.Fatalf("empty payload = %+v, want non-nil empty", evEmpty.Payload)
	}
}

func testInteractions(t *testing.T, factory Factory) {
	ctx := context.Background()
	s := factory(t)
	defer func() { _ = s.Close() }()
	if err := s.Projects().Create(ctx, "proj-inter-contract", "/tmp/proj"); err != nil {
		t.Fatalf("create proj: %v", err)
	}
	exec := &store.Execution{ID: "exec-inter-c", ProjectID: "proj-inter-contract", WorkflowSource: "wf.yaml", Status: "pending", WorkspaceMode: "isolated", WorkspaceRoot: "/tmp/ws", StartedAt: "2025-01-01T00:00:00Z"}
	if err := s.Executions().Create(ctx, exec); err != nil {
		t.Fatalf("create exec: %v", err)
	}
	step := &store.ExecutionStep{ExecutionID: "exec-inter-c", StepID: "step-inter", Type: "command", Status: "pending", DependsOn: "[]", Requires: "[]", Produces: "[]", WorkspaceMode: "isolated", CurrentGeneration: 0}
	if err := s.Steps().Create(ctx, step); err != nil {
		t.Fatalf("create step: %v", err)
	}
	gen := &store.Generation{ID: "gen-inter-c", ExecutionID: "exec-inter-c", StepID: "step-inter", Number: 1, CreatedAt: "2025-01-01T00:00:00Z"}
	if err := s.Generations().Create(ctx, gen); err != nil {
		t.Fatalf("create gen: %v", err)
	}
	att := &store.Attempt{ID: "att-inter-c", ExecutionID: "exec-inter-c", StepID: "step-inter", GenerationID: "gen-inter-c", Status: "running", StartedAt: "2025-01-01T00:00:00Z"}
	if err := s.Attempts().Create(ctx, att); err != nil {
		t.Fatalf("create att: %v", err)
	}
	inter := &store.Interaction{ID: "inter-c-1", AttemptID: "att-inter-c", Type: "permission", Status: "pending", IdempotencyKey: "key-c-1"}
	if err := s.Interactions().Create(ctx, inter); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := s.Interactions().Get(ctx, "inter-c-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != "pending" {
		t.Fatalf("status = %q want pending", got.Status)
	}
	resolved, err := s.Interactions().Resolve(ctx, "inter-c-1", "key-c-1", "allow")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.Status != "resolved" {
		t.Fatalf("resolved status = %q", resolved.Status)
	}
	// Idempotent same key
	resolved2, err := s.Interactions().Resolve(ctx, "inter-c-1", "key-c-1", "allow")
	if err != nil {
		t.Fatalf("Resolve idempotent: %v", err)
	}
	if resolved2.Status != "resolved" {
		t.Fatalf("idempotent status = %q", resolved2.Status)
	}
	// Duplicate key different id
	inter2 := &store.Interaction{ID: "inter-c-2", AttemptID: "att-inter-c", Type: "question", Status: "pending", IdempotencyKey: "key-c-1"}
	if err := s.Interactions().Create(ctx, inter2); !errors.Is(err, store.ErrUniqueViolation) {
		t.Fatalf("duplicate key expected ErrUniqueViolation got %v", err)
	}
	// Wrong key resolve
	inter3 := &store.Interaction{ID: "inter-c-3", AttemptID: "att-inter-c", Type: "permission", Status: "pending", IdempotencyKey: "key-c-3"}
	if err := s.Interactions().Create(ctx, inter3); err != nil {
		t.Fatalf("Create inter3: %v", err)
	}
	if _, err := s.Interactions().Resolve(ctx, "inter-c-3", "wrong-key", "deny"); !errors.Is(err, store.ErrUniqueViolation) {
		t.Fatalf("wrong key expected ErrUniqueViolation got %v", err)
	}
}

func testConstraints(t *testing.T, factory Factory) {
	ctx := context.Background()
	s := factory(t)
	defer func() { _ = s.Close() }()
	if err := s.Projects().Create(ctx, "proj-constr", "/tmp/proj"); err != nil {
		t.Fatalf("create proj: %v", err)
	}
	// Invalid execution status should be ErrCheckViolation
	badExec := &store.Execution{ID: "exec-bad-status", ProjectID: "proj-constr", WorkflowSource: "wf.yaml", Status: "bad_status", WorkspaceMode: "isolated", WorkspaceRoot: "/tmp/ws", StartedAt: "2025-01-01T00:00:00Z"}
	if err := s.Executions().Create(ctx, badExec); !errors.Is(err, store.ErrCheckViolation) {
		t.Fatalf("invalid status expected ErrCheckViolation got %v", err)
	}
	// Invalid step type
	if err := s.Executions().Create(ctx, &store.Execution{ID: "exec-good", ProjectID: "proj-constr", WorkflowSource: "wf.yaml", Status: "pending", WorkspaceMode: "isolated", WorkspaceRoot: "/tmp/ws", StartedAt: "2025-01-01T00:00:00Z"}); err != nil {
		t.Fatalf("create exec good: %v", err)
	}
	badStep := &store.ExecutionStep{ExecutionID: "exec-good", StepID: "bad-type", Type: "invalid_type", Status: "pending", DependsOn: "[]", Requires: "[]", Produces: "[]", WorkspaceMode: "isolated", CurrentGeneration: 0}
	if err := s.Steps().Create(ctx, badStep); !errors.Is(err, store.ErrCheckViolation) {
		t.Fatalf("invalid step type expected ErrCheckViolation got %v", err)
	}
	// FK violation
	badStep2 := &store.ExecutionStep{ExecutionID: "no-exec", StepID: "s1", Type: "command", Status: "pending", DependsOn: "[]", Requires: "[]", Produces: "[]", WorkspaceMode: "isolated", CurrentGeneration: 0}
	if err := s.Steps().Create(ctx, badStep2); !errors.Is(err, store.ErrForeignKeyViolation) {
		t.Fatalf("FK expected ErrForeignKeyViolation got %v", err)
	}
	// UNIQUE violation via generations
	goodStep := &store.ExecutionStep{ExecutionID: "exec-good", StepID: "step-good", Type: "command", Status: "pending", DependsOn: "[]", Requires: "[]", Produces: "[]", WorkspaceMode: "isolated", CurrentGeneration: 0}
	if err := s.Steps().Create(ctx, goodStep); err != nil {
		t.Fatalf("create good step: %v", err)
	}
	gen1 := &store.Generation{ID: "gen-constr-1", ExecutionID: "exec-good", StepID: "step-good", Number: 1, CreatedAt: "2025-01-01T00:00:00Z"}
	if err := s.Generations().Create(ctx, gen1); err != nil {
		t.Fatalf("create gen1: %v", err)
	}
	genDup := &store.Generation{ID: "gen-constr-2", ExecutionID: "exec-good", StepID: "step-good", Number: 1, CreatedAt: "2025-01-01T00:00:01Z"}
	if err := s.Generations().Create(ctx, genDup); !errors.Is(err, store.ErrUniqueViolation) {
		t.Fatalf("duplicate gen number expected ErrUniqueViolation got %v", err)
	}
	// Interaction invalid type CHECK
	gen2 := &store.Generation{ID: "gen-constr-3", ExecutionID: "exec-good", StepID: "step-good", Number: 2, CreatedAt: "2025-01-01T00:00:00Z"}
	if err := s.Generations().Create(ctx, gen2); err != nil {
		t.Fatalf("create gen2: %v", err)
	}
	att := &store.Attempt{ID: "att-constr", ExecutionID: "exec-good", StepID: "step-good", GenerationID: "gen-constr-1", Status: "running", StartedAt: "2025-01-01T00:00:00Z"}
	if err := s.Attempts().Create(ctx, att); err != nil {
		t.Fatalf("create att: %v", err)
	}
	badInter := &store.Interaction{ID: "inter-bad-type", AttemptID: "att-constr", Type: "invalid", Status: "pending", IdempotencyKey: "k1"}
	if err := s.Interactions().Create(ctx, badInter); !errors.Is(err, store.ErrCheckViolation) {
		t.Fatalf("invalid interaction type expected ErrCheckViolation got %v", err)
	}
	// FK for interactions
	badInter2 := &store.Interaction{ID: "inter-bad-fk", AttemptID: "no-attempt", Type: "permission", Status: "pending", IdempotencyKey: "k2"}
	if err := s.Interactions().Create(ctx, badInter2); !errors.Is(err, store.ErrForeignKeyViolation) {
		t.Fatalf("interaction FK expected ErrForeignKeyViolation got %v", err)
	}
}
