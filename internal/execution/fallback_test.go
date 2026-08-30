package execution

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestFallback_OrderedCleanNextTerminalFail(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	// Simulate three candidates: first clean failure, second terminal, third not tried
	candidates := []string{"opencode", "claudecode", "acp"}
	calls := []string{}
	runner := func(harness string, fallbackContext string) (string, error) {
		calls = append(calls, harness)
		switch harness {
		case "opencode":
			return "output opencode Bearer secret123", errors.New("exit 1")
		case "claudecode":
			return "output claude", errors.New("unsupported_capability")
		case "acp":
			return "should not be called", nil
		}
		return "", nil
	}
	// Use the fallback logic
	evidence, err := runFallbackWithRunner(candidates, runner)
	if err == nil {
		t.Fatalf("expected error from terminal")
	}
	if !strings.Contains(err.Error(), "unsupported_capability") {
		t.Fatalf("error = %q, want unsupported_capability", err.Error())
	}
	if len(calls) != 2 {
		t.Fatalf("calls = %v, want 2 (third not tried)", calls)
	}
	if calls[0] != "opencode" || calls[1] != "claudecode" {
		t.Fatalf("order wrong: %v", calls)
	}
	// Evidence should be sanitized (Redact) and visible bounded
	if strings.Contains(evidence, "secret123") {
		t.Fatalf("evidence not redacted: %q", evidence)
	}
	// Triangulate: clean failure advances to next which succeeds
	calls2 := []string{}
	runner2 := func(harness string, fallbackContext string) (string, error) {
		calls2 = append(calls2, harness)
		if harness == "opencode" {
			return "first fail", errors.New("exit 1")
		}
		return "second success", nil
	}
	evidence2, err := runFallbackWithRunner([]string{"opencode", "claudecode"}, runner2)
	if err != nil {
		t.Fatalf("expected success via fallback, got %v", err)
	}
	if len(calls2) != 2 {
		t.Fatalf("calls2 = %v, want 2", calls2)
	}
	if evidence2 == "" {
		t.Fatalf("evidence empty")
	}
}

func TestFallback_CleanFailureAdvancesWithoutSemantic(t *testing.T) {
	candidates := []string{"h1", "h2"}
	calls := 0
	runner := func(harness string, _ string) (string, error) {
		calls++
		if harness == "h1" {
			// Simulate clean failure with provider-specific exit code that should NOT be classified
			return "h1 output", errors.New("exit 2: some provider detail")
		}
		return "h2 success", nil
	}
	_, err := runFallbackWithRunner(candidates, runner)
	if err != nil {
		t.Fatalf("expected fallback success, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

// helper to call fallback logic; this will be implemented in fallback.go
func runFallbackWithRunner(candidates []string, runner func(string, string) (string, error)) (string, error) {
	// This wrapper calls the actual implementation
	return RunFallback(candidates, runner)
}
