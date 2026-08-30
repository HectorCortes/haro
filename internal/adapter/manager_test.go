package adapter

import (
	"context"
	"testing"
)

func TestManager_SinglePriorInitialize(t *testing.T) {
	ctx := context.Background()
	fa := &fakeAdapter{
		probeResult: ProbeResult{Available: true, Version: "1.0"},
	}
	mgr := NewManager(map[string]Adapter{"opencode": fa})
	// NewSession before Initialize should fail
	_, err := mgr.NewSession(ctx, "opencode", SessionBundle{Instructions: "hi"}, &fakeHost{})
	if err == nil {
		t.Fatalf("expected error when NewSession before Initialize")
	}
	if err != ErrNotInitialized {
		// allow wrapped
		if err.Error() != ErrNotInitialized.Error() && !contains(err.Error(), "not initialized") {
			t.Fatalf("wrong error before init: %v", err)
		}
	}
	// First Initialize should succeed
	core := Capabilities{ProtocolVersion: 1, Permission: true}
	neg, err := mgr.Initialize(ctx, "opencode", core)
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if neg.ProtocolVersion != 1 {
		t.Fatalf("neg ProtocolVersion = %d", neg.ProtocolVersion)
	}
	if fa.initCalled != 1 {
		t.Fatalf("initCalled = %d, want 1", fa.initCalled)
	}
	// Second Initialize should not re-call adapter (cached) or should return same
	neg2, err := mgr.Initialize(ctx, "opencode", core)
	if err != nil {
		t.Fatalf("second Initialize: %v", err)
	}
	if neg2.ProtocolVersion != neg.ProtocolVersion {
		t.Fatalf("second negotiate mismatch")
	}
	if fa.initCalled != 1 {
		t.Fatalf("second initialize should be cached, initCalled = %d", fa.initCalled)
	}
	// Now NewSession should succeed
	sess, err := mgr.NewSession(ctx, "opencode", SessionBundle{Instructions: "hello"}, &fakeHost{})
	if err != nil {
		t.Fatalf("NewSession after init: %v", err)
	}
	if sess == nil {
		t.Fatalf("sess nil")
	}
	// Triangulate: another NewSession should still succeed without re-initialize
	sess2, err := mgr.NewSession(ctx, "opencode", SessionBundle{Instructions: "second"}, &fakeHost{})
	if err != nil {
		t.Fatalf("second NewSession: %v", err)
	}
	if sess2 == nil {
		t.Fatalf("sess2 nil")
	}
	if fa.initCalled != 1 {
		t.Fatalf("NewSession should not trigger Initialize, initCalled = %d", fa.initCalled)
	}
}

func TestManager_Probe(t *testing.T) {
	ctx := context.Background()
	fa := &fakeAdapter{probeResult: ProbeResult{Available: true, Version: "2.0"}}
	mgr := NewManager(map[string]Adapter{"acp": fa})
	results, err := mgr.Probe(ctx)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if !results["acp"].Available {
		t.Fatalf("probe not available")
	}
}
