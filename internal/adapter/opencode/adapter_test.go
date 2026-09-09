package opencode

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HectorCortes/haro/internal/adapter"
)

// writeFixture writes an executable shell fixture that pins the JSONL
// envelope emitted by `opencode run --format json`. It records argv and the
// stdin prompt into invocations.log next to itself so tests can assert the
// fixed argv, absence of a shell, and prompt-on-stdin transport.
func writeFixture(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "oc-fixture.sh")
	script := "#!/bin/sh\n" +
		"stdin=$(cat)\n" +
		"printf 'argv:%s\\nstdin:%s\\n' \"$*\" \"$stdin\" >> \"$(dirname \"$0\")/invocations.log\"\n" +
		body + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

const fixtureEnvelope = `printf '%s\n' '{"type":"step_start","sessionID":"sess_fixture_1"}'
printf '%s\n' '{"type":"part","sessionID":"sess_fixture_1","part":{"type":"text","text":"hello from fixture"}}'
printf '%s\n' '{"type":"step_finish","sessionID":"sess_fixture_1"}'`

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

// TestOpenCodeRealSessionLifecycle covers the F-01 real-session scenario:
// exec.CommandContext with fixed argv, no shell, prompt on stdin, bounded
// JSONL frames, bounded timeout, and idempotent cancellation.
func TestOpenCodeRealSessionLifecycle(t *testing.T) {
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
		if len(deltas) == 0 || !strings.Contains(strings.Join(deltas, ""), "hello from fixture") {
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
		if st.NativeSessionID != "sess_fixture_1" {
			t.Fatalf("native session id = %q", st.NativeSessionID)
		}
		if st.ProtocolVersion != 1 {
			t.Fatalf("protocol version = %d", st.ProtocolVersion)
		}
		// Subprocess contract: fixed argv, prompt on stdin, no shell.
		logData, err := os.ReadFile(filepath.Join(filepath.Dir(binary), "invocations.log"))
		if err != nil {
			t.Fatalf("fixture log: %v", err)
		}
		logged := string(logData)
		if !strings.Contains(logged, "argv:run --format json") {
			t.Fatalf("fixture argv must be exactly `run --format json`, log: %q", logged)
		}
		if !strings.Contains(logged, "stdin:please summarize") {
			t.Fatalf("prompt must arrive on stdin, log: %q", logged)
		}
		// One subprocess invocation per prompt.
		if got := strings.Count(logged, "argv:"); got != 1 {
			t.Fatalf("expected exactly 1 fixture invocation, got %d", got)
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

	t.Run("zero-text step_start fails cleanly at EOF", func(t *testing.T) {
		binary := writeFixture(t, `printf '%s\n' '{"type":"step_start","sessionID":"sess_zero"}'`)
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
	})

	t.Run("error envelope maps to failed", func(t *testing.T) {
		binary := writeFixture(t, `printf '%s\n' '{"type":"error","sessionID":"sess_e","error":"boom"}'`)
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

	t.Run("oversized frame rejected as protocol error", func(t *testing.T) {
		binary := writeFixture(t, `printf '{"type":"part","part":{"text":"'}}; head -c 10485761 /dev/zero | tr '\0' 'a'; printf '"}}\n'`)
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
		if !strings.Contains(failedPayload, "protocol") {
			t.Fatalf("oversized frame must be a protocol error, got %q", failedPayload)
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
		firstDuration := time.Since(start)
		if firstDuration > 5*time.Second {
			t.Fatalf("first cancel must wait for exit promptly, took %v", firstDuration)
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

	t.Run("prompt transport failure surfaces as clean error", func(t *testing.T) {
		// Missing binary passes probe as unavailable; NewSession on an
		// unprobed-but-deleted binary surfaces a start error.
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
}

type fakeHost struct{}

func (fakeHost) RequestPermission(context.Context, adapter.PermissionRequest) (adapter.PermissionDecision, error) {
	return adapter.PermissionDecision{}, adapter.ErrUnsupportedCapability
}

// TestFactoryBinaryPrecedence lives in internal/adapter/factory; the adapter
// package keeps only the session contract tests above.

// TestCancelReturnsAfterLaunchFailure covers the remediation finding: every
// Prompt early-return path (launch failure, pipe errors) must still close
// the session completion signal so Cancel with a non-cancellable context
// returns promptly instead of deadlocking. Also re-proves idempotent Cancel
// on the failure path.
func TestCancelReturnsAfterLaunchFailure(t *testing.T) {
	ctx := context.Background()

	t.Run("launch failure settles Cancel promptly", func(t *testing.T) {
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
		// Cancel with a non-cancellable context must return promptly.
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

	t.Run("successful path Cancel still settles promptly", func(t *testing.T) {
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
		for range ch {
		}
		done := make(chan struct{})
		go func() {
			_ = sess.Cancel(context.Background())
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("Cancel deadlocked on settled successful session")
		}
	})
}
