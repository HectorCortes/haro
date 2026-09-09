package opencode

import (
	"strings"
	"testing"
)

func TestParser_ValidJSONL(t *testing.T) {
	input := `{"cursor":1,"delta":{"text":"hello"}}
{"cursor":2,"delta":{"text":"world"}}
`
	events, err := ParseJSONL(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseJSONL: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events len = %d, want 2", len(events))
	}
	if events[0].Cursor != 1 {
		t.Fatalf("cursor 0 = %d", events[0].Cursor)
	}
}

func TestParser_EnvelopeCapture(t *testing.T) {
	// The fixture-pinned envelope: common real sessionID, type field,
	// part.text, and error fields must all be captured.
	input := `{"type":"step_start","sessionID":"sess_9"}
{"type":"part","sessionID":"sess_9","part":{"type":"text","text":"hello"}}
{"type":"error","sessionID":"sess_9","error":"boom"}
`
	events, err := ParseJSONL(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseJSONL: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("events len = %d, want 3", len(events))
	}
	for i, want := range []string{"sess_9", "sess_9", "sess_9"} {
		if events[i].SessionID != want {
			t.Fatalf("event %d sessionID = %q, want %q", i, events[i].SessionID, want)
		}
	}
	if events[0].Type != "step_start" || events[1].Type != "part" || events[2].Type != "error" {
		t.Fatalf("types = %q %q %q", events[0].Type, events[1].Type, events[2].Type)
	}
	if events[1].Text() != "hello" {
		t.Fatalf("part.text = %q, want hello", events[1].Text())
	}
	if events[2].Error != "boom" {
		t.Fatalf("error = %q, want boom", events[2].Error)
	}
	// Legacy frames without the envelope still parse with empty fields.
	legacy, err := ParseJSONL(strings.NewReader(`{"cursor":1,"delta":{"text":"x"}}` + "\n"))
	if err != nil {
		t.Fatalf("legacy parse: %v", err)
	}
	if legacy[0].SessionID != "" || legacy[0].Type != "" {
		t.Fatalf("legacy envelope must stay empty, got %+v", legacy[0])
	}
}

func TestParser_OversizeViaCodec(t *testing.T) {
	// Oversize payload >10 MiB should be rejected
	large := strings.Repeat("a", 10*1024*1024+1)
	input := `{"cursor":1,"payload":"` + large + `"}` + "\n"
	_, err := ParseJSONL(strings.NewReader(input))
	if err == nil {
		t.Fatalf("expected oversize error")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("error = %q, want too large", err.Error())
	}
	// Triangulate: just under limit should succeed
	justUnder := strings.Repeat("b", 10*1024*1024-500)
	input2 := `{"cursor":1,"payload":"` + justUnder + `"}` + "\n"
	events, err := ParseJSONL(strings.NewReader(input2))
	if err != nil {
		t.Fatalf("just under limit should succeed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events len = %d", len(events))
	}
}

func TestParser_Malformed(t *testing.T) {
	input := `{"cursor":1,"delta":` + "\n" // truncated
	_, err := ParseJSONL(strings.NewReader(input))
	if err == nil {
		t.Fatalf("expected malformed error")
	}
}
