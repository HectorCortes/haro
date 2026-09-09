package execution

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HectorCortes/haro/internal/adapter"
	"github.com/HectorCortes/haro/internal/project"
	"github.com/HectorCortes/haro/internal/store"
)

// TestAttemptEvidenceInlineBoundedAndRedacted proves v2-no-regresion/F-12 and
// the v2-evidencia inline scenarios: silent commands still yield non-empty
// `$ <argv>` payloads; agent compositions identify harness, one-based index,
// total, and mode, with instructions bounded to 4 KiB; credentials are
// redacted and everything is bounded to 16 KiB; new runs persist inline with
// a null payload_ref and never create evidence files, while snapshots stay.
func TestAttemptEvidenceInlineBoundedAndRedacted(t *testing.T) {
	root := t.TempDir()
	if err := project.Init(root); err != nil {
		t.Fatalf("init: %v", err)
	}
	wfDir := filepath.Join(root, ".haro", "workflows", "inline")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	wfYAML := `version: 2
name: inline
steps:
  - id: cmd
    type: command
    run: echo hello
    produces: [out.txt]
  - id: ag
    type: agent
    harness: [opencode, claudecode]
    instructions: do the thing
    mode: headless
`
	if err := os.WriteFile(filepath.Join(wfDir, "workflow.yaml"), []byte(wfYAML), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	ctx := context.Background()
	s, err := store.Open(ctx, filepath.Join(root, ".haro", "store.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = s.Close() }()
	artifacts := filepath.Join(root, ".haro", "artifacts")

	t.Run("silent command persists non-empty argv header inline", func(t *testing.T) {
		// Silent command: no stdout, no stderr.
		fake := &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
			_ = os.MkdirAll(artifacts, 0o755)
			_ = os.WriteFile(filepath.Join(artifacts, "out.txt"), []byte("out"), 0o600)
			return 0, "", "", nil
		}}
		eng := NewEngine(s, fake, root)
		execID, err := eng.CreateExecution(ctx, "inline")
		if err != nil {
			t.Fatalf("create exec: %v", err)
		}
		if err := eng.RunStep(ctx, execID, "cmd", ""); err != nil {
			t.Fatalf("run cmd: %v", err)
		}
		rows, err := s.QueryForTest(ctx, `SELECT e.event_type, e.payload_ref, e.payload FROM attempt_events e JOIN attempts a ON a.id = e.attempt_id WHERE a.execution_id = ? AND a.step_id = ? AND e.event_type = 'output_delta'`, execID, "cmd")
		if err != nil {
			t.Fatalf("query events: %v", err)
		}
		var payload *string
		var ref *string
		count := 0
		for rows.Next() {
			count++
			var typ string
			var r, p sql.NullString
			if err := rows.Scan(&typ, &r, &p); err != nil {
				t.Fatalf("scan: %v", err)
			}
			if r.Valid {
				ref = &r.String
			}
			if p.Valid {
				payload = &p.String
			}
		}
		_ = rows.Close()
		if count != 1 {
			t.Fatalf("output_delta events = %d, want 1", count)
		}
		if payload == nil || *payload == "" {
			t.Fatalf("silent command must persist non-empty inline payload, got %v", payload)
		}
		if !strings.HasPrefix(*payload, "$ echo hello") {
			t.Fatalf("payload missing `$ <argv>` header: %q", (*payload)[:min(len(*payload), 64)])
		}
		if ref != nil {
			t.Fatalf("payload_ref must be null for inline evidence, got %q", *ref)
		}
		// No evidence files or directory for new runs.
		if _, err := os.Stat(filepath.Join(artifacts, "evidence")); !os.IsNotExist(err) {
			t.Fatalf("evidence directory must not be created, stat err = %v", err)
		}
		// Snapshots remain supported.
		snap := filepath.Join(artifacts, "snapshots")
		found := false
		_ = filepath.Walk(snap, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && strings.HasSuffix(path, "out.txt") {
				found = true
			}
			return nil
		})
		if !found {
			t.Fatalf("snapshot for out.txt must remain")
		}
	})

	t.Run("command evidence composer", func(t *testing.T) {
		// Silent command evidence is non-empty and carries the header.
		got := CommandEvidence([]string{"echo", "hello"}, "", "")
		if got == "" {
			t.Fatalf("silent command evidence must be non-empty")
		}
		if !strings.Contains(got, "$ echo hello") {
			t.Fatalf("command evidence missing argv header: %q", got)
		}
		// Output joins after the header and shares the 16 KiB budget.
		big := strings.Repeat("x", 32*1024)
		gotBig := CommandEvidence([]string{"big"}, big, "err")
		if len(gotBig) > VisibleLimit {
			t.Fatalf("command evidence %d > %d", len(gotBig), VisibleLimit)
		}
		if !strings.Contains(gotBig, "$ big") {
			t.Fatalf("composed evidence missing header: %q", gotBig[:min(len(gotBig), 32)])
		}
	})

	t.Run("agent evidence composer redacts and bounds", func(t *testing.T) {
		oversized := strings.Repeat("A", 8*1024) // > 4 KiB instructions
		out := "token=supersecret123 credentials leaked"
		got := AgentEvidence("opencode", 1, 2, "headless", oversized, out)
		if len(got) > VisibleLimit {
			t.Fatalf("agent evidence %d > %d", len(got), VisibleLimit)
		}
		if strings.Contains(got, "supersecret123") {
			t.Fatalf("agent evidence not redacted: %q", got)
		}
		for _, want := range []string{"harness: opencode (1/2)", "mode: headless"} {
			if !strings.Contains(got, want) {
				t.Fatalf("agent evidence missing %q: %q", want, got)
			}
		}
		// Instructions are bounded to 4 KiB before composition: only 4 KiB of
		// the 8 KiB run appears.
		if !strings.Contains(got, strings.Repeat("A", AgentInstructionsLimit)) {
			t.Fatalf("agent evidence missing bounded instructions")
		}
		if strings.Count(got, "AAAA")*4 > AgentInstructionsLimit+8 {
			t.Fatalf("instructions exceed 4 KiB bound")
		}
		if !strings.Contains(got, "--- output ---") {
			t.Fatalf("agent evidence missing output delimiter: %q", got[:min(len(got), 128)])
		}
		// One-based index triangulation: last candidate.
		gotLast := AgentEvidence("claudecode", 2, 2, "headless", "do", "out")
		if !strings.Contains(gotLast, "harness: claudecode (2/2)") {
			t.Fatalf("agent evidence missing (2/2) index: %q", gotLast)
		}
	})

	t.Run("agent attempts persist identified inline evidence", func(t *testing.T) {
		fake := &FakeRunner{Handler: func(ctx context.Context, cwd string, argv []string, env map[string]string) (int, string, string, error) {
			return 0, "", "", nil
		}}
		eng := NewEngine(s, fake, root)
		// The harness configuration makes both candidates usable and the
		// first one fails cleanly so the fallback records (1/2) then (2/2).
		cfgYAML := "version: 2\nharnesses:\n  opencode:\n    binary: /nonexistent/first\n  claudecode:\n    binary: /nonexistent/second\n"
		if err := os.WriteFile(filepath.Join(root, ".haro", "config.yaml"), []byte(cfgYAML), 0o600); err != nil {
			t.Fatalf("write config: %v", err)
		}
		first := newFakeHarnessAdapter(true, "failed", "first tried and failed")
		second := newFakeHarnessAdapter(true, "completed", "second output")
		setupFakeManager(t, eng, map[string]adapter.Adapter{
			"opencode":   first,
			"claudecode": second,
		})
		execID, err := eng.CreateExecution(ctx, "inline")
		if err != nil {
			t.Fatalf("create exec: %v", err)
		}
		if err := eng.RunStep(ctx, execID, "ag", ""); err != nil {
			t.Fatalf("run agent: %v", err)
		}
		rows, err := s.QueryForTest(ctx, `SELECT e.payload FROM attempt_events e JOIN attempts a ON a.id = e.attempt_id WHERE a.execution_id = ? AND a.step_id = ? AND e.event_type = 'output_delta' ORDER BY e.id`, execID, "ag")
		if err != nil {
			t.Fatalf("query agent events: %v", err)
		}
		defer func() { _ = rows.Close() }()
		var payloads []*string
		for rows.Next() {
			var p sql.NullString
			_ = rows.Scan(&p)
			if p.Valid {
				v := p.String
				payloads = append(payloads, &v)
			} else {
				payloads = append(payloads, nil)
			}
		}
		if len(payloads) < 2 {
			t.Fatalf("agent fallback should record one output_delta per candidate, got %d", len(payloads))
		}
		// First (failed) candidate identifies itself as (1/2); the successful
		// last candidate as (2/2). Both carry the composed agent header.
		firstPayload := payloads[0]
		if firstPayload == nil || !strings.Contains(*firstPayload, "harness: opencode (1/2)") {
			t.Fatalf("first agent evidence missing harness header: %v", firstPayload)
		}
		// The last (successful) candidate carries the one-based (2/2) index.
		last := payloads[len(payloads)-1]
		if !strings.Contains(*last, "harness: claudecode (2/2)") {
			t.Fatalf("last agent evidence missing (2/2): %q", (*last)[:min(len(*last), 64)])
		}
		if !strings.Contains(*last, "mode: headless") {
			t.Fatalf("agent evidence missing mode: %q", (*last)[:min(len(*last), 64)])
		}
		if _, err := os.Stat(filepath.Join(artifacts, "evidence")); !os.IsNotExist(err) {
			t.Fatalf("agent runs must not create evidence directory, stat err = %v", err)
		}
	})
}
