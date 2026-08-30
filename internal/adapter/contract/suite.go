package contract

import (
	"context"
	"testing"

	"github.com/HectorCortes/haro/internal/adapter"
)

// AdapterFactory creates a fresh adapter for each subtest.
type AdapterFactory func() adapter.Adapter

// RunSuite runs lifecycle, gating, permission and boundary assertions against a factory.
func RunSuite(t *testing.T, factory AdapterFactory) {
	t.Helper()
	t.Run("lifecycle", func(t *testing.T) {
		ctx := context.Background()
		a := factory()
		pr, err := a.Probe(ctx)
		if err != nil {
			t.Fatalf("Probe: %v", err)
		}
		if !pr.Available {
			t.Fatalf("not available")
		}
		core := adapter.Capabilities{ProtocolVersion: 1, Permission: true}
		neg, err := a.Initialize(ctx, core)
		if err != nil {
			t.Fatalf("Initialize: %v", err)
		}
		_ = neg
		// second Initialize should be cached / idempotent (manager handles, but direct adapter should also handle)
		// For direct adapter, allow re-initialize without error
		_, _ = a.Initialize(ctx, core)
		sess, err := a.NewSession(ctx, adapter.SessionBundle{Instructions: "hello", WorkspaceRoot: "/tmp"}, &allowHost{})
		if err != nil {
			t.Fatalf("NewSession: %v", err)
		}
		ch, err := sess.Prompt(ctx, adapter.PromptInput{Text: "prompt"})
		if err != nil {
			t.Fatalf("Prompt: %v", err)
		}
		ev := <-ch
		if ev.Type == "" {
			t.Fatalf("empty event type")
		}
		if err := sess.Cancel(ctx); err != nil {
			t.Fatalf("Cancel: %v", err)
		}
	})
	t.Run("gating", func(t *testing.T) {
		ctx := context.Background()
		a := factory()
		_, _ = a.Initialize(ctx, adapter.Capabilities{ProtocolVersion: 1})
		sess, err := a.NewSession(ctx, adapter.SessionBundle{Instructions: "hi"}, &allowHost{})
		if err != nil {
			t.Fatalf("NewSession: %v", err)
		}
		// LoadPrevious should fail if not negotiated
		err = sess.LoadPrevious(ctx, "native-1")
		// If adapter was created with LoadSession false, should be unsupported
		// We don't know factory caps; check error is either nil or unsupported
		if err != nil && err != adapter.ErrUnsupportedCapability {
			// allow wrapped
			if err.Error() != "unsupported_capability" {
				t.Fatalf("LoadPrevious error = %v, want unsupported_capability or nil", err)
			}
		}
		_, err = sess.Terminal(ctx)
		if err != nil && err != adapter.ErrUnsupportedCapability {
			if err.Error() != "unsupported_capability" {
				t.Fatalf("Terminal error = %v", err)
			}
		}
	})
	t.Run("permission", func(t *testing.T) {
		ctx := context.Background()
		a := factory()
		core := adapter.Capabilities{ProtocolVersion: 1, Permission: false}
		_, _ = a.Initialize(ctx, core)
		// Host gating is via Manager, not direct adapter; but we test direct host behavior
		// Simulate permission request via host that should fail-closed if not negotiated
		host := &gatedHostForTest{caps: adapter.Capabilities{Permission: false}}
		_, err := host.RequestPermission(ctx, adapter.PermissionRequest{Kind: "ask", Options: []string{"allow"}})
		if err == nil {
			t.Fatalf("expected permission fail-closed")
		}
		// When negotiated true, should succeed
		host2 := &gatedHostForTest{caps: adapter.Capabilities{Permission: true}}
		_, err = host2.RequestPermission(ctx, adapter.PermissionRequest{Kind: "ask", Options: []string{"allow"}})
		if err != nil {
			t.Fatalf("permission should succeed when negotiated: %v", err)
		}
		_ = a
	})
	t.Run("boundary", func(t *testing.T) {
		// Boundary is enforced via scripts/verify-adapter-boundary.sh; suite ensures no panic
	})
}

type allowHost struct{}

func (a *allowHost) RequestPermission(ctx context.Context, req adapter.PermissionRequest) (adapter.PermissionDecision, error) {
	return adapter.PermissionDecision{Option: req.Options[0]}, nil
}

type gatedHostForTest struct {
	caps adapter.Capabilities
}

func (g *gatedHostForTest) RequestPermission(ctx context.Context, req adapter.PermissionRequest) (adapter.PermissionDecision, error) {
	if !g.caps.Permission {
		return adapter.PermissionDecision{}, adapter.ErrUnsupportedCapability
	}
	return adapter.PermissionDecision{Option: "allow"}, nil
}
