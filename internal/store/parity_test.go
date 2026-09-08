package store

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestAttemptEventPayloadBackendParity proves v2-store/U-01 for the inline
// evidence delta: null and non-null attempt_events payloads round-trip
// identically on SQLite and on the fake backend, through the same
// PriorOutputDelta repository read used by feedback reconstruction.
func TestAttemptEventPayloadBackendParity(t *testing.T) {
	for _, name := range []string{"fake", "sqlite"} {
		t.Run(name, func(t *testing.T) {
			var s Store
			var cleanup func()
			if name == "fake" {
				s = NewFakeStore()
				cleanup = func() {}
			} else {
				dir := t.TempDir()
				dbPath := filepath.Join(dir, "payload-parity.db")
				sqlStore, err := Open(context.Background(), dbPath)
				if err != nil {
					t.Fatalf("Open: %v", err)
				}
				s = sqlStore
				cleanup = func() { _ = sqlStore.Close() }
			}
			defer cleanup()
			ctx := context.Background()
			if err := s.Projects().Create(ctx, "proj-payload", "/tmp/proj"); err != nil {
				t.Fatalf("create proj: %v", err)
			}
			if err := s.Executions().Create(ctx, &Execution{ID: "exec-payload", ProjectID: "proj-payload", WorkflowSource: "wf.yaml", Status: "pending", WorkspaceMode: "isolated", WorkspaceRoot: "/tmp/ws", StartedAt: "2025-01-01T00:00:00Z"}); err != nil {
				t.Fatalf("create exec: %v", err)
			}
			if err := s.Steps().Create(ctx, &ExecutionStep{ExecutionID: "exec-payload", StepID: "s1", Type: "command", Status: "pending", DependsOn: "[]", Requires: "[]", Produces: "[]", WorkspaceMode: "isolated", CurrentGeneration: 0}); err != nil {
				t.Fatalf("create step: %v", err)
			}
			if err := s.Generations().Create(ctx, &Generation{ID: "gen-payload-1", ExecutionID: "exec-payload", StepID: "s1", Number: 1, CreatedAt: "2025-01-01T00:00:00Z"}); err != nil {
				t.Fatalf("create gen1: %v", err)
			}
			if err := s.Generations().Create(ctx, &Generation{ID: "gen-payload-2", ExecutionID: "exec-payload", StepID: "s1", Number: 2, CreatedAt: "2025-01-01T00:01:00Z"}); err != nil {
				t.Fatalf("create gen2: %v", err)
			}
			if err := s.Generations().Create(ctx, &Generation{ID: "gen-payload-3", ExecutionID: "exec-payload", StepID: "s1", Number: 3, CreatedAt: "2025-01-01T00:02:00Z"}); err != nil {
				t.Fatalf("create gen3: %v", err)
			}
			// att-a: legacy event (payload NULL, payload_ref set).
			if err := s.Attempts().Create(ctx, &Attempt{ID: "att-a", ExecutionID: "exec-payload", StepID: "s1", GenerationID: "gen-payload-1", Status: "completed", StartedAt: "2025-01-01T00:00:00Z"}); err != nil {
				t.Fatalf("create att-a: %v", err)
			}
			legacyRef := "evidence/att-a.txt"
			if err := s.Events().CreateAttemptEvent(ctx, &AttemptEvent{AttemptID: "att-a", Cursor: 0, EventType: "output_delta", PayloadRef: &legacyRef, OccurredAt: "2025-01-01T00:00:01Z"}); err != nil {
				t.Fatalf("create att-a event: %v", err)
			}
			// att-c: current attempt on s1 with its own inline event; its own
			// event must not be returned as "prior".
			if err := s.Attempts().Create(ctx, &Attempt{ID: "att-c", ExecutionID: "exec-payload", StepID: "s1", GenerationID: "gen-payload-3", Status: "running", StartedAt: "2025-01-01T00:02:00Z"}); err != nil {
				t.Fatalf("create att-c: %v", err)
			}
			current := "current delta"
			if err := s.Events().CreateAttemptEvent(ctx, &AttemptEvent{AttemptID: "att-c", Cursor: 0, EventType: "output_delta", Payload: &current, OccurredAt: "2025-01-01T00:02:01Z"}); err != nil {
				t.Fatalf("create att-c event: %v", err)
			}
			// Inline composition used by the s2 scenario below.
			inline := "$ echo hi\ninline visible delta"

			// att-c is the current attempt on s1 whose only prior is the
			// legacy att-a: null payload round-trips as nil with payload_ref kept.
			evLegacy, err := s.Events().PriorOutputDelta(ctx, "att-c")
			if err != nil {
				t.Fatalf("PriorOutputDelta att-c: %v", err)
			}
			if evLegacy.AttemptID != "att-a" {
				t.Fatalf("prior attempt = %q, want att-a", evLegacy.AttemptID)
			}
			if evLegacy.Payload != nil {
				t.Fatalf("payload = %q, want nil for legacy event", *evLegacy.Payload)
			}
			if evLegacy.PayloadRef == nil || *evLegacy.PayloadRef != legacyRef {
				t.Fatalf("payload_ref = %v, want %q", evLegacy.PayloadRef, legacyRef)
			}

			// Second step: inline prior round-trip (non-null payload, null ref).
			if err := s.Steps().Create(ctx, &ExecutionStep{ExecutionID: "exec-payload", StepID: "s2", Type: "command", Status: "pending", DependsOn: "[]", Requires: "[]", Produces: "[]", WorkspaceMode: "isolated", CurrentGeneration: 0}); err != nil {
				t.Fatalf("create step s2: %v", err)
			}
			if err := s.Generations().Create(ctx, &Generation{ID: "gen-payload-s2a", ExecutionID: "exec-payload", StepID: "s2", Number: 1, CreatedAt: "2025-01-01T00:03:00Z"}); err != nil {
				t.Fatalf("create gen s2a: %v", err)
			}
			if err := s.Generations().Create(ctx, &Generation{ID: "gen-payload-s2b", ExecutionID: "exec-payload", StepID: "s2", Number: 2, CreatedAt: "2025-01-01T00:04:00Z"}); err != nil {
				t.Fatalf("create gen s2b: %v", err)
			}
			if err := s.Attempts().Create(ctx, &Attempt{ID: "att-b", ExecutionID: "exec-payload", StepID: "s2", GenerationID: "gen-payload-s2a", Status: "completed", StartedAt: "2025-01-01T00:03:00Z"}); err != nil {
				t.Fatalf("create att-b: %v", err)
			}
			if err := s.Events().CreateAttemptEvent(ctx, &AttemptEvent{AttemptID: "att-b", Cursor: 0, EventType: "output_delta", Payload: &inline, OccurredAt: "2025-01-01T00:03:01Z"}); err != nil {
				t.Fatalf("create att-b event: %v", err)
			}
			if err := s.Attempts().Create(ctx, &Attempt{ID: "att-d", ExecutionID: "exec-payload", StepID: "s2", GenerationID: "gen-payload-s2b", Status: "running", StartedAt: "2025-01-01T00:04:00Z"}); err != nil {
				t.Fatalf("create att-d: %v", err)
			}
			ev, err := s.Events().PriorOutputDelta(ctx, "att-d")
			if err != nil {
				t.Fatalf("PriorOutputDelta att-d: %v", err)
			}
			if ev.AttemptID != "att-b" {
				t.Fatalf("prior attempt = %q, want att-b", ev.AttemptID)
			}
			if ev.Payload == nil || *ev.Payload != inline {
				t.Fatalf("payload = %v, want %q", ev.Payload, inline)
			}
			if ev.PayloadRef != nil {
				t.Fatalf("payload_ref = %q, want nil for inline event", *ev.PayloadRef)
			}
			if ev.EventType != "output_delta" {
				t.Fatalf("event_type = %q, want output_delta", ev.EventType)
			}

			// Empty non-nil payload must survive the round trip (non-nil wins
			// even when empty — the caller must not fall back to legacy).
			empty := ""
			if err := s.Events().CreateAttemptEvent(ctx, &AttemptEvent{AttemptID: "att-b", Cursor: 1, EventType: "output_delta", Payload: &empty, OccurredAt: "2025-01-01T00:03:02Z"}); err != nil {
				t.Fatalf("create att-b empty event: %v", err)
			}
			evEmpty, err := s.Events().PriorOutputDelta(ctx, "att-d")
			if err != nil {
				t.Fatalf("PriorOutputDelta after empty event: %v", err)
			}
			if evEmpty.Payload == nil || *evEmpty.Payload != "" {
				t.Fatalf("empty payload must round-trip as non-nil empty, got %+v", evEmpty.Payload)
			}

			// No prior attempt: typed not-found (att-a is s1's only attempt
			// besides the current att-c; a third step has a lone attempt).
			if err := s.Steps().Create(ctx, &ExecutionStep{ExecutionID: "exec-payload", StepID: "s3", Type: "command", Status: "pending", DependsOn: "[]", Requires: "[]", Produces: "[]", WorkspaceMode: "isolated", CurrentGeneration: 0}); err != nil {
				t.Fatalf("create step s3: %v", err)
			}
			if err := s.Generations().Create(ctx, &Generation{ID: "gen-payload-s3", ExecutionID: "exec-payload", StepID: "s3", Number: 1, CreatedAt: "2025-01-01T00:05:00Z"}); err != nil {
				t.Fatalf("create gen s3: %v", err)
			}
			if err := s.Attempts().Create(ctx, &Attempt{ID: "att-z", ExecutionID: "exec-payload", StepID: "s3", GenerationID: "gen-payload-s3", Status: "running", StartedAt: "2025-01-01T00:05:00Z"}); err != nil {
				t.Fatalf("create att-z: %v", err)
			}
			if _, err := s.Events().PriorOutputDelta(ctx, "att-z"); !errors.Is(err, ErrNotFound) {
				t.Fatalf("PriorOutputDelta without prior = %v, want ErrNotFound", err)
			}
		})
	}
}

