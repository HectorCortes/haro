package workflow

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestU01(t *testing.T) {
	t.Run("basic nested hash twice identical", func(t *testing.T) {
		root := t.TempDir()
		base := filepath.Join(root, ".haro", "workflows")
		mainPath := filepath.Join(base, "main", "workflow.yaml")
		libPath := filepath.Join(base, "lib", "workflow.yaml")
		nestedPath := filepath.Join(base, "nested", "workflow.yaml")
		files := map[string][]byte{
			filepath.Clean(mainPath):   []byte("version: 2\nname: main\nsteps:\n  - id: wf\n    type: workflow\n    source: ../lib/workflow.yaml\n"),
			filepath.Clean(libPath):    []byte("version: 2\nname: lib\nsteps:\n  - id: inner\n    type: workflow\n    source: ../nested/workflow.yaml\n"),
			filepath.Clean(nestedPath): []byte("version: 2\nname: nested\nsteps:\n  - id: leaf\n    type: command\n    run: echo hi\n"),
		}
		readFile := func(p string) ([]byte, error) {
			if d, ok := files[filepath.Clean(p)]; ok {
				return d, nil
			}
			return nil, nil
		}
		dag1, err := Flatten(mainPath, readFile)
		if err != nil {
			t.Fatalf("flatten1: %v", err)
		}
		dag2, err := Flatten(mainPath, readFile)
		if err != nil {
			t.Fatalf("flatten2: %v", err)
		}
		if dag1.Hash != dag2.Hash {
			t.Fatalf("hash mismatch %q vs %q", dag1.Hash, dag2.Hash)
		}
		if len(dag1.Steps) != len(dag2.Steps) {
			t.Fatalf("steps length mismatch")
		}
		for i := range dag1.Steps {
			if dag1.Steps[i].ID != dag2.Steps[i].ID {
				t.Fatalf("order mismatch %d", i)
			}
		}
	})
}

