package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrationsExactReferenceSchema(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "exact.db")
	s, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = s.Close() }()

	// Verify all 11 reference tables exist
	expected := []string{"projects", "executions", "execution_steps", "generations", "attempts", "attempt_transport", "leases", "step_transition_events", "attempt_events", "interactions", "path_claims"}
	rows, err := s.db.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
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
	for _, want := range expected {
		found := false
		for _, g := range got {
			if g == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("table %q not found, got %v", want, got)
		}
	}
	if len(got) != len(expected) {
		// Allow extra sqlite internal tables not in expected, but at least verify expected present; strict check for exact 11 ignoring sqlite_sequence
		filtered := []string{}
		for _, g := range got {
			if g == "sqlite_sequence" {
				continue
			}
			filtered = append(filtered, g)
		}
		if len(filtered) != len(expected) {
			t.Fatalf("expected %d tables, got %v", len(expected), filtered)
		}
	}

	// Verify leases DDL verbatim via migrationStatements and table existence
	foundLeases := false
	foundInteractions := false
	for _, stmt := range migrationStatements() {
		if strings.Contains(stmt, "CREATE TABLE IF NOT EXISTS leases") {
			foundLeases = true
			if !strings.Contains(stmt, "execution_id TEXT NOT NULL, step_id TEXT NOT NULL, holder TEXT NOT NULL, fencing_token INTEGER NOT NULL, acquired_at TEXT NOT NULL, expires_at TEXT NOT NULL, PRIMARY KEY (execution_id, step_id)") {
				t.Fatalf("leases verbatim mismatch: %q", stmt)
			}
		}
		if strings.Contains(stmt, "CREATE TABLE IF NOT EXISTS interactions") {
			foundInteractions = true
			if !strings.Contains(stmt, "CREATE TABLE IF NOT EXISTS interactions (id TEXT PRIMARY KEY, attempt_id TEXT NOT NULL REFERENCES attempts(id), type TEXT NOT NULL CHECK (type IN ('permission','question')), status TEXT NOT NULL CHECK (status IN ('pending','resolved')), decision TEXT, idempotency_key TEXT NOT NULL, resolved_at TEXT, UNIQUE (attempt_id, idempotency_key))") {
				t.Fatalf("interactions verbatim mismatch: %q", stmt)
			}
		}
	}
	if !foundLeases {
		t.Fatalf("leases DDL not found in migrationStatements")
	}
	if !foundInteractions {
		t.Fatalf("interactions DDL not found in migrationStatements")
	}
	// Also verify table sql via sqlite_master contains expected columns
	var leasesSQL string
	if err := s.db.QueryRowContext(ctx, "SELECT sql FROM sqlite_master WHERE type='table' AND name='leases'").Scan(&leasesSQL); err != nil {
		t.Fatalf("leases sql: %v", err)
	}
	if !strings.Contains(leasesSQL, "execution_id TEXT NOT NULL") || !strings.Contains(leasesSQL, "step_id TEXT NOT NULL") {
		t.Fatalf("leases DDL missing columns: %q", leasesSQL)
	}
	if !strings.Contains(leasesSQL, "PRIMARY KEY (execution_id, step_id)") {
		t.Fatalf("leases missing composite PK: %q", leasesSQL)
	}
	if strings.Contains(strings.ToUpper(leasesSQL), "REFERENCES") {
		t.Fatalf("leases must not have FK: %q", leasesSQL)
	}
	if strings.Contains(strings.ToUpper(leasesSQL), "CHECK") {
		t.Fatalf("leases must not have CHECK: %q", leasesSQL)
	}
	if !strings.Contains(leasesSQL, "fencing_token INTEGER NOT NULL") {
		t.Fatalf("leases missing fencing_token: %q", leasesSQL)
	}
	if !strings.Contains(leasesSQL, "acquired_at TEXT NOT NULL") || !strings.Contains(leasesSQL, "expires_at TEXT NOT NULL") {
		t.Fatalf("leases missing timestamps: %q", leasesSQL)
	}

	// Verify interactions DDL via sqlite_master
	var interSQL string
	if err := s.db.QueryRowContext(ctx, "SELECT sql FROM sqlite_master WHERE type='table' AND name='interactions'").Scan(&interSQL); err != nil {
		t.Fatalf("interactions sql: %v", err)
	}
	if !strings.Contains(interSQL, "id TEXT PRIMARY KEY") {
		t.Fatalf("interactions missing id PK: %q", interSQL)
	}
	if !strings.Contains(interSQL, "attempt_id TEXT NOT NULL REFERENCES attempts(id)") {
		t.Fatalf("interactions missing FK: %q", interSQL)
	}
	if !strings.Contains(interSQL, "CHECK (type IN ('permission','question'))") {
		t.Fatalf("interactions missing type CHECK: %q", interSQL)
	}
	if !strings.Contains(interSQL, "CHECK (status IN ('pending','resolved'))") {
		t.Fatalf("interactions missing status CHECK: %q", interSQL)
	}
	if !strings.Contains(interSQL, "UNIQUE (attempt_id, idempotency_key)") {
		t.Fatalf("interactions missing UNIQUE: %q", interSQL)
	}
	if !strings.Contains(interSQL, "idempotency_key TEXT NOT NULL") {
		t.Fatalf("interactions missing idempotency_key: %q", interSQL)
	}

	// Verify sole-owned tables still present verbatim (attempt_transport, path_claims, dag_hash, base_commit not redefined)
	var apSQL string
	if err := s.db.QueryRowContext(ctx, "SELECT sql FROM sqlite_master WHERE type='table' AND name='attempt_transport'").Scan(&apSQL); err != nil {
		t.Fatalf("attempt_transport sql: %v", err)
	}
	if !strings.Contains(apSQL, "adapter_name TEXT NOT NULL") {
		t.Fatalf("attempt_transport missing adapter_name: %q", apSQL)
	}
	var pcSQL string
	if err := s.db.QueryRowContext(ctx, "SELECT sql FROM sqlite_master WHERE type='table' AND name='path_claims'").Scan(&pcSQL); err != nil {
		t.Fatalf("path_claims sql: %v", err)
	}
	if !strings.Contains(pcSQL, "logical_path TEXT NOT NULL") {
		t.Fatalf("path_claims missing logical_path: %q", pcSQL)
	}
	// Verify dag_hash and base_commit via PRAGMA
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
			t.Fatalf("scan info: %v", err)
		}
		if name == "dag_hash" {
			hasDag = true
		}
		if name == "base_commit" {
			hasBase = true
		}
	}
	_ = rows2.Close()
	if !hasDag {
		t.Fatalf("dag_hash column missing")
	}
	if !hasBase {
		t.Fatalf("base_commit column missing")
	}
}

