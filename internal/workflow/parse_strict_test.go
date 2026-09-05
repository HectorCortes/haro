package workflow

import (
	"strings"
	"testing"
)

func TestParse_Strict(t *testing.T) {
	t.Run("unknown field at root errors with field path", func(t *testing.T) {
		yaml := "version: 2\nname: x\nunknown_field: foo\nsteps:\n  - id: a\n    type: command\n    run: echo hi\n"
		_, err := Parse(strings.NewReader(yaml))
		if err == nil {
			t.Fatalf("expected error for unknown field, got nil")
		}
		ve, ok := err.(*ValidationError)
		if !ok {
			t.Fatalf("expected ValidationError, got %T: %v", err, err)
		}
		if ve.Field == "" {
			t.Fatalf("expected field path, got empty")
		}
	})

	t.Run("Inputs Outputs unmarshal", func(t *testing.T) {
		yaml := "version: 2\nname: x\ninputs:\n  - name: my_input\n    satisfied_by: step_a\noutputs:\n  - name: my_output\n    produced_by: step_b\nsteps:\n  - id: a\n    type: command\n    run: echo hi\n"
		wf, err := Parse(strings.NewReader(yaml))
		if err != nil {
			t.Fatalf("parse inputs/outputs: %v", err)
		}
		if len(wf.Inputs) != 1 || wf.Inputs[0].Name != "my_input" || wf.Inputs[0].SatisfiedBy != "step_a" {
			t.Fatalf("inputs not parsed: %+v", wf.Inputs)
		}
		if len(wf.Outputs) != 1 || wf.Outputs[0].Name != "my_output" || wf.Outputs[0].ProducedBy != "step_b" {
			t.Fatalf("outputs not parsed: %+v", wf.Outputs)
		}
	})

	t.Run("Source and Bindings unmarshal", func(t *testing.T) {
		yaml := "version: 2\nname: x\nsteps:\n  - id: wf_node\n    type: workflow\n    source: ./lib/workflow.yaml\n    bindings:\n      inp: parent_req\n"
		wf, err := Parse(strings.NewReader(yaml))
		if err != nil {
			t.Fatalf("parse source/bindings: %v", err)
		}
		if wf.Steps[0].Source == nil || *wf.Steps[0].Source != "./lib/workflow.yaml" {
			t.Fatalf("source not parsed: %+v", wf.Steps[0].Source)
		}
		if wf.Steps[0].Bindings["inp"] != "parent_req" {
			t.Fatalf("bindings not parsed: %+v", wf.Steps[0].Bindings)
		}
	})

	t.Run("unknown field in step errors", func(t *testing.T) {
		yaml := "version: 2\nname: x\nsteps:\n  - id: a\n    type: command\n    run: echo hi\n    foobar: baz\n"
		_, err := Parse(strings.NewReader(yaml))
		if err == nil {
			t.Fatalf("expected error for unknown field in step")
		}
		ve, ok := err.(*ValidationError)
		if !ok {
			t.Fatalf("expected ValidationError, got %T: %v", err, err)
		}
		if ve.Field == "" {
			t.Fatalf("expected field path for step unknown field")
		}
	})
}
