package execution

import (
	"strings"
	"testing"
)

func TestEvidence_FallbackPipeline(t *testing.T) {
	// Redact -> VisibleEvidence -> FallbackBytes <=2MiB
	oversizedWithSecret := strings.Repeat("a", 100) + " Bearer abc.def.ghi " + strings.Repeat("b", 3*1024*1024)
	redacted := Redact(oversizedWithSecret)
	if strings.Contains(redacted, "abc.def.ghi") {
		t.Fatalf("not redacted")
	}
	visible := VisibleEvidence(redacted)
	if len(visible) > VisibleLimit {
		t.Fatalf("visible > limit")
	}
	fallback := FallbackEvidence(visible + strings.Repeat("c", 3*1024*1024))
	if len(fallback) > FallbackLimit {
		t.Fatalf("fallback > limit")
	}
	// Ensure pipeline: Redact before Visible before Fallback
	pipeline := FallbackEvidence(VisibleEvidence(Redact(oversizedWithSecret + strings.Repeat("x", 3*1024*1024))))
	if len(pipeline) > FallbackLimit {
		t.Fatalf("pipeline fallback > limit")
	}
	if strings.Contains(pipeline, "abc.def") {
		t.Fatalf("pipeline not redacted")
	}
}

func TestEvidence_Exhaustion(t *testing.T) {
	candidates := []string{"h1", "h2", "h3"}
	// Simulate exhaustion: all candidates fail cleanly
	runner := func(harness string, _ string) (string, error) {
		return "output from " + harness + " with Bearer token123", &cleanError{msg: "fail " + harness}
	}
	evidence, err := RunFallback(candidates, runner)
	if err == nil {
		t.Fatalf("expected exhaustion error")
	}
	if !strings.Contains(err.Error(), "exhausted") {
		t.Fatalf("error = %q, want exhausted", err.Error())
	}
	// Evidence should contain sanitized output from every candidate, bounded
	if strings.Contains(evidence, "token123") {
		t.Fatalf("evidence not redacted")
	}
	if len(evidence) > FallbackLimit {
		t.Fatalf("exhaustion evidence > limit: %d", len(evidence))
	}
	// Should contain outputs from all candidates (at least truncated)
	if evidence == "" {
		t.Fatalf("exhaustion evidence empty")
	}
}

type cleanError struct{ msg string }

func (e *cleanError) Error() string { return e.msg }
