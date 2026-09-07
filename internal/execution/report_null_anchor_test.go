package execution

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/HectorCortes/haro/internal/project"
	"github.com/HectorCortes/haro/internal/store"
)

func TestReportNullAnchor(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	if err := project.Init(dir); err != nil {
		t.Fatalf("init: %v", err)
	}
	dbPath := filepath.Join(dir, ".haro", "store.db")
	s, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = s.Close() }()
	_ = s.Projects().Create(ctx, dir, dir)
	base := "abc123base"
	_ = base
	// Create terminal execution with NULL base_commit (legacy)
	execID := "exec-null-anchor"
	sExec := &store.Execution{
		ID:             execID,
		ProjectID:      dir,
		WorkflowSource: "/tmp/wf.yaml",
		Status:         "completed",
		WorkspaceMode:  "shared",
		WorkspaceRoot:  dir,
		StartedAt:      "2025-01-01T00:00:00Z",
		BaseCommit:     nil,
	}
	if err := s.Executions().Create(ctx, sExec); err != nil {
		t.Fatalf("create exec null base: %v", err)
	}
	eng := NewEngine(s, &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
		return 0, "", "", nil
	}}, dir)
	_, err = eng.Report(ctx, execID)
	if err == nil {
		t.Fatalf("expected not_available error for null anchor")
	}
	re, ok := err.(*ReportError)
	if !ok {
		t.Fatalf("expected ReportError, got %T: %v", err, err)
	}
	if re.Code != "not_available" {
		t.Fatalf("code = %q want not_available", re.Code)
	}
	if re.Field != "base_commit" {
		t.Fatalf("field = %q want base_commit", re.Field)
	}
	// Also test ChangedFiles path
	_, err = eng.ChangedFiles(ctx, execID)
	if err == nil {
		t.Fatalf("ChangedFiles should also return not_available")
	}
	re2, ok := err.(*ReportError)
	if !ok || re2.Code != "not_available" {
		t.Fatalf("ChangedFiles not_available failed: %v", err)
	}
	// Also test failed status with null still not_available (terminal)
	execID2 := "exec-null-anchor-failed"
	sExec2 := &store.Execution{
		ID:             execID2,
		ProjectID:      dir,
		WorkflowSource: "/tmp/wf.yaml",
		Status:         "failed",
		WorkspaceMode:  "shared",
		WorkspaceRoot:  dir,
		StartedAt:      "2025-01-01T00:00:00Z",
		BaseCommit:     nil,
	}
	_ = s.Executions().Create(ctx, sExec2)
	_, err = eng.Report(ctx, execID2)
	if err == nil || err.(*ReportError).Code != "not_available" {
		t.Fatalf("failed terminal with null should be not_available, got %v", err)
	}
}
