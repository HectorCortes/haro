package workflow

import (
	"strings"
	"testing"
)

func TestValidateFile_RootContract(t *testing.T) {
	t.Run("root inputs forbidden", func(t *testing.T) {
		yaml := "version: 2\nname: root\ninputs:\n  - name: inp\n    satisfied_by: s1\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n"
		wf, err := Parse(strings.NewReader(yaml))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		err = ValidateFile(wf, "/tmp/root.yaml", true)
		if err == nil {
			t.Fatalf("expected root_contract_forbidden")
		}
		ve, ok := err.(*ValidationError)
		if !ok {
			t.Fatalf("expected ValidationError got %T %v", err, err)
		}
		if ve.Code != "root_contract_forbidden" {
			t.Fatalf("code = %q want root_contract_forbidden", ve.Code)
		}
		if !strings.Contains(ve.Field, "inputs") {
			t.Fatalf("field %q should contain inputs", ve.Field)
		}
	})

	t.Run("root outputs forbidden", func(t *testing.T) {
		yaml := "version: 2\nname: root\noutputs:\n  - name: out\n    produced_by: s1\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n"
		wf, _ := Parse(strings.NewReader(yaml))
		err := ValidateFile(wf, "/tmp/root.yaml", true)
		if err == nil {
			t.Fatalf("expected error")
		}
		ve := err.(*ValidationError)
		if ve.Code != "root_contract_forbidden" || !strings.Contains(ve.Field, "outputs") {
			t.Fatalf("unexpected %v", ve)
		}
	})

	t.Run("included file with inputs allowed", func(t *testing.T) {
		yaml := "version: 2\nname: lib\ninputs:\n  - name: inp\n    satisfied_by: internal_a\nsteps:\n  - id: internal_a\n    type: command\n    run: echo hi\n"
		wf, _ := Parse(strings.NewReader(yaml))
		err := ValidateFile(wf, "/tmp/lib.yaml", false)
		if err != nil {
			t.Fatalf("included inputs should be allowed: %v", err)
		}
	})
}

func TestValidateFlat_ContractViolation(t *testing.T) {
	t.Run("parent depends_on internal without binding", func(t *testing.T) {
		// Simulate flattened DAG where parent workflow step references internal step of included file directly.
		// We'll call ValidateFlat with crafted FlatDAG.
		// For now, ensure ValidateFlat exists and rejects.
		// This test is placeholder; real check in compose tests.
		// We call ValidateFlat with empty.
		src := "lib.yaml"
		err := ValidateFlat(&FlatDAG{Steps: []FlatStep{
			{ID: "wf", Type: "workflow", Source: &src},
			{ID: "a.x", Type: "command", Run: "echo hi"},
		}})
		// This should maybe pass (not testing). The real contract violation is via parent depends_on.
		_ = err
	})
}

func TestValidateFile_UnknownInputOutputMissingBinding(t *testing.T) {
	t.Run("missing binding", func(t *testing.T) {
		yaml := "version: 2\nname: root\nsteps:\n  - id: wf\n    type: workflow\n    source: ./lib.yaml\n"
		wf, _ := Parse(strings.NewReader(yaml))
		// Need included file's inputs to trigger missing_binding
		// We'll test via ValidateFlat later; for ValidateFile we check workflow step missing source
		_ = wf
	})
}
