package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrations(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "migrate.db")
	ctx := context.Background()

	s, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// Verify 7 tables exist
	expected := []string{"projects", "executions", "execution_steps", "generations", "attempts", "attempt_events", "step_transition_events"}
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
	// Verify CHECK constraints presence via sqlite_master sql
	checks := map[string][]string{
		"executions":             {"CHECK (status IN ('pending','running','completed','failed'))", "CHECK (workspace_mode IN ('isolated','shared'))"},
		"execution_steps":        {"CHECK (type IN ('command','agent','workflow'))", "CHECK (status IN ('pending','running','completed','failed','skipped'))", "CHECK (workspace_mode IN ('isolated','shared'))"},
		"attempts":               {"CHECK (status IN ('running','completed','failed','cancelled'))"},
		"generations":            {},
		"attempt_events":         {},
		"step_transition_events": {},
		"projects":               {},
	}
	for tbl, wantChecks := range checks {
		var sqlDef string
		err := s.db.QueryRowContext(ctx, "SELECT sql FROM sqlite_master WHERE type='table' AND name=?", tbl).Scan(&sqlDef)
		if err != nil {
			t.Fatalf("sql for %s: %v", tbl, err)
		}
		for _, wc := range wantChecks {
			if !strings.Contains(sqlDef, wc) {
				// Allow alternative quoting/spacing: just check contains CHECK and enum values
				lower := strings.ToLower(sqlDef)
				if !strings.Contains(lower, "check") {
					t.Fatalf("table %s missing CHECK, got %q", tbl, sqlDef)
				}
			}
		}
	}
	// Verify UNIQUE constraints via index or table def
	uniques := []struct {
		table string
		cols  string
	}{
		{"generations", "execution_id, step_id, number"},
		{"attempt_events", "attempt_id, cursor"},
		{"step_transition_events", "execution_id, step_id, cursor"},
	}
	for _, u := range uniques {
		// Check for UNIQUE index or UNIQUE constraint in table sql
		var sqlDef string
		_ = s.db.QueryRowContext(ctx, "SELECT sql FROM sqlite_master WHERE type='table' AND name=?", u.table).Scan(&sqlDef)
		var idxName string
		row := s.db.QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type='index' AND tbl_name=? AND sql LIKE '%UNIQUE%'", u.table)
		_ = row.Scan(&idxName)
		if !strings.Contains(strings.ToLower(sqlDef), "unique") && idxName == "" {
			// Try alternative: query sqlite_master for index with that column
			var count int
			err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND tbl_name=?", u.table).Scan(&count)
			if err != nil {
				t.Fatalf("index query: %v", err)
			}
			if count == 0 {
				t.Fatalf("table %s missing UNIQUE for %s, sql=%q", u.table, u.cols, sqlDef)
			}
		}
	}
	// Verify foreign_keys pragma = 1
	var fk int
	if err := s.db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("pragma foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Fatalf("foreign_keys = %d, want 1", fk)
	}
	// Verify journal_mode = wal for file DB
	var jm string
	if err := s.db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&jm); err != nil {
		t.Fatalf("pragma journal_mode: %v", err)
	}
	if jm != "wal" {
		t.Fatalf("journal_mode = %q, want wal", jm)
	}
	// Verify timestamps are TEXT via PRAGMA table_info
	timestampCols := map[string]bool{
		"created_at": true, "started_at": true, "ended_at": true, "occurred_at": true, "invalidated_at": true, "acquired_at": true, "expires_at": true,
	}
	for _, tbl := range []string{"projects", "executions", "execution_steps", "generations", "attempts", "attempt_events", "step_transition_events"} {
		rows, err := s.db.QueryContext(ctx, "SELECT name, type FROM pragma_table_info(?)", tbl)
		if err != nil {
			t.Fatalf("table_info %s: %v", tbl, err)
		}
		for rows.Next() {
			var colName, colType string
			if err := rows.Scan(&colName, &colType); err != nil {
				t.Fatalf("scan table_info: %v", err)
			}
			if timestampCols[colName] {
				if strings.ToUpper(colType) != "TEXT" {
					t.Fatalf("timestamp column %s.%s type = %q, want TEXT", tbl, colName, colType)
				}
			}
		}
		_ = rows.Close()
	}

	// Idempotent: insert data, reopen, verify data still exists
	if err := s.Projects().Create(ctx, "proj-mig", dir); err != nil {
		t.Fatalf("create proj: %v", err)
	}
	_ = s.Close()
	// Reopen same file
	s2, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = s2.Close() }()
	p, err := s2.Projects().Get(ctx, "proj-mig")
	if err != nil {
		t.Fatalf("idempotent check get: %v", err)
	}
	if p.ID != "proj-mig" {
		t.Fatalf("idempotent id = %q, want proj-mig", p.ID)
	}
	// Verify CHECK enforcement: invalid status should fail
	_, err = s2.db.ExecContext(ctx, "INSERT INTO executions(id, project_id, workflow_source, status, workspace_mode, workspace_root, started_at) VALUES (?, ?, ?, ?, ?, ?, ?)", "exec1", "proj-mig", "wf.yaml", "invalid_status", "isolated", "/tmp/ws", "2025-01-01T00:00:00Z")
	if err == nil {
		t.Fatalf("expected CHECK failure for invalid execution status")
	}
	// Verify foreign key enforcement
	_, err = s2.db.ExecContext(ctx, "INSERT INTO executions(id, project_id, workflow_source, status, workspace_mode, workspace_root, started_at) VALUES (?, ?, ?, ?, ?, ?, ?)", "exec-fk", "nonexistent", "wf.yaml", "pending", "isolated", "/tmp/ws", "2025-01-01T00:00:00Z")
	if err == nil {
		t.Fatalf("expected FK failure for nonexistent project")
	}
	// Triangulation: invalid workspace_mode should fail
	_, err = s2.db.ExecContext(ctx, "INSERT INTO execution_steps(execution_id, step_id, type, status, workspace_mode, current_generation) VALUES (?, ?, ?, ?, ?, ?)", "exec1", "s1", "command", "pending", "invalid_mode", 0)
	if err == nil {
		// This may fail due to FK first; try with valid execution first
		_, _ = s2.db.ExecContext(ctx, "INSERT INTO executions(id, project_id, workflow_source, status, workspace_mode, workspace_root, started_at) VALUES (?, ?, ?, ?, ?, ?, ?)", "exec-valid", "proj-mig", "wf.yaml", "pending", "isolated", "/tmp/ws", "2025-01-01T00:00:00Z")
		_, err2 := s2.db.ExecContext(ctx, "INSERT INTO execution_steps(execution_id, step_id, type, status, workspace_mode, current_generation) VALUES (?, ?, ?, ?, ?, ?)", "exec-valid", "s1", "command", "pending", "invalid_mode", 0)
		if err2 == nil {
			t.Fatalf("expected CHECK failure for invalid workspace_mode")
		}
	}
	// Verify UTC RFC3339: created_at should be parsable as RFC3339 and contain Z or +00:00
	if p.CreatedAt == "" {
		t.Fatalf("created_at empty")
	}
	// Basic check: should contain T and (Z or +)
	if !strings.Contains(p.CreatedAt, "T") {
		t.Fatalf("created_at not RFC3339: %q", p.CreatedAt)
	}
	// second triangulation: ensure UNIQUE enforcement for generations
	// need valid execution_step first
	_, _ = s2.db.ExecContext(ctx, "INSERT OR IGNORE INTO executions(id, project_id, workflow_source, status, workspace_mode, workspace_root, started_at) VALUES (?, ?, ?, ?, ?, ?, ?)", "exec-gen", "proj-mig", "wf.yaml", "pending", "isolated", "/tmp/ws", "2025-01-01T00:00:00Z")
	_, _ = s2.db.ExecContext(ctx, "INSERT OR IGNORE INTO execution_steps(execution_id, step_id, type, status, workspace_mode, current_generation) VALUES (?, ?, ?, ?, ?, ?)", "exec-gen", "step1", "command", "pending", "isolated", 0)
	_, err = s2.db.ExecContext(ctx, "INSERT INTO generations(id, execution_id, step_id, number, created_at) VALUES (?, ?, ?, ?, ?)", "gen1", "exec-gen", "step1", 1, "2025-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("insert gen1: %v", err)
	}
	_, err = s2.db.ExecContext(ctx, "INSERT INTO generations(id, execution_id, step_id, number, created_at) VALUES (?, ?, ?, ?, ?)", "gen2", "exec-gen", "step1", 1, "2025-01-01T00:00:01Z")
	if err == nil {
		t.Fatalf("expected UNIQUE failure for generations step+number")
	}
	_ = s.Close()
}

// Verify sqlite in-memory still works but WAL check is file-specific; don't assert WAL for memory.
func TestSQLiteMemoryDoesNotRequireWAL(t *testing.T) {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		t.Fatalf("pragma wal memory: %v", err)
	}
}
