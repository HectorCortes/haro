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
