package execution

import (
	"context"
	"strings"
	"testing"

	"github.com/HectorCortes/haro/internal/adapter"
	"github.com/HectorCortes/haro/internal/store"
)

// TestAgentStepPersistsRealEvidenceAndTransport covers U-04: real attempts
// persist the actual native session id, adapter name, protocol version, and
// metadata in attempt_transport; required artifacts are resolved and passed
// to the session; harness output is redacted once, bounded to 16 KiB, and
// stored in attempt_events.payload; attempts stay transport-neutral.
func TestAgentStepPersistsRealEvidenceAndTransport(t *testing.T) {
	ctx := context.Background()
	wfYAML := `version: 2
name: agentwf
steps:
  - id: ag
    type: agent
    harness: [oc]
    instructions: summarize req.txt
    mode: headless
    requires: [req.txt]
`
	root, eng := newAgentEngine(t, wfYAML, `version: 2
harnesses:
  oc:
    binary: /nonexistent/oc
`)
	writeRequiredArtifacts(t, root)
	execID, err := eng.CreateExecution(ctx, "agentwf")
	if err != nil {
		t.Fatalf("create execution: %v", err)
	}

	oversized := "Bearer sk-abc123secret " + strings.Repeat("A", 20*1024)
	sessAdapter := newFakeHarnessAdapter(true, "completed", oversized)
	setupFakeManager(t, eng, map[string]adapter.Adapter{"oc": sessAdapter})

	if err := eng.RunStep(ctx, execID, "ag", ""); err != nil {
		t.Fatalf("run agent step: %v", err)
	}
	assertStepStatus(t, eng, execID, "ag", "completed")

	// Locate the single attempt.
	attemptID := eng.singleAttemptID(t, execID, "ag")

	// Transport identity: the real (fake) native session id, adapter name,
	// negotiated protocol version, and JSON metadata replaced the
	// identity-empty row.
	tr, err := eng.store.Transport().Get(ctx, attemptID)
	if err != nil {
		t.Fatalf("get transport: %v", err)
	}
	if tr.AdapterName != "oc" {
		t.Fatalf("adapter_name = %q", tr.AdapterName)
	}
	if tr.NativeSessionID == nil || *tr.NativeSessionID != "fake-sess-x" {
		t.Fatalf("native_session_id = %v, want fake-sess-x", tr.NativeSessionID)
	}
	if tr.ProtocolVersion == nil || *tr.ProtocolVersion != 1 {
		t.Fatalf("protocol_version = %v, want 1", tr.ProtocolVersion)
	}
	if !strings.Contains(tr.Extra, "fake") {
		t.Fatalf("extra metadata must be persisted, got %q", tr.Extra)
	}

	// Inline evidence: payload present, bounded to 16 KiB, redacted once,
	// and no legacy payload_ref for the output delta.
	rows, err := eng.store.(*store.SQLiteStore).QueryForTest(ctx,
		`SELECT payload, payload_ref FROM attempt_events WHERE attempt_id = ? AND event_type = 'output_delta'`, attemptID)
	if err != nil {
		t.Fatalf("query attempt events: %v", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		t.Fatalf("output_delta event missing")
	}
	var payload, payloadRef *string
	if err := rows.Scan(&payload, &payloadRef); err != nil {
		t.Fatal(err)
	}
	if payload == nil {
		t.Fatalf("payload must be persisted inline")
	}
	if len(*payload) > VisibleLimit {
		t.Fatalf("payload %d exceeds 16 KiB", len(*payload))
	}
	if strings.Contains(*payload, "sk-abc123secret") {
		t.Fatalf("payload must be redacted: %q", (*payload)[:min(len(*payload), 128)])
	}
	if !strings.Contains(*payload, "harness: oc (1/1)") {
		t.Fatalf("payload must carry the agent header")
	}
	if payloadRef != nil {
		t.Fatalf("real attempts store inline payload, payload_ref = %q", *payloadRef)
	}

	// attempts stay transport-neutral: no native_session_id/adapter_name/
	// protocol_version/extra columns.
	cols := eng.tableColumns(t, "attempts")
	for _, banned := range []string{"native_session_id", "adapter_name", "protocol_version", "extra"} {
		for _, c := range cols {
			if c == banned {
				t.Fatalf("attempts must not carry transport column %q", banned)
			}
		}
	}

	// SessionBundle.Requires reached the session with the required artifact.
	bundle := sessAdapter.LastBundle()
	if _, ok := bundle.Requires["req.txt"]; !ok {
		t.Fatalf("session bundle must carry required artifacts, got %v", bundle.Requires)
	}
}

// TestAgentStepTransportStaysIdentityEmptyWhenSessionHasNoIdentity covers the
// optional-row scenario at the engine level: a session that captures no
// identity leaves the identity-empty transport row untouched and the attempt
// still succeeds.
func TestAgentStepTransportStaysIdentityEmptyWhenSessionHasNoIdentity(t *testing.T) {
	ctx := context.Background()
	wfYAML := `version: 2
name: agentwf
steps:
  - id: ag
    type: agent
    harness: [oc]
    instructions: go
    mode: headless
`
	_, eng := newAgentEngine(t, wfYAML, `version: 2
harnesses:
  oc:
    binary: /nonexistent/oc
`)
	execID, err := eng.CreateExecution(ctx, "agentwf")
	if err != nil {
		t.Fatalf("create execution: %v", err)
	}
	sessAdapter := newFakeHarnessAdapter(true, "completed", "some output")
	sessAdapter.noIdentity = true
	setupFakeManager(t, eng, map[string]adapter.Adapter{"oc": sessAdapter})

	if err := eng.RunStep(ctx, execID, "ag", ""); err != nil {
		t.Fatalf("run agent step: %v", err)
	}
	assertStepStatus(t, eng, execID, "ag", "completed")
	attemptID := eng.singleAttemptID(t, execID, "ag")
	tr, err := eng.store.Transport().Get(ctx, attemptID)
	if err != nil {
		t.Fatalf("identity-empty transport row must exist: %v", err)
	}
	if tr.NativeSessionID != nil {
		t.Fatalf("native_session_id must stay absent, got %q", *tr.NativeSessionID)
	}
	if tr.ProtocolVersion != nil {
		t.Fatalf("protocol_version must stay absent, got %d", *tr.ProtocolVersion)
	}
	if tr.AdapterName != "oc" {
		t.Fatalf("adapter_name = %q", tr.AdapterName)
	}
}

// singleAttemptID returns the only attempt id for a step (or fails).
func (e *Engine) singleAttemptID(t *testing.T, execID, stepID string) string {
	t.Helper()
	ctx := context.Background()
	count, err := e.store.Attempts().CountByStep(ctx, execID, stepID)
	if err != nil {
		t.Fatalf("count attempts: %v", err)
	}
	if count != 1 {
		t.Fatalf("attempts = %d, want 1", count)
	}
	rows, err := e.store.(*store.SQLiteStore).QueryForTest(ctx,
		`SELECT id FROM attempts WHERE execution_id = ? AND step_id = ?`, execID, stepID)
	if err != nil {
		t.Fatalf("query attempts: %v", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		t.Fatalf("attempt row missing")
	}
	var id string
	if err := rows.Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// tableColumns lists column names of a table via PRAGMA table_info.
func (e *Engine) tableColumns(t *testing.T, table string) []string {
	t.Helper()
	ctx := context.Background()
	rows, err := e.store.(*store.SQLiteStore).QueryForTest(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		t.Fatalf("pragma table_info(%s): %v", table, err)
	}
	defer func() { _ = rows.Close() }()
	var cols []string
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt *string
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		cols = append(cols, name)
	}
	return cols
}
