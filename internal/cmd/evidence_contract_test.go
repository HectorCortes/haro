package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HectorCortes/haro/internal/execution"
	"github.com/HectorCortes/haro/internal/store"
)

// Doctrine and traceability contract tests for inline attempt evidence.
// v2-ipc/U-03 pins the §2/§2.1 payload doctrine; v2-flujo-sdd/U-01 keeps
// criterion-to-test traceability executable and fail-closed.

const techSpecRelPath = "docs/v2/haro-especificacion-tecnica.md"

// TestAttemptEventPayloadDoctrine proves v2-ipc/U-03: the Technical
// Specification declares nullable attempt_events.payload that may hold a
// sanitized bounded evidence delta and never raw/full output, payload_ref
// remains an external/legacy reference, and persisted inline evidence is
// bounded with a null payload_ref.
func TestAttemptEventPayloadDoctrine(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("..", "..", techSpecRelPath))
	if err != nil {
		t.Fatalf("read %s: %v", techSpecRelPath, err)
	}
	// Normalize column-alignment whitespace and SQL list punctuation so the
	// doctrine text can be asserted literally.
	doc := strings.Join(strings.Fields(string(spec)), " ")
	doc = strings.ReplaceAll(doc, ", --", " --")

	// §2 DDL: nullable inline payload declared with its bounded sanitized
	// doctrine, never raw/full output.
	const ddlComment = "payload TEXT -- sanitized bounded evidence delta; never raw/full output"
	if !strings.Contains(doc, ddlComment) {
		t.Fatalf("§2 DDL must declare %q", ddlComment)
	}
	if !strings.Contains(doc, "payload_ref TEXT") {
		t.Fatalf("§2 DDL must retain the legacy payload_ref column")
	}

	// §2.1 keeps the narrow distinction: inline payload only sanitized
	// bounded deltas; payload_ref stays external/legacy and is never inline.
	notes := section(t, string(spec), "### 2.1 Notes")
	if !strings.Contains(notes, "payload") || !strings.Contains(notes, "sanitized bounded delta") {
		t.Fatalf("§2.1 must state payload may hold only a sanitized bounded delta, got %q", notes)
	}
	if !strings.Contains(notes, "payload_ref") || !strings.Contains(notes, "external") || !strings.Contains(notes, "legacy") {
		t.Fatalf("§2.1 must keep payload_ref as an external/legacy reference, got %q", notes)
	}

	// Persisted fields: a new attempt's inline evidence is bounded to the
	// 16 KiB visible budget with a null payload_ref.
	root := t.TempDir()
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(root, "store.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = s.Close() }()
	if err := s.Projects().Create(ctx, "proj-doctrine", "/tmp/proj"); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err := s.Executions().Create(ctx, &store.Execution{ID: "exec-doctrine", ProjectID: "proj-doctrine", WorkflowSource: "wf.yaml", Status: "pending", WorkspaceMode: "isolated", WorkspaceRoot: "/tmp/ws", StartedAt: "2025-01-01T00:00:00Z"}); err != nil {
		t.Fatalf("create execution: %v", err)
	}
	if err := s.Steps().Create(ctx, &store.ExecutionStep{ExecutionID: "exec-doctrine", StepID: "s1", Type: "command", Status: "pending", DependsOn: "[]", Requires: "[]", Produces: "[]", WorkspaceMode: "isolated", CurrentGeneration: 0}); err != nil {
		t.Fatalf("create step: %v", err)
	}
	if err := s.Generations().Create(ctx, &store.Generation{ID: "gen-doctrine", ExecutionID: "exec-doctrine", StepID: "s1", Number: 1, CreatedAt: "2025-01-01T00:00:00Z"}); err != nil {
		t.Fatalf("create generation: %v", err)
	}
	if err := s.Attempts().Create(ctx, &store.Attempt{ID: "att-prior", ExecutionID: "exec-doctrine", StepID: "s1", GenerationID: "gen-doctrine", Status: "completed", StartedAt: "2025-01-01T00:00:00Z"}); err != nil {
		t.Fatalf("create prior attempt: %v", err)
	}
	if err := s.Attempts().Create(ctx, &store.Attempt{ID: "att-doctrine", ExecutionID: "exec-doctrine", StepID: "s1", GenerationID: "gen-doctrine", Status: "running", StartedAt: "2025-01-01T00:00:01Z"}); err != nil {
		t.Fatalf("create attempt: %v", err)
	}
	visible := execution.CommandEvidence([]string{"echo", "hello"}, strings.Repeat("x", 64*1024), "")
	if len(visible) > execution.VisibleLimit {
		t.Fatalf("composed evidence %d > %d", len(visible), execution.VisibleLimit)
	}
	if err := s.Events().CreateAttemptEvent(ctx, &store.AttemptEvent{AttemptID: "att-prior", Cursor: 0, EventType: "output_delta", Payload: &visible, PayloadRef: nil}); err != nil {
		t.Fatalf("create event: %v", err)
	}
	ev, err := s.Events().PriorOutputDelta(ctx, "att-doctrine")
	if err != nil {
		t.Fatalf("PriorOutputDelta: %v", err)
	}
	if ev.Payload == nil || len(*ev.Payload) == 0 || len(*ev.Payload) > execution.VisibleLimit {
		t.Fatalf("inline payload must be non-empty and bounded to 16 KiB, len %d", len(*ev.Payload))
	}
	if ev.PayloadRef != nil {
		t.Fatalf("inline evidence must keep payload_ref null, got %q", *ev.PayloadRef)
	}
	if !strings.HasPrefix(*ev.Payload, "$ echo hello") {
		t.Fatalf("inline payload must carry the argv header: %q", (*ev.Payload)[:64])
	}
}

// TestEvidenceCriterionTraceability proves v2-flujo-sdd/U-01: every criterion
// named in the v2-evidencia acceptance traceability table has its named Go
// test present in the repository, and the mapping is fail-closed.
func TestEvidenceCriterionTraceability(t *testing.T) {
	traceability := []struct {
		criterion string
		testName  string
		file      string
	}{
		{"v2-no-regresion/F-05", "TestFeedbackReconstructionPrefersPayloadAndFallsBackToLegacyReference", "internal/execution/feedback_test.go"},
		{"v2-no-regresion/F-07", "TestReopenRequiresCurrentGenerationRecovery", "internal/execution/state_test.go"},
		{"v2-no-regresion/F-12", "TestAttemptEvidenceInlineBoundedAndRedacted", "internal/execution/evidence_inline_test.go"},
		{"v2-store/F-02", "TestAttemptEventsPayloadSchema", "internal/store/migrations_payload_test.go"},
		{"v2-store/U-01", "TestAttemptEventPayloadBackendParity", "internal/store/parity_test.go"},
		{"v2-store/U-02", "TestMigratePayloadIdempotent", "internal/store/migrations_payload_test.go"},
		{"v2-ipc/U-03", "TestAttemptEventPayloadDoctrine", "internal/cmd/evidence_contract_test.go"},
		{"v2-flujo-sdd/U-01", "TestEvidenceCriterionTraceability", "internal/cmd/evidence_contract_test.go"},
	}
	root := filepath.Join("..", "..")
	for _, tr := range traceability {
		t.Run(tr.criterion, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join(root, tr.file))
			if err != nil {
				t.Fatalf("read %s: %v", tr.file, err)
			}
			decl := "func " + tr.testName + "(t *testing.T)"
			if !strings.Contains(string(src), decl) {
				t.Fatalf("criterion %s: test %s not found in %s (want %q)", tr.criterion, tr.testName, tr.file, decl)
			}
		})
	}
}
