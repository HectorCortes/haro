package claude

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HectorCortes/haro/internal/adapter"
)

// writeFixture writes an executable shell fixture that pins the stream-json
// envelope emitted by `claude -p --output-format stream-json`. It records
// argv and stdin diagnostics into invocations.log next to itself so tests
// can assert the fixed argv, absence of a shell, prompt-on-stdin transport,
// and the applied permission mode. Executables are always generated in
// t.TempDir(); no real or authenticated claude binary is used in short mode.
func writeFixture(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "claude-fixture.sh")
	script := "#!/bin/sh\n" +
		"stdin=$(cat)\n" +
		"printf 'argv:%s\\nstdin:%s\\nstdin_len:%s\\n' \"$*\" \"$stdin\" \"${#stdin}\" >> \"$(dirname \"$0\")/invocations.log\"\n" +
		"case \"$stdin\" in *CLAUDE_PROMPT_MARKER*) printf 'marker:present\\n' >> \"$(dirname \"$0\")/invocations.log\";; esac\n" +
		body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// fixtureEnvelope pins the assumed 2.1.245 success envelope: init with the
// native session_id, assistant text, and a success result. The nested
// stream_event delta shape is exercised separately (N2).
const fixtureEnvelope = `printf '%s\n' '{"type":"system","subtype":"init","session_id":"sess_claude_fixture_1"}'
printf '%s\n' '{"type":"assistant","message":{"content":[{"type":"text","text":"hello from claude fixture"}]}}'
printf '%s\n' '{"type":"result","subtype":"success","is_error":false,"result":"done","session_id":"sess_claude_fixture_1"}'`

// initializedAdapter returns an adapter probed and initialized against the
// fixture, mirroring the factory lifecycle.
func initializedAdapter(t *testing.T, binary string, timeout time.Duration) *Adapter {
	t.Helper()
	ctx := context.Background()
	a := NewAdapter(binary, nil, timeout)
	pr, err := a.Probe(ctx)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if !pr.Available {
		t.Fatalf("fixture must be available, got %+v", pr)
	}
	if _, err := a.Initialize(ctx, adapter.Capabilities{ProtocolVersion: 1}); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	return a
}

