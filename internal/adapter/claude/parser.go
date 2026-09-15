package claude

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/HectorCortes/haro/internal/ipc/jsonrpc"
	"github.com/HectorCortes/haro/internal/limits"
)

// Content is one Claude message content block; text blocks carry output.
type Content struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

// Message is the assistant/system message envelope of a stream-json frame.
type Message struct {
	Content []Content `json:"content,omitempty"`
}

// Delta is the payload of a content_block_delta stream event.
type Delta struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

// StreamEvent is the nested `event` object of a `stream_event` frame. The
// nested shape (`stream_event.event.delta`) is a pinned envelope expectation
// (N2), not live-confirmed against the real binary; the opt-in real-binary
// variant fails loudly on drift.
type StreamEvent struct {
	Type  string `json:"type,omitempty"`
	Delta Delta  `json:"delta,omitempty"`
}

// Event is a parsed Claude Code stream-json frame. Field names pin the
// 2.1.245 CLI envelope: snake_case `session_id`, `message.content[]` text
// blocks, nested `stream_event` deltas, and a terminal `result`.
type Event struct {
	Type      string          `json:"type,omitempty"`
	Subtype   string          `json:"subtype,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	Message   *Message        `json:"message,omitempty"`
	Stream    *StreamEvent    `json:"event,omitempty"`
	Result    string          `json:"result,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
	Error     string          `json:"error,omitempty"`
	Raw       json.RawMessage `json:"-"`
}

// Texts returns the text carried by assistant text content blocks, if any.
func (e Event) Texts() []string {
	if e.Message == nil {
		return nil
	}
	var out []string
	for _, c := range e.Message.Content {
		if c.Type == "text" && c.Text != "" {
			out = append(out, c.Text)
		}
	}
	return out
}

// DeltaText returns the text carried by a text_delta stream event, if any.
func (e Event) DeltaText() string {
	if e.Stream == nil || e.Stream.Delta.Type != "text_delta" {
		return ""
	}
	return e.Stream.Delta.Text
}

// FirstSessionID returns the first non-empty session_id across the parsed
// stream (N5 first-wins): a late init frame can never replace the identity
// captured from the first frame that carried one.
func FirstSessionID(events []Event) string {
	for _, ev := range events {
		if ev.SessionID != "" {
			return ev.SessionID
		}
	}
	return ""
}

// ParseStreamJSON parses newline-delimited Claude stream-json frames,
// rejecting frames over jsonrpc.MaxMessageSize and bounding retained frames.
// It uses UseNumber so numeric literals never lose precision. Any malformed,
// oversized, or over-retained stream is a protocol error; already parsed
// events are returned so callers can preserve preceding terminal evidence.
func ParseStreamJSON(r io.Reader) ([]Event, error) {
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
			return events, fmt.Errorf("decode stream-json frame: %w", err)
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
