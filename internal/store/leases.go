package store

import (
	"context"
	"database/sql"
	"time"
)

// leasesRepo implements LeaseRepository with fencing tokens.
type leasesRepo struct {
	store *SQLiteStore
}

func (r *leasesRepo) Acquire(ctx context.Context, executionID, stepID, holder string) (int64, error) {
	if executionID == "" || stepID == "" || holder == "" {
		return 0, ErrNotFound
	}
	// Use dedicated connection to serialize with BEGIN IMMEDIATE.
	conn, err := r.store.db.Conn(ctx)
	if err != nil {
		return 0, normalizeSQLiteError(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		return 0, normalizeSQLiteError(err)
	}
	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return 0, normalizeSQLiteError(err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()

	// Check existing lease
	var existingToken sql.NullInt64
	var existingHolder sql.NullString
	var existingExpires sql.NullString
	err = conn.QueryRowContext(ctx, `SELECT holder, fencing_token, expires_at FROM leases WHERE execution_id = ? AND step_id = ?`, executionID, stepID).Scan(&existingHolder, &existingToken, &existingExpires)
	now := time.Now().UTC().Format(time.RFC3339)
	expires := time.Now().UTC().Add(time.Minute).Format(time.RFC3339)
	var nextToken int64
	if err != nil {
		if err == sql.ErrNoRows {
			nextToken = 1
			_, err = conn.ExecContext(ctx, `INSERT INTO leases(execution_id, step_id, holder, fencing_token, acquired_at, expires_at) VALUES (?, ?, ?, ?, ?, ?)`, executionID, stepID, holder, nextToken, now, expires)
			if err != nil {
				return 0, normalizeSQLiteError(err)
			}
		} else {
			return 0, normalizeSQLiteError(err)
		}
	} else {
		// Existing row: compute MAX+1 retained (single row, so +1)
		nextToken = existingToken.Int64 + 1
		_, err = conn.ExecContext(ctx, `UPDATE leases SET holder = ?, fencing_token = ?, acquired_at = ?, expires_at = ? WHERE execution_id = ? AND step_id = ?`, holder, nextToken, now, expires, executionID, stepID)
		if err != nil {
			return 0, normalizeSQLiteError(err)
		}
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return 0, normalizeSQLiteError(err)
	}
	committed = true
	return nextToken, nil
}

func (r *leasesRepo) Renew(ctx context.Context, executionID, stepID, holder string, fencingToken int64) error {
	expires := time.Now().UTC().Add(time.Minute).Format(time.RFC3339)
	var res sql.Result
	var err error
	if r.store.tx != nil {
		res, err = r.store.tx.ExecContext(ctx, `UPDATE leases SET expires_at = ? WHERE execution_id = ? AND step_id = ? AND holder = ? AND fencing_token = ?`, expires, executionID, stepID, holder, fencingToken)
	} else {
		res, err = r.store.db.ExecContext(ctx, `UPDATE leases SET expires_at = ? WHERE execution_id = ? AND step_id = ? AND holder = ? AND fencing_token = ?`, expires, executionID, stepID, holder, fencingToken)
	}
	if err != nil {
		return normalizeSQLiteError(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return normalizeSQLiteError(err)
	}
	if n != 1 {
		return ErrNotFound
	}
	return nil
}

func (r *leasesRepo) Release(ctx context.Context, executionID, stepID, holder string, fencingToken int64) error {
	now := time.Now().UTC().Format(time.RFC3339)
	var res sql.Result
	var err error
	if r.store.tx != nil {
		res, err = r.store.tx.ExecContext(ctx, `UPDATE leases SET expires_at = ? WHERE execution_id = ? AND step_id = ? AND holder = ? AND fencing_token = ?`, now, executionID, stepID, holder, fencingToken)
	} else {
		res, err = r.store.db.ExecContext(ctx, `UPDATE leases SET expires_at = ? WHERE execution_id = ? AND step_id = ? AND holder = ? AND fencing_token = ?`, now, executionID, stepID, holder, fencingToken)
	}
	if err != nil {
		return normalizeSQLiteError(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return normalizeSQLiteError(err)
	}
	if n != 1 {
		return ErrNotFound
	}
	return nil
}

func (r *leasesRepo) Get(ctx context.Context, executionID, stepID string) (*Lease, error) {
	row := r.store.queryRow(ctx, `SELECT execution_id, step_id, holder, fencing_token, acquired_at, expires_at FROM leases WHERE execution_id = ? AND step_id = ?`, executionID, stepID)
	var l Lease
	if err := row.Scan(&l.ExecutionID, &l.StepID, &l.Holder, &l.FencingToken, &l.AcquiredAt, &l.ExpiresAt); err != nil {
		return nil, normalizeSQLiteError(err)
	}
	return &l, nil
}
