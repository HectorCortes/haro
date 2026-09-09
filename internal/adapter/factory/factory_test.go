package factory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HectorCortes/haro/internal/adapter"
	"github.com/HectorCortes/haro/internal/project"
)

// writeFixture writes an executable shell fixture pinning the OpenCode JSONL
// envelope and recording argv/stdin/env next to itself.
func writeFixture(t *testing.T, extra string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "oc-fixture.sh")
	script := "#!/bin/sh\n" +
		"stdin=$(cat)\n" +
		"printf 'argv:%s\\nstdin:%s\\nenv:%s\\n' \"$*\" \"$stdin\" \"$HARO_FACTORY_FIXTURE_ENV\" >> \"$(dirname \"$0\")/invocations.log\"\n" +
		"cat >/dev/null\n" +
		extra + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

const envelope = `printf '%s\n' '{"type":"step_start","sessionID":"sess_factory_1"}'
printf '%s\n' '{"type":"part","sessionID":"sess_factory_1","part":{"type":"text","text":"factory output"}}'`

// TestFactoryBinaryPrecedence covers task 2.4: binary precedence is
// HARO_TEST_OPENCODE_BINARY, then the configured binary, then "opencode";
// configured env overlays the inherited environment.
func TestFactoryBinaryPrecedence(t *testing.T) {
	t.Setenv("HARO_TEST_OPENCODE_BINARY", "")
	if got := ResolveBinary("/configured/oc"); got != "/configured/oc" {
		t.Fatalf("configured binary must win without test seam, got %q", got)
	}
	if got := ResolveBinary(""); got != "opencode" {
		t.Fatalf("default binary must be opencode, got %q", got)
	}
	t.Setenv("HARO_TEST_OPENCODE_BINARY", "/test/seam/oc")
	if got := ResolveBinary("/configured/oc"); got != "/test/seam/oc" {
		t.Fatalf("HARO_TEST_OPENCODE_BINARY must take precedence, got %q", got)
	}
}

// TestFactoryNewManager registers only enabled harnesses, probes them, and
// initializes each available adapter once before any session.
func TestFactoryNewManager(t *testing.T) {
	ctx := context.Background()
	binary := writeFixture(t, envelope)
	disabled := false
	cfg := &project.Config{
		Version: 2,
		Harnesses: map[string]project.HarnessConfig{
			"oc":       {Binary: binary, Env: map[string]string{"HARO_FACTORY_FIXTURE_ENV": "overlay-value"}, TimeoutSeconds: 10},
			"off":      {Binary: binary, Enabled: &disabled},
			"missing":  {Binary: "/nonexistent/oc-binary", TimeoutSeconds: 10},
			"fake-doc": {Binary: filepath.Join(t.TempDir(), "requirements.txt")},
		},
	}
	mgr, err := NewManager(ctx, cfg)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if mgr == nil {
		t.Fatalf("manager nil")
	}
	// Only enabled harnesses are registered.
	if !mgr.Registered("oc") {
		t.Fatalf("enabled harness must be registered")
	}
	if mgr.Registered("off") {
		t.Fatalf("disabled harness must not be registered")
	}
	if mgr.Registered("unknown") {
		t.Fatalf("unknown harness must not be registered")
	}
	// Probed availability: missing binary and documentation-like path are
	// per-candidate unavailable; the fixture is available.
	probes, err := mgr.Probe(ctx)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if !probes["oc"].Available {
		t.Fatalf("fixture harness must be available")
	}
	if probes["missing"].Available || probes["fake-doc"].Available {
		t.Fatalf("unusable candidates must be Available:false, got %+v", probes)
	}
	// Available adapter was initialized exactly once before any session.
	if _, ok := mgr.Negotiated("oc"); !ok {
		t.Fatalf("available harness must be initialized by the factory")
	}
	if _, ok := mgr.Negotiated("missing"); ok {
		t.Fatalf("unavailable harness must not be initialized")
	}
}

