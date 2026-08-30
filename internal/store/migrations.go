package store

import (
	"context"
	"database/sql"
)

// migrate creates the 7-table DDL idempotently with CHECKs, FK, WAL.
// It is called once per connection after PRAGMA foreign_keys=ON and journal_mode=WAL.
func migrate(ctx context.Context, db *sql.DB) error {
	statements := []string{
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
	}
	for _, stmt := range statements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
