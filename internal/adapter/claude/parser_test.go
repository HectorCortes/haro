package claude

import (
	"strings"
	"testing"
)

// TestClaudeParseStreamJSON pins the Claude Code 2.1.245 stream-json envelope
// mapping for v2-adapter/F-05: init captures the native session_id, assistant
// and stream_event deltas carry text, result maps to success/error, and
// malformed or oversized frames fail. The nested `stream_event.event.delta`
// shape is a pinned envelope expectation (N2): it is not live-confirmed
// against the real binary; the opt-in real-binary variant fails loudly on
// drift.
func TestClaudeParseStreamJSON(t *testing.T) {
	t.Run("init frame captures type, subtype, and session_id", func(t *testing.T) {
		input := `{"type":"system","subtype":"init","session_id":"sess_claude_1"}
`
		events, err := ParseStreamJSON(strings.NewReader(input))
		if err != nil {
			t.Fatalf("ParseStreamJSON: %v", err)
		}
		if len(events) != 1 {
			t.Fatalf("events len = %d, want 1", len(events))
		}
		if events[0].Type != "system" || events[0].Subtype != "init" {
			t.Fatalf("type/subtype = %q/%q, want system/init", events[0].Type, events[0].Subtype)
		}
		if events[0].SessionID != "sess_claude_1" {
			t.Fatalf("session_id = %q, want sess_claude_1", events[0].SessionID)
		}
	})

	t.Run("assistant text content maps to text parts", func(t *testing.T) {
		input := `{"type":"assistant","message":{"content":[{"type":"text","text":"hello from claude"}]}}
`
		events, err := ParseStreamJSON(strings.NewReader(input))
		if err != nil {
			t.Fatalf("ParseStreamJSON: %v", err)
		}
		if len(events) != 1 {
			t.Fatalf("events len = %d, want 1", len(events))
		}
		if got := events[0].Texts(); len(got) != 1 || got[0] != "hello from claude" {
			t.Fatalf("texts = %q, want [hello from claude]", got)
		}
	})

	t.Run("nested stream_event delta maps to text chunk", func(t *testing.T) {
		input := `{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"chunk"}}}
`
		events, err := ParseStreamJSON(strings.NewReader(input))
		if err != nil {
			t.Fatalf("ParseStreamJSON: %v", err)
		}
		if len(events) != 1 {
			t.Fatalf("events len = %d, want 1", len(events))
		}
		if events[0].Stream == nil {
			t.Fatalf("stream_event must be captured")
		}
		if events[0].Stream.Type != "content_block_delta" {
			t.Fatalf("event.type = %q, want content_block_delta", events[0].Stream.Type)
		}
		if events[0].Stream.Delta.Type != "text_delta" {
			t.Fatalf("delta.type = %q, want text_delta", events[0].Stream.Delta.Type)
		}
		if got := events[0].DeltaText(); got != "chunk" {
			t.Fatalf("delta text = %q, want chunk", got)
		}
	})

	t.Run("result success carries result text and is_error false", func(t *testing.T) {
		input := `{"type":"result","subtype":"success","is_error":false,"result":"final answer","session_id":"sess_claude_1"}
`
		events, err := ParseStreamJSON(strings.NewReader(input))
		if err != nil {
			t.Fatalf("ParseStreamJSON: %v", err)
		}
		if events[0].Type != "result" || events[0].Subtype != "success" {
			t.Fatalf("type/subtype = %q/%q, want result/success", events[0].Type, events[0].Subtype)
		}
		if events[0].IsError {
			t.Fatalf("is_error must be false for success result")
		}
		if events[0].Result != "final answer" {
			t.Fatalf("result = %q, want final answer", events[0].Result)
		}
	})

	t.Run("result error carries is_error true and error message", func(t *testing.T) {
		input := `{"type":"result","subtype":"error_max_turns","is_error":true,"error":"maximum turns reached"}
`
		events, err := ParseStreamJSON(strings.NewReader(input))
		if err != nil {
			t.Fatalf("ParseStreamJSON: %v", err)
		}
		if !events[0].IsError {
			t.Fatalf("is_error must be true for error result")
		}
		if events[0].Error != "maximum turns reached" {
			t.Fatalf("error = %q, want maximum turns reached", events[0].Error)
		}
	})

	t.Run("top-level error frame carries error message", func(t *testing.T) {
		input := `{"type":"error","error":"boom"}
`
		events, err := ParseStreamJSON(strings.NewReader(input))
		if err != nil {
			t.Fatalf("ParseStreamJSON: %v", err)
		}
		if events[0].Error != "boom" {
			t.Fatalf("error = %q, want boom", events[0].Error)
		}
	})

	t.Run("malformed frame fails", func(t *testing.T) {
		input := `{"type":"assistant","message":` + "\n"
		_, err := ParseStreamJSON(strings.NewReader(input))
		if err == nil {
			t.Fatalf("expected malformed error")
		}
	})

	t.Run("oversized frame over 10 MiB fails", func(t *testing.T) {
		large := strings.Repeat("a", 10*1024*1024+1)
		input := `{"type":"assistant","message":{"content":[{"type":"text","text":"` + large + `"}]}}` + "\n"
		_, err := ParseStreamJSON(strings.NewReader(input))
		if err == nil {
			t.Fatalf("expected oversize error")
		}
		if !strings.Contains(err.Error(), "too large") {
			t.Fatalf("error = %q, want too large", err.Error())
		}
		// Triangulate: a frame just under the limit parses.
		justUnder := strings.Repeat("b", 10*1024*1024-500)
		input2 := `{"type":"assistant","message":{"content":[{"type":"text","text":"` + justUnder + `"}]}}` + "\n"
		events, err := ParseStreamJSON(strings.NewReader(input2))
		if err != nil {
			t.Fatalf("frame just under the limit must parse: %v", err)
		}
		if len(events) != 1 {
			t.Fatalf("events len = %d, want 1", len(events))
		}
	})

	t.Run("numbers are preserved verbatim in raw frames", func(t *testing.T) {
		// UseNumber decoding must not round large integers; the raw frame
		// bytes must survive untouched for downstream diagnostics.
		raw := `{"type":"result","subtype":"success","is_error":false,"result":"ok","duration_ms":12345678901234567890}`
		events, err := ParseStreamJSON(strings.NewReader(raw + "\n"))
		if err != nil {
			t.Fatalf("ParseStreamJSON: %v", err)
		}
		if string(events[0].Raw) != raw {
			t.Fatalf("raw frame not preserved verbatim:\n got %q\nwant %q", string(events[0].Raw), raw)
		}
	})

	t.Run("blank lines are skipped and missing trailing newline still parses", func(t *testing.T) {
		input := "\n" + `{"type":"system","subtype":"init","session_id":"s1"}` + "\n\n" + `{"type":"result","subtype":"success","is_error":false,"result":"done"}`
		events, err := ParseStreamJSON(strings.NewReader(input))
		if err != nil {
			t.Fatalf("ParseStreamJSON: %v", err)
		}
		if len(events) != 2 {
			t.Fatalf("events len = %d, want 2", len(events))
		}
	})
}

// TestClaudeParseStreamJSON_FirstSessionIDWins pins N5: the native session_id
// is captured first-wins across the stream, so a late init frame can never
// replace the identity captured from the first non-empty session_id.
func TestClaudeParseStreamJSON_FirstSessionIDWins(t *testing.T) {
	input := `{"type":"system","subtype":"init","session_id":"sess_first"}
{"type":"system","subtype":"init","session_id":"sess_second"}
{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]}}
{"type":"result","subtype":"success","is_error":false,"result":"done","session_id":"sess_late"}
`
	events, err := ParseStreamJSON(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseStreamJSON: %v", err)
	}
	if got := FirstSessionID(events); got != "sess_first" {
		t.Fatalf("FirstSessionID = %q, want sess_first (first-wins)", got)
	}
	// Triangulate: a stream without any session_id yields the empty string,
	// so the adapter leaves transport identity uncaptured instead of
	// inventing one.
	if got := FirstSessionID([]Event{{Type: "assistant"}}); got != "" {
		t.Fatalf("FirstSessionID without init = %q, want empty", got)
	}
}
