package workflow

import (
	"fmt"
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
	// Parent wiring against non-declared internal must be contract_violation at steps[i].depends_on[j] or requires[j]
	t.Run("contract_violation depends_on targets non-declared internal", func(t *testing.T) {
		// Flattened DAG with a step that depends_on a non-existent internal is contract_violation
		// Using ValidateFlat directly: missing dependency yields contract_violation
		dag := &FlatDAG{Steps: []FlatStep{
			{ID: "a.x", Type: "command", Run: "echo hi"},
			{ID: "bad", Type: "command", Run: "echo bad", DependsOn: []string{"a.x.nonexistent"}},
		}}
		err := ValidateFlat(dag)
		if err == nil {
			t.Fatalf("expected contract_violation for depends_on targeting missing internal")
		}
		ve, ok := err.(*ValidationError)
		if !ok {
			t.Fatalf("expected ValidationError got %T %v", err, err)
		}
		if ve.Code != "contract_violation" {
			t.Fatalf("code = %q want contract_violation", ve.Code)
		}
		if !strings.Contains(ve.Field, "depends_on") {
			t.Fatalf("field %q should contain depends_on", ve.Field)
		}
	})

	t.Run("contract_violation requires targets internal step id", func(t *testing.T) {
		// ValidateFlat rejects requires that equals a step ID (artifact vs internal confusion)
		dag := &FlatDAG{Steps: []FlatStep{
			{ID: "a.x", Type: "command", Run: "echo hi"},
			{ID: "consumer", Type: "command", Run: "echo hi", Requires: []string{"a.x"}},
		}}
		err := ValidateFlat(dag)
		if err == nil {
			t.Fatalf("expected contract_violation for requires targeting step")
		}
		ve := err.(*ValidationError)
		if ve.Code != "contract_violation" {
			t.Fatalf("code = %q want contract_violation", ve.Code)
		}
		if !strings.Contains(ve.Field, "requires") {
			t.Fatalf("field %q should contain requires", ve.Field)
		}
	})

	t.Run("contract_violation via flatten parent depends_on internal", func(t *testing.T) {
		// End-to-end: main has workflow node wf including lib with internal step, and a sibling bad depends_on wf.internal
		root := "/tmp/root/.haro/workflows/main/workflow.yaml"
		files := map[string][]byte{
			root: []byte("version: 2\nname: main\nsteps:\n  - id: wf\n    type: workflow\n    source: ../lib/workflow.yaml\n  - id: bad\n    type: command\n    run: echo bad\n    depends_on: [wf.internal]\n"),
			"/tmp/root/.haro/workflows/lib/workflow.yaml": []byte("version: 2\nname: lib\nsteps:\n  - id: internal\n    type: command\n    run: echo internal\n"),
		}
		_, err := Flatten(root, func(p string) ([]byte, error) {
			if d, ok := files[p]; ok {
				return d, nil
			}
			// Clean fallback
			for k, v := range files {
				if strings.Contains(p, "lib") && strings.Contains(k, "lib") {
					return v, nil
				}
				if strings.Contains(p, "main") && strings.Contains(k, "main") {
					return v, nil
				}
			}
			return nil, fmt.Errorf("not found %q", p)
		})
		if err == nil {
			t.Fatalf("expected contract_violation via parent depends_on internal")
		}
		ve, ok := err.(*ValidationError)
		if !ok {
			t.Fatalf("expected ValidationError got %T %v", err, err)
		}
		if ve.Code != "contract_violation" {
			t.Fatalf("code = %q want contract_violation, field %q", ve.Code, ve.Field)
		}
		if !strings.Contains(ve.Field, "depends_on") {
			t.Fatalf("field %q should contain depends_on", ve.Field)
		}
	})

	t.Run("contract_violation inputs satisfied_by not found", func(t *testing.T) {
		wf, _ := Parse(strings.NewReader("version: 2\nname: lib\ninputs:\n  - name: in1\n    satisfied_by: missing_step\nsteps:\n  - id: c1\n    type: command\n    run: echo hi\n"))
		err := ValidateFile(wf, "/tmp/lib.yaml", false)
		if err == nil {
			t.Fatalf("expected contract_violation for inputs satisfied_by missing")
		}
		ve := err.(*ValidationError)
		if ve.Code != "contract_violation" || !strings.Contains(ve.Field, "inputs[0].satisfied_by") {
			t.Fatalf("unexpected %v field %q", ve, ve.Field)
		}
	})

	t.Run("contract_violation outputs produced_by not found", func(t *testing.T) {
		wf, _ := Parse(strings.NewReader("version: 2\nname: lib\noutputs:\n  - name: out1\n    produced_by: missing_step\nsteps:\n  - id: c1\n    type: command\n    run: echo hi\n"))
		err := ValidateFile(wf, "/tmp/lib.yaml", false)
		if err == nil {
			t.Fatalf("expected contract_violation for outputs produced_by missing")
		}
		ve := err.(*ValidationError)
		if ve.Code != "contract_violation" || !strings.Contains(ve.Field, "outputs[0].produced_by") {
			t.Fatalf("unexpected %v", ve)
		}
	})

	t.Run("root_contract_forbidden inputs", func(t *testing.T) {
		wf, _ := Parse(strings.NewReader("version: 2\nname: root\ninputs:\n  - name: inp\n    satisfied_by: s1\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n"))
		err := ValidateFile(wf, "/tmp/root.yaml", true)
		if err == nil {
			t.Fatalf("expected root_contract_forbidden")
		}
		ve := err.(*ValidationError)
		if ve.Code != "root_contract_forbidden" || !strings.Contains(ve.Field, "inputs") {
			t.Fatalf("unexpected %v", ve)
		}
	})

	t.Run("root_contract_forbidden outputs", func(t *testing.T) {
		wf, _ := Parse(strings.NewReader("version: 2\nname: root\noutputs:\n  - name: out\n    produced_by: s1\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n"))
		err := ValidateFile(wf, "/tmp/root.yaml", true)
		if err == nil {
			t.Fatalf("expected root_contract_forbidden")
		}
		ve := err.(*ValidationError)
		if ve.Code != "root_contract_forbidden" || !strings.Contains(ve.Field, "outputs") {
			t.Fatalf("unexpected %v", ve)
		}
	})
}

