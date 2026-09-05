package workflow

import (
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
	t.Run("root contract", func(t *testing.T) {
		wf, _ := Parse(strings.NewReader("version: 2\nname: root\ninputs:\n  - name: inp\n    satisfied_by: s1\nsteps:\n  - id: s1\n    type: command\n    run: echo hi\n"))
		err := ValidateFile(wf, "/tmp/root.yaml", true)
		if err == nil {
			t.Fatalf("expected root_contract_forbidden")
		}
		ve, _ := err.(*ValidationError)
		if ve.Code != "root_contract_forbidden" {
			t.Fatalf("code %q", ve.Code)
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
