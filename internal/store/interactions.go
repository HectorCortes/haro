package store

import (
	"context"
	"database/sql"
	"time"
)

// interactionsRepo implements InteractionRepository with CAS idempotency.
type interactionsRepo struct {
	store *SQLiteStore
}

func (r *interactionsRepo) Create(ctx context.Context, i *Interaction) error {
	if i.ID == "" || i.AttemptID == "" || i.Type == "" || i.Status == "" || i.IdempotencyKey == "" {
		return ErrCheckViolation
	}
	_, err := r.store.exec(ctx, `INSERT INTO interactions(id, attempt_id, type, status, decision, idempotency_key, resolved_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, i.ID, i.AttemptID, i.Type, i.Status, i.Decision, i.IdempotencyKey, i.ResolvedAt)
	if err != nil {
		return normalizeSQLiteError(err)
	}
	return nil
}

func (r *interactionsRepo) Get(ctx context.Context, id string) (*Interaction, error) {
	row := r.store.queryRow(ctx, `SELECT id, attempt_id, type, status, decision, idempotency_key, resolved_at FROM interactions WHERE id = ?`, id)
	var it Interaction
	var dec sql.NullString
	var resAt sql.NullString
	if err := row.Scan(&it.ID, &it.AttemptID, &it.Type, &it.Status, &dec, &it.IdempotencyKey, &resAt); err != nil {
		return nil, normalizeSQLiteError(err)
	}
	if dec.Valid {
		it.Decision = &dec.String
	}
	if resAt.Valid {
		it.ResolvedAt = &resAt.String
	}
	return &it, nil
}

func (r *interactionsRepo) Resolve(ctx context.Context, id, idempotencyKey, decision string) (*Interaction, error) {
	// Fetch current
	current, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	// CAS check: idempotency_key must match stored key
	if current.IdempotencyKey != idempotencyKey {
		return nil, ErrUniqueViolation
	}
	if current.Status == "resolved" {
		// Idempotent: same key returns stored result
		return current, nil
	}
	// Pending -> try to resolve
	now := time.Now().UTC().Format(time.RFC3339)
	var res sql.Result
	// Use CAS update where status=pending and idempotency_key matches
	if r.store.tx != nil {
		res, err = r.store.tx.ExecContext(ctx, `UPDATE interactions SET status = 'resolved', decision = ?, resolved_at = ? WHERE id = ? AND status = 'pending' AND idempotency_key = ?`, decision, now, id, idempotencyKey)
	} else {
		res, err = r.store.db.ExecContext(ctx, `UPDATE interactions SET status = 'resolved', decision = ?, resolved_at = ? WHERE id = ? AND status = 'pending' AND idempotency_key = ?`, decision, now, id, idempotencyKey)
	}
	if err != nil {
		return nil, normalizeSQLiteError(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, normalizeSQLiteError(err)
	}
	if n == 0 {
		// Concurrent resolve or already resolved: fetch again
		again, err2 := r.Get(ctx, id)
		if err2 != nil {
			return nil, err2
		}
		if again.Status == "resolved" && again.IdempotencyKey == idempotencyKey {
			return again, nil
		}
		return nil, ErrUniqueViolation
	}
	return r.Get(ctx, id)
}
