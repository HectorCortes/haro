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