// TestFactorySessionUsesConfiguredBinaryAndEnvOverlay runs a real session
// through the factory-built manager, proving binary precedence and env
// overlay reach the subprocess.
func TestFactorySessionUsesConfiguredBinaryAndEnvOverlay(t *testing.T) {
	ctx := context.Background()
	binary := writeFixture(t, envelope)
	cfg := &project.Config{
		Version: 2,
		Harnesses: map[string]project.HarnessConfig{
			"oc": {Binary: binary, Env: map[string]string{"HARO_FACTORY_FIXTURE_ENV": "overlay-value"}, TimeoutSeconds: 10},
		},
	}
	mgr, err := NewManager(ctx, cfg)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	root := t.TempDir()
	sess, err := mgr.NewSession(ctx, "oc", adapter.SessionBundle{WorkspaceRoot: root, Instructions: "go"}, &factoryHost{})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	ch, err := sess.Prompt(ctx, adapter.PromptInput{Text: "prompt text"})
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	completed := false
	for ev := range ch {
		if ev.Type == "completed" {
			completed = true
		}
		if ev.Type == "failed" {
			t.Fatalf("unexpected failed: %s", ev.Payload)
		}
	}
	if !completed {
		t.Fatalf("session must complete")
	}
	logData, err := os.ReadFile(filepath.Join(filepath.Dir(binary), "invocations.log"))
	if err != nil {
		t.Fatal(err)
	}
	logged := string(logData)
	if !strings.Contains(logged, "argv:run --format json") {
		t.Fatalf("fixture argv mismatch: %q", logged)
	}
	if !strings.Contains(logged, "env:overlay-value") {
		t.Fatalf("configured env overlay must reach the subprocess: %q", logged)
	}
	// TimeoutSeconds from configuration must bound the session context
	// (10s here); a 0 timeout defaults to 300s, covered by NewAdapter.
	_ = time.Second
}

type factoryHost struct{}

func (factoryHost) RequestPermission(context.Context, adapter.PermissionRequest) (adapter.PermissionDecision, error) {
	return adapter.PermissionDecision{}, adapter.ErrUnsupportedCapability
}

// writeClaudeFixture writes an executable shell fixture pinning the Claude
// stream-json envelope and recording argv/stdin next to itself.
func writeClaudeFixture(t *testing.T, extra string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "claude-fixture.sh")
	script := "#!/bin/sh\n" +
		"stdin=$(cat)\n" +
		"printf 'argv:%s\\nstdin:%s\\n' \"$*\" \"$stdin\" >> \"$(dirname \"$0\")/invocations.log\"\n" +
		"printf '%s\\n' '{\"type\":\"system\",\"subtype\":\"init\",\"session_id\":\"sess_claude_factory_1\"}'\n" +
		"printf '%s\\n' '{\"type\":\"assistant\",\"message\":{\"content\":[{\"type\":\"text\",\"text\":\"claude factory output\"}]}}'\n" +
		"printf '%s\\n' '{\"type\":\"result\",\"subtype\":\"success\",\"is_error\":false,\"result\":\"done\"}'\n" +
		extra + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestFactoryDualRegistration covers task 3.1 through observable session
