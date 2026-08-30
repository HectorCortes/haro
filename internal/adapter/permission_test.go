package adapter

import (
	"context"
	"testing"
)

type capturingHost struct {
	called bool
}

func (c *capturingHost) RequestPermission(_ context.Context, req PermissionRequest) (PermissionDecision, error) {
	c.called = true
	return PermissionDecision{Option: "allow"}, nil
}

type permissionAdapter struct {
	caps Capabilities
	host SessionHost
}

func (p *permissionAdapter) Probe(_ context.Context) (ProbeResult, error) {
	return ProbeResult{Available: true}, nil
}
func (p *permissionAdapter) Initialize(_ context.Context, core Capabilities) (Capabilities, error) {
	// Negotiate: permission is core && agent
	p.caps = Capabilities{
		ProtocolVersion: 1,
		Permission:      core.Permission && true,
	}
	return p.caps, nil
}
func (p *permissionAdapter) NewSession(_ context.Context, _ SessionBundle, host SessionHost) (Session, error) {
	p.host = host
	return &fakeSession{host: host}, nil
}

func TestPermission_FailClosedWhenNotNegotiated(t *testing.T) {
	ctx := context.Background()
	// Case 1: core Permission false => negotiated false => RequestPermission should fail closed
	pa := &permissionAdapter{}
	mgr := NewManager(map[string]Adapter{"test": pa})
	_, err := mgr.Initialize(ctx, "test", Capabilities{ProtocolVersion: 1, Permission: false})
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	inner := &capturingHost{}
	sess, err := mgr.NewSession(ctx, "test", SessionBundle{Instructions: "hi"}, inner)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	// Get the wrapped host via adapter's stored host
	fakeSess, ok := sess.(*fakeSession)
	if !ok {
		t.Fatalf("not fakeSession")
	}
	_, err = fakeSess.host.RequestPermission(ctx, PermissionRequest{Kind: "write", Description: "need", Options: []string{"allow", "deny"}})
	if err == nil {
		t.Fatalf("expected fail-closed error")
	}
	if err != ErrUnsupportedCapability && err.Error() != "unsupported_capability" {
		t.Fatalf("wrong error: %v", err)
	}
	if inner.called {
		t.Fatalf("inner host should not be called when not negotiated")
	}
	// Triangulate: when negotiated true, inner is called
	pa2 := &permissionAdapter{}
	mgr2 := NewManager(map[string]Adapter{"test2": pa2})
	_, err = mgr2.Initialize(ctx, "test2", Capabilities{ProtocolVersion: 1, Permission: true})
	if err != nil {
		t.Fatalf("init2: %v", err)
	}
	inner2 := &capturingHost{}
	sess2, err := mgr2.NewSession(ctx, "test2", SessionBundle{}, inner2)
	if err != nil {
		t.Fatalf("NewSession2: %v", err)
	}
	fakeSess2 := sess2.(*fakeSession)
	decision, err := fakeSess2.host.RequestPermission(ctx, PermissionRequest{Kind: "write", Options: []string{"allow"}})
	if err != nil {
		t.Fatalf("negotiated permission should succeed: %v", err)
	}
	if decision.Option != "allow" {
		t.Fatalf("decision = %q", decision.Option)
	}
	if !inner2.called {
		t.Fatalf("inner host should be called when negotiated")
	}
}
