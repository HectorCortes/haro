package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// helper query
func (s *SQLiteStore) query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if s.tx != nil {
		return s.tx.QueryContext(ctx, query, args...)
	}
	return s.db.QueryContext(ctx, query, args...)
}

// executionsRepo

type executionsRepo struct{ store *SQLiteStore }

func (r *executionsRepo) Create(ctx context.Context, e *Execution) error {
	if e.StartedAt == "" {
		e.StartedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := r.store.exec(ctx, `INSERT INTO executions(id, project_id, workflow_source, status, workspace_mode, workspace_root, started_at, ended_at, dag_hash, base_commit) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.ProjectID, e.WorkflowSource, e.Status, e.WorkspaceMode, e.WorkspaceRoot, e.StartedAt, e.EndedAt, e.DagHash, e.BaseCommit)
	if err != nil && isMissingColumn(err, "base_commit") {
		_, err = r.store.exec(ctx, `INSERT INTO executions(id, project_id, workflow_source, status, workspace_mode, workspace_root, started_at, ended_at, dag_hash) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			e.ID, e.ProjectID, e.WorkflowSource, e.Status, e.WorkspaceMode, e.WorkspaceRoot, e.StartedAt, e.EndedAt, e.DagHash)
	}
	if err != nil && isMissingColumn(err, "dag_hash") {
		_, err = r.store.exec(ctx, `INSERT INTO executions(id, project_id, workflow_source, status, workspace_mode, workspace_root, started_at, ended_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			e.ID, e.ProjectID, e.WorkflowSource, e.Status, e.WorkspaceMode, e.WorkspaceRoot, e.StartedAt, e.EndedAt)
	}
	if err != nil {
		return normalizeSQLiteError(err)
	}
	return nil
}

func (r *executionsRepo) Get(ctx context.Context, id string) (*Execution, error) {
	row := r.store.queryRow(ctx, `SELECT id, project_id, workflow_source, status, workspace_mode, workspace_root, started_at, ended_at, dag_hash, base_commit FROM executions WHERE id = ?`, id)
	var e Execution
	var endedAt sql.NullString
	var dagHash sql.NullString
	var baseCommit sql.NullString
	if err := row.Scan(&e.ID, &e.ProjectID, &e.WorkflowSource, &e.Status, &e.WorkspaceMode, &e.WorkspaceRoot, &e.StartedAt, &endedAt, &dagHash, &baseCommit); err != nil {
		if isMissingColumn(err, "base_commit") {
			row2 := r.store.queryRow(ctx, `SELECT id, project_id, workflow_source, status, workspace_mode, workspace_root, started_at, ended_at, dag_hash FROM executions WHERE id = ?`, id)
			var e2 Execution
			var ended2 sql.NullString
			var dag2 sql.NullString
			if err2 := row2.Scan(&e2.ID, &e2.ProjectID, &e2.WorkflowSource, &e2.Status, &e2.WorkspaceMode, &e2.WorkspaceRoot, &e2.StartedAt, &ended2, &dag2); err2 != nil {
				if isMissingColumn(err2, "dag_hash") {
					row3 := r.store.queryRow(ctx, `SELECT id, project_id, workflow_source, status, workspace_mode, workspace_root, started_at, ended_at FROM executions WHERE id = ?`, id)
					var e3 Execution
					var ended3 sql.NullString
					if err3 := row3.Scan(&e3.ID, &e3.ProjectID, &e3.WorkflowSource, &e3.Status, &e3.WorkspaceMode, &e3.WorkspaceRoot, &e3.StartedAt, &ended3); err3 != nil {
						return nil, normalizeSQLiteError(err3)
					}
					if ended3.Valid {
						e3.EndedAt = &ended3.String
					}
					return &e3, nil
				}
				return nil, normalizeSQLiteError(err2)
			}
			if ended2.Valid {
				e2.EndedAt = &ended2.String
			}
			if dag2.Valid {
				e2.DagHash = &dag2.String
			}
			return &e2, nil
		}
		if isMissingColumn(err, "dag_hash") {
			row2 := r.store.queryRow(ctx, `SELECT id, project_id, workflow_source, status, workspace_mode, workspace_root, started_at, ended_at FROM executions WHERE id = ?`, id)
			var e2 Execution
			var ended2 sql.NullString
			if err2 := row2.Scan(&e2.ID, &e2.ProjectID, &e2.WorkflowSource, &e2.Status, &e2.WorkspaceMode, &e2.WorkspaceRoot, &e2.StartedAt, &ended2); err2 != nil {
				return nil, normalizeSQLiteError(err2)
			}
			if ended2.Valid {
				e2.EndedAt = &ended2.String
			}
			return &e2, nil
		}
		return nil, normalizeSQLiteError(err)
	}
	if endedAt.Valid {
		e.EndedAt = &endedAt.String
	}
	if dagHash.Valid {
		e.DagHash = &dagHash.String
	}
	if baseCommit.Valid {
		e.BaseCommit = &baseCommit.String
	}
	return &e, nil
}

func isMissingColumn(err error, col string) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "no such column") && strings.Contains(msg, col)
}

var _ = isMissingColumn

func (r *executionsRepo) UpdateStatus(ctx context.Context, id, status string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	var endedAt any
	if status == "completed" || status == "failed" {
		endedAt = now
	}
	_, err := r.store.exec(ctx, `UPDATE executions SET status = ?, ended_at = COALESCE(?, ended_at) WHERE id = ?`, status, endedAt, id)
	if err != nil {
		return normalizeSQLiteError(err)
	}
	return nil
}

// stepsRepo

type stepsRepo struct{ store *SQLiteStore }

func (r *stepsRepo) Create(ctx context.Context, s *ExecutionStep) error {
	_, err := r.store.exec(ctx, `INSERT INTO execution_steps(execution_id, step_id, type, status, depends_on, requires, produces, workspace_mode, current_generation) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ExecutionID, s.StepID, s.Type, s.Status, s.DependsOn, s.Requires, s.Produces, s.WorkspaceMode, s.CurrentGeneration)
	if err != nil {
		return normalizeSQLiteError(err)
	}
	return nil
}

func (r *stepsRepo) Get(ctx context.Context, executionID, stepID string) (*ExecutionStep, error) {
	row := r.store.queryRow(ctx, `SELECT execution_id, step_id, type, status, depends_on, requires, produces, workspace_mode, current_generation FROM execution_steps WHERE execution_id = ? AND step_id = ?`, executionID, stepID)
	var s ExecutionStep
	if err := row.Scan(&s.ExecutionID, &s.StepID, &s.Type, &s.Status, &s.DependsOn, &s.Requires, &s.Produces, &s.WorkspaceMode, &s.CurrentGeneration); err != nil {
		return nil, normalizeSQLiteError(err)
	}
	return &s, nil
}

func (r *stepsRepo) List(ctx context.Context, executionID string) ([]*ExecutionStep, error) {
	rows, err := r.store.query(ctx, `SELECT execution_id, step_id, type, status, depends_on, requires, produces, workspace_mode, current_generation FROM execution_steps WHERE execution_id = ? ORDER BY step_id`, executionID)
	if err != nil {
		return nil, normalizeSQLiteError(err)
	}
	defer func() { _ = rows.Close() }()
	var out []*ExecutionStep
	for rows.Next() {
		var s ExecutionStep
		if err := rows.Scan(&s.ExecutionID, &s.StepID, &s.Type, &s.Status, &s.DependsOn, &s.Requires, &s.Produces, &s.WorkspaceMode, &s.CurrentGeneration); err != nil {
			return nil, normalizeSQLiteError(err)
		}
		out = append(out, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, normalizeSQLiteError(err)
	}
	return out, nil
}

func (r *stepsRepo) UpdateStatus(ctx context.Context, executionID, stepID, status string) error {
	_, err := r.store.exec(ctx, `UPDATE execution_steps SET status = ? WHERE execution_id = ? AND step_id = ?`, status, executionID, stepID)
	if err != nil {
		return normalizeSQLiteError(err)
	}
	return nil
}

func (r *stepsRepo) UpdateGeneration(ctx context.Context, executionID, stepID string, gen int) error {
	_, err := r.store.exec(ctx, `UPDATE execution_steps SET current_generation = ? WHERE execution_id = ? AND step_id = ?`, gen, executionID, stepID)
	if err != nil {
		return normalizeSQLiteError(err)
	}
	return nil
}

// attemptsRepo

type attemptsRepo struct{ store *SQLiteStore }

func (r *attemptsRepo) Create(ctx context.Context, a *Attempt) error {
	if a.StartedAt == "" {
		a.StartedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := r.store.exec(ctx, `INSERT INTO attempts(id, execution_id, step_id, generation_id, status, started_at, ended_at, termination_reason, result_digest) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.ExecutionID, a.StepID, a.GenerationID, a.Status, a.StartedAt, a.EndedAt, a.TerminationReason, a.ResultDigest)
	if err != nil {
		return normalizeSQLiteError(err)
	}
	return nil
}

func (r *attemptsRepo) Get(ctx context.Context, id string) (*Attempt, error) {
	row := r.store.queryRow(ctx, `SELECT id, execution_id, step_id, generation_id, status, started_at, ended_at, termination_reason, result_digest FROM attempts WHERE id = ?`, id)
	var a Attempt
	var endedAt, term, digest sql.NullString
	if err := row.Scan(&a.ID, &a.ExecutionID, &a.StepID, &a.GenerationID, &a.Status, &a.StartedAt, &endedAt, &term, &digest); err != nil {
		return nil, normalizeSQLiteError(err)
	}
	if endedAt.Valid {
		a.EndedAt = &endedAt.String
	}
	if term.Valid {
		a.TerminationReason = &term.String
	}
	if digest.Valid {
		a.ResultDigest = &digest.String
	}
	return &a, nil
}

func (r *attemptsRepo) UpdateStatus(ctx context.Context, id, status string, endedAt *string, terminationReason *string, resultDigest *string) error {
	if endedAt == nil {
		now := time.Now().UTC().Format(time.RFC3339)
		endedAt = &now
	}
	_, err := r.store.exec(ctx, `UPDATE attempts SET status = ?, ended_at = ?, termination_reason = ?, result_digest = ? WHERE id = ?`, status, endedAt, terminationReason, resultDigest, id)
	if err != nil {
		return normalizeSQLiteError(err)
	}
	return nil
}

func (r *attemptsRepo) CountByStep(ctx context.Context, executionID, stepID string) (int, error) {
	row := r.store.queryRow(ctx, `SELECT COUNT(*) FROM attempts WHERE execution_id = ? AND step_id = ?`, executionID, stepID)
	var n int
	if err := row.Scan(&n); err != nil {
		return 0, normalizeSQLiteError(err)
	}
	return n, nil
}

// generationsRepo

type generationsRepo struct{ store *SQLiteStore }

func (r *generationsRepo) Create(ctx context.Context, g *Generation) error {
	if g.CreatedAt == "" {
		g.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := r.store.exec(ctx, `INSERT INTO generations(id, execution_id, step_id, number, created_at, invalidated_at, invalidated_by_step) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		g.ID, g.ExecutionID, g.StepID, g.Number, g.CreatedAt, g.InvalidatedAt, g.InvalidatedByStep)
	if err != nil {
		return normalizeSQLiteError(err)
	}
	return nil
}

func (r *generationsRepo) Get(ctx context.Context, id string) (*Generation, error) {
	row := r.store.queryRow(ctx, `SELECT id, execution_id, step_id, number, created_at, invalidated_at, invalidated_by_step FROM generations WHERE id = ?`, id)
	var g Generation
	var invAt, invBy sql.NullString
	if err := row.Scan(&g.ID, &g.ExecutionID, &g.StepID, &g.Number, &g.CreatedAt, &invAt, &invBy); err != nil {
		return nil, normalizeSQLiteError(err)
	}
	if invAt.Valid {
		g.InvalidatedAt = &invAt.String
	}
	if invBy.Valid {
		g.InvalidatedByStep = &invBy.String
	}
	return &g, nil
}

func (r *generationsRepo) GetByNumber(ctx context.Context, executionID, stepID string, number int) (*Generation, error) {
	row := r.store.queryRow(ctx, `SELECT id, execution_id, step_id, number, created_at, invalidated_at, invalidated_by_step FROM generations WHERE execution_id = ? AND step_id = ? AND number = ?`, executionID, stepID, number)
	var g Generation
	var invAt, invBy sql.NullString
	if err := row.Scan(&g.ID, &g.ExecutionID, &g.StepID, &g.Number, &g.CreatedAt, &invAt, &invBy); err != nil {
		return nil, normalizeSQLiteError(err)
	}
	if invAt.Valid {
		g.InvalidatedAt = &invAt.String
	}
	if invBy.Valid {
		g.InvalidatedByStep = &invBy.String
	}
	return &g, nil
}

func (r *generationsRepo) ListByStep(ctx context.Context, executionID, stepID string) ([]*Generation, error) {
	rows, err := r.store.query(ctx, `SELECT id, execution_id, step_id, number, created_at, invalidated_at, invalidated_by_step FROM generations WHERE execution_id = ? AND step_id = ? ORDER BY number`, executionID, stepID)
	if err != nil {
		return nil, normalizeSQLiteError(err)
	}
	defer func() { _ = rows.Close() }()
	var out []*Generation
	for rows.Next() {
		var g Generation
		var invAt, invBy sql.NullString
		if err := rows.Scan(&g.ID, &g.ExecutionID, &g.StepID, &g.Number, &g.CreatedAt, &invAt, &invBy); err != nil {
			return nil, normalizeSQLiteError(err)
		}
		if invAt.Valid {
			g.InvalidatedAt = &invAt.String
		}
		if invBy.Valid {
			g.InvalidatedByStep = &invBy.String
		}
		out = append(out, &g)
	}
	if err := rows.Err(); err != nil {
		return nil, normalizeSQLiteError(err)
	}
	return out, nil
}

func (r *generationsRepo) InvalidateByStep(ctx context.Context, executionID, stepID, invalidatedBy string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.store.exec(ctx, `UPDATE generations SET invalidated_at = ?, invalidated_by_step = ? WHERE execution_id = ? AND step_id = ? AND invalidated_at IS NULL`, now, invalidatedBy, executionID, stepID)
	if err != nil {
		return normalizeSQLiteError(err)
	}
	return nil
}

// eventsRepo

type eventsRepo struct{ store *SQLiteStore }

func (r *eventsRepo) CreateAttemptEvent(ctx context.Context, e *AttemptEvent) error {
	if e.OccurredAt == "" {
		e.OccurredAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := r.store.exec(ctx, `INSERT INTO attempt_events(attempt_id, cursor, event_type, payload_ref, occurred_at) VALUES (?, ?, ?, ?, ?)`,
		e.AttemptID, e.Cursor, e.EventType, e.PayloadRef, e.OccurredAt)
	if err != nil {
		return normalizeSQLiteError(err)
	}
	return nil
}

func (r *eventsRepo) CreateTransition(ctx context.Context, e *StepTransitionEvent) error {
	if e.OccurredAt == "" {
		e.OccurredAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := r.store.exec(ctx, `INSERT INTO step_transition_events(execution_id, step_id, cursor, from_status, to_status, occurred_at) VALUES (?, ?, ?, ?, ?, ?)`,
		e.ExecutionID, e.StepID, e.Cursor, e.FromStatus, e.ToStatus, e.OccurredAt)
	if err != nil {
		return normalizeSQLiteError(err)
	}
	return nil
}

func (r *eventsRepo) ListTransitions(ctx context.Context, executionID, stepID string) ([]*StepTransitionEvent, error) {
	rows, err := r.store.query(ctx, `SELECT id, execution_id, step_id, cursor, from_status, to_status, occurred_at FROM step_transition_events WHERE execution_id = ? AND step_id = ? ORDER BY cursor`, executionID, stepID)
	if err != nil {
		return nil, normalizeSQLiteError(err)
	}
	defer func() { _ = rows.Close() }()
	var out []*StepTransitionEvent
	for rows.Next() {
		var e StepTransitionEvent
		var from sql.NullString
		if err := rows.Scan(&e.ID, &e.ExecutionID, &e.StepID, &e.Cursor, &from, &e.ToStatus, &e.OccurredAt); err != nil {
			return nil, normalizeSQLiteError(err)
		}
		if from.Valid {
			e.FromStatus = &from.String
		}
		out = append(out, &e)
	}
	if err := rows.Err(); err != nil {
		return nil, normalizeSQLiteError(err)
	}
	return out, nil
}

func (r *eventsRepo) NextAttemptCursor(ctx context.Context, attemptID string) (int, error) {
	row := r.store.queryRow(ctx, `SELECT COALESCE(MAX(cursor), -1) + 1 FROM attempt_events WHERE attempt_id = ?`, attemptID)
	var n int
	if err := row.Scan(&n); err != nil {
		return 0, normalizeSQLiteError(err)
	}
	return n, nil
}

func (r *eventsRepo) NextTransitionCursor(ctx context.Context, executionID, stepID string) (int, error) {
	row := r.store.queryRow(ctx, `SELECT COALESCE(MAX(cursor), -1) + 1 FROM step_transition_events WHERE execution_id = ? AND step_id = ?`, executionID, stepID)
	var n int
	if err := row.Scan(&n); err != nil {
		return 0, normalizeSQLiteError(err)
	}
	return n, nil
}

// Ensure imports used
var _ = fmt.Sprintf