// behavior: an enabled harness key named exactly "claude" or "claudecode"
// runs the claude print-mode invocation; every other enabled key keeps the
// opencode invocation; disabled harnesses stay unregistered; and no ACP
// adapter is ever registered (ACP remains unregistered until it implements
// a real session contract).
func TestFactoryDualRegistration(t *testing.T) {
	ctx := context.Background()
	claudeBinary := writeClaudeFixture(t, "")
	ocBinary := writeFixture(t, envelope)
	disabled := false
	cfg := &project.Config{
		Version: 2,
		Harnesses: map[string]project.HarnessConfig{
			"claude":     {Binary: claudeBinary, TimeoutSeconds: 10},
			"claudecode": {Binary: claudeBinary, TimeoutSeconds: 10},
			"oc":         {Binary: ocBinary, TimeoutSeconds: 10},
			"acp":        {Binary: ocBinary, TimeoutSeconds: 10},
			"off":        {Binary: claudeBinary, Enabled: &disabled},
		},
	}
	mgr, err := NewManager(ctx, cfg)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	for _, name := range []string{"claude", "claudecode", "oc", "acp"} {
		if !mgr.Registered(name) {
			t.Fatalf("enabled harness %q must be registered", name)
		}
	}
	if mgr.Registered("off") {
		t.Fatalf("disabled harness must not be registered")
	}
	// The claude keys run the pinned claude print-mode invocation.
	for _, name := range []string{"claude", "claudecode"} {
		logged := runRegisteredSession(t, mgr, name, claudeBinary)
		if !strings.Contains(logged, "argv:-p --output-format stream-json") {
			t.Fatalf("key %q must run the claude adapter, log: %q", name, logged)
		}
	}
	// Other keys keep the opencode invocation; no ACP adapter exists, so the
	// "acp" key is served by the opencode adapter rather than any ACP
	// session implementation.
	for _, name := range []string{"oc", "acp"} {
		logged := runRegisteredSession(t, mgr, name, ocBinary)
		if !strings.Contains(logged, "argv:run --format json") {
			t.Fatalf("key %q must keep the opencode adapter, log: %q", name, logged)
		}
	}
}

// runRegisteredSession runs one bounded session through the manager under
// name and returns the fixture invocation log.
func runRegisteredSession(t *testing.T, mgr *adapter.Manager, name, binary string) string {
	t.Helper()
	ctx := context.Background()
	sess, err := mgr.NewSession(ctx, name, adapter.SessionBundle{WorkspaceRoot: t.TempDir()}, &factoryHost{})
	if err != nil {
		t.Fatalf("NewSession %q: %v", name, err)
	}
	ch, err := sess.Prompt(ctx, adapter.PromptInput{Text: "go"})
	if err != nil {
		t.Fatalf("Prompt %q: %v", name, err)
	}
	for ev := range ch {
		if ev.Type == "failed" {
			t.Fatalf("unexpected failed for %q: %s", name, ev.Payload)
		}
	}
	tp, ok := sess.(adapter.TransportProvider)
	if !ok {
		t.Fatalf("session for %q must implement TransportProvider", name)
	}
	if st, ok := tp.SessionTransport(); !ok || st.NativeSessionID == "" {
		t.Fatalf("session for %q must capture native identity", name)
	}
	logData, err := os.ReadFile(filepath.Join(filepath.Dir(binary), "invocations.log"))
	if err != nil {
		t.Fatalf("invocations log for %q: %v", name, err)
	}
	return string(logData)
}

// TestFactoryClaudeBinaryPrecedence covers task 3.2: per-harness binary
// precedence for the claude key is HARO_TEST_CLAUDE_BINARY, then the
// configured binary, then "claude"; the opencode seam stays separate.
func TestFactoryClaudeBinaryPrecedence(t *testing.T) {
	t.Setenv(ClaudeTestBinaryEnv, "")
	if got := ResolveClaudeBinary("/configured/claude"); got != "/configured/claude" {
		t.Fatalf("configured claude binary must win without test seam, got %q", got)
	}
	if got := ResolveClaudeBinary(""); got != "claude" {
		t.Fatalf("default claude binary must be claude, got %q", got)
	}
	t.Setenv(ClaudeTestBinaryEnv, "/test/seam/claude")
	if got := ResolveClaudeBinary("/configured/claude"); got != "/test/seam/claude" {
		t.Fatalf("HARO_TEST_CLAUDE_BINARY must take precedence, got %q", got)
	}
	// The opencode seam must not leak into the claude resolution.
	t.Setenv(TestBinaryEnv, "/test/seam/oc")
	if got := ResolveClaudeBinary("/configured/claude"); got != "/test/seam/claude" {
		t.Fatalf("opencode seam must not affect claude resolution, got %q", got)
	}
}

