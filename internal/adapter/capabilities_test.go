package adapter

import (
	"encoding/json"
	"testing"
)

func TestCapabilitiesRoundtrip(t *testing.T) {
	original := Capabilities{
		ProtocolVersion: 2,
		Permission:      true,
		Terminal:        false,
		LoadSession:     true,
		Extra: map[string]any{
			"_custom":   "value",
			"_internal": 42.0, // JSON numbers decode as float64
			"plain":     "keep",
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded Capabilities
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if decoded.ProtocolVersion != original.ProtocolVersion {
		t.Fatalf("ProtocolVersion = %d, want %d", decoded.ProtocolVersion, original.ProtocolVersion)
	}
	if decoded.Permission != original.Permission {
		t.Fatalf("Permission = %v, want %v", decoded.Permission, original.Permission)
	}
	if decoded.Terminal != original.Terminal {
		t.Fatalf("Terminal = %v, want %v", decoded.Terminal, original.Terminal)
	}
	if decoded.LoadSession != original.LoadSession {
		t.Fatalf("LoadSession = %v, want %v", decoded.LoadSession, original.LoadSession)
	}

	// Verify "_" prefixed extra keys are preserved.
	for k, v := range original.Extra {
		got, ok := decoded.Extra[k]
		if !ok {
			t.Fatalf("Extra missing key %q", k)
		}
		// JSON unmarshals numbers as float64; compare via string or type-agnostic.
		if gotStr := asString(got); gotStr != asString(v) {
			t.Fatalf("Extra[%q] = %v (%T), want %v (%T)", k, got, got, v, v)
		}
	}

	// Also verify JSON contains expected field names.
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal raw: %v", err)
	}
	for _, wantKey := range []string{"protocolVersion", "permission", "terminal", "loadSession", "extra"} {
		if _, ok := raw[wantKey]; !ok {
			t.Fatalf("JSON missing key %q", wantKey)
		}
	}
}

func asString(v any) string {
	// Simple helper to compare values via JSON string.
	b, _ := json.Marshal(v)
	return string(b)
}
