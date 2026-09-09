package execution

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HectorCortes/haro/internal/adapter"
	"github.com/HectorCortes/haro/internal/adapter/claude"
	"github.com/HectorCortes/haro/internal/store"
)

// writeClaudeEvidenceFixture writes an executable claude fixture that emits
// an oversized stream containing Bearer and sk-ant- credentials.
func writeClaudeEvidenceFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "claude-fixture.sh")
	script := "#!/bin/sh\n" +
		"stdin=$(cat)\n" +
		"cat <<'CLAUDE_EOF'\n" +
		`{"type":"system","subtype":"init","session_id":"sess_claude_evidence_1"}` + "\n" +
		"CLAUDE_EOF\n" +
		`printf '{"type":"assistant","message":{"content":[{"type":"text","text":"Bearer sk-abc123secret sk-ant-api03-secretvalue ` + "'\n" +
		"head -c 20480 /dev/zero | tr '\\0' 'A'\n" +
		`printf '"}]}}\n'` + "\n" +
		"cat <<'CLAUDE_EOF'\n" +
		`{"type":"result","subtype":"success","is_error":false,"result":"done"}` + "\n" +
		"CLAUDE_EOF\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestClaudeEvidenceBoundedAndRedacted covers v2-adapter/F-05 at the engine
// level: an oversized claude stream containing Bearer and sk-ant- credentials
// is redacted exactly once, the inline payload stays within 16 KiB, and the
// attempt_transport row keeps the claude identity (adapter_name, native
// session id, protocol version, stream-json extra metadata).
func TestClaudeEvidenceBoundedAndRedacted(t *testing.T) {
	ctx := context.Background()
	wfYAML := `version: 2
name: agentwf
steps:
  - id: ag
    type: agent
    harness: [claude]
    instructions: summarize req.txt
    mode: headless
    requires: [req.txt]
`
	root, eng := newAgentEngine(t, wfYAML, `version: 2
harnesses:
  claude:
    binary: /nonexistent/claude
`)
	writeRequiredArtifacts(t, root)
	execID, err := eng.CreateExecution(ctx, "agentwf")
	if err != nil {
		t.Fatalf("create execution: %v", err)
	}

	fixture := writeClaudeEvidenceFixture(t)
	claudeAdapter := claude.NewAdapter(fixture, nil, 10_000_000_000)
	setupFakeManager(t, eng, map[string]adapter.Adapter{"claude": claudeAdapter})

	if err := eng.RunStep(ctx, execID, "ag", ""); err != nil {
		t.Fatalf("run agent step: %v", err)
	}
	assertStepStatus(t, eng, execID, "ag", "completed")
	attemptID := eng.singleAttemptID(t, execID, "ag")

	// Transport identity: adapter_name=claude, real native session id,
	// protocol version, and the stream-json extra metadata.
	tr, err := eng.store.Transport().Get(ctx, attemptID)
	if err != nil {
		t.Fatalf("get transport: %v", err)
	}
	if tr.AdapterName != "claude" {
		t.Fatalf("adapter_name = %q, want claude", tr.AdapterName)
	}
	if tr.NativeSessionID == nil || *tr.NativeSessionID != "sess_claude_evidence_1" {
		t.Fatalf("native_session_id = %v, want sess_claude_evidence_1", tr.NativeSessionID)
	}
	if tr.ProtocolVersion == nil || *tr.ProtocolVersion != 1 {
		t.Fatalf("protocol_version = %v, want 1", tr.ProtocolVersion)
	}
	if !strings.Contains(tr.Extra, "stream-json") {
		t.Fatalf("extra metadata must record the stream-json transport, got %q", tr.Extra)
	}

	// Inline evidence: redacted exactly once, bounded to 16 KiB, and
	// composed with the claude agent header.
	rows, err := eng.store.(*store.SQLiteStore).QueryForTest(ctx,
		`SELECT payload FROM attempt_events WHERE attempt_id = ? AND event_type = 'output_delta'`, attemptID)
	if err != nil {
		t.Fatalf("query attempt events: %v", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		t.Fatalf("output_delta event missing")
	}
	var payload *string
	if err := rows.Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if payload == nil {
		t.Fatalf("payload must be persisted inline")
	}
	if len(*payload) > VisibleLimit {
		t.Fatalf("payload %d exceeds 16 KiB", len(*payload))
	}
	for _, secret := range []string{"sk-abc123secret", "sk-ant-api03-secretvalue"} {
		if strings.Contains(*payload, secret) {
			t.Fatalf("payload must be redacted (%s present): %q", secret, (*payload)[:min(len(*payload), 160)])
		}
	}
	if !strings.Contains(*payload, "harness: claude (1/1)") {
		t.Fatalf("payload must carry the claude agent header, got %q", (*payload)[:min(len(*payload), 160)])
	}
}
