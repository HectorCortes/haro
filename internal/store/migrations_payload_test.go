package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestAttemptEventsPayloadSchema proves v2-store/F-02: a fresh database
// exposes attempt_events with both payload columns — the new nullable inline
// `payload TEXT` alongside the retained legacy `payload_ref`.
func TestAttemptEventsPayloadSchema(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "payload-schema.db")
	s, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = s.Close() }()

	rows, err := s.QueryForTest(ctx, "PRAGMA table_info(attempt_events)")
	if err != nil {
		t.Fatalf("table_info: %v", err)
	}
	type col struct {
		name    string
		typ     string
		notnull int
	}
	var cols []col
	for rows.Next() {
		var cid int
		var c col
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &c.name, &c.typ, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		c.notnull = notnull
		cols = append(cols, c)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("rows err: %v", err)
	}

	var payload *col
	for i := range cols {
		if cols[i].name == "payload" {
			payload = &cols[i]
		}
	}
	if payload == nil {
		t.Fatalf("attempt_events has no payload column, got %+v", cols)
	}
	if payload.typ != "TEXT" {
		t.Fatalf("payload type = %q, want TEXT", payload.typ)
	}
	if payload.notnull != 0 {
		t.Fatalf("payload must be nullable (notnull=%d)", payload.notnull)
	}

	// The canonical DDL declares the inline payload next to the legacy ref.
	var sqlDef string
	if err := s.db.QueryRowContext(ctx, "SELECT sql FROM sqlite_master WHERE type='table' AND name='attempt_events'").Scan(&sqlDef); err != nil {
		t.Fatalf("read DDL: %v", err)
	}
	if !strings.Contains(sqlDef, "payload TEXT") {
		t.Fatalf("attempt_events DDL missing `payload TEXT`: %q", sqlDef)
	}
	if !strings.Contains(sqlDef, "payload_ref TEXT") {
		t.Fatalf("attempt_events DDL must retain `payload_ref TEXT`: %q", sqlDef)
	}
}

// TestMigratePayloadIdempotent proves v2-store/U-02: a pre-payload database
// gains attempt_events.payload additively; repeated and concurrent opens all
// succeed and leave existing records unchanged.
func TestMigratePayloadIdempotent(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "pre-payload.db")

	// Build a legacy (pre-payload) database: create the current schema, then
	// rewind attempt_events to its historical shape without `payload`.
	legacy, err := sql.Open("sqlite", "file:"+dbPath+"?cache=shared&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	if err := migrate(ctx, legacy); err != nil {
		t.Fatalf("migrate legacy: %v", err)
	}
	for _, stmt := range []string{
		`DROP TABLE attempt_events;`,
		`CREATE TABLE attempt_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			attempt_id TEXT NOT NULL REFERENCES attempts(id),
			cursor INTEGER NOT NULL,
			event_type TEXT NOT NULL,
			payload_ref TEXT,
			occurred_at TEXT NOT NULL,
			UNIQUE (attempt_id, cursor)
		);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_attempt_events_cursor ON attempt_events(attempt_id, cursor);`,
	} {
		if _, err := legacy.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("rewind stmt %q: %v", stmt, err)
		}
	}
	// Seed records into the legacy database.
	if _, err := legacy.ExecContext(ctx, `INSERT INTO projects(id, root_path, created_at) VALUES ('proj-mig', '/tmp/proj', '2025-01-01T00:00:00Z')`); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if _, err := legacy.ExecContext(ctx, `INSERT INTO executions(id, project_id, workflow_source, status, workspace_mode, workspace_root, started_at) VALUES ('exec-mig', 'proj-mig', 'wf.yaml', 'pending', 'isolated', '/tmp/ws', '2025-01-01T00:00:00Z')`); err != nil {
		t.Fatalf("seed execution: %v", err)
	}
	if _, err := legacy.ExecContext(ctx, `INSERT INTO execution_steps(execution_id, step_id, type, status, depends_on, requires, produces, workspace_mode, current_generation) VALUES ('exec-mig', 's1', 'command', 'pending', '[]', '[]', '[]', 'isolated', 0)`); err != nil {
		t.Fatalf("seed step: %v", err)
	}
	if _, err := legacy.ExecContext(ctx, `INSERT INTO generations(id, execution_id, step_id, number, created_at) VALUES ('gen-mig', 'exec-mig', 's1', 1, '2025-01-01T00:00:00Z')`); err != nil {
		t.Fatalf("seed generation: %v", err)
	}
	if _, err := legacy.ExecContext(ctx, `INSERT INTO attempts(id, execution_id, step_id, generation_id, status, started_at) VALUES ('att-mig', 'exec-mig', 's1', 'gen-mig', 'running', '2025-01-01T00:00:00Z')`); err != nil {
		t.Fatalf("seed attempt: %v", err)
	}
	if _, err := legacy.ExecContext(ctx, `INSERT INTO attempt_events(attempt_id, cursor, event_type, payload_ref, occurred_at) VALUES ('att-mig', 0, 'output_delta', 'legacy/evidence.txt', '2025-01-01T00:00:01Z')`); err != nil {
		t.Fatalf("seed event 0: %v", err)
	}
	if _, err := legacy.ExecContext(ctx, `INSERT INTO attempt_events(attempt_id, cursor, event_type, payload_ref, occurred_at) VALUES ('att-mig', 1, 'feedback', NULL, '2025-01-01T00:00:02Z')`); err != nil {
		t.Fatalf("seed event 1: %v", err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatalf("close legacy: %v", err)
	}

	// Repeated opens succeed.
	for i := 0; i < 3; i++ {
		s, err := Open(ctx, dbPath)
		if err != nil {
			t.Fatalf("open #%d: %v", i, err)
		}
		if err := s.Close(); err != nil {
			t.Fatalf("close #%d: %v", i, err)
		}
	}

	// Concurrent opens succeed.
	var wg sync.WaitGroup
	errCh := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := Open(ctx, dbPath)
			if err != nil {
				errCh <- err
				return
			}
			_ = s.Close()
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent open: %v", err)
		}
	}

	// Records unchanged; exactly one nullable payload column.
	s, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("final open: %v", err)
	}
	defer func() { _ = s.Close() }()

	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM attempt_events`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("record count = %d err %v, want 2 nil", n, err)
	}
	var ref0 string
	if err := s.db.QueryRowContext(ctx, `SELECT payload_ref FROM attempt_events WHERE cursor = 0`).Scan(&ref0); err != nil || ref0 != "legacy/evidence.txt" {
		t.Fatalf("event 0 payload_ref = %q err %v, want legacy/evidence.txt", ref0, err)
	}
	var nullRef sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT payload_ref FROM attempt_events WHERE cursor = 1`).Scan(&nullRef); err != nil || nullRef.Valid {
		t.Fatalf("event 1 payload_ref = %+v err %v, want NULL", nullRef, err)
	}
	var payloadCount, refCount int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('attempt_events') WHERE name = 'payload'`).Scan(&payloadCount); err != nil || payloadCount != 1 {
		t.Fatalf("payload column count = %d err %v, want 1", payloadCount, err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('attempt_events') WHERE name = 'payload_ref'`).Scan(&refCount); err != nil || refCount != 1 {
		t.Fatalf("payload_ref column count = %d err %v, want 1", refCount, err)
	}
}