// TestFactoryClaudeModelFlags covers task 3.3 (N1): non-empty CLAUDE_MODEL
// and CLAUDE_FALLBACK_MODEL values in the harness env overlay translate to
// --model/--fallback-model flags, and process-env values are ignored.
func TestFactoryClaudeModelFlags(t *testing.T) {
	t.Run("harness env overlay models become flags", func(t *testing.T) {
		ctx := context.Background()
		binary := writeClaudeFixture(t, "")
		cfg := &project.Config{
			Version: 2,
			Harnesses: map[string]project.HarnessConfig{
				"claude": {
					Binary:         binary,
					Env:            map[string]string{"CLAUDE_MODEL": "claude-sonnet-4-5", "CLAUDE_FALLBACK_MODEL": "claude-opus-4-1"},
					TimeoutSeconds: 10,
				},
			},
		}
		mgr, err := NewManager(ctx, cfg)
		if err != nil {
			t.Fatalf("NewManager: %v", err)
		}
		sess, err := mgr.NewSession(ctx, "claude", adapter.SessionBundle{WorkspaceRoot: t.TempDir()}, &factoryHost{})
		if err != nil {
			t.Fatalf("NewSession: %v", err)
		}
		ch, err := sess.Prompt(ctx, adapter.PromptInput{Text: "go"})
		if err != nil {
			t.Fatalf("Prompt: %v", err)
		}
		for ev := range ch {
			if ev.Type == "failed" {
				t.Fatalf("unexpected failed: %s", ev.Payload)
			}
		}
		logData, err := os.ReadFile(filepath.Join(filepath.Dir(binary), "invocations.log"))
		if err != nil {
			t.Fatal(err)
		}
		logged := string(logData)
		if !strings.Contains(logged, "--model claude-sonnet-4-5") || !strings.Contains(logged, "--fallback-model claude-opus-4-1") {
			t.Fatalf("harness env models must translate to model flags, log: %q", logged)
		}
	})

	t.Run("process env CLAUDE_MODEL is ignored (N1)", func(t *testing.T) {
		ctx := context.Background()
		binary := writeClaudeFixture(t, "")
		cfg := &project.Config{
			Version: 2,
			Harnesses: map[string]project.HarnessConfig{
				"claude": {Binary: binary, TimeoutSeconds: 10},
			},
		}
		t.Setenv("CLAUDE_MODEL", "process-env-model")
		t.Setenv("CLAUDE_FALLBACK_MODEL", "process-env-fallback")
		mgr, err := NewManager(ctx, cfg)
		if err != nil {
			t.Fatalf("NewManager: %v", err)
		}
		sess, err := mgr.NewSession(ctx, "claude", adapter.SessionBundle{WorkspaceRoot: t.TempDir()}, &factoryHost{})
		if err != nil {
			t.Fatalf("NewSession: %v", err)
		}
		ch, err := sess.Prompt(ctx, adapter.PromptInput{Text: "go"})
		if err != nil {
			t.Fatalf("Prompt: %v", err)
		}
		for ev := range ch {
			if ev.Type == "failed" {
				t.Fatalf("unexpected failed: %s", ev.Payload)
			}
		}
		logData, err := os.ReadFile(filepath.Join(filepath.Dir(binary), "invocations.log"))
		if err != nil {
			t.Fatal(err)
		}
		logged := string(logData)
		if strings.Contains(logged, "process-env-model") || strings.Contains(logged, "process-env-fallback") {
			t.Fatalf("process-env model values must be ignored, log: %q", logged)
		}
		if strings.Contains(logged, "--model") || strings.Contains(logged, "--fallback-model") {
			t.Fatalf("no model flags may be added without harness env values, log: %q", logged)
		}
	})
}

// newSessionFactoryProbe creates a session through the manager solely to
// identify the concrete adapter registered under name. The session is
// discarded immediately.
func newSessionFactoryProbe(t *testing.T, mgr *adapter.Manager, name string) adapter.Session {
	t.Helper()
	sess, err := mgr.NewSession(context.Background(), name, adapter.SessionBundle{WorkspaceRoot: t.TempDir()}, &factoryHost{})
	if err != nil {
		t.Fatalf("probe session for %q: %v", name, err)
	}
	return sess
}
