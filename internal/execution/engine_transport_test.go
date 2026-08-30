package execution

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/HectorCortes/haro/internal/project"
	"github.com/HectorCortes/haro/internal/store"
)

func TestAgentTransport_WithTx(t *testing.T) {
	root := t.TempDir()
	if err := project.Init(root); err != nil {
		t.Fatalf("init: %v", err)
	}
	wfDir := filepath.Join(root, ".haro", "workflows", "agent")
	_ = wfDir
	ctx := context.Background()
	dbPath := filepath.Join(root, ".haro", "store.db")
	s, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = s.Close() }()

	// Create an agent workflow manually via execution engine? Use direct DB inserts to simulate agent attempt + transport in WithTx
	// Test that Store.Transport().Put inside WithTx alongside Attempts().Create is atomic and succeeds
	err = s.WithTx(ctx, func(tx store.Store) error {
		// Need to create minimal execution/step/generation/attempt
		if err := tx.Projects().Create(ctx, "proj", root); err != nil {
			// ignore duplicate
		}
		_, _ = tx.(*store.SQLiteStore).QueryForTest(ctx, "INSERT OR IGNORE INTO executions(id, project_id, workflow_source, status, workspace_mode, workspace_root, started_at) VALUES (?, ?, ?, ?, ?, ?, ?)", "exec1", "proj", "wf.yaml", "running", "isolated", root, "2025-01-01T00:00:00Z")
		_, _ = tx.(*store.SQLiteStore).QueryForTest(ctx, "INSERT OR IGNORE INTO execution_steps(execution_id, step_id, type, status, workspace_mode, current_generation) VALUES (?, ?, ?, ?, ?, ?)", "exec1", "step-agent", "agent", "running", "isolated", 1)
		_, _ = tx.(*store.SQLiteStore).QueryForTest(ctx, "INSERT OR IGNORE INTO generations(id, execution_id, step_id, number, created_at) VALUES (?, ?, ?, ?, ?)", "gen1", "exec1", "step-agent", 1, "2025-01-01T00:00:00Z")
		if err := tx.Attempts().Create(ctx, &store.Attempt{ID: "att1", ExecutionID: "exec1", StepID: "step-agent", GenerationID: "gen1", Status: "running", StartedAt: "2025-01-01T00:00:00Z"}); err != nil {
			return err
		}
		native := "sess-native-123"
		ver := 1
		return tx.Transport().Put(ctx, &store.Transport{AttemptID: "att1", AdapterName: "opencode", NativeSessionID: &native, ProtocolVersion: &ver, Extra: "{}"})
	})
	if err != nil {
		t.Fatalf("WithTx attempt+transport: %v", err)
	}
	got, err := s.Transport().Get(ctx, "att1")
	if err != nil {
		t.Fatalf("Get transport: %v", err)
	}
	if got.AdapterName != "opencode" {
		t.Fatalf("AdapterName = %q", got.AdapterName)
	}
	if got.NativeSessionID == nil || *got.NativeSessionID != "sess-native-123" {
		t.Fatalf("NativeSessionID mismatch")
	}
	// Triangulate: second attempt with different adapter
	err = s.WithTx(ctx, func(tx store.Store) error {
		_, _ = tx.(*store.SQLiteStore).QueryForTest(ctx, "INSERT OR IGNORE INTO generations(id, execution_id, step_id, number, created_at) VALUES (?, ?, ?, ?, ?)", "gen2", "exec1", "step-agent", 2, "2025-01-01T00:00:01Z")
		if err := tx.Attempts().Create(ctx, &store.Attempt{ID: "att2", ExecutionID: "exec1", StepID: "step-agent", GenerationID: "gen2", Status: "running", StartedAt: "2025-01-01T00:00:01Z"}); err != nil {
			return err
		}
		return tx.Transport().Put(ctx, &store.Transport{AttemptID: "att2", AdapterName: "claudecode", Extra: "{}"})
	})
	if err != nil {
		t.Fatalf("second WithTx: %v", err)
	}
	got2, err := s.Transport().Get(ctx, "att2")
	if err != nil {
		t.Fatalf("Get2: %v", err)
	}
	if got2.AdapterName != "claudecode" {
		t.Fatalf("second AdapterName = %q", got2.AdapterName)
	}
}
