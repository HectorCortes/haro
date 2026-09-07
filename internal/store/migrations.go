package store

import (
	"context"
	"database/sql"
	"strings"
)

// migrationStatements returns the canonical DDL statements (IF NOT EXISTS).
// It is split for test seam injection.
func migrationStatements() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS projects (
			id TEXT PRIMARY KEY,
			root_path TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS executions (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL REFERENCES projects(id),
			workflow_source TEXT NOT NULL,
			status TEXT NOT NULL CHECK (status IN ('pending','running','completed','failed')),
			workspace_mode TEXT NOT NULL CHECK (workspace_mode IN ('isolated','shared')),
			workspace_root TEXT NOT NULL,
			started_at TEXT NOT NULL,
			ended_at TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS idx_executions_project ON executions(project_id, status);`,
		`CREATE TABLE IF NOT EXISTS execution_steps (
			execution_id TEXT NOT NULL REFERENCES executions(id),
			step_id TEXT NOT NULL,
			type TEXT NOT NULL CHECK (type IN ('command','agent','workflow')),
			status TEXT NOT NULL CHECK (status IN ('pending','running','completed','failed','skipped')),
			depends_on TEXT NOT NULL DEFAULT '[]',
			requires TEXT NOT NULL DEFAULT '[]',
			produces TEXT NOT NULL DEFAULT '[]',
			workspace_mode TEXT NOT NULL CHECK (workspace_mode IN ('isolated','shared')),
			current_generation INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (execution_id, step_id)
		);`,
		`CREATE TABLE IF NOT EXISTS generations (
			id TEXT PRIMARY KEY,
			execution_id TEXT NOT NULL,
			step_id TEXT NOT NULL,
			number INTEGER NOT NULL,
			created_at TEXT NOT NULL,
			invalidated_at TEXT,
			invalidated_by_step TEXT,
			FOREIGN KEY (execution_id, step_id) REFERENCES execution_steps(execution_id, step_id),
			UNIQUE (execution_id, step_id, number)
		);`,
		`CREATE TABLE IF NOT EXISTS attempts (
			id TEXT PRIMARY KEY,
			execution_id TEXT NOT NULL,
			step_id TEXT NOT NULL,
			generation_id TEXT NOT NULL REFERENCES generations(id),
			status TEXT NOT NULL CHECK (status IN ('running','completed','failed','cancelled')),
			started_at TEXT NOT NULL,
			ended_at TEXT,
			termination_reason TEXT,
			result_digest TEXT,
			FOREIGN KEY (execution_id, step_id) REFERENCES execution_steps(execution_id, step_id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_attempts_step ON attempts(execution_id, step_id, status);`,
		`CREATE TABLE IF NOT EXISTS attempt_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			attempt_id TEXT NOT NULL REFERENCES attempts(id),
			cursor INTEGER NOT NULL,
			event_type TEXT NOT NULL,
			payload_ref TEXT,
			occurred_at TEXT NOT NULL,
			UNIQUE (attempt_id, cursor)
		);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_attempt_events_cursor ON attempt_events(attempt_id, cursor);`,
		`CREATE TABLE IF NOT EXISTS step_transition_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			execution_id TEXT NOT NULL,
			step_id TEXT NOT NULL,
			cursor INTEGER NOT NULL,
			from_status TEXT,
			to_status TEXT NOT NULL,
			occurred_at TEXT NOT NULL,
			UNIQUE (execution_id, step_id, cursor)
		);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_step_events_cursor ON step_transition_events(execution_id, step_id, cursor);`,
		`CREATE TABLE IF NOT EXISTS attempt_transport (
			attempt_id TEXT PRIMARY KEY REFERENCES attempts(id),
			adapter_name TEXT NOT NULL,
			native_session_id TEXT,
			protocol_version INTEGER,
			extra TEXT NOT NULL DEFAULT '{}'
		);`,
		`CREATE TABLE IF NOT EXISTS path_claims (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project_id TEXT NOT NULL REFERENCES projects(id),
			logical_path TEXT NOT NULL,
			mode TEXT NOT NULL,
			owner_execution_id TEXT NOT NULL,
			owner_step_id TEXT NOT NULL,
			acquired_at TEXT NOT NULL,
			released_at TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS idx_path_claims_active ON path_claims(project_id, logical_path) WHERE released_at IS NULL;`,
		`CREATE TABLE IF NOT EXISTS leases (execution_id TEXT NOT NULL, step_id TEXT NOT NULL, holder TEXT NOT NULL, fencing_token INTEGER NOT NULL, acquired_at TEXT NOT NULL, expires_at TEXT NOT NULL, PRIMARY KEY (execution_id, step_id));`,
		`CREATE TABLE IF NOT EXISTS interactions (id TEXT PRIMARY KEY, attempt_id TEXT NOT NULL REFERENCES attempts(id), type TEXT NOT NULL CHECK (type IN ('permission','question')), status TEXT NOT NULL CHECK (status IN ('pending','resolved')), decision TEXT, idempotency_key TEXT NOT NULL, resolved_at TEXT, UNIQUE (attempt_id, idempotency_key));`,
	}
}

// migrateWithStatements executes statements transactionally. It is the test seam for injected failures.
func migrateWithStatements(ctx context.Context, db *sql.DB, statements []string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	// Ensure rollback on error
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	for _, stmt := range statements {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

// migrate creates the DDL idempotently with CHECKs, FK, WAL.
// It is called once per connection after PRAGMA foreign_keys=ON and journal_mode=WAL.
func migrate(ctx context.Context, db *sql.DB) error {
	if err := migrateWithStatements(ctx, db, migrationStatements()); err != nil {
		return err
	}
	// First ALTER migration: dag_hash on executions (nullable, idempotent)
	if err := ensureDagHashColumn(ctx, db); err != nil {
		return err
	}
	if err := ensureBaseCommitColumn(ctx, db); err != nil {
		return err
	}
	return nil
}

func ensureDagHashColumn(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, "PRAGMA table_info(executions)")
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	has := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == "dag_hash" {
			has = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !has {
		if _, err := db.ExecContext(ctx, "ALTER TABLE executions ADD COLUMN dag_hash TEXT"); err != nil {
			// If column already exists (race), ignore duplicate error
			if !isDuplicateColumnError(err) {
				return err
			}
		}
	}
	return nil
}

func ensureBaseCommitColumn(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, "PRAGMA table_info(executions)")
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	has := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == "base_commit" {
			has = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !has {
		if _, err := db.ExecContext(ctx, "ALTER TABLE executions ADD COLUMN base_commit TEXT"); err != nil {
			if !isDuplicateColumnError(err) {
				return err
			}
		}
	}
	return nil
}

func isDuplicateColumnError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate column") || strings.Contains(msg, "already exists")
}
