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

// TestRedactAnthropicTokens covers the U4 redaction extension: the single
// redaction point must cover Anthropic-style sk-ant- tokens and generic sk-
// secret keys alongside Bearer, while innocuous words that merely contain
// "sk-" as a substring stay untouched.
func TestRedactAnthropicTokens(t *testing.T) {
	t.Run("sk-ant token redacted", func(t *testing.T) {
		in := "Authorization: Bearer sk-ant-api03-AbCdEfGh123456_IjKlMnOp"
		got := Redact(in)
		if strings.Contains(got, "sk-ant-api03") {
			t.Fatalf("sk-ant token not redacted: %q", got)
		}
		// Triangulate: a bare sk-ant token with no Bearer prefix must also
		// be redacted by the dedicated pattern.
		bare := "credentials sk-ant-api03-ZqWw12ErTyUiOpAsDf leaked"
		bareGot := Redact(bare)
		if strings.Contains(bareGot, "sk-ant-api03-ZqWw12ErTyUiOpAsDf") {
			t.Fatalf("bare sk-ant token not redacted: %q", bareGot)
		}
	})
	t.Run("generic sk- secret redacted", func(t *testing.T) {
		cases := []string{
			"key sk-1234567890abcdef tail",
			"openai sk-ProjAb12Cd34Ef56Gh78",
			"path=/creds sk-9f8e7d6c5b4a3210f",
		}
		for _, c := range cases {
			got := Redact(c)
			if strings.Contains(got, "sk-1234567890abcdef") || strings.Contains(got, "sk-ProjAb") || strings.Contains(got, "sk-9f8e7d6c5b4a3210f") {
				t.Fatalf("sk- secret not redacted for %q -> %q", c, got)
			}
		}
	})
	t.Run("bearer behavior unchanged", func(t *testing.T) {
		got := Redact("Authorization: Bearer abc.def.ghi-123_456")
		if strings.Contains(got, "abc.def.ghi") {
			t.Fatalf("bearer regression: %q", got)
		}
		if !strings.Contains(got, "Bearer ***") {
			t.Fatalf("bearer placeholder changed: %q", got)
		}
	})
	t.Run("substring sk- inside words stays untouched", func(t *testing.T) {
		in := "task-list and risk-assessment and ask-user stay"
		if got := Redact(in); got != in {
			t.Fatalf("innocuous text must be unchanged, got %q", got)
		}
	})
	t.Run("visible bound applies after sk-ant redaction", func(t *testing.T) {
		oversized := strings.Repeat("a", 20*1024) + " Bearer tok123 sk-ant-api03-secretvalue"
		got := VisibleEvidence(oversized)
		if len(got) > 16*1024 {
			t.Fatalf("visible len = %d, want <= 16384", len(got))
		}
		if strings.Contains(got, "sk-ant-api03-secretvalue") || strings.Contains(got, "tok123") {
			t.Fatalf("secrets must be redacted before truncation: %q", got[:min(len(got), 128)])
		}
	})
}
