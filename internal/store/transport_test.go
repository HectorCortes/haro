package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestTransport_MigrationIdempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "transport.db")
	ctx := context.Background()
	s, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = s.Close() }()

	// attempt_transport table must exist
	var name string
	err = s.db.QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name='attempt_transport'").Scan(&name)
	if err != nil {
		t.Fatalf("attempt_transport table missing: %v", err)
	}
	if name != "attempt_transport" {
		t.Fatalf("table name = %q, want attempt_transport", name)
	}
	// Verify columns
	rows, err := s.db.QueryContext(ctx, "SELECT sql FROM sqlite_master WHERE type='table' AND name='attempt_transport'")
	if err != nil {
		t.Fatalf("query sql: %v", err)
	}
	defer rows.Close()
	var sqlDef string
	if rows.Next() {
		_ = rows.Scan(&sqlDef)
	}
	if !strings.Contains(sqlDef, "attempt_id") || !strings.Contains(sqlDef, "adapter_name") {
		t.Fatalf("attempt_transport DDL missing expected columns, got %q", sqlDef)
	}
	// Idempotent: reopen same file should succeed and preserve data
	_ = s.Close()
	s2, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("reopen idempotent: %v", err)
	}
	defer func() { _ = s2.Close() }()
	var name2 string
	err = s2.db.QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name='attempt_transport'").Scan(&name2)
	if err != nil {
		t.Fatalf("reopen missing table: %v", err)
	}
}

func TestTransport_AttemptsNeutral(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "neutral.db")
	ctx := context.Background()
	s, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = s.Close() }()

	// Ensure attempts table has no transport columns
	rows, err := s.db.QueryContext(ctx, "SELECT name FROM pragma_table_info('attempts')")
	if err != nil {
		t.Fatalf("pragma attempts: %v", err)
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var c string
		_ = rows.Scan(&c)
		cols = append(cols, c)
	}
	for _, wantAbsent := range []string{"adapter_name", "native_session_id", "protocol_version", "extra"} {
		for _, c := range cols {
			if c == wantAbsent {
				t.Fatalf("attempts contains transport column %q, columns=%v", wantAbsent, cols)
			}
		}
	}
	// Triangulate: attempt_transport should have those columns
	rows2, err := s.db.QueryContext(ctx, "SELECT name FROM pragma_table_info('attempt_transport')")
	if err != nil {
		t.Fatalf("pragma attempt_transport: %v", err)
	}
	defer rows2.Close()
	var tcols []string
	for rows2.Next() {
		var c string
		_ = rows2.Scan(&c)
		tcols = append(tcols, c)
	}
	for _, want := range []string{"attempt_id", "adapter_name", "native_session_id", "protocol_version", "extra"} {
		found := false
		for _, c := range tcols {
			if c == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("attempt_transport missing column %q, got %v", want, tcols)
		}
	}
}

func TestTransport_PutGet(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "putget.db")
	ctx := context.Background()
	s, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = s.Close() }()

	// Need to create minimal execution/steps/generation/attempt to satisfy FK
	setupAttempt(t, s, ctx, "exec1", "step1", "gen1", "att1")

	extra := `{"foo":"bar"}`
	native := "sess-123"
	ver := 1
	tr := &Transport{
		AttemptID:       "att1",
		AdapterName:     "opencode",
		NativeSessionID: &native,
		ProtocolVersion: &ver,
		Extra:           extra,
	}
	if err := s.Transport().Put(ctx, tr); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := s.Transport().Get(ctx, "att1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.AdapterName != "opencode" {
		t.Fatalf("AdapterName = %q, want opencode", got.AdapterName)
	}
	if got.NativeSessionID == nil || *got.NativeSessionID != "sess-123" {
		t.Fatalf("NativeSessionID = %v, want sess-123", got.NativeSessionID)
	}
	if got.ProtocolVersion == nil || *got.ProtocolVersion != 1 {
		t.Fatalf("ProtocolVersion = %v, want 1", got.ProtocolVersion)
	}
	if got.Extra != extra {
		t.Fatalf("Extra = %q, want %q", got.Extra, extra)
	}
	// Triangulate: different adapter with null optional fields
	setupAttempt(t, s, ctx, "exec1", "step1", "gen2", "att2")
	tr2 := &Transport{
		AttemptID:   "att2",
		AdapterName: "claudecode",
		Extra:       "{}",
	}
	if err := s.Transport().Put(ctx, tr2); err != nil {
		t.Fatalf("Put2: %v", err)
	}
	got2, err := s.Transport().Get(ctx, "att2")
	if err != nil {
		t.Fatalf("Get2: %v", err)
	}
	if got2.NativeSessionID != nil {
		t.Fatalf("NativeSessionID expected nil, got %v", *got2.NativeSessionID)
	}
	if got2.ProtocolVersion != nil {
		t.Fatalf("ProtocolVersion expected nil, got %v", *got2.ProtocolVersion)
	}
}

