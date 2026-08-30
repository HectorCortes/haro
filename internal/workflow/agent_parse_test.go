package workflow

import (
	"strings"
	"testing"
)

func TestParseAgent(t *testing.T) {
	yaml := "version: 2\nname: demo\nsteps:\n  - id: gen\n    type: agent\n    harness: [opencode, claudecode]\n    instructions: \"do something\"\n    mode: headless\n"
	wf, err := Parse(strings.NewReader(yaml))
	if err != nil {
		t.Fatalf("Parse agent: %v", err)
	}
	if len(wf.Steps) != 1 {
		t.Fatalf("steps len = %d", len(wf.Steps))
	}
	s := wf.Steps[0]
	if s.Type != "agent" {
		t.Fatalf("type = %q, want agent", s.Type)
	}
	if len(s.Harness) != 2 || s.Harness[0] != "opencode" || s.Harness[1] != "claudecode" {
		t.Fatalf("harness = %v, want [opencode claudecode]", s.Harness)
	}
	if s.Instructions != "do something" {
		t.Fatalf("instructions = %q, want do something", s.Instructions)
	}
	if s.Mode != "headless" {
		t.Fatalf("mode = %q, want headless", s.Mode)
	}
	// Terminal false: mode terminal should be rejected by validation (deferred)
	// But for this slice, we enforce Terminal false via validation: PTY deferred
	// So parsing mode terminal should still parse but Validate should fail
	yaml2 := "version: 2\nname: demo\nsteps:\n  - id: gen2\n    type: agent\n    harness: [opencode]\n    instructions: \"x\"\n    mode: terminal\n"
	wf2, err := Parse(strings.NewReader(yaml2))
	if err != nil {
		t.Fatalf("Parse terminal mode: %v", err)
	}
	if wf2.Steps[0].Mode != "terminal" {
		t.Fatalf("mode = %q, want terminal", wf2.Steps[0].Mode)
	}
	// Validation should reject terminal mode when PTY deferred (Terminal false)
	if err := Validate(wf2); err == nil {
		t.Fatalf("expected Validate to reject terminal mode, got nil")
	}
	// Triangulate: minimal agent with single harness
	yaml3 := "version: 2\nname: demo\nsteps:\n  - id: a1\n    type: agent\n    harness: [acp]\n    instructions: \"minimal\"\n"
	wf3, err := Parse(strings.NewReader(yaml3))
	if err != nil {
		t.Fatalf("Parse minimal agent: %v", err)
	}
	if len(wf3.Steps[0].Harness) != 1 || wf3.Steps[0].Harness[0] != "acp" {
		t.Fatalf("harness minimal = %v", wf3.Steps[0].Harness)
	}
}
