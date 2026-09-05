package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/HectorCortes/haro/internal/claim"
)

// PathClaim is a persisted claim.
type PathClaim struct {
	ID               int64
	ProjectID        string
	LogicalPath      string
	Mode             string
	OwnerExecutionID string
	OwnerStepID      string
	AcquiredAt       string
	ReleasedAt       *string
}

// PathClaimRepository manages path_claims.
type PathClaimRepository interface {
	Acquire(ctx context.Context, c PathClaim) (bool, *PathClaim, error)
	Release(ctx context.Context, projectID, logicalPath, ownerExecutionID, ownerStepID string) error
	ListActive(ctx context.Context, projectID string) ([]PathClaim, error)
}

type pathClaimRepo struct {
	store *SQLiteStore
}

func (r *pathClaimRepo) Acquire(ctx context.Context, c PathClaim) (bool, *PathClaim, error) {
	if c.ProjectID == "" || c.LogicalPath == "" || c.Mode == "" || c.OwnerExecutionID == "" || c.OwnerStepID == "" {
		return false, nil, fmt.Errorf("missing claim fields")
	}
	// Use a dedicated connection with BEGIN IMMEDIATE to serialize.
	conn, err := r.store.db.Conn(ctx)
	if err != nil {
		return false, nil, fmt.Errorf("conn: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return false, nil, fmt.Errorf("begin immediate: %w", err)
	}
	// Ensure rollback on failure, commit on success.
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()

	// Scan active claims for this project.
	rows, err := conn.QueryContext(ctx, `SELECT id, project_id, logical_path, mode, owner_execution_id, owner_step_id, acquired_at, released_at FROM path_claims WHERE project_id = ? AND released_at IS NULL`, c.ProjectID)
	if err != nil {
		return false, nil, fmt.Errorf("scan active: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var actives []PathClaim
	for rows.Next() {
		var pc PathClaim
		var rel sql.NullString
		if err := rows.Scan(&pc.ID, &pc.ProjectID, &pc.LogicalPath, &pc.Mode, &pc.OwnerExecutionID, &pc.OwnerStepID, &pc.AcquiredAt, &rel); err != nil {
			return false, nil, fmt.Errorf("scan: %w", err)
		}
		if rel.Valid {
			pc.ReleasedAt = &rel.String
		}
		actives = append(actives, pc)
	}
	if err := rows.Err(); err != nil {
		return false, nil, err
	}
	// Check for conflicting active claim using prefix + policy.
	for _, ex := range actives {
		if claim.Overlaps(ex.LogicalPath, c.LogicalPath) {
			// Determine if should block. Use default block; isolated/isolated allows.
			if claim.ShouldBlock(ex.Mode, c.Mode, "block", false) {
				// Conflict
				_, _ = conn.ExecContext(ctx, "ROLLBACK")
				committed = true // prevent double rollback
				conf := ex
				return false, &conf, nil
			}
		}
	}
	// No conflict: insert
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = conn.ExecContext(ctx, `INSERT INTO path_claims(project_id, logical_path, mode, owner_execution_id, owner_step_id, acquired_at, released_at) VALUES (?, ?, ?, ?, ?, ?, NULL)`,
		c.ProjectID, c.LogicalPath, c.Mode, c.OwnerExecutionID, c.OwnerStepID, now)
	if err != nil {
		return false, nil, fmt.Errorf("insert claim: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return false, nil, fmt.Errorf("commit: %w", err)
	}
	committed = true
	return true, nil, nil
}

func (r *pathClaimRepo) Release(ctx context.Context, projectID, logicalPath, ownerExecutionID, ownerStepID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.store.exec(ctx, `UPDATE path_claims SET released_at = ? WHERE project_id = ? AND logical_path = ? AND owner_execution_id = ? AND owner_step_id = ? AND released_at IS NULL`, now, projectID, logicalPath, ownerExecutionID, ownerStepID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("release: no active claim owned by %s/%s for %q", ownerExecutionID, ownerStepID, logicalPath)
	}
	return nil
}

func (r *pathClaimRepo) ListActive(ctx context.Context, projectID string) ([]PathClaim, error) {
	rows, err := r.store.query(ctx, `SELECT id, project_id, logical_path, mode, owner_execution_id, owner_step_id, acquired_at, released_at FROM path_claims WHERE project_id = ? AND released_at IS NULL ORDER BY logical_path`, projectID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []PathClaim
	for rows.Next() {
		var pc PathClaim
		var rel sql.NullString
		if err := rows.Scan(&pc.ID, &pc.ProjectID, &pc.LogicalPath, &pc.Mode, &pc.OwnerExecutionID, &pc.OwnerStepID, &pc.AcquiredAt, &rel); err != nil {
			return nil, err
		}
		if rel.Valid {
			pc.ReleasedAt = &rel.String
		}
		out = append(out, pc)
	}
	return out, rows.Err()
}
