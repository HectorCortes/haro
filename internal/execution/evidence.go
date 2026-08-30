package execution

import (
	"regexp"
	"strings"
)

const (
	VisibleLimit  = 16 * 1024
	SnapshotLimit = 1 * 1024 * 1024
	FallbackLimit = 2 * 1024 * 1024
)

var (
	bearerRe = regexp.MustCompile(`Bearer\s+[A-Za-z0-9\-_\.]+`)
	basicRe  = regexp.MustCompile(`Basic\s+[A-Za-z0-9+/=]+`)
	// token assignment: key like token, secret, api_token, api-key, etc. followed by : or = and value
	tokenAssignRe = regexp.MustCompile(`(?i)\b(token|secret|api[_-]?token|api[_-]?key|secret[_-]?key|password|passwd|pwd)\b\s*[:=]\s*([^\s"']+)`)
	// GitHub PAT-like tokens
	ghpRe = regexp.MustCompile(`ghp_[A-Za-z0-9]+`)
	// Generic key assignment with token in value? Also handle bearer already covered.
)

// Redact replaces credentials with placeholder.
func Redact(s string) string {
	// Bearer
	s = bearerRe.ReplaceAllString(s, "Bearer ***")
	// Basic
	s = basicRe.ReplaceAllString(s, "Basic ***")
	// token assignments: replace value with ***
	s = tokenAssignRe.ReplaceAllStringFunc(s, func(m string) string {
		idx := strings.Index(m, "=")
		if idx == -1 {
			idx = strings.Index(m, ":")
		}
		if idx == -1 {
			return m
		}
		return strings.TrimSpace(m[:idx+1]) + " ***"
	})
	// ghp tokens
	s = ghpRe.ReplaceAllString(s, "***")
	return s
}

// VisibleEvidence redacts then bounds to 16 KiB.
func VisibleEvidence(s string) string {
	s = Redact(s)
	if len(s) > VisibleLimit {
		return s[:VisibleLimit]
	}
	return s
}

// SnapshotEvidence bounds to 1 MiB (snapshot of produces).
func SnapshotEvidence(s string) string {
	if len(s) > SnapshotLimit {
		return s[:SnapshotLimit]
	}
	return s
}

// FallbackEvidence bounds to 2 MiB (prior context reconstruction).
func FallbackEvidence(s string) string {
	if len(s) > FallbackLimit {
		return s[:FallbackLimit]
	}
	return s
}

// SnapshotBytes is bytes variant for file snapshots (also 1 MiB).
func SnapshotBytes(b []byte) []byte {
	if len(b) > SnapshotLimit {
		return b[:SnapshotLimit]
	}
	return b
}

// FallbackBytes bytes variant for 2 MiB.
func FallbackBytes(b []byte) []byte {
	if len(b) > FallbackLimit {
		return b[:FallbackLimit]
	}
	return b
}