// TestClaudeRealSessionLifecycle covers the v2-adapter/F-05 real-session
// scenario: one bounded real subprocess via exec.CommandContext with fixed
// argv, no shell, Dir=WorkspaceRoot, env overlay, bounded frames, bounded
// timeout, exactly one terminal outcome, and idempotent cancellation.
func TestClaudeRealSessionLifecycle(t *testing.T) {
	ctx := context.Background()

	t.Run("session runs fixture with fixed argv, stdin prompt, and completed events", func(t *testing.T) {
		binary := writeFixture(t, fixtureEnvelope)
		a := initializedAdapter(t, binary, 10*time.Second)
		sess, err := a.NewSession(ctx, adapter.SessionBundle{WorkspaceRoot: t.TempDir(), Instructions: "do work"}, &fakeHost{})
		if err != nil {
			t.Fatalf("new session: %v", err)
		}
		ch, err := sess.Prompt(ctx, adapter.PromptInput{Text: "please summarize"})
		if err != nil {
			t.Fatalf("prompt: %v", err)
		}
		var deltas []string
		completed := false
		for ev := range ch {
			switch ev.Type {
			case "output_delta":
				deltas = append(deltas, string(ev.Payload))
			case "completed":
				completed = true
			case "failed":
				t.Fatalf("unexpected failed event: %s", ev.Payload)
			}
		}
		if !completed {
			t.Fatalf("session must reach completed")
		}
		if len(deltas) == 0 || !strings.Contains(strings.Join(deltas, ""), "hello from claude fixture") {
			t.Fatalf("output deltas missing fixture text: %v", deltas)
		}
		// Real transport identity is exposed via the optional accessor.
		tp, ok := sess.(adapter.TransportProvider)
		if !ok {
			t.Fatalf("session must implement TransportProvider")
		}
		st, ok := tp.SessionTransport()
		if !ok {
			t.Fatalf("settled session must expose transport identity")
		}
		if st.NativeSessionID != "sess_claude_fixture_1" {
			t.Fatalf("native session id = %q", st.NativeSessionID)
		}
		if st.ProtocolVersion != 1 {
			t.Fatalf("protocol version = %d", st.ProtocolVersion)
		}
		if st.Extra["transport"] != "stream-json" {
			t.Fatalf("transport extra = %v, want stream-json", st.Extra)
		}
		// Subprocess contract: fixed argv, prompt on stdin, no shell.
		logData, err := os.ReadFile(filepath.Join(filepath.Dir(binary), "invocations.log"))
		if err != nil {
			t.Fatalf("fixture log: %v", err)
		}
		logged := string(logData)
		if !strings.Contains(logged, "argv:-p --output-format stream-json --include-partial-messages --permission-mode dontAsk") {
			t.Fatalf("fixture argv must be exactly the pinned print-mode invocation, log: %q", logged)
		}
		if !strings.Contains(logged, "stdin:please summarize") {
			t.Fatalf("prompt must arrive on stdin, log: %q", logged)
		}
		// One subprocess invocation per prompt.
		if got := strings.Count(logged, "argv:"); got != 1 {
			t.Fatalf("expected exactly 1 fixture invocation, got %d", got)
		}
	})

	t.Run("workspace root becomes the subprocess working directory", func(t *testing.T) {
		dir := t.TempDir()
		binary := writeFixture(t, `printf 'cwd:%s\n' "$(pwd)" >> "$(dirname "$0")/invocations.log"`+"\n"+fixtureEnvelope)
		a := initializedAdapter(t, binary, 10*time.Second)
		sess, err := a.NewSession(ctx, adapter.SessionBundle{WorkspaceRoot: dir}, &fakeHost{})
		if err != nil {
			t.Fatalf("new session: %v", err)
		}
		ch, err := sess.Prompt(ctx, adapter.PromptInput{Text: "go"})
		if err != nil {
			t.Fatalf("prompt: %v", err)
		}
		for ev := range ch {
			if ev.Type == "failed" {
				t.Fatalf("unexpected failed event: %s", ev.Payload)
			}
		}
		logData, err := os.ReadFile(filepath.Join(filepath.Dir(binary), "invocations.log"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(logData), "cwd:"+dir) {
			t.Fatalf("subprocess must run with Dir=WorkspaceRoot %q, log: %q", dir, string(logData))
		}
	})

	t.Run("configured env overlay reaches the subprocess", func(t *testing.T) {
		binary := writeFixture(t, `printf 'env:%s\n' "$HARO_CLAUDE_FIXTURE_ENV" >> "$(dirname "$0")/invocations.log"`+"\n"+fixtureEnvelope)
		a := NewAdapter(binary, map[string]string{"HARO_CLAUDE_FIXTURE_ENV": "overlay-value"}, 10*time.Second)
		initialized := initializedAdapterFrom(t, a)
		sess, err := initialized.NewSession(ctx, adapter.SessionBundle{WorkspaceRoot: t.TempDir()}, &fakeHost{})
		if err != nil {
			t.Fatalf("new session: %v", err)
		}
		ch, err := sess.Prompt(ctx, adapter.PromptInput{Text: "go"})
		if err != nil {
			t.Fatalf("prompt: %v", err)
		}
		for ev := range ch {
			if ev.Type == "failed" {
				t.Fatalf("unexpected failed event: %s", ev.Payload)
			}
		}
		logData, err := os.ReadFile(filepath.Join(filepath.Dir(binary), "invocations.log"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(logData), "env:overlay-value") {
			t.Fatalf("configured env overlay must reach the subprocess: %q", string(logData))
		}
	})

	t.Run("probe rejects non-executable and documentation-like paths", func(t *testing.T) {
		cases := []struct {
			name    string
			content string
			mode    os.FileMode
		}{
			{"requirements.txt", "requests==2.0\n", 0o644},
			{"CMakeLists.txt", "project(foo)\n", 0o644},
			{"README.sh", "#!/bin/sh\necho hi\n", 0o644},
			{"notes.md", "# notes\n", 0o755},
			{"notes.mdx", "# notes\n", 0o755},
			{"no-shebang.sh", "echo hi\n", 0o755},
		}
		for _, tc := range cases {
			dir := t.TempDir()
			path := filepath.Join(dir, tc.name)
			if err := os.WriteFile(path, []byte(tc.content), tc.mode); err != nil {
				t.Fatal(err)
			}
			a := NewAdapter(path, nil, time.Second)
			pr, err := a.Probe(context.Background())
			if err != nil {
				t.Fatalf("probe %s must not error (clean fallback), got %v", tc.name, err)
			}
			if pr.Available {
				t.Fatalf("probe must reject %s (mode %o)", tc.name, tc.mode)
			}
		}
	})

	t.Run("probe accepts executable ELF and shebang programs", func(t *testing.T) {
		dir := t.TempDir()
		sh := filepath.Join(dir, "README.sh")
		if err := os.WriteFile(sh, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		a := NewAdapter(sh, nil, time.Second)
		pr, err := a.Probe(context.Background())
		if err != nil {
			t.Fatalf("probe README.sh: %v", err)
		}
		if !pr.Available {
			t.Fatalf("executable shebang script must probe available")
		}
		if !strings.Contains(pr.Version, "claude") {
			t.Fatalf("probe version = %q", pr.Version)
		}
	})

	t.Run("probe resolves bare binary names through PATH", func(t *testing.T) {
		dir := t.TempDir()
		sh := filepath.Join(dir, "cl-fixture")
		if err := os.WriteFile(sh, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", dir)
		a := NewAdapter("cl-fixture", nil, time.Second)
		pr, err := a.Probe(context.Background())
		if err != nil {
			t.Fatalf("probe bare name: %v", err)
		}
		if !pr.Available {
			t.Fatalf("bare binary name resolvable through PATH must probe available")
		}
	})

	t.Run("missing binary probes unavailable without error", func(t *testing.T) {
		a := NewAdapter("/nonexistent/claude-binary", nil, time.Second)
		pr, err := a.Probe(context.Background())
		if err != nil {
			t.Fatalf("probe missing binary must not error, got %v", err)
		}
		if pr.Available {
			t.Fatalf("missing binary must be Available:false")
		}
	})

	t.Run("timeout fails terminally", func(t *testing.T) {
		binary := writeFixture(t, `sleep 5`)
		a := initializedAdapter(t, binary, 100*time.Millisecond)
		sess, _ := a.NewSession(ctx, adapter.SessionBundle{WorkspaceRoot: t.TempDir()}, &fakeHost{})
		ch, err := sess.Prompt(ctx, adapter.PromptInput{Text: "go"})
		if err != nil {
			t.Fatalf("prompt: %v", err)
		}
		var failedPayload string
		for ev := range ch {
			if ev.Type == "failed" {
				failedPayload = string(ev.Payload)
			}
		}
		if !strings.Contains(failedPayload, "timeout") {
			t.Fatalf("timeout must surface in failed payload, got %q", failedPayload)
		}
	})

	t.Run("oversized frame rejected as terminal protocol error", func(t *testing.T) {
		// The oversized payload is one giant line ending with a newline so
		// the parser must read the whole frame before rejecting it.
		binary := writeFixture(t, `printf '{"type":"assistant","message":{"content":[{"type":"text","text":"'}}; head -c 10485761 /dev/zero | tr '\0' 'a'; printf '"}]}}\n'`)
		a := initializedAdapter(t, binary, 30*time.Second)
		sess, _ := a.NewSession(ctx, adapter.SessionBundle{WorkspaceRoot: t.TempDir()}, &fakeHost{})
		ch, err := sess.Prompt(ctx, adapter.PromptInput{Text: "go"})
		if err != nil {
			t.Fatalf("prompt: %v", err)
		}
		var failedPayload string
		for ev := range ch {
			if ev.Type == "failed" {
				failedPayload = string(ev.Payload)
			}
		}
		if !strings.Contains(failedPayload, "protocol error") {
			t.Fatalf("oversized frame must be a terminal protocol error, got %q", failedPayload)
		}
	})

	t.Run("error envelope maps to failed", func(t *testing.T) {
		binary := writeFixture(t, `printf '%s\n' '{"type":"result","subtype":"error_during_execution","is_error":true,"error":"boom"}'`)
		a := initializedAdapter(t, binary, 10*time.Second)
		sess, _ := a.NewSession(ctx, adapter.SessionBundle{WorkspaceRoot: t.TempDir()}, &fakeHost{})
		ch, err := sess.Prompt(ctx, adapter.PromptInput{Text: "go"})
		if err != nil {
			t.Fatalf("prompt: %v", err)
		}
		sawFailed := false
		for ev := range ch {
			if ev.Type == "failed" && strings.Contains(string(ev.Payload), "boom") {
				sawFailed = true
			}
			if ev.Type == "completed" {
				t.Fatalf("error envelope must not complete")
			}
		}
		if !sawFailed {
			t.Fatalf("error envelope must map to failed")
		}
	})

	t.Run("cancel is idempotent and waits for exit", func(t *testing.T) {
		binary := writeFixture(t, `sleep 30`)
		a := initializedAdapter(t, binary, 30*time.Second)
		sess, _ := a.NewSession(ctx, adapter.SessionBundle{WorkspaceRoot: t.TempDir()}, &fakeHost{})
		ch, err := sess.Prompt(ctx, adapter.PromptInput{Text: "go"})
		if err != nil {
			t.Fatalf("prompt: %v", err)
		}
		cancelCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		start := time.Now()
		if err := sess.Cancel(cancelCtx); err != nil {
			t.Fatalf("first cancel: %v", err)
		}
		if time.Since(start) > 5*time.Second {
			t.Fatalf("first cancel must wait for exit promptly")
		}
		// Idempotent: second cancel returns promptly and does not panic.
		start = time.Now()
		if err := sess.Cancel(cancelCtx); err != nil {
			t.Fatalf("second cancel: %v", err)
		}
		if time.Since(start) > time.Second {
			t.Fatalf("second cancel must return promptly")
		}
		// Channel must be closed by the cancelled run.
		for range ch {
		}
	})

	t.Run("cancel settles promptly after launch failure", func(t *testing.T) {
		binary := writeFixture(t, fixtureEnvelope)
		a := initializedAdapter(t, binary, 10*time.Second)
		if err := os.Remove(binary); err != nil {
			t.Fatal(err)
		}
		sess, err := a.NewSession(ctx, adapter.SessionBundle{WorkspaceRoot: t.TempDir()}, &fakeHost{})
		if err != nil {
			t.Fatalf("new session must still construct: %v", err)
		}
		if _, err := sess.Prompt(ctx, adapter.PromptInput{Text: "go"}); err == nil {
			t.Fatalf("prompt on deleted binary must fail")
		}
		done := make(chan struct{})
		go func() {
			_ = sess.Cancel(context.Background())
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("Cancel deadlocked after launch failure (non-cancellable context)")
		}
		// Idempotent: a second Cancel also returns promptly.
		done2 := make(chan struct{})
		go func() {
			_ = sess.Cancel(context.Background())
			close(done2)
		}()
		select {
		case <-done2:
		case <-time.After(2 * time.Second):
			t.Fatalf("second Cancel deadlocked after launch failure")
		}
	})

	t.Run("late init frame cannot replace the first captured session_id", func(t *testing.T) {
		binary := writeFixture(t, `printf '%s\n' '{"type":"system","subtype":"init","session_id":"sess_first"}'
printf '%s\n' '{"type":"system","subtype":"init","session_id":"sess_second"}'
printf '%s\n' '{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]}}'
printf '%s\n' '{"type":"result","subtype":"success","is_error":false,"result":"done"}'`)
		a := initializedAdapter(t, binary, 10*time.Second)
		sess, _ := a.NewSession(ctx, adapter.SessionBundle{WorkspaceRoot: t.TempDir()}, &fakeHost{})
		ch, err := sess.Prompt(ctx, adapter.PromptInput{Text: "go"})
		if err != nil {
			t.Fatalf("prompt: %v", err)
		}
		for ev := range ch {
			if ev.Type == "failed" {
				t.Fatalf("unexpected failed event: %s", ev.Payload)
			}
		}
		tp := sess.(adapter.TransportProvider)
		st, ok := tp.SessionTransport()
		if !ok {
			t.Fatalf("transport identity must be captured")
		}
		if st.NativeSessionID != "sess_first" {
			t.Fatalf("native session id = %q, want sess_first (first-wins, N5)", st.NativeSessionID)
		}
	})

	t.Run("prompt transport failure surfaces as clean error", func(t *testing.T) {
		binary := writeFixture(t, fixtureEnvelope)
		a := initializedAdapter(t, binary, 10*time.Second)
		if err := os.Remove(binary); err != nil {
			t.Fatal(err)
		}
		sess, err := a.NewSession(ctx, adapter.SessionBundle{WorkspaceRoot: t.TempDir()}, &fakeHost{})
		if err != nil {
			t.Fatalf("new session must still construct: %v", err)
		}
		if _, err := sess.Prompt(ctx, adapter.PromptInput{Text: "go"}); err == nil {
			t.Fatalf("prompt on deleted binary must fail")
		}
	})

	t.Run("NewSession before Initialize fails closed", func(t *testing.T) {
		binary := writeFixture(t, fixtureEnvelope)
		a := NewAdapter(binary, nil, time.Second)
		if _, err := a.NewSession(ctx, adapter.SessionBundle{WorkspaceRoot: t.TempDir()}, &fakeHost{}); err != adapter.ErrNotInitialized {
			t.Fatalf("NewSession before Initialize = %v, want ErrNotInitialized", err)
		}
	})
}

// TestClaudePromptViaStdin covers the F-05 prompt transport scenario: the
// complete prompt reaches the subprocess through stdin and is absent from
// positional argv, so a 2 MiB fallback prompt can never exceed the ~128 KiB
// per-argument kernel limit.
func TestClaudePromptViaStdin(t *testing.T) {
	ctx := context.Background()
	binary := writeFixture(t, fixtureEnvelope)
	a := initializedAdapter(t, binary, 30*time.Second)
	sess, err := a.NewSession(ctx, adapter.SessionBundle{WorkspaceRoot: t.TempDir()}, &fakeHost{})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	prompt := "CLAUDE_PROMPT_MARKER " + strings.Repeat("x", 1024*1024)
	ch, err := sess.Prompt(ctx, adapter.PromptInput{Text: prompt})
	if err != nil {
		t.Fatalf("prompt: %v", err)
	}
	for ev := range ch {
		if ev.Type == "failed" {
			t.Fatalf("unexpected failed event: %s", ev.Payload)
		}
	}
	logData, err := os.ReadFile(filepath.Join(filepath.Dir(binary), "invocations.log"))
	if err != nil {
		t.Fatal(err)
	}
	logged := string(logData)
	if !strings.Contains(logged, fmt.Sprintf("stdin_len:%d", len(prompt))) {
		t.Fatalf("complete 1 MiB prompt must arrive on stdin (len %d), log: %q", len(prompt), truncate(logged, 512))
	}
	if !strings.Contains(logged, "marker:present") {
		t.Fatalf("prompt marker must be present in stdin, log: %q", truncate(logged, 512))
	}
	if strings.Contains(logged, "CLAUDE_PROMPT_MARKER") && strings.Contains(logged, "argv:CLAUDE_PROMPT_MARKER") {
		t.Fatalf("prompt must never appear in positional argv, log: %q", truncate(logged, 512))
	}
}

// TestClaudePermissionDontAsk covers the F-05 fail-closed permission
// scenario: every headless session carries `--permission-mode dontAsk` so
// execution never waits for a TTY, and a denied session maps to a clean
// failure the engine can fall through from. The deny-vs-auto-allow
// semantics of dontAsk is a pinned envelope expectation (N2): it is not
// live-confirmed against the real binary; the opt-in real-binary variant
// fails loudly on drift.
func TestClaudePermissionDontAsk(t *testing.T) {
	ctx := context.Background()

	t.Run("dontAsk is applied without an explicit permission request", func(t *testing.T) {
		binary := writeFixture(t, fixtureEnvelope)
		a := initializedAdapter(t, binary, 10*time.Second)
		sess, err := a.NewSession(ctx, adapter.SessionBundle{WorkspaceRoot: t.TempDir()}, &fakeHost{})
		if err != nil {
			t.Fatalf("new session: %v", err)
		}
		ch, err := sess.Prompt(ctx, adapter.PromptInput{Text: "go"})
		if err != nil {
			t.Fatalf("prompt: %v", err)
		}
		for ev := range ch {
			if ev.Type == "failed" {
				t.Fatalf("unexpected failed event: %s", ev.Payload)
			}
		}
		logData, err := os.ReadFile(filepath.Join(filepath.Dir(binary), "invocations.log"))
		if err != nil {
			t.Fatal(err)
		}
		logged := string(logData)
		if !strings.Contains(logged, "--permission-mode dontAsk") {
			t.Fatalf("dontAsk permission mode must be applied, log: %q", logged)
		}
	})

	t.Run("denied session maps to clean failure without hanging", func(t *testing.T) {
		// Simulated denial: the harness reports an error result instead of
		// waiting for a TTY answer.
		binary := writeFixture(t, `printf '%s\n' '{"type":"system","subtype":"init","session_id":"sess_deny"}'
printf '%s\n' '{"type":"result","subtype":"error_during_execution","is_error":true,"error":"permission denied"}'`)
		a := initializedAdapter(t, binary, 10*time.Second)
		sess, _ := a.NewSession(ctx, adapter.SessionBundle{WorkspaceRoot: t.TempDir()}, &fakeHost{})
		ch, err := sess.Prompt(ctx, adapter.PromptInput{Text: "go"})
		if err != nil {
			t.Fatalf("prompt: %v", err)
		}
		var failedPayload string
		for ev := range ch {
			if ev.Type == "completed" {
				t.Fatalf("denied session must not complete")
			}
			if ev.Type == "failed" {
				failedPayload = string(ev.Payload)
			}
		}
		if !strings.Contains(failedPayload, "permission denied") {
			t.Fatalf("denied session must fail cleanly, got %q", failedPayload)
		}
	})
}

// TestClaudeZeroTextFailsCleanly covers the F-05 zero-text scenario: a
// stream that ends without assistant text or result text fails cleanly at
// EOF and never reports completion.
func TestClaudeZeroTextFailsCleanly(t *testing.T) {
	ctx := context.Background()
	binary := writeFixture(t, `printf '%s\n' '{"type":"system","subtype":"init","session_id":"sess_zero"}'`)
	a := initializedAdapter(t, binary, 10*time.Second)
	sess, err := a.NewSession(ctx, adapter.SessionBundle{WorkspaceRoot: t.TempDir()}, &fakeHost{})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	ch, err := sess.Prompt(ctx, adapter.PromptInput{Text: "go"})
	if err != nil {
		t.Fatalf("prompt: %v", err)
	}
	var failedPayload string
	for ev := range ch {
		if ev.Type == "failed" {
			failedPayload = string(ev.Payload)
		}
		if ev.Type == "completed" {
			t.Fatalf("zero-text session must not complete")
		}
	}
	if failedPayload == "" {
		t.Fatalf("zero-text session must fail with a reason")
	}
	// Clean failure: message must not classify as terminal.
	for _, marker := range []string{"protocol", "timeout", "terminal", "process_start", "unsupported"} {
		if strings.Contains(failedPayload, marker) {
			t.Fatalf("zero-text failure must be clean, got %q (marker %q)", failedPayload, marker)
		}
	}
}

// TestClaudeAdapter_OptIn exercises the real, opted-in claude binary. It is
// skipped in short mode and without HARO_TEST_CLAUDE_BINARY; when it runs it
// fails loudly on envelope or flag drift (cycle-end gate only).
func TestClaudeAdapter_OptIn(t *testing.T) {
	if testing.Short() {
		t.Skip("skip Claude real-binary variant in short mode")
	}
	bin := os.Getenv("HARO_TEST_CLAUDE_BINARY")
	if bin == "" {
		t.Skip("HARO_TEST_CLAUDE_BINARY not set")
	}
	var _ adapter.Adapter = (*Adapter)(nil)
	ctx := context.Background()
	a := NewAdapter(bin, nil, 300*time.Second)
	pr, err := a.Probe(ctx)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if !pr.Available {
		t.Fatalf("opted-in binary must be available, got %+v", pr)
	}
	if _, err := a.Initialize(ctx, adapter.Capabilities{ProtocolVersion: 1, Permission: true}); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	sess, err := a.NewSession(ctx, adapter.SessionBundle{Instructions: "reply with the single word ok", WorkspaceRoot: t.TempDir()}, &fakeHost{})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	ch, err := sess.Prompt(ctx, adapter.PromptInput{Text: "reply with the single word ok"})
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	sawText := false
	completed := false
	for ev := range ch {
		switch ev.Type {
		case "output_delta":
			sawText = true
		case "completed":
			completed = true
		}
	}
	if !completed || !sawText {
		t.Fatalf("real binary drift: completed=%v sawText=%v", completed, sawText)
	}
	tp, ok := sess.(adapter.TransportProvider)
	if !ok {
		t.Fatalf("session must implement TransportProvider")
	}
	if st, ok := tp.SessionTransport(); !ok || st.NativeSessionID == "" {
		t.Fatalf("real binary must capture a native session_id")
	}
	_ = sess.Cancel(context.Background())
}

// initializedAdapterFrom finishes the probe/initialize lifecycle on an
// already-configured adapter.
func initializedAdapterFrom(t *testing.T, a *Adapter) *Adapter {
	t.Helper()
	ctx := context.Background()
	pr, err := a.Probe(ctx)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if !pr.Available {
		t.Fatalf("fixture must be available, got %+v", pr)
	}
	if _, err := a.Initialize(ctx, adapter.Capabilities{ProtocolVersion: 1}); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	return a
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

type fakeHost struct{}

func (fakeHost) RequestPermission(context.Context, adapter.PermissionRequest) (adapter.PermissionDecision, error) {
	return adapter.PermissionDecision{}, adapter.ErrUnsupportedCapability
}

// Compile-time probes of the session contract surface.
var (
	_ adapter.Session           = (*session)(nil)
	_ adapter.TransportProvider = (*session)(nil)
)
