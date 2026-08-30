package adapter

import (
	"testing"
)

func TestNegotiate_DriftTable(t *testing.T) {
	cases := []struct {
		name  string
		core  Capabilities
		agent Capabilities
		want  Capabilities
	}{
		{
			name:  "empty",
			core:  Capabilities{ProtocolVersion: 1},
			agent: Capabilities{ProtocolVersion: 1},
			want:  Capabilities{ProtocolVersion: 1},
		},
		{
			name:  "partial permission",
			core:  Capabilities{ProtocolVersion: 1, Permission: true},
			agent: Capabilities{ProtocolVersion: 1, Permission: false},
			want:  Capabilities{ProtocolVersion: 1, Permission: false},
		},
		{
			name:  "complete all",
			core:  Capabilities{ProtocolVersion: 1, Permission: true, Terminal: true, LoadSession: true, Extra: map[string]any{"_a": "1"}},
			agent: Capabilities{ProtocolVersion: 1, Permission: true, Terminal: true, LoadSession: true, Extra: map[string]any{"_a": "1", "_b": "2"}},
			want:  Capabilities{ProtocolVersion: 1, Permission: true, Terminal: true, LoadSession: true, Extra: map[string]any{"_a": "1", "_b": "2"}},
		},
		{
			name:  "future additive",
			core:  Capabilities{ProtocolVersion: 1, Extra: map[string]any{"_future": "new"}},
			agent: Capabilities{ProtocolVersion: 1, Extra: map[string]any{"_future": "new", "_extra2": "x"}},
			want:  Capabilities{ProtocolVersion: 1, Extra: map[string]any{"_future": "new", "_extra2": "x"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Negotiate(tc.core, tc.agent)
			if err != nil {
				t.Fatalf("Negotiate: %v", err)
			}
			if got.ProtocolVersion != tc.want.ProtocolVersion {
				t.Fatalf("ProtocolVersion = %d, want %d", got.ProtocolVersion, tc.want.ProtocolVersion)
			}
			if got.Permission != tc.want.Permission {
				t.Fatalf("Permission = %v, want %v", got.Permission, tc.want.Permission)
			}
			if got.Terminal != tc.want.Terminal {
				t.Fatalf("Terminal = %v, want %v", got.Terminal, tc.want.Terminal)
			}
			if got.LoadSession != tc.want.LoadSession {
				t.Fatalf("LoadSession = %v, want %v", got.LoadSession, tc.want.LoadSession)
			}
			// Check Extra additive preservation
			for k, v := range tc.want.Extra {
				if got.Extra[k] != v {
					t.Fatalf("Extra[%q] = %v, want %v (got %v)", k, got.Extra[k], v, got.Extra)
				}
			}
			// Ensure no major bump needed for additive: version unchanged
			if got.ProtocolVersion != 1 {
				t.Fatalf("unexpected major bump")
			}
		})
	}
}

func TestNegotiate_UnsupportedCapability(t *testing.T) {
	// Missing Terminal and LoadSession should gate
	caps := Capabilities{ProtocolVersion: 1, Permission: false, Terminal: false, LoadSession: false}
	if err := CheckCapability(caps, "Terminal"); err == nil {
		t.Fatalf("expected unsupported_capability for Terminal")
	} else if err.Error() != "unsupported_capability" && err != ErrUnsupportedCapability {
		t.Fatalf("wrong error for Terminal: %v", err)
	}
	if err := CheckCapability(caps, "LoadSession"); err == nil {
		t.Fatalf("expected unsupported_capability for LoadSession")
	}
	// When capability present, should not error
	caps2 := Capabilities{ProtocolVersion: 1, Terminal: true, LoadSession: true}
	if err := CheckCapability(caps2, "Terminal"); err != nil {
		t.Fatalf("Terminal should be supported: %v", err)
	}
	if err := CheckCapability(caps2, "LoadSession"); err != nil {
		t.Fatalf("LoadSession should be supported: %v", err)
	}
	// Permission gate
	if err := CheckCapability(caps, "Permission"); err == nil {
		t.Fatalf("expected unsupported for Permission")
	}
	// Triangulate: unknown additive capability should not gate
	if err := CheckCapability(caps, "_future_unknown"); err != nil {
		t.Fatalf("unknown additive should be allowed (no gate): %v", err)
	}
}

func TestNegotiate_AdditiveRoundTrip(t *testing.T) {
	core := Capabilities{ProtocolVersion: 1, Extra: map[string]any{"_custom": "core", "_shared": "a"}}
	agent := Capabilities{ProtocolVersion: 1, Extra: map[string]any{"_custom": "agent", "_new": "b"}}
	negotiated, err := Negotiate(core, agent)
	if err != nil {
		t.Fatalf("Negotiate: %v", err)
	}
	// Extra should contain union? Our implementation merges agent's extra plus core's
	if negotiated.Extra["_new"] != "b" {
		t.Fatalf("additive _new missing: %v", negotiated.Extra)
	}
	// Ensure future additive does not require major bump
	if negotiated.ProtocolVersion != 1 {
		t.Fatalf("major bump on additive")
	}
}
