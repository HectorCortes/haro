package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestMigrationRollbackOnInjectedFailure(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "rollback.db")

	// Open a raw DB without migration to test seam
	dsn := "file:" + dbPath + "?cache=shared&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("pragma: %v", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		t.Fatalf("pragma wal: %v", err)
	}

	stmts := migrationStatements()
	if len(stmts) < 2 {
		t.Fatalf("migrationStatements too short: %d", len(stmts))
	}
	// Inject bad statement mid-list
	badStmts := make([]string, 0, len(stmts)+1)
	badStmts = append(badStmts, stmts[0])
	badStmts = append(badStmts, "THIS IS NOT VALID SQL;")
	badStmts = append(badStmts, stmts[1:]...)

	err = migrateWithStatements(ctx, db, badStmts)
	if err == nil {
		t.Fatalf("expected error from bad statement")
	}

	// Verify no partial schema: leases should not exist (or if it was before bad stmt, rollback should remove it if in txn)
	// Since we use transaction, the first stmt should be rolled back
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='leases'").Scan(&count)
	if err != nil {
		t.Fatalf("query leases: %v", err)
	}
	if count != 0 {
		t.Fatalf("leases table should not exist after rollback, count=%d", count)
	}
	// Also check projects should not exist if transaction covered all
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='projects'").Scan(&count)
	if err != nil {
		t.Fatalf("query projects: %v", err)
	}
	if count != 0 {
		t.Fatalf("projects table should not exist after rollback, count=%d (transaction not atomic)", count)
	}

	// Retry with canonical list should succeed and create complete schema
	if err := migrateWithStatements(ctx, db, stmts); err != nil {
		t.Fatalf("retry migrate: %v", err)
	}
	// Verify leases now exists
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='leases'").Scan(&count)
	if err != nil {
		t.Fatalf("query leases after retry: %v", err)
	}
	if count != 1 {
		t.Fatalf("leases table should exist after retry, count=%d", count)
	}
	// Also verify interactions
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='interactions'").Scan(&count)
	if err != nil {
		t.Fatalf("query interactions: %v", err)
	}
	if count != 1 {
		t.Fatalf("interactions table should exist after retry, count=%d", count)
	}

	// Verify DB is usable via Store abstraction: reopen via Open should not error and be idempotent
	_ = db.Close()
	s, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open after retry: %v", err)
	}
	defer func() { _ = s.Close() }()
	// Should be able to create project
	if err := s.Projects().Create(ctx, "proj-rollback", dir); err != nil {
		t.Fatalf("create after rollback: %v", err)
	}
}
