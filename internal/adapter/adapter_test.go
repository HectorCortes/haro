package adapter

import (
	"context"
	"errors"
	"testing"
)

// fakeSession is a minimal Session for adapter tests.
type fakeSession struct {
	promptCalled int
	cancelCalled int
	loadCalled   int
	terminalCalled int
	host       SessionHost
}

func (f *fakeSession) Prompt(_ context.Context, input PromptInput) (<-chan SessionEvent, error) {
	f.promptCalled++
	ch := make(chan SessionEvent, 1)
	ch <- SessionEvent{Cursor: 1, Type: "completed", Payload: []byte(input.Text)}
	close(ch)
	return ch, nil
}
func (f *fakeSession) Cancel(_ context.Context) error {
	f.cancelCalled++
	return nil
}
func (f *fakeSession) LoadPrevious(_ context.Context, _ string) error {
	f.loadCalled++
	return nil
}
func (f *fakeSession) Terminal(_ context.Context) (TerminalHandle, error) {
	f.terminalCalled++
	return TerminalHandle{}, nil
}

// fakeAdapter implements Adapter for tests.
type fakeAdapter struct {
	probeResult ProbeResult
	initCalled  int
	sessions    int
}

func (f *fakeAdapter) Probe(_ context.Context) (ProbeResult, error) {
	return f.probeResult, nil
}
func (f *fakeAdapter) Initialize(_ context.Context, core Capabilities) (Capabilities, error) {
	f.initCalled++
	// Return negotiated caps: echo core plus extra
	return Capabilities{
		ProtocolVersion: 1,
		Permission:      core.Permission,
		Terminal:        false,
		LoadSession:     core.LoadSession,
		Extra:           core.Extra,
	}, nil
}
func (f *fakeAdapter) NewSession(_ context.Context, _ SessionBundle, host SessionHost) (Session, error) {
	f.sessions++
	return &fakeSession{host: host}, nil
}

func TestAdapter_ProbeInitializeNewSession(t *testing.T) {
	ctx := context.Background()
	adapter := &fakeAdapter{
		probeResult: ProbeResult{Available: true, Version: "1.0", Capabilities: Capabilities{ProtocolVersion: 1}},
	}
	pr, err := adapter.Probe(ctx)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if !pr.Available {
		t.Fatalf("Available false")
	}
	core := Capabilities{ProtocolVersion: 1, Permission: true, LoadSession: true, Extra: map[string]any{"_custom": "v"}}
	negotiated, err := adapter.Initialize(ctx, core)
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if negotiated.ProtocolVersion != 1 {
		t.Fatalf("ProtocolVersion = %d", negotiated.ProtocolVersion)
	}
	sess, err := adapter.NewSession(ctx, SessionBundle{Instructions: "hello", WorkspaceRoot: "/tmp"}, &fakeHost{})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	ch, err := sess.Prompt(ctx, PromptInput{Text: "prompt"})
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	ev := <-ch
	if ev.Type != "completed" {
		t.Fatalf("event type = %q", ev.Type)
	}
	if err := sess.Cancel(ctx); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	// Triangulate: second session
	sess2, err := adapter.NewSession(ctx, SessionBundle{Instructions: "second"}, &fakeHost{})
	if err != nil {
		t.Fatalf("second NewSession: %v", err)
	}
	if sess2 == nil {
		t.Fatalf("sess2 nil")
	}
}

func TestAdapter_RequestPermissionFailClosed(t *testing.T) {
	// F-03: permission request via host should fail closed if not negotiated
	hostNegotiated := &negotiatedHost{core: Capabilities{Permission: true}}
	decision, err := hostNegotiated.RequestPermission(context.Background(), PermissionRequest{Kind: "ask", Description: "need", Options: []string{"allow", "deny"}})
	if err != nil {
		t.Fatalf("negotiated host should succeed: %v", err)
	}
	if decision.Option == "" {
		t.Fatalf("decision empty")
	}
	hostNotNegotiated := &negotiatedHost{core: Capabilities{Permission: false}}
	_, err = hostNotNegotiated.RequestPermission(context.Background(), PermissionRequest{Kind: "ask", Description: "need", Options: []string{"allow", "deny"}})
	if err == nil {
		t.Fatalf("expected fail-closed error when permission not negotiated")
	}
	if !errors.Is(err, ErrUnsupportedCapability) && !contains(err.Error(), "unsupported_capability") {
		t.Fatalf("error = %q, want unsupported_capability", err.Error())
	}
	// Triangulate: ensure error is fail-closed not nil bypass
	if err.Error() == "" {
		t.Fatalf("error empty")
	}
}

type fakeHost struct{}

func (f *fakeHost) RequestPermission(_ context.Context, req PermissionRequest) (PermissionDecision, error) {
	return PermissionDecision{Option: req.Options[0]}, nil
}

// negotiatedHost is a host that checks capability gating
type negotiatedHost struct {
	core Capabilities
}

func (h *negotiatedHost) RequestPermission(_ context.Context, _ PermissionRequest) (PermissionDecision, error) {
	if !h.core.Permission {
		return PermissionDecision{}, ErrUnsupportedCapability
	}
	return PermissionDecision{Option: "allow"}, nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}
