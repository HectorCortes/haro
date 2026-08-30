package claude

import (
	"context"
	"os"
	"testing"

	"github.com/HectorCortes/haro/internal/adapter"
)

func TestClaudeAdapter_OptIn(t *testing.T) {
	if testing.Short() {
		t.Skip("skip Claude integration in short mode")
	}
	bin := os.Getenv("HARO_TEST_CLAUDE_BINARY")
	if bin == "" {
		t.Skip("HARO_TEST_CLAUDE_BINARY not set")
	}
	var _ adapter.Adapter = (*Adapter)(nil)
	a := NewAdapter(bin)
	pr, err := a.Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	_ = pr
	caps, err := a.Initialize(context.Background(), adapter.Capabilities{ProtocolVersion: 1, Permission: true})
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if caps.ProtocolVersion != 1 {
		t.Fatalf("cap version")
	}
	sess, err := a.NewSession(context.Background(), adapter.SessionBundle{Instructions: "hello"}, &fakeHostClaude{})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	ch, err := sess.Prompt(context.Background(), adapter.PromptInput{Text: "prompt"})
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	ev := <-ch
	if ev.Type != "completed" {
		t.Fatalf("event type = %q", ev.Type)
	}
	_ = sess.Cancel(context.Background())
}

func TestClaudeAdapter_SkipsShort(t *testing.T) {
	a := NewAdapter("claude")
	if a == nil {
		t.Fatalf("NewAdapter nil")
	}
	_, _ = a.Probe(context.Background())
}

type fakeHostClaude struct{}

func (f *fakeHostClaude) RequestPermission(ctx context.Context, req adapter.PermissionRequest) (adapter.PermissionDecision, error) {
	return adapter.PermissionDecision{Option: "allow"}, nil
}