func TestTransport_OptionalRow(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "optional.db")
	ctx := context.Background()
	s, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = s.Close() }()

	// Insert attempt without transport should succeed
	setupAttempt(t, s, ctx, "exec1", "step1", "gen1", "att-no-transport")
	// Verify we can read attempt without transport row
	a, err := s.Attempts().Get(ctx, "att-no-transport")
	if err != nil {
		t.Fatalf("Get attempt without transport: %v", err)
	}
	if a.ID != "att-no-transport" {
		t.Fatalf("ID mismatch")
	}
	// Get transport should return not found or nil error? We expect sql.ErrNoRows
	_, err = s.Transport().Get(ctx, "att-no-transport")
	if err == nil {
		t.Fatalf("expected error for missing transport row")
	}
	// Triangulate: Insert with transport and verify both succeed
	setupAttempt(t, s, ctx, "exec1", "step1", "gen2", "att-with")
	native := "native-1"
	ver := 2
	if err := s.Transport().Put(ctx, &Transport{AttemptID: "att-with", AdapterName: "acp-generic", NativeSessionID: &native, ProtocolVersion: &ver, Extra: "{}"}); err != nil {
		t.Fatalf("Put with transport: %v", err)
	}
	a2, err := s.Attempts().Get(ctx, "att-with")
	if err != nil {
		t.Fatalf("Get with transport attempt: %v", err)
	}
	if a2.ID != "att-with" {
		t.Fatalf("ID mismatch2")
	}
}

func TestTransport_WithTx(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "tx.db")
	ctx := context.Background()
	s, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = s.Close() }()

	// Need project/execution/step setup for FK
	if err := s.Projects().Create(ctx, "proj-tx", dir); err != nil {
		t.Fatalf("create proj: %v", err)
	}
	// Insert execution/step generically via Store methods? Use direct helpers
	// Use WithTx to insert attempt + transport atomically
	err = s.WithTx(ctx, func(tx Store) error {
		// create execution
		if _, err := tx.(*SQLiteStore).db.ExecContext(ctx, "INSERT INTO executions(id, project_id, workflow_source, status, workspace_mode, workspace_root, started_at) VALUES (?, ?, ?, ?, ?, ?, ?)", "exec-tx", "proj-tx", "wf.yaml", "running", "isolated", dir, "2025-01-01T00:00:00Z"); err != nil {
			return err
		}
		if _, err := tx.(*SQLiteStore).db.ExecContext(ctx, "INSERT INTO execution_steps(execution_id, step_id, type, status, workspace_mode, current_generation) VALUES (?, ?, ?, ?, ?, ?)", "exec-tx", "step-tx", "agent", "running", "isolated", 1); err != nil {
			return err
		}
		if _, err := tx.(*SQLiteStore).db.ExecContext(ctx, "INSERT INTO generations(id, execution_id, step_id, number, created_at) VALUES (?, ?, ?, ?, ?)", "gen-tx", "exec-tx", "step-tx", 1, "2025-01-01T00:00:00Z"); err != nil {
			return err
		}
		if err := tx.Attempts().Create(ctx, &Attempt{ID: "att-tx", ExecutionID: "exec-tx", StepID: "step-tx", GenerationID: "gen-tx", Status: "running", StartedAt: "2025-01-01T00:00:00Z"}); err != nil {
			return err
		}
		native := "native-tx"
		ver := 3
		return tx.Transport().Put(ctx, &Transport{AttemptID: "att-tx", AdapterName: "opencode", NativeSessionID: &native, ProtocolVersion: &ver, Extra: `{"x":1}`})
	})
	if err != nil {
		t.Fatalf("WithTx put: %v", err)
	}
	got, err := s.Transport().Get(ctx, "att-tx")
	if err != nil {
		t.Fatalf("Get after tx: %v", err)
	}
	if got.AdapterName != "opencode" {
		t.Fatalf("after tx AdapterName = %q", got.AdapterName)
	}
	// Triangulate: rollback on error should not leave partial attempt nor transport
	err = s.WithTx(ctx, func(tx Store) error {
		if _, err := tx.(*SQLiteStore).db.ExecContext(ctx, "INSERT INTO generations(id, execution_id, step_id, number, created_at) VALUES (?, ?, ?, ?, ?)", "gen-tx2", "exec-tx", "step-tx", 2, "2025-01-01T00:00:01Z"); err != nil {
			return err
		}
		if err := tx.Attempts().Create(ctx, &Attempt{ID: "att-rollback", ExecutionID: "exec-tx", StepID: "step-tx", GenerationID: "gen-tx2", Status: "running", StartedAt: "2025-01-01T00:00:01Z"}); err != nil {
			return err
		}
		if err := tx.Transport().Put(ctx, &Transport{AttemptID: "att-rollback", AdapterName: "opencode", Extra: "{}"}); err != nil {
			return err
		}
		return context.Canceled
	})
	if err == nil {
		t.Fatalf("expected rollback error")
	}
	_, err = s.Attempts().Get(ctx, "att-rollback")
	if err == nil {
		t.Fatalf("attempt should not exist after rollback")
	}
	_, err = s.Transport().Get(ctx, "att-rollback")
	if err == nil {
		t.Fatalf("transport should not exist after rollback")
	}
}

