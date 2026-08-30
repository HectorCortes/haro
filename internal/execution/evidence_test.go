package execution

import (
	"strings"
	"testing"
)

func TestEvidenceBudgets(t *testing.T) {
	// Redaction: Bearer, Basic, token assignments
	t.Run("redact bearer", func(t *testing.T) {
		in := "Authorization: Bearer abc.def.ghi-123_456 and more"
		got := Redact(in)
		if strings.Contains(got, "abc.def.ghi") {
			t.Fatalf("bearer not redacted: %q", got)
		}
		if !strings.Contains(got, "Bearer") {
			t.Fatalf("Bearer keyword should remain, got %q", got)
		}
	})
	t.Run("redact basic", func(t *testing.T) {
		in := "Authorization: Basic dXNlcjpwYXNz"
		got := Redact(in)
		if strings.Contains(got, "dXNlcjpwYXNz") {
			t.Fatalf("basic not redacted: %q", got)
		}
	})
	t.Run("redact token assignment", func(t *testing.T) {
		cases := []string{
			"token=secret123",
			"api_token: my-secret-value",
			"SECRET_KEY=abcd1234",
			"ghp_1234567890abcdef",
		}
		for _, c := range cases {
			got := Redact(c)
			// Should not contain original secret literal after redaction (at least part)
			if strings.Contains(got, "secret123") || strings.Contains(got, "my-secret-value") || strings.Contains(got, "abcd1234") {
				t.Fatalf("token not redacted for %q -> %q", c, got)
			}
		}
	})

	// Visible ≤16 KiB after redaction
	t.Run("visible budget", func(t *testing.T) {
		oversized := strings.Repeat("a", 20*1024) + " Bearer token123"
		got := VisibleEvidence(oversized)
		if len(got) > 16*1024 {
			t.Fatalf("visible len = %d, want <= 16384", len(got))
		}
		if strings.Contains(got, "token123") {
			t.Fatalf("visible should redact before truncating")
		}
		// Triangulation: small input unchanged except redaction
		small := "hello Bearer abc123 world"
		got2 := VisibleEvidence(small)
		if len(got2) > 16*1024 {
			t.Fatalf("small visible too large")
		}
		if strings.Contains(got2, "abc123") {
			t.Fatalf("small visible not redacted")
		}
	})

	t.Run("snapshot budget", func(t *testing.T) {
		// snapshots ≤1 MiB
		oversized := strings.Repeat("b", 2*1024*1024)
		got := SnapshotEvidence(oversized)
		if len(got) > 1*1024*1024 {
			t.Fatalf("snapshot len = %d, want <= 1048576", len(got))
		}
		// triangulation: small snapshot unchanged
		small := "small snapshot"
		got2 := SnapshotEvidence(small)
		if got2 != small {
			t.Fatalf("small snapshot changed: %q vs %q", got2, small)
		}
	})

	t.Run("fallback budget", func(t *testing.T) {
		// fallback ≤2 MiB (bounded prior context + feedback)
		oversized := strings.Repeat("c", 3*1024*1024)
		got := FallbackEvidence(oversized)
		if len(got) > 2*1024*1024 {
			t.Fatalf("fallback len = %d, want <= 2097152", len(got))
		}
		// triangulation: small fallback unchanged
		small := "fallback small"
		got2 := FallbackEvidence(small)
		if got2 != small {
			t.Fatalf("small fallback changed")
		}
	})

	// Ensure pure functions and deterministic truncation
	t.Run("visible truncation deterministic", func(t *testing.T) {
		data := strings.Repeat("x", 16*1024+100)
		got := VisibleEvidence(data)
		if len(got) != 16*1024 {
			t.Fatalf("visible truncation len = %d, want 16384", len(got))
		}
	})
}
