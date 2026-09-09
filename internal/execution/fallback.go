package execution

import (
	"fmt"
	"strings"
)

// RunFallback iterates harness candidates in order, carrying sanitized accumulated context <=2MiB.
// Clean failures advance to next candidate without semantic classification.
// Terminal failures (unsupported_capability, permission, protocol, terminal, process_start) fail immediately.
// On exhaustion, returns sanitized evidence from all candidates bounded to 2MiB.
func RunFallback(candidates []string, runner func(harness string, fallbackContext string) (string, error)) (string, error) {
	if len(candidates) == 0 {
		return "", fmt.Errorf("no candidates")
	}
	var evidences []string
	var accumulated string
	for i, harness := range candidates {
		output, err := runner(harness, accumulated)
		// Sanitize: Redact -> VisibleEvidence (which already redacts, but ensure)
		visible := VisibleEvidence(output)
		evidences = append(evidences, visible)
		if err == nil {
			// Success: return accumulated visible evidence (all)
			all := strings.Join(evidences, "")
			// Already visible bounded, but fallback also bounded
			all = FallbackEvidence(all)
			return all, nil
		}
		if isTerminal(err) {
			all := strings.Join(evidences, "")
			all = FallbackEvidence(all)
			return all, err
		}
		// Clean failure: accumulate bounded
		accumulated = FallbackEvidence(strings.Join(evidences, ""))
		if i == len(candidates)-1 {
			return accumulated, fmt.Errorf("exhausted candidates: %w", err)
		}
		// continue to next harness, passing accumulated as fallback context
	}
	return "", fmt.Errorf("no candidates")
}

func isTerminal(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// Fail-closed terminal categories
	if strings.Contains(msg, "unsupported_capability") {
		return true
	}
	if strings.Contains(msg, "permission") && strings.Contains(msg, "unsupported") {
		return true
	}
	// Generic terminal markers
	if strings.Contains(msg, "terminal") {
		return true
	}
	if strings.Contains(msg, "protocol") {
		return true
	}
	if strings.Contains(msg, "process_start") {
		return true
	}
	// Also check for ErrUnsupportedCapability string
	if strings.Contains(strings.ToLower(msg), "unsupported") {
		// But not all unsupported are terminal? For F-02, unsupported_capability is terminal (fail immediately)
		return true
	}
	// Timeout is terminal (session deadline exceeded).
	if strings.Contains(msg, "timeout") {
		return true
	}
	// Contract violations (invalid session contract, e.g. requirement
	// containment) are terminal.
	if strings.Contains(msg, "contract") {
		return true
	}
	return false
}
