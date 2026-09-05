package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMigrateDagHash_Idempotent(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "daghash.db")

	// First open: migration should create dag_hash via PRAGMA table_info guard
	s1, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	// Verify PRAGMA table_info(executions) contains dag_hash (guard path)
	rows, err := s1.db.QueryContext(ctx, "PRAGMA table_info(executions)")
	if err != nil {
		t.Fatalf("PRAGMA table_info: %v", err)
	}
	hasDagHash := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt *string
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if name == "dag_hash" {
			hasDagHash = true
			if typ != "TEXT" {
				t.Fatalf("dag_hash type = %q want TEXT", typ)
			}
		}
	}
	_ = rows.Close()
	if rows.Err() != nil {
		t.Fatalf("rows err: %v", rows.Err())
	}
	if !hasDagHash {
		t.Fatalf("dag_hash column not found after first migration (PRAGMA guard failed)")
	}

	// Insert a row to verify preservation across re-run
	if err := s1.Projects().Create(ctx, "proj-dag", dir); err != nil {
		t.Fatalf("create proj: %v", err)
	}
	hash := "abc123hash"
	exec := &Execution{
		ID:             "exec-dag-1",
		ProjectID:      "proj-dag",
		WorkflowSource: "/tmp/wf.yaml",
		Status:         "pending",
		WorkspaceMode:  "isolated",
		WorkspaceRoot:  dir,
		StartedAt:      "2025-01-01T00:00:00Z",
		DagHash:        &hash,
	}
	if err := s1.Executions().Create(ctx, exec); err != nil {
		t.Fatalf("create exec: %v", err)
	}
	// Also create without dag_hash (nullable) to ensure nullable
	exec2 := &Execution{
		ID:             "exec-dag-2",
		ProjectID:      "proj-dag",
		WorkflowSource: "/tmp/wf2.yaml",
		Status:         "pending",
		WorkspaceMode:  "isolated",
		WorkspaceRoot:  dir,
		StartedAt:      "2025-01-01T00:00:00Z",
		DagHash:        nil,
	}
	if err := s1.Executions().Create(ctx, exec2); err != nil {
		t.Fatalf("create exec2: %v", err)
	}
	_ = s1.Close()

	// Second open: ALTER re-run safety — should not error, should preserve rows, PRAGMA guard must skip ALTER
	s2, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("second Open (re-run safe): %v", err)
	}
	defer func() { _ = s2.Close() }()

	// PRAGMA still shows dag_hash
	rows2, err := s2.db.QueryContext(ctx, "PRAGMA table_info(executions)")
	if err != nil {
		t.Fatalf("second PRAGMA: %v", err)
	}
	hasDagHash = false
	for rows2.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt *string
		_ = rows2.Scan(&cid, &name, &typ, &notnull, &dflt, &pk)
		if name == "dag_hash" {
			hasDagHash = true
		}
	}
	_ = rows2.Close()
	if !hasDagHash {
		t.Fatalf("dag_hash missing after second open")
	}

	// Existing rows preserved
	got, err := s2.Executions().Get(ctx, "exec-dag-1")
	if err != nil {
		t.Fatalf("get exec-dag-1 after reopen: %v", err)
	}
	if got.DagHash == nil || *got.DagHash != hash {
		t.Fatalf("dag_hash preserved = %v want %q", got.DagHash, hash)
	}
	got2, err := s2.Executions().Get(ctx, "exec-dag-2")
	if err != nil {
		t.Fatalf("get exec-dag-2: %v", err)
	}
	if got2.DagHash != nil {
		t.Fatalf("exec-dag-2 dag_hash should remain nil, got %q", *got2.DagHash)
	}

	// Third open to ensure idempotent even with explicit ensureDagHashColumn double-call
	s3, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("third Open: %v", err)
	}
	_ = s3.Close()
}