func TestMigrationIdempotentPreservesRows(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "idempotent.db")
	s, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	if err := s.Projects().Create(ctx, "proj-idem", dir); err != nil {
		t.Fatalf("create proj: %v", err)
	}
	execID := "exec-idem"
	if err := s.Executions().Create(ctx, &Execution{
		ID:             execID,
		ProjectID:      "proj-idem",
		WorkflowSource: "wf.yaml",
		Status:         "pending",
		WorkspaceMode:  "isolated",
		WorkspaceRoot:  dir,
		StartedAt:      "2025-01-01T00:00:00Z",
	}); err != nil {
		t.Fatalf("create exec: %v", err)
	}
	_ = s.Close()

	// Second open should be idempotent and preserve rows
	s2, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer func() { _ = s2.Close() }()
	p, err := s2.Projects().Get(ctx, "proj-idem")
	if err != nil {
		t.Fatalf("get proj: %v", err)
	}
	if p.ID != "proj-idem" {
		t.Fatalf("id = %q want proj-idem", p.ID)
	}
	e, err := s2.Executions().Get(ctx, execID)
	if err != nil {
		t.Fatalf("get exec: %v", err)
	}
	if e.ID != execID {
		t.Fatalf("exec id = %q want %q", e.ID, execID)
	}

	// Third open also succeeds
	_ = s2.Close()
	s3, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("third Open: %v", err)
	}
	defer func() { _ = s3.Close() }()
	if _, err := s3.Projects().Get(ctx, "proj-idem"); err != nil {
		t.Fatalf("third get: %v", err)
	}
}