func setupAttempt(t *testing.T, s Store, ctx context.Context, execID, stepID, genID, attemptID string) {
	t.Helper()
	sqlite, ok := s.(*SQLiteStore)
	if !ok {
		t.Fatalf("not SQLiteStore")
	}
	// Ensure project/execution/step exist (idempotent)
	_, _ = sqlite.db.ExecContext(ctx, "INSERT OR IGNORE INTO projects(id, root_path, created_at) VALUES (?, ?, ?)", "proj1", "/tmp/proj1", "2025-01-01T00:00:00Z")
	_, _ = sqlite.db.ExecContext(ctx, "INSERT OR IGNORE INTO executions(id, project_id, workflow_source, status, workspace_mode, workspace_root, started_at) VALUES (?, ?, ?, ?, ?, ?, ?)", execID, "proj1", "wf.yaml", "running", "isolated", "/tmp/ws", "2025-01-01T00:00:00Z")
	_, _ = sqlite.db.ExecContext(ctx, "INSERT OR IGNORE INTO execution_steps(execution_id, step_id, type, status, workspace_mode, current_generation) VALUES (?, ?, ?, ?, ?, ?)", execID, stepID, "agent", "running", "isolated", 1)
	_, _ = sqlite.db.ExecContext(ctx, "INSERT OR IGNORE INTO generations(id, execution_id, step_id, number, created_at) VALUES (?, ?, ?, ?, ?)", genID, execID, stepID, 1, "2025-01-01T00:00:00Z")
	// For second generation with same exec/step but different genID need different number; handle duplicate by incrementing
	if genID == "gen2" {
		_, _ = sqlite.db.ExecContext(ctx, "INSERT OR IGNORE INTO generations(id, execution_id, step_id, number, created_at) VALUES (?, ?, ?, ?, ?)", genID, execID, stepID, 2, "2025-01-01T00:00:01Z")
	}
	if attemptID == "att2" {
		// already handled via gen2
	}
	// Use attempts repo for creation
	err := s.Attempts().Create(ctx, &Attempt{ID: attemptID, ExecutionID: execID, StepID: stepID, GenerationID: genID, Status: "running", StartedAt: "2025-01-01T00:00:00Z"})
	if err != nil && !strings.Contains(err.Error(), "UNIQUE") && !strings.Contains(err.Error(), "constraint") {
		// ignore duplicate attempt?
		t.Fatalf("setup attempt %s: %v", attemptID, err)
	}
}
