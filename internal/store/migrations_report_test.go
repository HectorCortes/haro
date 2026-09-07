package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestEnsureBaseCommitColumn(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "basecommit.db")

	s1, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	rows, err := s1.db.QueryContext(ctx, "PRAGMA table_info(executions)")
	if err != nil {
		t.Fatalf("PRAGMA table_info: %v", err)
	}
	hasBase := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt *string
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if name == "base_commit" {
			hasBase = true
			if typ != "TEXT" {
				t.Fatalf("base_commit type = %q want TEXT", typ)
			}
			if notnull != 0 {
				t.Fatalf("base_commit notnull = %d want 0 nullable", notnull)
			}
		}
	}
	_ = rows.Close()
	if rows.Err() != nil {
		t.Fatalf("rows err: %v", rows.Err())
	}
	if !hasBase {
		t.Fatalf("base_commit column not found after first migration (PRAGMA guard failed)")
	}

	if err := s1.Projects().Create(ctx, "proj-base", dir); err != nil {
		t.Fatalf("create proj: %v", err)
	}
	hash := "abc123"
	base := "deadbeef1234567890abcdef"
	exec := &Execution{
		ID:             "exec-base-1",
		ProjectID:      "proj-base",
		WorkflowSource: "/tmp/wf.yaml",
		Status:         "pending",
		WorkspaceMode:  "isolated",
		WorkspaceRoot:  dir,
		StartedAt:      "2025-01-01T00:00:00Z",
		DagHash:        &hash,
		BaseCommit:     &base,
	}
	if err := s1.Executions().Create(ctx, exec); err != nil {
		t.Fatalf("create exec with base_commit: %v", err)
	}
	exec2 := &Execution{
		ID:             "exec-base-2",
		ProjectID:      "proj-base",
		WorkflowSource: "/tmp/wf2.yaml",
		Status:         "pending",
		WorkspaceMode:  "isolated",
		WorkspaceRoot:  dir,
		StartedAt:      "2025-01-01T00:00:00Z",
		DagHash:        nil,
		BaseCommit:     nil,
	}
	if err := s1.Executions().Create(ctx, exec2); err != nil {
		t.Fatalf("create exec2 nullable: %v", err)
	}
	_ = s1.Close()

	s2, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer func() { _ = s2.Close() }()

	rows2, err := s2.db.QueryContext(ctx, "PRAGMA table_info(executions)")
	if err != nil {
		t.Fatalf("second PRAGMA: %v", err)
	}
	hasBase = false
	for rows2.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt *string
		_ = rows2.Scan(&cid, &name, &typ, &notnull, &dflt, &pk)
		if name == "base_commit" {
			hasBase = true
		}
	}
	_ = rows2.Close()
	if !hasBase {
		t.Fatalf("base_commit missing after second open")
	}
	got, err := s2.Executions().Get(ctx, "exec-base-1")
	if err != nil {
		t.Fatalf("get exec-base-1: %v", err)
	}
	if got.BaseCommit == nil || *got.BaseCommit != base {
		t.Fatalf("base_commit preserved = %v want %q", got.BaseCommit, base)
	}
	if got.DagHash == nil || *got.DagHash != hash {
		t.Fatalf("dag_hash preserved = %v want %q", got.DagHash, hash)
	}
	got2, err := s2.Executions().Get(ctx, "exec-base-2")
	if err != nil {
		t.Fatalf("get exec-base-2: %v", err)
	}
	if got2.BaseCommit != nil {
		t.Fatalf("exec-base-2 base_commit should remain nil, got %q", *got2.BaseCommit)
	}
	s3, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("third Open: %v", err)
	}
	_ = s3.Close()
}

func TestExecutionBaseCommitMapping(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "mapping.db")
	s, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = s.Close() }()
	if err := s.Projects().Create(ctx, "proj-map", dir); err != nil {
		t.Fatalf("create proj: %v", err)
	}
	base := "abc123base"
	exec := &Execution{
		ID:             "exec-map-1",
		ProjectID:      "proj-map",
		WorkflowSource: "/tmp/wf.yaml",
		Status:         "running",
		WorkspaceMode:  "isolated",
		WorkspaceRoot:  dir,
		StartedAt:      "2025-01-01T00:00:00Z",
		BaseCommit:     &base,
	}
	if err := s.Executions().Create(ctx, exec); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := s.Executions().Get(ctx, "exec-map-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.BaseCommit == nil || *got.BaseCommit != base {
		t.Fatalf("BaseCommit mapping failed: got %v want %q", got.BaseCommit, base)
	}
	// Null case
	exec2 := &Execution{
		ID:             "exec-map-2",
		ProjectID:      "proj-map",
		WorkflowSource: "/tmp/wf2.yaml",
		Status:         "pending",
		WorkspaceMode:  "shared",
		WorkspaceRoot:  dir,
		StartedAt:      "2025-01-01T00:00:00Z",
		BaseCommit:     nil,
	}
	if err := s.Executions().Create(ctx, exec2); err != nil {
		t.Fatalf("create nullable: %v", err)
	}
	got2, err := s.Executions().Get(ctx, "exec-map-2")
	if err != nil {
		t.Fatalf("get2: %v", err)
	}
	if got2.BaseCommit != nil {
		t.Fatalf("expected nil BaseCommit, got %v", *got2.BaseCommit)
	}
}
