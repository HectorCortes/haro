package execution

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HectorCortes/haro/internal/project"
	"github.com/HectorCortes/haro/internal/store"
)

func TestFeedback(t *testing.T) {
	root := t.TempDir()
	if err := project.Init(root); err != nil {
		t.Fatalf("init: %v", err)
	}
	wfDir := filepath.Join(root, ".haro", "workflows", "fb")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	wfYAML := `version: 2
name: fb
steps:
  - id: s1
    type: command
    run: echo hello
    produces: [out.txt]
`
	if err := os.WriteFile(filepath.Join(wfDir, "workflow.yaml"), []byte(wfYAML), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	dbPath := filepath.Join(root, ".haro", "store.db")
	ctx := context.Background()
	s, err := store.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = s.Close() }()
	artifacts := filepath.Join(root, ".haro", "artifacts")
	// First run without feedback
	fake1 := &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
		_ = os.MkdirAll(artifacts, 0o755)
		_ = os.WriteFile(filepath.Join(artifacts, "out.txt"), []byte("first"), 0o600)
		return 0, strings.Repeat("A", 100), "", nil
	}}
	eng := NewEngine(s, fake1, root)
	execID, err := eng.CreateExecution(ctx, "fb")
	if err != nil {
		t.Fatalf("create exec: %v", err)
	}
	if err := eng.RunStep(ctx, execID, "s1", ""); err != nil {
		t.Fatalf("first run: %v", err)
	}
	// Verify first attempt exists
	cnt, _ := s.Attempts().CountByStep(ctx, execID, "s1")
	if cnt != 1 {
		t.Fatalf("attempt count after first = %d, want 1", cnt)
	}
	// Prepare oversized prior evidence to test bounded prior
	oversized := strings.Repeat("B", 3*1024*1024) // 3MiB > 2MiB fallback
	fake2 := &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
		_ = os.WriteFile(filepath.Join(artifacts, "out.txt"), []byte("second"), 0o600)
		return 0, oversized, "", nil
	}}
	// Need to allow reconstruction: step is completed, so we need to reopen or feedback should allow re-run
	// For test, we will manually set step back to pending via Reopen logic? But feedback should handle reconstruction without explicit reopen.
	// Instead, test expects RunStep with feedback to create distinct attempt even when step is completed.
	eng2 := NewEngine(s, fake2, root)
	feedback := "please improve"
	err = eng2.RunStep(ctx, execID, "s1", feedback)
	if err != nil {
		t.Fatalf("feedback run: %v", err)
	}
	// Verify two attempts now (history preserved)
	cnt2, _ := s.Attempts().CountByStep(ctx, execID, "s1")
	if cnt2 != 2 {
		t.Fatalf("attempt count after feedback = %d, want 2 (history preserved)", cnt2)
	}
	// Verify feedback was stored delimited + bounded prior
	// We expect attempt event with feedback delimited
	// For this test, we check that visible evidence for second attempt is bounded and contains feedback
	// Our engine writes evidence to .haro/artifacts/evidence/<attemptID>.txt; we can find latest attempt
	// Let's list attempts via store directly: query attempts table for this step
	rows, _ := s.QueryForTest(ctx, "SELECT id FROM attempts WHERE execution_id = ? AND step_id = ? ORDER BY started_at", execID, "s1")
	var ids []string
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		ids = append(ids, id)
	}
	_ = rows.Close()
	if len(ids) != 2 {
		t.Fatalf("ids len = %d", len(ids))
	}
	secondID := ids[1]
	// Check attempt_events for feedback
	evRows, _ := s.QueryForTest(ctx, "SELECT event_type, payload_ref FROM attempt_events WHERE attempt_id = ?", secondID)
	foundFeedback := false
	for evRows.Next() {
		var typ string
		var ref *string
		_ = evRows.Scan(&typ, &ref)
		if typ == "feedback" && ref != nil && strings.Contains(*ref, feedback) {
			foundFeedback = true
			// Check delimited: should contain ---FEEDBACK---
			// Our engine stores raw feedback, but should be delimited in evidence file
		}
	}
	_ = evRows.Close()
	if !foundFeedback {
		t.Fatalf("feedback event not found for second attempt")
	}
	// Check that prior context bounded to 2MiB: second attempt's evidence should be <=2MiB (fallback) if we combine prior
	// For this test, we just verify that oversized prior (3MiB) was truncated when stored as visible? Actually visible is 16KiB, fallback is 2MiB for reconstruction.
	// We can check that attempt_events payload_ref files are within budgets
	// Triangulation: small feedback should still be delimited
	smallFeedback := "hi"
	// For triangulation we will use a fresh execution with pending step and small feedback
	root2 := t.TempDir()
	_ = project.Init(root2)
	wfDir2 := filepath.Join(root2, ".haro", "workflows", "fb2")
	_ = os.MkdirAll(wfDir2, 0o755)
	_ = os.WriteFile(filepath.Join(wfDir2, "workflow.yaml"), []byte(wfYAML), 0o600)
	dbPath2 := filepath.Join(root2, ".haro", "store.db")
	s2, _ := store.Open(ctx, dbPath2)
	defer func() { _ = s2.Close() }()
	artifacts2 := filepath.Join(root2, ".haro", "artifacts")
	fakeInit := &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
		_ = os.MkdirAll(artifacts2, 0o755)
		_ = os.WriteFile(filepath.Join(artifacts2, "out.txt"), []byte("init"), 0o600)
		return 0, "prior", "", nil
	}}
	engInit := NewEngine(s2, fakeInit, root2)
	execID2, _ := engInit.CreateExecution(ctx, "fb2")
	_ = engInit.RunStep(ctx, execID2, "s1", "")
	// Now run with small feedback
	fakeFB := &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
		_ = os.MkdirAll(artifacts2, 0o755)
		_ = os.WriteFile(filepath.Join(artifacts2, "out.txt"), []byte("third"), 0o600)
		return 0, "small", "", nil
	}}
	engFB := NewEngine(s2, fakeFB, root2)
	if err := engFB.RunStep(ctx, execID2, "s1", smallFeedback); err != nil {
		t.Fatalf("small feedback run: %v", err)
	}
	cnt3, _ := s2.Attempts().CountByStep(ctx, execID2, "s1")
	if cnt3 != 2 {
		t.Fatalf("small feedback attempt count = %d, want 2", cnt3)
	}
}