func TestStoredTimestampsUTC(t *testing.T) {
	for _, name := range []string{"fake", "sqlite"} {
		t.Run(name, func(t *testing.T) {
			var s Store
			var cleanup func()
			if name == "fake" {
				fs := NewFakeStore()
				s = fs
				cleanup = func() {}
			} else {
				dir := t.TempDir()
				dbPath := filepath.Join(dir, "ts.db")
				sqlStore, err := Open(context.Background(), dbPath)
				if err != nil {
					t.Fatalf("Open: %v", err)
				}
				s = sqlStore
				cleanup = func() { _ = sqlStore.Close() }
			}
			defer cleanup()
			ctx := context.Background()
			if err := s.Projects().Create(ctx, "proj-ts", "/tmp/proj"); err != nil {
				t.Fatalf("create proj: %v", err)
			}
			proj, err := s.Projects().Get(ctx, "proj-ts")
			if err != nil {
				t.Fatalf("get proj: %v", err)
			}
			checkRFC3339UTC(t, proj.CreatedAt)

			exec := &Execution{ID: "exec-ts", ProjectID: "proj-ts", WorkflowSource: "wf.yaml", Status: "pending", WorkspaceMode: "isolated", WorkspaceRoot: "/tmp/ws", StartedAt: ""}
			if err := s.Executions().Create(ctx, exec); err != nil {
				t.Fatalf("create exec: %v", err)
			}
			gotExec, err := s.Executions().Get(ctx, "exec-ts")
			if err != nil {
				t.Fatalf("get exec: %v", err)
			}
			checkRFC3339UTC(t, gotExec.StartedAt)

			step := &ExecutionStep{ExecutionID: "exec-ts", StepID: "step-ts", Type: "command", Status: "pending", DependsOn: "[]", Requires: "[]", Produces: "[]", WorkspaceMode: "isolated", CurrentGeneration: 0}
			if err := s.Steps().Create(ctx, step); err != nil {
				t.Fatalf("create step: %v", err)
			}
			gen := &Generation{ID: "gen-ts", ExecutionID: "exec-ts", StepID: "step-ts", Number: 1, CreatedAt: ""}
			if err := s.Generations().Create(ctx, gen); err != nil {
				t.Fatalf("create gen: %v", err)
			}
			gotGen, err := s.Generations().Get(ctx, "gen-ts")
			if err != nil {
				t.Fatalf("get gen: %v", err)
			}
			checkRFC3339UTC(t, gotGen.CreatedAt)

			att := &Attempt{ID: "att-ts", ExecutionID: "exec-ts", StepID: "step-ts", GenerationID: "gen-ts", Status: "running", StartedAt: ""}
			if err := s.Attempts().Create(ctx, att); err != nil {
				t.Fatalf("create att: %v", err)
			}
			gotAtt, err := s.Attempts().Get(ctx, "att-ts")
			if err != nil {
				t.Fatalf("get att: %v", err)
			}
			checkRFC3339UTC(t, gotAtt.StartedAt)

			// Lease timestamps
			tok, err := s.Leases().Acquire(ctx, "exec-ts", "step-ts", "holder-ts")
			if err != nil {
				t.Fatalf("lease acquire: %v", err)
			}
			_ = tok
			lease, err := s.Leases().Get(ctx, "exec-ts", "step-ts")
			if err != nil {
				t.Fatalf("get lease: %v", err)
			}
			checkRFC3339UTC(t, lease.AcquiredAt)
			checkRFC3339UTC(t, lease.ExpiresAt)

			// Path claim timestamp
			claim := PathClaim{ProjectID: "proj-ts", LogicalPath: "src/file.ts", Mode: "shared", OwnerExecutionID: "exec-ts", OwnerStepID: "step-ts"}
			ok, _, err := s.PathClaims().Acquire(ctx, claim)
			if err != nil || !ok {
				t.Fatalf("claim acquire: %v ok %v", err, ok)
			}
			active, err := s.PathClaims().ListActive(ctx, "proj-ts")
			if err != nil {
				t.Fatalf("list active: %v", err)
			}
			if len(active) > 0 {
				checkRFC3339UTC(t, active[0].AcquiredAt)
			}

			// Interaction resolve timestamp
			inter := &Interaction{ID: "inter-ts", AttemptID: "att-ts", Type: "permission", Status: "pending", IdempotencyKey: "key-ts"}
			if err := s.Interactions().Create(ctx, inter); err != nil {
				t.Fatalf("create inter: %v", err)
			}
			resolved, err := s.Interactions().Resolve(ctx, "inter-ts", "key-ts", "allow")
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if resolved.ResolvedAt == nil {
				t.Fatalf("resolved_at nil")
			}
			checkRFC3339UTC(t, *resolved.ResolvedAt)
		})
	}
}

