package workflow

import (
	"path/filepath"
	"testing"
)

func TestHash_Repeatability(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, ".haro", "workflows")
	mainPath := filepath.Join(base, "main", "workflow.yaml")
	libPath := filepath.Join(base, "lib", "workflow.yaml")
	files := map[string][]byte{
		filepath.Clean(mainPath): []byte("version: 2\nname: main\nsteps:\n  - id: a\n    type: workflow\n    source: ../lib/workflow.yaml\n"),
		filepath.Clean(libPath):  []byte("version: 2\nname: lib\nsteps:\n  - id: x\n    type: command\n    run: echo hi\n  - id: y\n    type: command\n    run: echo hi2\n    depends_on: [x]\n"),
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
		t.Fatalf("hash not repeatable: %q vs %q", dag1.Hash, dag2.Hash)
	}
	if len(dag1.Steps) != len(dag2.Steps) {
		t.Fatalf("steps length mismatch")
	}
	for i := range dag1.Steps {
		if dag1.Steps[i].ID != dag2.Steps[i].ID {
			t.Fatalf("order not stable at %d: %q vs %q", i, dag1.Steps[i].ID, dag2.Steps[i].ID)
		}
	}
}