func TestValidateFile_UnknownInputOutputMissingBinding(t *testing.T) {
	t.Run("unknown_input binding", func(t *testing.T) {
		root := "/tmp/root/.haro/workflows/main/workflow.yaml"
		files := map[string][]byte{
			root: []byte("version: 2\nname: main\nsteps:\n  - id: wf\n    type: workflow\n    source: ../lib/workflow.yaml\n    bindings:\n      unknownKey: some.txt\n"),
			"/tmp/root/.haro/workflows/lib/workflow.yaml": []byte("version: 2\nname: lib\ninputs:\n  - name: in1\n    satisfied_by: c1\noutputs:\n  - name: out1\n    produced_by: p1\nsteps:\n  - id: c1\n    type: command\n    run: echo hi\n    requires: [in1]\n  - id: p1\n    type: command\n    run: echo hi\n    produces: [out1]\n"),
		}
		_, err := Flatten(root, func(p string) ([]byte, error) {
			for k, v := range files {
				if p == k {
					return v, nil
				}
			}
			return nil, fmt.Errorf("not found %q", p)
		})
		if err == nil {
			t.Fatalf("expected unknown_input")
		}
		ve, ok := err.(*ValidationError)
		if !ok {
			t.Fatalf("expected ValidationError got %T %v", err, err)
		}
		if ve.Code != "unknown_input" && ve.Code != "unknown_output" {
			t.Fatalf("code = %q want unknown_input|unknown_output", ve.Code)
		}
		if !strings.Contains(ve.Field, "bindings.unknownKey") {
			t.Fatalf("field %q should contain bindings.unknownKey", ve.Field)
		}
	})

	t.Run("missing_binding for input", func(t *testing.T) {
		root := "/tmp/root/.haro/workflows/main/workflow.yaml"
		files := map[string][]byte{
			root: []byte("version: 2\nname: main\nsteps:\n  - id: wf\n    type: workflow\n    source: ../lib/workflow.yaml\n"),
			"/tmp/root/.haro/workflows/lib/workflow.yaml": []byte("version: 2\nname: lib\ninputs:\n  - name: src\n    satisfied_by: consumer\nsteps:\n  - id: consumer\n    type: command\n    run: echo hi\n    requires: [src]\n"),
		}
		_, err := Flatten(root, func(p string) ([]byte, error) {
			for k, v := range files {
				if p == k {
					return v, nil
				}
			}
			return nil, fmt.Errorf("not found %q", p)
		})
		if err == nil {
			t.Fatalf("expected missing_binding")
		}
		ve := err.(*ValidationError)
		if ve.Code != "missing_binding" {
			t.Fatalf("code = %q want missing_binding", ve.Code)
		}
		if !strings.Contains(ve.Field, "bindings.src") {
			t.Fatalf("field %q should contain bindings.src", ve.Field)
		}
	})

	t.Run("missing_binding for output", func(t *testing.T) {
		root := "/tmp/root/.haro/workflows/main/workflow.yaml"
		files := map[string][]byte{
			root: []byte("version: 2\nname: main\nsteps:\n  - id: wf\n    type: workflow\n    source: ../lib/workflow.yaml\n"),
			"/tmp/root/.haro/workflows/lib/workflow.yaml": []byte("version: 2\nname: lib\noutputs:\n  - name: out\n    produced_by: producer\nsteps:\n  - id: producer\n    type: command\n    run: echo hi\n    produces: [out]\n"),
		}
		_, err := Flatten(root, func(p string) ([]byte, error) {
			for k, v := range files {
				if p == k {
					return v, nil
				}
			}
			return nil, fmt.Errorf("not found %q", p)
		})
		if err == nil {
			t.Fatalf("expected missing_binding for output")
		}
		ve := err.(*ValidationError)
		if ve.Code != "missing_binding" {
			t.Fatalf("code = %q want missing_binding", ve.Code)
		}
		if !strings.Contains(ve.Field, "bindings.out") {
			t.Fatalf("field %q should contain bindings.out", ve.Field)
		}
	})

	t.Run("F-04 bound artifacts success path", func(t *testing.T) {
		// Valid contracts and complete bindings must rewire parent requires/produces into included inputs/outputs correctly
		root := "/tmp/r/.haro/workflows/main/workflow.yaml"
		files := map[string][]byte{
			root: []byte("version: 2\nname: main\nsteps:\n  - id: wf\n    type: workflow\n    source: ../lib/workflow.yaml\n    bindings:\n      src: parent_src.txt\n      out: parent_out.txt\n  - id: downstream\n    type: command\n    run: echo downstream\n    requires: [parent_out.txt]\n    depends_on: [wf]\n"),
			"/tmp/r/.haro/workflows/lib/workflow.yaml": []byte("version: 2\nname: lib\ninputs:\n  - name: src\n    satisfied_by: consumer\noutputs:\n  - name: out\n    produced_by: producer\nsteps:\n  - id: consumer\n    type: command\n    run: echo consumer\n    requires: [src]\n  - id: producer\n    type: command\n    run: echo producer\n    produces: [out]\n"),
		}
		dag, err := Flatten(root, func(p string) ([]byte, error) {
			for k, v := range files {
				if p == k {
					return v, nil
				}
			}
			return nil, fmt.Errorf("not found %q", p)
		})
		if err != nil {
			t.Fatalf("Flatten should succeed for valid bindings: %v", err)
		}
		// Verify rewiring: consumer requires parent_src.txt, producer produces parent_out.txt
		var consumer, producer, downstream *FlatStep
		for i := range dag.Steps {
			switch dag.Steps[i].ID {
			case "wf.consumer":
				consumer = &dag.Steps[i]
			case "wf.producer":
				producer = &dag.Steps[i]
			case "downstream":
				downstream = &dag.Steps[i]
			}
		}
		if consumer == nil || producer == nil || downstream == nil {
			t.Fatalf("steps not found: %+v", dag.Steps)
		}
		if len(consumer.Requires) != 1 || consumer.Requires[0] != "parent_src.txt" {
			t.Fatalf("consumer requires = %v want [parent_src.txt]", consumer.Requires)
		}
		if len(producer.Produces) != 1 || producer.Produces[0] != "parent_out.txt" {
			t.Fatalf("producer produces = %v want [parent_out.txt]", producer.Produces)
		}
		// downstream depends_on wf should have been expanded to producer
		found := false
		for _, d := range downstream.DependsOn {
			if d == "wf.producer" {
				found = true
			}
		}
		if !found {
			t.Fatalf("downstream depends_on = %v should contain wf.producer", downstream.DependsOn)
		}
		// ValidateFlat should pass
		if err := ValidateFlat(dag); err != nil {
			t.Fatalf("ValidateFlat should pass: %v", err)
		}
	})
}