func checkRFC3339UTC(t *testing.T, ts string) {
	t.Helper()
	if ts == "" {
		t.Fatalf("timestamp empty")
	}
	parsed, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t.Fatalf("timestamp %q parse RFC3339: %v", ts, err)
	}
	if parsed.Location() != time.UTC && parsed.UTC().Format(time.RFC3339) != ts {
		// Check that string ends with Z (UTC offset)
		if !strings.HasSuffix(ts, "Z") {
			t.Fatalf("timestamp %q not UTC Z", ts)
		}
	}
	if !strings.HasSuffix(ts, "Z") {
		t.Fatalf("timestamp %q not UTC Z suffix", ts)
	}
}

func TestConstraintsParity(t *testing.T) {
	for _, name := range []string{"fake", "sqlite"} {
		t.Run(name, func(t *testing.T) {
			var s Store
			var cleanup func()
			if name == "fake" {
				s = NewFakeStore()
				cleanup = func() {}
			} else {
				dir := t.TempDir()
				dbPath := filepath.Join(dir, "constr.db")
				sqlStore, err := Open(context.Background(), dbPath)
				if err != nil {
					t.Fatalf("Open: %v", err)
				}
				s = sqlStore
				cleanup = func() { _ = sqlStore.Close() }
			}
			defer cleanup()
			ctx := context.Background()
			if err := s.Projects().Create(ctx, "proj-c", "/tmp/proj"); err != nil {
				t.Fatalf("create proj: %v", err)
			}
			// Enum: invalid execution status -> ErrCheckViolation
			badExec := &Execution{ID: "exec-bad-" + name, ProjectID: "proj-c", WorkflowSource: "wf.yaml", Status: "invalid", WorkspaceMode: "isolated", WorkspaceRoot: "/tmp/ws", StartedAt: "2025-01-01T00:00:00Z"}
			if err := s.Executions().Create(ctx, badExec); !errors.Is(err, ErrCheckViolation) {
				t.Fatalf("invalid exec status expected ErrCheckViolation got %v", err)
			}
			// FK: missing project
			badExec2 := &Execution{ID: "exec-fk-" + name, ProjectID: "no-proj", WorkflowSource: "wf.yaml", Status: "pending", WorkspaceMode: "isolated", WorkspaceRoot: "/tmp/ws", StartedAt: "2025-01-01T00:00:00Z"}
			if err := s.Executions().Create(ctx, badExec2); !errors.Is(err, ErrForeignKeyViolation) {
				t.Fatalf("FK expected ErrForeignKeyViolation got %v", err)
			}
			// Valid exec for further tests
			if err := s.Executions().Create(ctx, &Execution{ID: "exec-c", ProjectID: "proj-c", WorkflowSource: "wf.yaml", Status: "pending", WorkspaceMode: "isolated", WorkspaceRoot: "/tmp/ws", StartedAt: "2025-01-01T00:00:00Z"}); err != nil {
				t.Fatalf("create exec-c: %v", err)
			}
			// Invalid step type
			badStep := &ExecutionStep{ExecutionID: "exec-c", StepID: "bad-type", Type: "bad", Status: "pending", DependsOn: "[]", Requires: "[]", Produces: "[]", WorkspaceMode: "isolated", CurrentGeneration: 0}
			if err := s.Steps().Create(ctx, badStep); !errors.Is(err, ErrCheckViolation) {
				t.Fatalf("invalid step type expected ErrCheckViolation got %v", err)
			}
			// UNIQUE via generations
			if err := s.Steps().Create(ctx, &ExecutionStep{ExecutionID: "exec-c", StepID: "step-c", Type: "command", Status: "pending", DependsOn: "[]", Requires: "[]", Produces: "[]", WorkspaceMode: "isolated", CurrentGeneration: 0}); err != nil {
				t.Fatalf("create step-c: %v", err)
			}
			gen1 := &Generation{ID: "gen-c-1-" + name, ExecutionID: "exec-c", StepID: "step-c", Number: 1, CreatedAt: "2025-01-01T00:00:00Z"}
			if err := s.Generations().Create(ctx, gen1); err != nil {
				t.Fatalf("create gen1: %v", err)
			}
			genDup := &Generation{ID: "gen-c-2-" + name, ExecutionID: "exec-c", StepID: "step-c", Number: 1, CreatedAt: "2025-01-01T00:00:01Z"}
			if err := s.Generations().Create(ctx, genDup); !errors.Is(err, ErrUniqueViolation) {
				t.Fatalf("duplicate gen expected ErrUniqueViolation got %v", err)
			}
			// FK for attempt
			badAtt := &Attempt{ID: "att-bad-" + name, ExecutionID: "exec-c", StepID: "no-step", GenerationID: "gen-c-1-" + name, Status: "running", StartedAt: "2025-01-01T00:00:00Z"}
			if err := s.Attempts().Create(ctx, badAtt); !errors.Is(err, ErrForeignKeyViolation) {
				t.Fatalf("attempt FK expected ErrForeignKeyViolation got %v", err)
			}
			// Valid attempt
			att := &Attempt{ID: "att-c-" + name, ExecutionID: "exec-c", StepID: "step-c", GenerationID: "gen-c-1-" + name, Status: "running", StartedAt: "2025-01-01T00:00:00Z"}
			if err := s.Attempts().Create(ctx, att); err != nil {
				t.Fatalf("create att: %v", err)
			}
			// UNIQUE for interactions
			inter1 := &Interaction{ID: "inter-c1-" + name, AttemptID: att.ID, Type: "permission", Status: "pending", IdempotencyKey: "key-dup"}
			if err := s.Interactions().Create(ctx, inter1); err != nil {
				t.Fatalf("create inter1: %v", err)
			}
			interDup := &Interaction{ID: "inter-c2-" + name, AttemptID: att.ID, Type: "question", Status: "pending", IdempotencyKey: "key-dup"}
			if err := s.Interactions().Create(ctx, interDup); !errors.Is(err, ErrUniqueViolation) {
				t.Fatalf("duplicate key expected ErrUniqueViolation got %v", err)
			}
			// CHECK for interactions type
			badInter := &Interaction{ID: "inter-bad-type-" + name, AttemptID: att.ID, Type: "bad", Status: "pending", IdempotencyKey: "k-bad"}
			if err := s.Interactions().Create(ctx, badInter); !errors.Is(err, ErrCheckViolation) {
				t.Fatalf("bad interaction type expected ErrCheckViolation got %v", err)
			}
			// FK for interactions
			badInter2 := &Interaction{ID: "inter-bad-fk-" + name, AttemptID: "no-att", Type: "permission", Status: "pending", IdempotencyKey: "k2"}
			if err := s.Interactions().Create(ctx, badInter2); !errors.Is(err, ErrForeignKeyViolation) {
				t.Fatalf("inter FK expected ErrForeignKeyViolation got %v", err)
			}
		})
	}
}

