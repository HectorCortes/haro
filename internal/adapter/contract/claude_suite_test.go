package contract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HectorCortes/haro/internal/adapter"
	"github.com/HectorCortes/haro/internal/adapter/claude"
)

// pinnedClaudeEnvelope reads the pinned synthetic.2 claude envelope from the
// versioned fixture directory so the generated executable emits exactly the
// checked-in frames (N4 provenance: envelope-pinned JSON checked in,
// executables generated in t.TempDir()).
func pinnedClaudeEnvelope(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "fixtures", "synthetic", "v2.0.0-synthetic.2", "claude-fixture.jsonl"))
	if err != nil {
		t.Fatalf("pinned claude fixture: %v", err)
	}
	return string(data)
}

// writeClaudeEnvelopeFixture generates an executable shell fixture in
// t.TempDir() that records argv and stdin and emits the pinned envelope.
func writeClaudeEnvelopeFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "claude-fixture.sh")
	script := "#!/bin/sh\n" +
		"stdin=$(cat)\n" +
		"printf 'argv:%s\\nstdin:%s\\n' \"$*\" \"$stdin\" >> \"$(dirname \"$0\")/invocations.log\"\n" +
		"cat <<'CLAUDE_PINNED_EOF'\n" +
		pinnedClaudeEnvelope(t) +
		"CLAUDE_PINNED_EOF\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestClaudeContractSuiteFixture runs the public-boundary suite against a
// claude adapter backed by the pinned v2.0.0-synthetic.2 envelope, proving
// the subprocess→protocol→JSON→adapter-settlement boundary in -short mode
// without an installed or authenticated real binary (v2-adapter/F-05
// public-boundary fixture scenario). Store and engine settlement assertions
// live in internal/execution; this package stays adapter-focused.
func TestClaudeContractSuiteFixture(t *testing.T) {
	binary := writeClaudeEnvelopeFixture(t)
	factory := func() adapter.Adapter {
		return claude.NewAdapter(binary, nil, 30_000_000_000)
	}
	RunSuite(t, factory)

	// The fixture subprocess received the pinned print-mode argv with the
	// prompt on stdin and no shell.
	logData, err := os.ReadFile(filepath.Join(filepath.Dir(binary), "invocations.log"))
	if err != nil {
		t.Fatalf("fixture log: %v", err)
	}
	logged := string(logData)
	if !strings.Contains(logged, "argv:-p --output-format stream-json --include-partial-messages --permission-mode dontAsk") {
		t.Fatalf("fixture argv must be the pinned print-mode invocation, log: %q", logged)
	}
	if !strings.Contains(logged, "stdin:prompt") {
		t.Fatalf("prompt must arrive on stdin, log: %q", logged)
	}
	if got := strings.Count(logged, "argv:"); got != 1 {
		t.Fatalf("expected exactly 1 fixture invocation, got %d", got)
	}
}

// TestClaudeContractSuiteReal is the opt-in real-binary boundary variant: it
// requires non-short mode plus HARO_TEST_CLAUDE_BINARY and fails loudly on
// envelope or flag drift. Authenticated, cost-bearing E2E runs only at cycle
// end.
func TestClaudeContractSuiteReal(t *testing.T) {
	if testing.Short() {
		t.Skip("skip real-binary variant in short mode")
	}
	bin := os.Getenv("HARO_TEST_CLAUDE_BINARY")
	if bin == "" {
		t.Skip("HARO_TEST_CLAUDE_BINARY not set")
	}
	factory := func() adapter.Adapter {
		return claude.NewAdapter(bin, nil, 0)
	}
	RunSuite(t, factory)
}
