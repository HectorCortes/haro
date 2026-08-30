package cmd

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"syscall"
)

// writeJSON writes v as JSON to w, handling EPIPE as success.
func writeJSON(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	if err != nil {
		if isEPIPE(err) {
			return nil
		}
		return err
	}
	return nil
}

// writeJSONError writes error response {"error":msg,"code":code} to w, handling EPIPE.
func writeJSONError(w io.Writer, msg, code string) error {
	return writeJSON(w, map[string]string{"error": msg, "code": code})
}

// isEPIPE checks if err is EPIPE (including broken/closed pipe).
func isEPIPE(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.EPIPE) {
		return true
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "broken pipe") || strings.Contains(lower, "closed pipe") || strings.Contains(lower, "file already closed") || strings.Contains(lower, "already closed") {
		return true
	}
	return false
}

func containsFold(s, substr string) bool {
	// case-insensitive contains
	return len(s) >= len(substr) && (func() bool {
		lowerS := ""
		lowerSub := ""
		// simple lowercasing without importing strings to avoid cycle? but we can inline
		for _, r := range s {
			if r >= 'A' && r <= 'Z' {
				lowerS += string(r + 32)
			} else {
				lowerS += string(r)
			}
		}
		for _, r := range substr {
			if r >= 'A' && r <= 'Z' {
				lowerSub += string(r + 32)
			} else {
				lowerSub += string(r)
			}
		}
		return len(lowerS) >= len(lowerSub) && (func() bool {
			for i := 0; i <= len(lowerS)-len(lowerSub); i++ {
				if lowerS[i:i+len(lowerSub)] == lowerSub {
					return true
				}
			}
			return false
		})()
	})()
}