func TestU03_ContractTable(t *testing.T) {
	// Full rejection table: root contracts, direct internal reference, unknown input/output, missing binding, unsatisfied mapping
	t.Run("root_contract_forbidden inputs", func(t *testing.T) {
		wf, _ := Parse(strings.NewReader("version: 2\nname: root\ninputs:\n  - name: inp\n    satisfied_by: s1\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n"))
		err := ValidateFile(wf, "/tmp/root.yaml", true)
		if err == nil {
			t.Fatalf("expected root_contract_forbidden")
		}
		ve, _ := err.(*ValidationError)
		if ve.Code != "root_contract_forbidden" || ve.Field != "inputs" {
			t.Fatalf("code=%q field=%q want root_contract_forbidden@inputs", ve.Code, ve.Field)
		}
	})
	t.Run("root_contract_forbidden outputs", func(t *testing.T) {
		wf, _ := Parse(strings.NewReader("version: 2\nname: root\noutputs:\n  - name: out\n    produced_by: s1\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n"))
		err := ValidateFile(wf, "/tmp/root.yaml", true)
		if err == nil {
			t.Fatalf("expected root_contract_forbidden")
		}
		ve, _ := err.(*ValidationError)
		if ve.Code != "root_contract_forbidden" || ve.Field != "outputs" {
			t.Fatalf("code=%q field=%q want root_contract_forbidden@outputs", ve.Code, ve.Field)
		}
	})
	t.Run("contract_violation direct internal depends_on", func(t *testing.T) {
		// Parent step depends_on wf.internal where internal is inside included file not exposed
		root := "/tmp/root/.haro/workflows/main/workflow.yaml"
		files := map[string][]byte{
			root: []byte("version: 2\nname: main\nsteps:\n  - id: wf\n    type: workflow\n    source: ../lib/workflow.yaml\n  - id: bad\n    type: command\n    run: echo bad\n    depends_on: [wf.internal]\n"),
			"/tmp/root/.haro/workflows/lib/workflow.yaml": []byte("version: 2\nname: lib\nsteps:\n  - id: internal\n    type: command\n    run: echo internal\n"),
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
			t.Fatalf("expected contract_violation for direct internal reference")
		}
		ve, ok := err.(*ValidationError)
		if !ok || ve.Code != "contract_violation" || !strings.Contains(ve.Field, "depends_on") {
			t.Fatalf("expected contract_violation@steps[i].depends_on[j] got %v", err)
		}
	})
	t.Run("contract_violation requires targets internal step", func(t *testing.T) {
		dag := &FlatDAG{Steps: []FlatStep{
			{ID: "a.x", Type: "command", Run: "echo hi"},
			{ID: "consumer", Type: "command", Run: "echo hi", Requires: []string{"a.x"}},
		}}
		err := ValidateFlat(dag)
		if err == nil {
			t.Fatalf("expected contract_violation@requires")
		}
		ve, _ := err.(*ValidationError)
		if ve.Code != "contract_violation" || !strings.Contains(ve.Field, "requires") {
			t.Fatalf("code=%q field=%q want contract_violation@steps[i].requires[j]", ve.Code, ve.Field)
		}
	})
	t.Run("unknown_input binding", func(t *testing.T) {
		root := "/tmp/root/.haro/workflows/main/workflow.yaml"
		files := map[string][]byte{
			root: []byte("version: 2\nname: main\nsteps:\n  - id: wf\n    type: workflow\n    source: ../lib/workflow.yaml\n    bindings:\n      unknownKey: some.txt\n"),
			"/tmp/root/.haro/workflows/lib/workflow.yaml": []byte("version: 2\nname: lib\ninputs:\n  - name: in1\n    satisfied_by: c1\nsteps:\n  - id: c1\n    type: command\n    run: echo hi\n    requires: [in1]\n"),
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
			t.Fatalf("expected unknown_input|unknown_output")
		}
		ve, _ := err.(*ValidationError)
		if ve.Code != "unknown_input" && ve.Code != "unknown_output" {
			t.Fatalf("code=%q want unknown_input|unknown_output", ve.Code)
		}
		if !strings.Contains(ve.Field, "bindings.unknownKey") {
			t.Fatalf("field=%q want bindings.unknownKey", ve.Field)
		}
		if ve.Field != "steps[0].bindings.unknownKey" {
			t.Fatalf("field = %q want steps[0].bindings.unknownKey", ve.Field)
		}
	})
	t.Run("missing_binding input", func(t *testing.T) {
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
		ve, _ := err.(*ValidationError)
		if ve.Code != "missing_binding" || ve.Field != "steps[0].bindings.src" {
			t.Fatalf("code=%q field=%q want missing_binding@steps[0].bindings.src", ve.Code, ve.Field)
		}
	})
	t.Run("missing_binding output", func(t *testing.T) {
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
		ve, _ := err.(*ValidationError)
		if ve.Code != "missing_binding" || ve.Field != "steps[0].bindings.out" {
			t.Fatalf("code=%q field=%q want missing_binding@steps[0].bindings.out", ve.Code, ve.Field)
		}
	})
	t.Run("contract_violation inputs satisfied_by", func(t *testing.T) {
		wf, _ := Parse(strings.NewReader("version: 2\nname: lib\ninputs:\n  - name: in1\n    satisfied_by: missing_step\nsteps:\n  - id: c1\n    type: command\n    run: echo hi\n"))
		err := ValidateFile(wf, "/tmp/lib.yaml", false)
		if err == nil {
			t.Fatalf("expected contract_violation")
		}
		ve, _ := err.(*ValidationError)
		if ve.Code != "contract_violation" || ve.Field != "inputs[0].satisfied_by" {
			t.Fatalf("code=%q field=%q want contract_violation@inputs[0].satisfied_by", ve.Code, ve.Field)
		}
	})
	t.Run("contract_violation outputs produced_by", func(t *testing.T) {
		wf, _ := Parse(strings.NewReader("version: 2\nname: lib\noutputs:\n  - name: out1\n    produced_by: missing_step\nsteps:\n  - id: c1\n    type: command\n    run: echo hi\n"))
		err := ValidateFile(wf, "/tmp/lib.yaml", false)
		if err == nil {
			t.Fatalf("expected contract_violation")
		}
		ve, _ := err.(*ValidationError)
		if ve.Code != "contract_violation" || ve.Field != "outputs[0].produced_by" {
			t.Fatalf("code=%q field=%q want contract_violation@outputs[0].produced_by", ve.Code, ve.Field)
		}
	})
}

func TestU04_SchemaParity(t *testing.T) {
	t.Run("additionalProperties", func(t *testing.T) {
		_, err := Parse(strings.NewReader("version: 2\nname: x\nunknown: foo\nsteps:\n  - id: a\n    type: command\n    run: echo hi\n"))
		if err == nil {
			t.Fatalf("expected unknown field")
		}
	})
	t.Run("invalid version", func(t *testing.T) {
		_, err := Parse(strings.NewReader("version: 1\nname: x\nsteps:\n  - id: a\n    type: command\n    run: echo hi\n"))
		if err == nil {
			t.Fatalf("expected invalid version")
		}
	})
	t.Run("invalid pattern id", func(t *testing.T) {
		wf, _ := Parse(strings.NewReader("version: 2\nname: x\nsteps:\n  - id: BadID\n    type: command\n    run: echo hi\n"))
		err := ValidateFile(wf, "/tmp/f.yaml", true)
		if err == nil {
			t.Fatalf("expected invalid pattern")
		}
	})
}