func TestMigrationsVerifySoleOwners(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "sole.db")
	s, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = s.Close() }()
	// Verify 11 tables present
	rows, err := s.QueryForTest(ctx, "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		t.Fatalf("query tables: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var got []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, name)
	}
	expected := []string{"projects", "executions", "execution_steps", "generations", "attempts", "attempt_transport", "leases", "step_transition_events", "attempt_events", "interactions", "path_claims"}
	for _, want := range expected {
		found := false
		for _, g := range got {
			if g == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("table %q missing, got %v", want, got)
		}
	}
	// Verify sole-owned tables not redefined: check DDL contains expected columns
	checks := map[string]string{
		"attempt_transport": "adapter_name TEXT NOT NULL",
		"path_claims":       "logical_path TEXT NOT NULL",
	}
	for tbl, substr := range checks {
		var sqlDef string
		if err := s.db.QueryRowContext(ctx, "SELECT sql FROM sqlite_master WHERE type='table' AND name=?", tbl).Scan(&sqlDef); err != nil {
			t.Fatalf("sql for %s: %v", tbl, err)
		}
		if !strings.Contains(sqlDef, substr) {
			t.Fatalf("table %s missing %q, got %q", tbl, substr, sqlDef)
		}
	}
	// Verify dag_hash and base_commit columns exist via PRAGMA table_info
	rows2, err := s.db.QueryContext(ctx, "PRAGMA table_info(executions)")
	if err != nil {
		t.Fatalf("table_info: %v", err)
	}
	hasDag, hasBase := false, false
	for rows2.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt *string
		if err := rows2.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if name == "dag_hash" {
			hasDag = true
			if typ != "TEXT" {
				t.Fatalf("dag_hash type %q", typ)
			}
		}
		if name == "base_commit" {
			hasBase = true
			if typ != "TEXT" {
				t.Fatalf("base_commit type %q", typ)
			}
		}
	}
	_ = rows2.Close()
	if !hasDag || !hasBase {
		t.Fatalf("dag_hash/base_commit missing: dag %v base %v", hasDag, hasBase)
	}
	// Verify leases and interactions are owned here (contain IF NOT EXISTS in migrationStatements)
	foundLeases, foundInter := false, false
	for _, stmt := range migrationStatements() {
		if strings.Contains(stmt, "CREATE TABLE IF NOT EXISTS leases") {
			foundLeases = true
		}
		if strings.Contains(stmt, "CREATE TABLE IF NOT EXISTS interactions") {
			foundInter = true
		}
	}
	if !foundLeases || !foundInter {
		t.Fatalf("leases/interactions not found in migrationStatements")
	}
}
