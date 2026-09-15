package opencode

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/HectorCortes/haro/internal/ipc/jsonrpc"
	"github.com/HectorCortes/haro/internal/limits"
)

// Part is an OpenCode message part; text parts carry the streamed text.
type Part struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

// Event is a parsed OpenCode JSONL event. The envelope pins the documented
// current CLI output: every frame may carry a common real sessionID and a
// type; `part.text` translates to output_delta and `error` to failed.
type Event struct {
	Cursor  int             `json:"cursor,omitempty"`
	Delta   json.RawMessage `json:"delta,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	// Type is the envelope event type, e.g. "step_start", "part", "error".
	Type string `json:"type,omitempty"`
	// SessionID is the common real session id emitted by the CLI.
	SessionID string `json:"sessionID,omitempty"`
	// Part holds the message part, when present.
	Part *Part `json:"part,omitempty"`
	// Error holds the harness error message for `error` events.
	Error string `json:"error,omitempty"`
	Raw   json.RawMessage
}

// Text returns the text carried by a text part, if any.
func (e Event) Text() string {
	if e.Part == nil {
		return ""
	}
	return e.Part.Text
}

// ParseJSONL parses newline-delimited JSON objects, rejecting frames over 10
// MiB and bounding retained frames. It uses the same size limit as the
// JSON-RPC codec and returns preceding events alongside protocol errors.
func ParseJSONL(r io.Reader) ([]Event, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), jsonrpc.MaxMessageSize+1)
	var events []Event
	retained := 0
	for scanner.Scan() {
		trimmed := strings.TrimSpace(scanner.Text())
		if trimmed == "" {
			continue
		}
		if len(trimmed) > jsonrpc.MaxMessageSize {
			return events, fmt.Errorf("message too large: %d > %d", len(trimmed), jsonrpc.MaxMessageSize)
		}
		if retained+len(trimmed) > limits.AdapterEventRetentionLimit {
			return events, fmt.Errorf("event retention limit exceeded: %d > %d", retained+len(trimmed), limits.AdapterEventRetentionLimit)
		}
		dec := json.NewDecoder(strings.NewReader(trimmed))
		dec.UseNumber()
		var ev Event
		if err := dec.Decode(&ev); err != nil {
			return events, fmt.Errorf("decode jsonl: %w", err)
		}
		ev.Raw = json.RawMessage(trimmed)
		events = append(events, ev)
		retained += len(trimmed)
	}
	if err := scanner.Err(); err != nil {
		if strings.Contains(err.Error(), "token too long") {
			return events, fmt.Errorf("message too large: line exceeds %d bytes", jsonrpc.MaxMessageSize)
		}
		return events, fmt.Errorf("read: %w", err)
	}
	return events, nil
}