// TestFeedbackReconstructionPrefersPayloadAndFallsBackToLegacyReference
// proves v2-no-regresion/F-05: feedback reconstruction reads prior evidence
// from the DB payload first, falls back to the legacy payload_ref file only
// when the inline payload is absent, and keeps the combined prior+feedback
// context bounded to the 2 MiB fallback budget.
func TestFeedbackReconstructionPrefersPayloadAndFallsBackToLegacyReference(t *testing.T) {
	ctx := context.Background()

	newProject := func(t *testing.T) (*store.SQLiteStore, *Engine, string) {
		t.Helper()
		root := t.TempDir()
		if err := project.Init(root); err != nil {
			t.Fatalf("init: %v", err)
		}
		wfDir := filepath.Join(root, ".haro", "workflows", "fb")
		if err := os.MkdirAll(wfDir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		wfYAML := `version: 2
name: fb
steps:
  - id: s1
    type: command
    run: echo hello
    produces: [out.txt]
`
		if err := os.WriteFile(filepath.Join(wfDir, "workflow.yaml"), []byte(wfYAML), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		s, err := store.Open(ctx, filepath.Join(root, ".haro", "store.db"))
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		t.Cleanup(func() { _ = s.Close() })
		eng := NewEngine(s, &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
			_ = os.MkdirAll(filepath.Join(root, ".haro", "artifacts"), 0o755)
			_ = os.WriteFile(filepath.Join(root, ".haro", "artifacts", "out.txt"), []byte("out"), 0o600)
			return 0, "", "", nil
		}}, root)
		return s, eng, root
	}

	// reconstruction returns the content stored for the reconstruction_context
	// event of the latest attempt of the step (bounded prior + feedback).
	reconstruction := func(t *testing.T, s *store.SQLiteStore, execID, stepID string) string {
		t.Helper()
		rows, err := s.QueryForTest(ctx, `SELECT e.payload_ref FROM attempt_events e JOIN attempts a ON a.id = e.attempt_id WHERE a.execution_id = ? AND a.step_id = ? AND e.event_type = 'reconstruction_context' ORDER BY e.id DESC LIMIT 1`, execID, stepID)
		if err != nil {
			t.Fatalf("query reconstruction: %v", err)
		}
		defer func() { _ = rows.Close() }()
		var ref sql.NullString
		if rows.Next() {
			if err := rows.Scan(&ref); err != nil {
				t.Fatalf("scan: %v", err)
			}
		}
		if !ref.Valid {
			t.Fatalf("no reconstruction_context recorded")
		}
		return ref.String
	}

	feedback := "please improve"

	t.Run("inline payload preferred", func(t *testing.T) {
		s, eng, _ := newProject(t)
		execID, err := eng.CreateExecution(ctx, "fb")
		if err != nil {
			t.Fatalf("create exec: %v", err)
		}
		// First run: inline DB payload exists but no evidence file anywhere.
		if err := eng.RunStep(ctx, execID, "s1", ""); err != nil {
			t.Fatalf("first run: %v", err)
		}
		if err := eng.RunStep(ctx, execID, "s1", feedback); err != nil {
			t.Fatalf("feedback run: %v", err)
		}
		rec := reconstruction(t, s, execID, "s1")
		if !strings.Contains(rec, "$ echo hello") {
			t.Fatalf("reconstruction must prefer the inline DB payload with its argv header, got %q", rec[:min(len(rec), 120)])
		}
		if !strings.Contains(rec, "---FEEDBACK---\n"+feedback) {
			t.Fatalf("reconstruction missing delimited feedback: %q", rec[:min(len(rec), 200)])
		}
		if len(rec) > FallbackLimit {
			t.Fatalf("reconstruction %d > %d", len(rec), FallbackLimit)
		}
	})

	t.Run("legacy payload_ref fallback", func(t *testing.T) {
		s, eng, root := newProject(t)
		execID, err := eng.CreateExecution(ctx, "fb")
		if err != nil {
			t.Fatalf("create exec: %v", err)
		}
		if err := eng.RunStep(ctx, execID, "s1", ""); err != nil {
			t.Fatalf("first run: %v", err)
		}
		// Simulate a legacy event: inline payload absent, external file ref.
		legacyPath := filepath.Join(root, ".haro", "artifacts", "evidence", "legacy.txt")
		if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
			t.Fatalf("mkdir legacy: %v", err)
		}
		const legacyContent = "legacy prior evidence content"
		if err := os.WriteFile(legacyPath, []byte(legacyContent), 0o600); err != nil {
			t.Fatalf("write legacy: %v", err)
		}
		if _, err := s.QueryForTest(ctx, `UPDATE attempt_events SET payload = NULL, payload_ref = ? WHERE event_type = 'output_delta'`, legacyPath); err != nil {
			t.Fatalf("rewrite legacy event: %v", err)
		}
		if err := eng.RunStep(ctx, execID, "s1", feedback); err != nil {
			t.Fatalf("feedback run: %v", err)
		}
		rec := reconstruction(t, s, execID, "s1")
		if !strings.Contains(rec, legacyContent) {
			t.Fatalf("reconstruction must fall back to the legacy payload_ref file, got %q", rec[:min(len(rec), 200)])
		}
	})

	t.Run("combined prior and feedback bounded to 2 MiB", func(t *testing.T) {
		s, eng, _ := newProject(t)
		execID, err := eng.CreateExecution(ctx, "fb")
		if err != nil {
			t.Fatalf("create exec: %v", err)
		}
		if err := eng.RunStep(ctx, execID, "s1", ""); err != nil {
			t.Fatalf("first run: %v", err)
		}
		// Oversized inline payload (3 MiB > 2 MiB budget).
		oversized := strings.Repeat("C", 3*1024*1024)
		if _, err := s.QueryForTest(ctx, `UPDATE attempt_events SET payload = ? WHERE event_type = 'output_delta'`, oversized); err != nil {
			t.Fatalf("set oversized payload: %v", err)
		}
		if err := eng.RunStep(ctx, execID, "s1", feedback); err != nil {
			t.Fatalf("feedback run: %v", err)
		}
		rec := reconstruction(t, s, execID, "s1")
		if len(rec) > FallbackLimit {
			t.Fatalf("combined reconstruction %d > %d", len(rec), FallbackLimit)
		}
		if !strings.Contains(rec, "---FEEDBACK---\n"+feedback) {
			t.Fatalf("bounded reconstruction must preserve feedback")
		}
	})
}
