package store

import (
	"context"
	"database/sql"
)

// transportRepo implements TransportRepository.
type transportRepo struct {
	store *SQLiteStore
}

// Put inserts or replaces the transport row.
func (r *transportRepo) Put(ctx context.Context, t *Transport) error {
	if t.Extra == "" {
		t.Extra = "{}"
	}
	_, err := r.store.exec(ctx, `INSERT OR REPLACE INTO attempt_transport(attempt_id, adapter_name, native_session_id, protocol_version, extra) VALUES (?, ?, ?, ?, ?)`,
		t.AttemptID, t.AdapterName, t.NativeSessionID, t.ProtocolVersion, t.Extra)
	if err != nil {
		return normalizeSQLiteError(err)
	}
	return nil
}

// Get retrieves transport for attemptID.
func (r *transportRepo) Get(ctx context.Context, attemptID string) (*Transport, error) {
	row := r.store.queryRow(ctx, `SELECT attempt_id, adapter_name, native_session_id, protocol_version, extra FROM attempt_transport WHERE attempt_id = ?`, attemptID)
	var tr Transport
	var native sql.NullString
	var ver sql.NullInt64
	var extra sql.NullString
	if err := row.Scan(&tr.AttemptID, &tr.AdapterName, &native, &ver, &extra); err != nil {
		return nil, normalizeSQLiteError(err)
	}
	if native.Valid {
		tr.NativeSessionID = &native.String
	}
	if ver.Valid {
		v := int(ver.Int64)
		tr.ProtocolVersion = &v
	}
	if extra.Valid {
		tr.Extra = extra.String
	} else {
		tr.Extra = "{}"
	}
	return &tr, nil
}
