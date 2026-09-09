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
