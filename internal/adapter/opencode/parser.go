package opencode

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/HectorCortes/haro/internal/ipc/jsonrpc"
)

// Event is a parsed OpenCode JSONL event.
type Event struct {
	Cursor  int             `json:"cursor"`
	Delta   json.RawMessage `json:"delta,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Raw     json.RawMessage
}

// ParseJSONL parses newline-delimited JSON objects, rejecting frames over 10 MiB.
// It uses the same size limit as the JSON-RPC codec.
func ParseJSONL(r io.Reader) ([]Event, error) {
	br := bufio.NewReader(r)
	var events []Event
	for {
		line, err := br.ReadString('\n')
		if err == io.EOF {
			if strings.TrimSpace(line) == "" {
				break
			}
			// No trailing newline, still parse
		} else if err != nil {
			return nil, fmt.Errorf("read: %w", err)
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if err == io.EOF {
				break
			}
			continue
		}
		if len(trimmed) > jsonrpc.MaxMessageSize {
			return nil, fmt.Errorf("message too large: %d > %d", len(trimmed), jsonrpc.MaxMessageSize)
		}
		// UseNumber for numbers
		dec := json.NewDecoder(strings.NewReader(trimmed))
		dec.UseNumber()
		var ev Event
		if err := dec.Decode(&ev); err != nil {
			return nil, fmt.Errorf("decode jsonl: %w", err)
		}
		ev.Raw = json.RawMessage(trimmed)
		events = append(events, ev)
		if err == io.EOF {
			break
		}
		if line == "" {
			break
		}
		// Peek if EOF next? Continue
		if err == io.EOF {
			break
		}
		// If we just processed line with newline, continue loop
		// Check if underlying has no more data: attempt to peek
		if _, err := br.Peek(1); err == io.EOF {
			break
		}
	}
	return events, nil
}
